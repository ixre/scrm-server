package services

import (
	"context"
	"errors"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------- 客户转移/分配 ----------

type CrmTransferService struct{}

func NewCrmTransfer() *CrmTransferService { return &CrmTransferService{} }

// Transfer 批量转移客户归属：改 owner + 写转移日志（事务）。
// 公海客户(pool_status=1)不可转移，需先认领。
func (s CrmTransferService) Transfer(ctx context.Context, operatorUserID string, customerIDs []string, toUserID, reason string) (int64, error) {
	ids, err := workflowIDs(customerIDs)
	if err != nil {
		return 0, err
	}
	var count int64
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		if !manager || actor.ID != operatorUserID {
			return ErrCustomerAccess
		}
		if _, err = lockTenantUser(ctx, tx, toUserID); err != nil {
			return err
		}
		var customers []models.CrmCustomer
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN ?", actor.TenantID, ids).Order("id").Find(&customers).Error; err != nil {
			return err
		}
		if len(customers) != len(ids) {
			return ErrCustomerAccess
		}
		for _, c := range customers {
			if c.PoolStatus == 1 {
				return errors.New("公海客户请使用分配操作")
			}
			if c.ReassignStatus != 0 {
				return errors.New("客户尚在等待接收，请先完成上一次转交")
			}
			if c.OwnerUserID == toUserID {
				continue
			}
			entry := models.CrmTransferLog{Model: models.Model{ID: id_generator.StringID()}, TenantID: actor.TenantID, CustomerID: c.ID, FromUserID: c.OwnerUserID, ToUserID: toUserID, Reason: reason, OperatorUserID: actor.ID}
			if err = createCRM(tx, &entry); err != nil {
				return err
			}
			updates := map[string]any{"owner_user_id": toUserID, "claimed_at": time.Now(), "reassign_status": 1, "reassign_from": nil}
			if c.OwnerUserID != "" {
				updates["reassign_from"] = c.OwnerUserID
			}
			if err = tx.Model(&c).Updates(updates).Error; err != nil {
				return err
			}
			count++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ListTransferLog 查某客户的转移历史（按时间倒序）。
func (s CrmTransferService) ListTransferLog(ctx context.Context, customerID string, pager *app.Pager) ([]models.CrmTransferLog, int64, error) {
	if _, err := NewCrmCustomer().Get(ctx, customerID); err != nil {
		return nil, 0, err
	}
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx).Where("customer_id = ?", customerID)
	var total int64
	if err := db.Model(&models.CrmTransferLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []models.CrmTransferLog
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("created_at desc, id desc").Find(&logs).Error
	return logs, total, err
}
