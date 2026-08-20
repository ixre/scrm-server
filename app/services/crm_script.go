package services

import (
	"core/common/identity"
	
	"context"
	
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"
)

// ---------- 销售话术推荐 ----------

type CrmSalesScriptService struct{}

func NewCrmSalesScript() *CrmSalesScriptService { return &CrmSalesScriptService{} }

// SalesScriptCreateReq 话术创建请求。
type SalesScriptCreateReq struct {
	Script models.CrmSalesScript `json:"script"`
}

// Create 创建话术。
func (s CrmSalesScriptService) Create(ctx context.Context, req models.CrmSalesScript) (models.CrmSalesScript, error) {
	tenantID := identity.TenantID(ctx)


	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		log.Sugar.Errorw("sales script create failed", "err", err)
		return req, err
	}
	return req, nil
}

// Update 更新话术。
func (s CrmSalesScriptService) Update(ctx context.Context, id string, req models.CrmSalesScript) (models.CrmSalesScript, error) {
	tenantID := identity.TenantID(ctx)


	updates := map[string]any{
		"title": req.Title, "stage": req.Stage, "industry": req.Industry,
		"scene": req.Scene, "content": req.Content, "sort": req.Sort, "status": req.Status,
	}
	if err := models.DB.Model(&models.CrmSalesScript{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(updates).Error; err != nil {
		return req, err
	}
	return req, nil
}

// Delete 删除话术。
func (s CrmSalesScriptService) Delete(ctx context.Context, ids []string) (int64, error) {


	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmSalesScript{})
	return res.RowsAffected, res.Error
}

// Query 列出话术（按阶段/行业/场景过滤）。
func (s CrmSalesScriptService) Query(ctx context.Context, stage, industry, scene string, pager *app.Pager) ([]models.CrmSalesScript, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx).Where("status = ?",  "active")
	if stage != "" {
		db = db.Where("stage = ?", stage)
	}
	if industry != "" {
		db = db.Where("(industry = ? OR industry = '')", industry)
	}
	if scene != "" {
		db = db.Where("scene = ?", scene)
	}
	var total int64
	if err := db.Model(&models.CrmSalesScript{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSalesScript
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("sort desc, id desc").Find(&items).Error
	return items, total, err
}

// Recommend 按客户推荐话术：根据客户行业 + 当前商机阶段匹配。
func (s CrmSalesScriptService) Recommend(ctx context.Context, customerID string) ([]models.CrmSalesScript, error) {


	var cust models.CrmCustomer
	if err := models.MustFor(ctx).Where("id = ?", customerID).First(&cust).Error; err != nil {
		return nil, err
	}
	// 查客户当前商机阶段
	var opp models.CrmOpportunity
	models.MustFor(ctx).Where("customer_id = ? AND status = ?",  customerID,  "open").
		Order("created_at desc").First(&opp)
	stage := "prospect"
	if opp.StageID != "" {
		var stageModel models.CrmPipelineStage
		models.DB.Where("id = ?", opp.StageID).First(&stageModel)
		if stageModel.Name != "" {
			stage = stageModel.Name
		}
	}
	// 按行业+阶段匹配，无匹配则按阶段匹配
	items, _, _ := s.Query(ctx, stage, cust.Industry, "", &app.Pager{Page: 1, PageSize: 20})
	if len(items) == 0 {
		items, _, _ = s.Query(ctx, stage, "", "", &app.Pager{Page: 1, PageSize: 20})
	}
	return items, nil
}
