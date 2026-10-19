package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
)

// ==================== 售后扩展：售后类型 ====================

// AftersalesTypeQueryReq 售后类型查询参数。
type AftersalesTypeQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
	Status   *bool // nil=全部 true=启用 false=禁用
}

// CrmAftersalesTypeService 售后类型服务。
type CrmAftersalesTypeService struct{}

func NewCrmAftersalesType() *CrmAftersalesTypeService { return &CrmAftersalesTypeService{} }

// Create 创建售后类型。
func (s CrmAftersalesTypeService) Create(ctx context.Context, t *models.CrmAftersalesType) error {
	tenantID := identity.TenantID(ctx)

	if t == nil {
		return errors.New("类型不能为空")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("类型名称不能为空")
	}
	t.ID = id_generator.StringID()
	t.TenantID = tenantID
	t.Name = strings.TrimSpace(t.Name)
	return createCRM(models.DB, t)
}

// Update 更新售后类型。
func (s CrmAftersalesTypeService) Update(ctx context.Context, id string, t *models.CrmAftersalesType) error {

	if t == nil {
		return errors.New("类型不能为空")
	}
	m := map[string]any{
		"name":   strings.TrimSpace(t.Name),
		"sort":   t.Sort,
		"status": t.Status,
	}
	return models.MustFor(ctx).Model(&models.CrmAftersalesType{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除售后类型。
func (s CrmAftersalesTypeService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmAftersalesType{})
	return res.RowsAffected, res.Error
}

// Query 分页查询售后类型。
func (s CrmAftersalesTypeService) Query(ctx context.Context, req AftersalesTypeQueryReq) ([]models.CrmAftersalesType, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		db = db.Where("name LIKE ?", "%"+kw+"%")
	}
	if req.Status != nil {
		db = db.Where("status = ?", *req.Status)
	}
	var total int64
	if err := db.Model(&models.CrmAftersalesType{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmAftersalesType
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("sort asc, id desc").Find(&items).Error
	return items, total, err
}

// ==================== 售后扩展：售后评论 ====================

// CrmAftersalesCommentService 售后评论服务。
type CrmAftersalesCommentService struct{}

func NewCrmAftersalesComment() *CrmAftersalesCommentService { return &CrmAftersalesCommentService{} }

// Create 创建售后评论。
func (s CrmAftersalesCommentService) Create(ctx context.Context, aftersalesID, userID, content string) error {
	tenantID := identity.TenantID(ctx)

	if aftersalesID == "" {
		return errors.New("售后单ID不能为空")
	}
	if userID == "" {
		return errors.New("评论人不能为空")
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("评论内容不能为空")
	}
	c := &models.CrmAftersalesComment{
		TenantID:     tenantID,
		AftersalesID: aftersalesID,
		UserID:       userID,
		Content:      strings.TrimSpace(content),
	}
	c.ID = id_generator.StringID()
	return createCRM(models.DB, c, "UserID")
}

// ListByAftersales 按售后单列出评论。
func (s CrmAftersalesCommentService) ListByAftersales(ctx context.Context, aftersalesID string) ([]models.CrmAftersalesComment, error) {

	var items []models.CrmAftersalesComment
	err := models.MustFor(ctx).Where("aftersales_id = ?", aftersalesID).
		Order("id desc").Find(&items).Error
	return items, err
}

// Delete 删除售后评论。
func (s CrmAftersalesCommentService) Delete(ctx context.Context, id string) error {

	res := models.MustFor(ctx).Where("id = ?", id).Delete(&models.CrmAftersalesComment{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("评论不存在")
	}
	return nil
}
