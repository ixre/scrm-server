package services

import (
	"core/common/identity"

	"context"

	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
)

// ==================== 知识库 ====================

// KnowledgeQueryReq 知识库查询参数。
type KnowledgeQueryReq struct {
	Page            int
	PageSize        int
	KnowledgeTypeID string
	Status          string
	Keyword         string
}

// CrmKnowledgeService 知识库服务。
type CrmKnowledgeService struct{}

func NewCrmKnowledge() *CrmKnowledgeService { return &CrmKnowledgeService{} }

// Create 创建知识库文章。
func (s CrmKnowledgeService) Create(ctx context.Context, k *models.CrmKnowledge) error {
	tenantID := identity.TenantID(ctx)

	if k == nil {
		return errors.New("知识库文章不能为空")
	}
	if strings.TrimSpace(k.Title) == "" {
		return errors.New("标题不能为空")
	}
	k.ID = id_generator.StringID()
	k.TenantID = tenantID
	k.Title = strings.TrimSpace(k.Title)
	if k.Status == "" {
		k.Status = "published"
	}
	return createCRM(models.DB, k, "CategoryID", "AuthorID")
}

// Update 更新知识库文章。
func (s CrmKnowledgeService) Update(ctx context.Context, id string, k *models.CrmKnowledge) error {

	if k == nil {
		return errors.New("知识库文章不能为空")
	}
	m := map[string]any{
		"title":             k.Title,
		"knowledge_type_id": k.KnowledgeTypeID,
		"content":           k.Content,
		"visibility":        k.Visibility,
		"status":            k.Status,
	}
	updatesCRM(m, "knowledge_type_id", "author_id")
	return models.MustFor(ctx).Model(&models.CrmKnowledge{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除知识库文章。
func (s CrmKnowledgeService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmKnowledge{})
	return res.RowsAffected, res.Error
}

// Query 分页查询知识库文章。
func (s CrmKnowledgeService) Query(ctx context.Context, req KnowledgeQueryReq) ([]models.CrmKnowledge, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.KnowledgeTypeID != "" {
		db = db.Where("knowledge_type_id = ?", req.KnowledgeTypeID)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("title LIKE ? OR content LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmKnowledge{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmKnowledge
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ==================== 公告 ====================

// NoticeQueryReq 公告查询参数。
type NoticeQueryReq struct {
	Page     int
	PageSize int
	Status   string
	Type     string
	Keyword  string
}

// CrmNoticeService 公告服务。
type CrmNoticeService struct{}

func NewCrmNotice() *CrmNoticeService { return &CrmNoticeService{} }

// Create 创建公告。
func (s CrmNoticeService) Create(ctx context.Context, n *models.CrmNotice) error {
	tenantID := identity.TenantID(ctx)

	if n == nil {
		return errors.New("公告不能为空")
	}
	if strings.TrimSpace(n.Title) == "" {
		return errors.New("标题不能为空")
	}
	n.ID = id_generator.StringID()
	n.TenantID = tenantID
	n.Title = strings.TrimSpace(n.Title)
	if n.Status == "" {
		n.Status = "draft"
	}
	if n.Status == "published" && n.PublishAt == nil {
		now := time.Now()
		n.PublishAt = &now
	}
	return models.DB.Create(n).Error
}

// Update 更新公告。
func (s CrmNoticeService) Update(ctx context.Context, id string, n *models.CrmNotice) error {

	if n == nil {
		return errors.New("公告不能为空")
	}
	m := map[string]any{
		"title":     n.Title,
		"content":   n.Content,
		"type":      n.Type,
		"status":    n.Status,
		"expire_at": n.ExpireAt,
	}
	if n.Status == "published" {
		now := time.Now()
		m["publish_at"] = &now
	}
	return models.MustFor(ctx).Model(&models.CrmNotice{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除公告。
func (s CrmNoticeService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmNotice{})
	return res.RowsAffected, res.Error
}

// Query 分页查询公告。
func (s CrmNoticeService) Query(ctx context.Context, req NoticeQueryReq) ([]models.CrmNotice, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		db = db.Where("title LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := db.Model(&models.CrmNotice{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmNotice
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ==================== 工作报告 ====================

// WorkReportQueryReq 工作报告查询参数。
type WorkReportQueryReq struct {
	Page     int
	PageSize int
	UserID   string
	Type     string
	Status   string
}

// CrmWorkReportService 工作报告服务。
type CrmWorkReportService struct{}

func NewCrmWorkReport() *CrmWorkReportService { return &CrmWorkReportService{} }

// Create 创建工作报告。
func (s CrmWorkReportService) Create(ctx context.Context, r *models.CrmWorkReport) error {
	tenantID := identity.TenantID(ctx)

	if r == nil {
		return errors.New("工作报告不能为空")
	}
	if strings.TrimSpace(r.Type) == "" {
		return errors.New("报告类型不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	if r.Status == "" {
		r.Status = "submitted"
	}
	return createCRM(models.DB, r, "UserID", "ReviewerID")
}

// Update 更新工作报告。
func (s CrmWorkReportService) Update(ctx context.Context, id string, r *models.CrmWorkReport) error {

	if r == nil {
		return errors.New("工作报告不能为空")
	}
	m := map[string]any{
		"type":        r.Type,
		"report_date": r.ReportDate,
		"content":     r.Content,
		"plan":        r.Plan,
	}
	updatesCRM(m, "user_id", "reviewer_id")
	return models.MustFor(ctx).Model(&models.CrmWorkReport{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除工作报告。
func (s CrmWorkReportService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmWorkReport{})
	return res.RowsAffected, res.Error
}

// Query 分页查询工作报告。
func (s CrmWorkReportService) Query(ctx context.Context, req WorkReportQueryReq) ([]models.CrmWorkReport, int64, error) {

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
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmWorkReport{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmWorkReport
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// Review 审阅工作报告。
func (s CrmWorkReportService) Review(ctx context.Context, id, reviewerID string) error {
	tenantID := identity.TenantID(ctx)

	if reviewerID == "" {
		return errors.New("审阅人不能为空")
	}
	res := models.DB.Model(&models.CrmWorkReport{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status = ?", id, "submitted").
		Updates(map[string]any{"status": "reviewed", "reviewer_id": reviewerID})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("工作报告不存在或不可审阅")
	}
	return nil
}
