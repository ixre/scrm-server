package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"fmt"
	"strings"

	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
)

// AiQueryReq AI销售分析查询参数。
type AiQueryReq struct {
	Page       int
	PageSize   int
	CustomerID string
	Type       string
	Status     string
}

// CrmAiAnalysisService AI销售分析服务。
type CrmAiAnalysisService struct{}

func NewCrmAiAnalysis() *CrmAiAnalysisService { return &CrmAiAnalysisService{} }

// Create 创建分析结果。
func (s CrmAiAnalysisService) Create(ctx context.Context, a *models.CrmAiAnalysis) error {
	tenantID := identity.TenantID(ctx)


	if a == nil {
		return errors.New("分析记录不能为空")
	}
	if a.CustomerID == "" {
		return errors.New("customer_id 不能为空")
	}
	if strings.TrimSpace(a.Type) == "" {
		return errors.New("分析类型不能为空")
	}
	a.ID = id_generator.StringID()
	a.TenantID = tenantID
	if a.Status == "" {
		a.Status = "completed"
	}
	return createCRM(models.DB, a, "CustomerID")
}

// Delete 删除分析记录。
func (s CrmAiAnalysisService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmAiAnalysis{})
	return res.RowsAffected, res.Error
}

// Query 分页查询分析记录。
func (s CrmAiAnalysisService) Query(ctx context.Context, req AiQueryReq) ([]models.CrmAiAnalysis, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmAiAnalysis{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmAiAnalysis
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("id desc").Find(&items).Error
	return items, total, err
}

// GetByCustomer 列出客户的所有分析记录。
func (s CrmAiAnalysisService) GetByCustomer(ctx context.Context, customerID string) ([]models.CrmAiAnalysis, error) {


	var items []models.CrmAiAnalysis
	err := models.MustFor(ctx).Where("customer_id = ?",  customerID).
		Order("id desc").Find(&items).Error
	return items, err
}

// AnalyzeCustomer 触发客户分析（简化版：基于跟进记录+消费记录生成摘要，创建分析结果）。
func (s CrmAiAnalysisService) AnalyzeCustomer(ctx context.Context, customerID string) error {


	if customerID == "" {
		return errors.New("customer_id 不能为空")
	}
	// 校验客户存在。
	var cust models.CrmCustomer
	if err := models.MustFor(ctx).Where("id = ?",  customerID).First(&cust).Error; err != nil {
		return fmt.Errorf("客户不存在: %w", err)
	}

	// 汇总跟进记录。
	var followups []models.CrmFollowup
	models.MustFor(ctx).Where("customer_id = ?",  customerID).
		Order("id desc").Limit(20).Find(&followups)
	followupCount := len(followups)

	// 汇总消费记录。
	var consumes []models.CrmConsume
	models.MustFor(ctx).Where("customer_id = ? AND status = ?",  customerID,  "normal").
		Order("id desc").Limit(20).Find(&consumes)
	var totalAmount float64
	for _, c := range consumes {
		totalAmount += c.Amount
	}

	// 生成客户画像摘要。
	summary := fmt.Sprintf("客户「%s」共跟进 %d 次，累计消费 ¥%.2f（%d 笔）。",
		cust.Name, followupCount, totalAmount, len(consumes))
	details := constants.StringArrayField{
		fmt.Sprintf("跟进次数: %d", followupCount),
		fmt.Sprintf("消费总额: ¥%.2f", totalAmount),
		fmt.Sprintf("消费笔数: %d", len(consumes)),
	}
	if cust.Company != "" {
		details = append(details, "公司: "+cust.Company)
	}
	if cust.Industry != "" {
		details = append(details, "行业: "+cust.Industry)
	}

	// 评分：消费额 + 跟进频次的简单加权。
	score := 0.0
	if totalAmount > 0 {
		score += totalAmount / 1000.0
	}
	score += float64(followupCount) * 2
	if score > 100 {
		score = 100
	}

	a := &models.CrmAiAnalysis{
		CustomerID: customerID,
		Type:       "customer_profile",
		Summary:    summary,
		Details:    details,
		Score:      score,
		Status:     "completed",
	}
	return s.Create(ctx, a)
}

// Stats 按类型统计分析数量。
func (s CrmAiAnalysisService) Stats(ctx context.Context) (map[string]int64, error) {
	tenantID := identity.TenantID(ctx)


	type row struct {
		Type  string
		Count int64
	}
	var rows []row
	err := models.DB.Model(&models.CrmAiAnalysis{}).
		Select("type, COUNT(*) as count").Scopes(models.TenantScope(tenantID)).
		Group("type").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Type] = r.Count
	}
	return out, nil
}
