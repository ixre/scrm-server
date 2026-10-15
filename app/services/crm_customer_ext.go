package services

import (
	"core/common/identity"

	"context"
	"errors"
	"strings"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CustomerPoolQueryReq 客户池扩展查询参数。
type CustomerPoolQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
}

func (r CustomerPoolQueryReq) pager() *app.Pager {
	p := &app.Pager{Page: r.Page, PageSize: r.PageSize}
	p.SetDefault()
	return p
}

type CrmCustomerExtService struct{}

func NewCrmCustomerExt() *CrmCustomerExtService { return &CrmCustomerExtService{} }

// Lock 批量锁客：设 IsLocked=true, LockReason, LockAt=now。
func (s CrmCustomerExtService) Lock(ctx context.Context, ids []string, reason string) error {
	_, err := updateCustomers(ctx, ids, map[string]any{"is_locked": true, "lock_reason": reason, "lock_at": time.Now()})
	return err

}

// Unlock 批量解锁。
func (s CrmCustomerExtService) Unlock(ctx context.Context, ids []string) error {
	_, err := updateCustomers(ctx, ids, map[string]any{"is_locked": false, "lock_reason": "", "lock_at": nil})
	return err

}

// Blacklist 批量拉黑。
func (s CrmCustomerExtService) Blacklist(ctx context.Context, ids []string, reason string) error {
	_, err := updateCustomers(ctx, ids, map[string]any{"is_blacklist": true, "blacklist_reason": reason, "blacklist_at": time.Now()})
	return err

}

// RemoveBlacklist 移出黑名单。
func (s CrmCustomerExtService) RemoveBlacklist(ctx context.Context, ids []string) error {
	_, err := updateCustomers(ctx, ids, map[string]any{"is_blacklist": false, "blacklist_reason": "", "blacklist_at": nil})
	return err

}

// SetKey 设置/取消重点客户。
func (s CrmCustomerExtService) SetKey(ctx context.Context, ids []string, isKey bool) error {
	_, err := updateCustomers(ctx, ids, map[string]any{"is_key": isKey})
	return err

}

// Share 设置共享可见人。
func (s CrmCustomerExtService) Share(ctx context.Context, customerID string, staffIDs []string) error {
	unique := make(map[string]bool)
	for _, id := range staffIDs {
		unique[id] = true
	}
	var count int64
	if len(staffIDs) > 0 {
		if err := models.DB.Model(&models.User{}).Where("tenant_id = ? AND id IN ? AND status = ?", identity.TenantID(ctx), staffIDs, 1).Count(&count).Error; err != nil {
			return err
		}
		if count != int64(len(unique)) {
			return ErrCustomerAccess
		}
	}
	_, err := updateCustomers(ctx, []string{customerID}, map[string]any{"shared_staff_ids": constants.StringArrayField(staffIDs)})
	return err

}

// ListBlacklist 黑名单列表。
func (s CrmCustomerExtService) ListBlacklist(ctx context.Context, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {

	p := req.pager()
	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where("is_blacklist = ?", true)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("blacklist_at desc, id desc").Find(&items).Error
	return items, total, err
}

// ListKey 重点客户列表。
func (s CrmCustomerExtService) ListKey(ctx context.Context, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {

	p := req.pager()
	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where("is_key = ?", true)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ListShared 共享客户列表（当前用户在 shared_staff_ids 中）。
func (s CrmCustomerExtService) ListShared(ctx context.Context, userID string, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {

	p := req.pager()
	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where("shared_staff_ids @> ?", constants.StringArrayField{userID})
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ListUnreceive 待分配客户（OwnerUserID 为空）。
func (s CrmCustomerExtService) ListUnreceive(ctx context.Context, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {

	p := req.pager()
	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where("owner_user_id IS NULL")
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ListReassign 当前用户待接收的客户（ReassignStatus=1 且 OwnerUserID=userID）。
func (s CrmCustomerExtService) ListReassign(ctx context.Context, userID string, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {

	p := req.pager()
	db := models.MustFor(ctx).Where("reassign_status = ? AND owner_user_id = ?", 1, userID)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// AcceptReassign 确认接收当前用户的再分配任务。
func (s CrmCustomerExtService) AcceptReassign(ctx context.Context, userID, id string) error {
	if userID != identity.UserID(ctx) {
		return ErrCustomerAccess
	}
	_, err := respondToReassign(ctx, []string{id}, true)
	return err
}
func (s CrmCustomerExtService) RejectReassign(ctx context.Context, id string) error {
	_, err := respondToReassign(ctx, []string{id}, false)
	return err
}
func (s CrmCustomerExtService) AcceptReassignBatch(ctx context.Context, userID string, ids []string) (int64, error) {
	if userID != identity.UserID(ctx) {
		return 0, ErrCustomerAccess
	}
	return respondToReassign(ctx, ids, true)
}
func (s CrmCustomerExtService) RejectReassignBatch(ctx context.Context, ids []string) (int64, error) {
	return respondToReassign(ctx, ids, false)
}

func respondToReassign(ctx context.Context, ids []string, accept bool) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	var count int64
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, _, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		var customers []models.CrmCustomer
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND owner_user_id = ? AND reassign_status = ? AND id IN ?", user.TenantID, user.ID, 1, ids).Order("id").Find(&customers).Error; err != nil {
			return err
		}
		unique := make(map[string]bool)
		for _, id := range ids {
			unique[id] = true
		}
		if len(customers) != len(unique) {
			return ErrCustomerAccess
		}
		for _, customer := range customers {
			changes := map[string]any{"reassign_status": 0, "reassign_from": nil}
			if !accept {
				var previous models.User
				err := tx.Where("tenant_id = ? AND id = ? AND status = ?", user.TenantID, zeroID(customer.ReassignFrom), 1).First(&previous).Error
				if err == nil {
					changes["owner_user_id"] = previous.ID
				} else if errors.Is(err, gorm.ErrRecordNotFound) {
					changes = recycledCustomerChanges(customer, time.Now())
					if err := poolLog(tx, user.TenantID, customer, "recycle", user, "", "接收被拒绝，原负责人不可用"); err != nil {
						return err
					}
				} else {
					return err
				}
			}
			if err := tx.Model(&customer).Updates(changes).Error; err != nil {
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

// ReassignByTransfer 触发再分配：由批量转移服务调用，将客户标记为待接收。
// owner 变为 toUserID，reassign_status=1，reassign_from=原 owner。
func (s CrmCustomerExtService) ReassignByTransfer(ctx context.Context, fromUserID, toUserID string, customerIDs []string) (int64, error) {
	tenantID := identity.TenantID(ctx)

	if len(customerIDs) == 0 || toUserID == "" {
		return 0, nil
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ? AND owner_user_id <> ?", customerIDs, toUserID).
		Updates(map[string]any{"owner_user_id": toUserID, "reassign_status": 1, "reassign_from": fromUserID})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer reassign failed", "err", res.Error)
	}
	return res.RowsAffected, res.Error
}
