package services

import (
	"errors"
	"strings"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/log"

	"gorm.io/gorm"
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
func (s CrmCustomerExtService) Lock(tenantID string, ids []string, reason string) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Updates(map[string]any{"is_locked": true, "lock_reason": reason, "lock_at": now})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer lock failed", "err", res.Error)
	}
	return res.Error
}

// Unlock 批量解锁。
func (s CrmCustomerExtService) Unlock(tenantID string, ids []string) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Updates(map[string]any{"is_locked": false, "lock_reason": "", "lock_at": nil})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer unlock failed", "err", res.Error)
	}
	return res.Error
}

// Blacklist 批量拉黑。
func (s CrmCustomerExtService) Blacklist(tenantID string, ids []string, reason string) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Updates(map[string]any{"is_blacklist": true, "blacklist_reason": reason, "blacklist_at": now})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer blacklist failed", "err", res.Error)
	}
	return res.Error
}

// RemoveBlacklist 移出黑名单。
func (s CrmCustomerExtService) RemoveBlacklist(tenantID string, ids []string) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Updates(map[string]any{"is_blacklist": false, "blacklist_reason": "", "blacklist_at": nil})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer remove blacklist failed", "err", res.Error)
	}
	return res.Error
}

// SetKey 设置/取消重点客户。
func (s CrmCustomerExtService) SetKey(tenantID string, ids []string, isKey bool) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Update("is_key", isKey)
	if res.Error != nil {
		log.Sugar.Errorw("crm customer set key failed", "err", res.Error)
	}
	return res.Error
}

// Share 设置共享可见人。
func (s CrmCustomerExtService) Share(tenantID, customerID string, staffIDs []string) error {
	if customerID == "" {
		return errors.New("customer_id 不能为空")
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  customerID).
		Update("shared_staff_ids", constants.StringArrayField(staffIDs))
	if res.Error != nil {
		log.Sugar.Errorw("crm customer share failed", "err", res.Error)
	}
	return res.Error
}

// ListBlacklist 黑名单列表。
func (s CrmCustomerExtService) ListBlacklist(tenantID string, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {
	p := req.pager()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("is_blacklist = ?",  true)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("blacklist_at desc, id desc").Find(&items).Error
	return items, total, err
}

// ListKey 重点客户列表。
func (s CrmCustomerExtService) ListKey(tenantID string, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {
	p := req.pager()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("is_key = ?",  true)
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

// ListShared 共享客户列表（当前用户在 shared_staff_ids 中）。
func (s CrmCustomerExtService) ListShared(tenantID, userID string, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {
	p := req.pager()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("shared_staff_ids @> ?",  constants.StringArrayField{userID})
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

// ListUnreceive 待分配客户（OwnerUserID 为空）。
func (s CrmCustomerExtService) ListUnreceive(tenantID string, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {
	p := req.pager()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("(owner_user_id IS NULL OR owner_user_id = '')", )
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

// ListReassign 再分配客户（ReassignStatus=1 且 ReassignFrom=userID）。
func (s CrmCustomerExtService) ListReassign(tenantID, userID string, req CustomerPoolQueryReq) ([]models.CrmCustomer, int64, error) {
	p := req.pager()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("reassign_status = ? AND reassign_from = ?",  1,  userID)
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

// AcceptReassign 确认接收再分配：归属改为当前用户，清空再分配状态。
func (s CrmCustomerExtService) AcceptReassign(tenantID, userID, id string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND reassign_status = ?",  id,  1).
		Updates(map[string]any{"owner_user_id": userID, "reassign_status": 0, "reassign_from": ""})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer accept reassign failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// RejectReassign 拒绝再分配：清空再分配状态，归属恢复为原转出人。
func (s CrmCustomerExtService) RejectReassign(tenantID, id string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	var c models.CrmCustomer
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).First(&c).Error; err != nil {
		return err
	}
	if c.ReassignStatus != 1 {
		return errors.New("该客户非待确认再分配状态")
	}
	// 拒绝：归属恢复为转出人，清空再分配状态。
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Updates(map[string]any{"owner_user_id": c.ReassignFrom, "reassign_status": 0, "reassign_from": ""})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer reject reassign failed", "err", res.Error)
	}
	return res.Error
}

// ReassignByTransfer 触发再分配：由批量转移服务调用，将客户标记为待接收。
// owner 变为 toUserID，reassign_status=1，reassign_from=原 owner。
func (s CrmCustomerExtService) ReassignByTransfer(tenantID, fromUserID, toUserID string, customerIDs []string) (int64, error) {
	if len(customerIDs) == 0 || toUserID == "" {
		return 0, nil
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ? AND owner_user_id <> ?",  customerIDs,  toUserID).
		Updates(map[string]any{"owner_user_id": toUserID, "reassign_status": 1, "reassign_from": fromUserID})
	if res.Error != nil {
		log.Sugar.Errorw("crm customer reassign failed", "err", res.Error)
	}
	return res.RowsAffected, res.Error
}
