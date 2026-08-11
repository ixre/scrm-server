package services

import (
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 客户转移/分配 ----------

type CrmTransferService struct{}

func NewCrmTransfer() *CrmTransferService { return &CrmTransferService{} }

// Transfer 批量转移客户归属：改 owner + 写转移日志（事务）。
// 公海客户(pool_status=1)不可转移，需先认领。
func (s CrmTransferService) Transfer(tenantID, operatorUserID string, customerIDs []string, toUserID, reason string) (int64, error) {
	if len(customerIDs) == 0 {
		return 0, nil
	}
	if toUserID == "" {
		return 0, nil
	}
	var logs []models.CrmTransferLog
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		// 查当前客户归属，构造转移日志。
		var cs []models.CrmCustomer
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  customerIDs).Find(&cs).Error; err != nil {
			return err
		}
		for _, c := range cs {
			if c.PoolStatus == 1 {
				continue // 公海客户跳过
			}
			if c.OwnerUserID == toUserID {
				continue // 归属不变跳过
			}
			logs = append(logs, models.CrmTransferLog{
				Model:          models.Model{ID: id_generator.StringID()},
				TenantID:       tenantID,
				CustomerID:     c.ID,
				FromUserID:     c.OwnerUserID,
				ToUserID:       toUserID,
				Reason:         reason,
				OperatorUserID: operatorUserID,
			})
		}
		// 批量改 owner（仅非公海客户）。
		res := tx.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ? AND pool_status = ?",  customerIDs,  0).
			Update("owner_user_id", toUserID)
		if res.Error != nil {
			return res.Error
		}
		// 写转移日志。
		for i := range logs {
			if err := tx.Create(&logs[i]).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Sugar.Errorw("crm transfer failed", "err", err)
		return 0, err
	}
	return int64(len(logs)), nil
}

// ListTransferLog 查某客户的转移历史（按时间倒序）。
func (s CrmTransferService) ListTransferLog(tenantID, customerID string, pager *app.Pager) ([]models.CrmTransferLog, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("customer_id = ?",  customerID)
	var total int64
	if err := db.Model(&models.CrmTransferLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var logs []models.CrmTransferLog
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("created_at desc").Find(&logs).Error
	return logs, total, err
}
