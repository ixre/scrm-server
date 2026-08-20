package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// CallListQueryReq 待拨电话列表查询参数。
type CallListQueryReq struct {
	Page     int
	PageSize int
	UserID   string
	Status   string
}

// CrmCallListService 待拨电话列表服务。
type CrmCallListService struct{}

func NewCrmCallList() *CrmCallListService { return &CrmCallListService{} }

// Create 创建待拨电话记录。
func (s CrmCallListService) Create(ctx context.Context, c *models.CrmCallList) error {
	tenantID := identity.TenantID(ctx)


	if c == nil {
		return errors.New("记录不能为空")
	}
	if strings.TrimSpace(c.Phone) == "" {
		return errors.New("电话号码不能为空")
	}
	c.ID = id_generator.StringID()
	c.TenantID = tenantID
	c.Phone = strings.TrimSpace(c.Phone)
	if c.Status == "" {
		c.Status = "pending"
	}
	return createCRM(models.DB, c, "UserID", "CustomerID")
}

// Update 更新待拨电话记录。
func (s CrmCallListService) Update(ctx context.Context, id string, c *models.CrmCallList) error {


	if c == nil {
		return errors.New("记录不能为空")
	}
	m := map[string]any{
		"user_id":      c.UserID,
		"customer_id":  c.CustomerID,
		"contact_name": c.ContactName,
		"phone":        c.Phone,
		"status":       c.Status,
		"remark":       c.Remark,
	}
	updatesCRM(m, "user_id", "customer_id")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除待拨电话记录。
func (s CrmCallListService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmCallList{})
	return res.RowsAffected, res.Error
}

// Query 分页查询待拨电话列表（按 user_id/status）。
func (s CrmCallListService) Query(ctx context.Context, req CallListQueryReq) ([]models.CrmCallList, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.UserID != "" {
		db = db.Where("user_id = ?", req.UserID)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmCallList{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCallList
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// BatchCreate 批量创建待拨电话记录。
func (s CrmCallListService) BatchCreate(ctx context.Context, items []models.CrmCallList) error {
	tenantID := identity.TenantID(ctx)


	if len(items) == 0 {
		return errors.New("记录不能为空")
	}
	for i := range items {
		if strings.TrimSpace(items[i].Phone) == "" {
			return errors.New("电话号码不能为空")
		}
		items[i].ID = id_generator.StringID()
		items[i].TenantID = tenantID
		items[i].Phone = strings.TrimSpace(items[i].Phone)
		if items[i].Status == "" {
			items[i].Status = "pending"
		}
	}
	if err := models.DB.Create(&items).Error; err != nil {
		log.Sugar.Errorw("crm call list batch create failed", "err", err)
		return err
	}
	return nil
}

// BatchAssign 批量分配待拨电话给指定员工。
func (s CrmCallListService) BatchAssign(ctx context.Context, userID string, ids []string) error {
	tenantID := identity.TenantID(ctx)


	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	if userID == "" {
		return errors.New("user_id 不能为空")
	}
	return models.DB.Model(&models.CrmCallList{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Update("user_id", userID).Error
}

// MarkDialed 标记为已拨打。
func (s CrmCallListService) MarkDialed(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)


	now := time.Now()
	return models.DB.Model(&models.CrmCallList{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Updates(map[string]any{"status": "dialed", "dialed_at": &now}).Error
}

// MarkSkipped 标记为已跳过。
func (s CrmCallListService) MarkSkipped(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)


	return models.DB.Model(&models.CrmCallList{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Update("status", "skipped").Error
}
