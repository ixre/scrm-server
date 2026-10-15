package services

import (
	"core/common/identity"

	"context"
	"errors"
	"gorm.io/gorm"
	"math"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// ---------- 快捷跟进话术 ----------

type CrmQuickFollowupService struct{}

func NewCrmQuickFollowupService() *CrmQuickFollowupService { return &CrmQuickFollowupService{} }

// Create 创建快捷跟进。
func (s CrmQuickFollowupService) Create(ctx context.Context, userID string, q *models.CrmQuickFollowup) error {
	tenantID := identity.TenantID(ctx)

	q.ID = id_generator.StringID()
	q.TenantID = tenantID
	q.UserID = userID
	if err := createCRM(models.DB, q); err != nil {
		log.Sugar.Errorw("quick followup create failed", "err", err)
		return err
	}
	return nil
}

// Update 更新快捷跟进。
func (s CrmQuickFollowupService) Update(ctx context.Context, userID, id string, q *models.CrmQuickFollowup) error {
	tenantID := identity.TenantID(ctx)

	updates := map[string]any{
		"content": q.Content, "type": q.Type, "sort": q.Sort,
	}
	res := models.DB.Model(&models.CrmQuickFollowup{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND user_id = ?", id, userID).Updates(updates)
	return res.Error
}

// Delete 删除快捷跟进。
func (s CrmQuickFollowupService) Delete(ctx context.Context, userID string, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("user_id = ? AND id IN ?", userID, ids).
		Delete(&models.CrmQuickFollowup{})
	return res.RowsAffected, res.Error
}

// List 列出当前用户的快捷跟进（按 sort 排序）。
func (s CrmQuickFollowupService) List(ctx context.Context, userID string) ([]models.CrmQuickFollowup, error) {

	var items []models.CrmQuickFollowup
	err := models.MustFor(ctx).Where("user_id = ?", userID).
		Order("sort desc, id desc").Find(&items).Error
	return items, err
}

// ---------- 拜访签到 ----------

type CrmSigninService struct{}

func NewCrmSigninService() *CrmSigninService { return &CrmSigninService{} }

// SigninQueryReq 签到查询请求。
type SigninQueryReq struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	CustomerID string `json:"customer_id"`
}

// Create 创建签到（自动设置 UserID）。
func (s CrmSigninService) Create(ctx context.Context, userID string, signin *models.CrmSignin) error {
	if userID == "" || userID != identity.UserID(ctx) {
		return ErrCustomerAccess
	}
	if math.IsNaN(signin.Latitude) || math.IsNaN(signin.Longitude) || math.IsInf(signin.Latitude, 0) || math.IsInf(signin.Longitude, 0) || signin.Latitude < -90 || signin.Latitude > 90 || signin.Longitude < -180 || signin.Longitude > 180 {
		return errors.New("签到坐标无效")
	}
	signin.ID, signin.TenantID, signin.UserID = id_generator.StringID(), identity.TenantID(ctx), userID
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := activeSalesCustomer(ctx, tx, signin.CustomerID); err != nil {
			return err
		}
		return createCRM(tx, signin)
	})
}

// Delete 删除签到。
func (s CrmSigninService) Delete(ctx context.Context, userID string, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("user_id = ? AND id IN ?", userID, ids).
		Delete(&models.CrmSignin{})
	return res.RowsAffected, res.Error
}

// Query 分页查询签到（可按 customer_id 筛选）。
func (s CrmSigninService) Query(ctx context.Context, userID string, req SigninQueryReq) ([]models.CrmSignin, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx).Where("user_id = ?", userID)
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	var total int64
	if err := db.Model(&models.CrmSignin{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSignin
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("created_at desc").Find(&items).Error
	return items, total, err
}

// ListByCustomer 按客户列出签到记录。
func (s CrmSigninService) ListByCustomer(ctx context.Context, customerID string) ([]models.CrmSignin, error) {
	if _, err := NewCrmCustomer().Get(ctx, customerID); err != nil {
		return nil, err
	}

	var items []models.CrmSignin
	err := models.MustFor(ctx).Where("customer_id = ?", customerID).
		Order("created_at desc").Find(&items).Error
	return items, err
}
