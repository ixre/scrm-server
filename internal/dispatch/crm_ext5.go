package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 办公扩展 dispatch handlers ----------

// ---- 知识库分类 ----

func crmKnowledgeTypeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.KnowledgeTypeQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmKnowledgeType().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmKnowledgeTypeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmKnowledgeType
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmKnowledgeType().Create(ctx, &c)
}

func crmKnowledgeTypeUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmKnowledgeType
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmKnowledgeType().Update(ctx, id, &c)
}

func crmKnowledgeTypeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmKnowledgeType().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---- 工作报告评论 ----

func crmWorkReportCommentList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ReportID string `json:"report_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmWorkReportComment().ListByReport(ctx, req.ReportID)
}

func crmWorkReportCommentCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ReportID string `json:"report_id"`
		Content  string `json:"content"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkReportComment().Create(ctx, req.ReportID, u, req.Content)
}

func crmWorkReportCommentDelete(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkReportComment().Delete(ctx, id)
}

// ---- 工作报告统计 ----

func crmWorkReportStats(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.ReportStatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmWorkReportStats().Stats(ctx, req)
}

// ---------- 售后扩展 dispatch handlers ----------

// ---- 售后类型 ----

func crmAftersalesTypeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Keyword string `json:"keyword"`
		Status  *bool  `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.AftersalesTypeQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword, Status: req.Status}
	items, total, err := services.NewCrmAftersalesType().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAftersalesTypeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmAftersalesType
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAftersalesType().Create(ctx, &c)
}

func crmAftersalesTypeUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmAftersalesType
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAftersalesType().Update(ctx, id, &c)
}

func crmAftersalesTypeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmAftersalesType().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---- 售后评论 ----

func crmAftersalesCommentList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		AftersalesID string `json:"aftersales_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmAftersalesComment().ListByAftersales(ctx, req.AftersalesID)
}

func crmAftersalesCommentCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		AftersalesID string `json:"aftersales_id"`
		Content      string `json:"content"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAftersalesComment().Create(ctx, req.AftersalesID, u, req.Content)
}

func crmAftersalesCommentDelete(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmAftersalesComment().Delete(ctx, id)
}
