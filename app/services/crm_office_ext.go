package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
)

// ==================== 办公扩展：知识库分类 ====================

// KnowledgeTypeQueryReq 知识库分类查询参数。
type KnowledgeTypeQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
}

// CrmKnowledgeTypeService 知识库分类服务。
type CrmKnowledgeTypeService struct{}

func NewCrmKnowledgeType() *CrmKnowledgeTypeService { return &CrmKnowledgeTypeService{} }

// Create 创建知识库分类。
func (s CrmKnowledgeTypeService) Create(ctx context.Context, t *models.CrmKnowledgeType) error {
	tenantID := identity.TenantID(ctx)


	if t == nil {
		return errors.New("分类不能为空")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("分类名称不能为空")
	}
	t.ID = id_generator.StringID()
	t.TenantID = tenantID
	t.Name = strings.TrimSpace(t.Name)
	return createCRM(models.DB, t)
}

// Update 更新知识库分类。
func (s CrmKnowledgeTypeService) Update(ctx context.Context, id string, t *models.CrmKnowledgeType) error {


	if t == nil {
		return errors.New("分类不能为空")
	}
	m := map[string]any{
		"name": strings.TrimSpace(t.Name),
		"sort": t.Sort,
	}
	return models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error
}

// Delete 删除知识库分类。
func (s CrmKnowledgeTypeService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmKnowledgeType{})
	return res.RowsAffected, res.Error
}

// Query 分页查询知识库分类。
func (s CrmKnowledgeTypeService) Query(ctx context.Context, req KnowledgeTypeQueryReq) ([]models.CrmKnowledgeType, int64, error) {


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
	var total int64
	if err := db.Model(&models.CrmKnowledgeType{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmKnowledgeType
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("sort asc, id desc").Find(&items).Error
	return items, total, err
}

// ==================== 办公扩展：工作报告评论 ====================

// CrmWorkReportCommentService 工作报告评论服务。
type CrmWorkReportCommentService struct{}

func NewCrmWorkReportComment() *CrmWorkReportCommentService { return &CrmWorkReportCommentService{} }

// Create 创建工作报告评论。
func (s CrmWorkReportCommentService) Create(ctx context.Context, reportID, userID, content string) error {
	tenantID := identity.TenantID(ctx)


	if reportID == "" {
		return errors.New("报告ID不能为空")
	}
	if userID == "" {
		return errors.New("评论人不能为空")
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("评论内容不能为空")
	}
	c := &models.CrmWorkReportComment{
		TenantID: tenantID,
		ReportID: reportID,
		UserID:   userID,
		Content:  strings.TrimSpace(content),
	}
	c.ID = id_generator.StringID()
	return createCRM(models.DB, c, "UserID")
}

// ListByReport 按报告列出评论。
func (s CrmWorkReportCommentService) ListByReport(ctx context.Context, reportID string) ([]models.CrmWorkReportComment, error) {


	var items []models.CrmWorkReportComment
	err := models.MustFor(ctx).Where("report_id = ?",  reportID).
		Order("id desc").Find(&items).Error
	return items, err
}

// Delete 删除工作报告评论。
func (s CrmWorkReportCommentService) Delete(ctx context.Context, id string) error {


	res := models.MustFor(ctx).Where("id = ?",  id).Delete(&models.CrmWorkReportComment{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("评论不存在")
	}
	return nil
}

// ==================== 办公扩展：工作报告统计 ====================

// ReportStatsReq 工作报告统计请求。
type ReportStatsReq struct {
	StartDate string
	EndDate   string
}

// CrmWorkReportStatsService 工作报告统计服务。
type CrmWorkReportStatsService struct{}

func NewCrmWorkReportStats() *CrmWorkReportStatsService { return &CrmWorkReportStatsService{} }

// Stats 工作报告统计：按用户/类型/日期范围统计报告数量。
func (s CrmWorkReportStatsService) Stats(ctx context.Context, req ReportStatsReq) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	db := models.DB.Model(&models.CrmWorkReport{}).Scopes(models.TenantScope(tenantID))
	if req.StartDate != "" {
		db = db.Where("report_date >= ?", req.StartDate)
	}
	if req.EndDate != "" {
		db = db.Where("report_date <= ?", req.EndDate+" 23:59:59")
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}

	// 按用户统计
	type userCount struct {
		UserID string `json:"user_id"`
		Name   string `json:"name"`
		Count  int64  `json:"count"`
	}
	var byUser []userCount
	if err := db.Select("user_id, count(*) as count").Group("user_id").Scan(&byUser).Error; err != nil {
		return nil, err
	}
	// 补充用户名
	for i, u := range byUser {
		var usr models.User
		if err := models.MustFor(ctx).Where("id = ?", u.UserID).First(&usr).Error; err == nil {
			byUser[i].Name = usr.Name
		}
	}

	// 按类型统计
	type typeCount struct {
		Type  string `json:"type"`
		Count int64  `json:"count"`
	}
	var byTypeRows []typeCount
	if err := db.Select("type, count(*) as count").Group("type").Scan(&byTypeRows).Error; err != nil {
		return nil, err
	}
	byType := map[string]int64{"daily": 0, "weekly": 0, "monthly": 0}
	for _, r := range byTypeRows {
		byType[r.Type] = r.Count
	}

	return map[string]any{
		"by_user": byUser,
		"by_type": byType,
		"total":   total,
	}, nil
}
