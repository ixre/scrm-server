package dispatch

import (
	"context"
	"encoding/json"
	"strconv"

	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- SOP 自动化 ----------

func crmSopCreateRule(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.SopRuleCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSop().CreateRule(ctx, req.Rule)
}

func crmSopUpdateRule(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
		services.SopRuleCreateReq
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSop().UpdateRule(ctx, req.ID, req.Rule)
}

func crmSopQueryRules(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status string `json:"status"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSop().QueryRules(ctx, req.Query.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSopDeleteRules(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSop().DeleteRules(ctx, req.IDs)
}

func crmSopListTasks(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status       string `json:"status"`
			AssignUserID string `json:"assign_user_id"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSop().ListTasks(ctx, req.Query.Status, req.Query.AssignUserID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSopMarkTaskDone(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCrmSop().MarkTaskDone(ctx, req.ID)
}

func crmSopMarkTaskSkipped(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCrmSop().MarkTaskSkipped(ctx, req.ID)
}

// ---------- 客户评分 ----------

func crmScoreTopByScore(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Limit string `json:"limit"`
	}
	_ = Bind(params, &req)
	limit, _ := strconv.Atoi(req.Limit)
	if limit <= 0 {
		limit = 10
	}
	return services.NewCrmScore().TopByScore(ctx, limit)
}

func crmScoreRecalculate(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireAnyRole(ctx, "admin", "manager"); err != nil {
		return nil, err
	}
	return nil, services.NewCrmScore().RecalculateAll(ctx)
}

// ---------- 自动打标签 ----------

func crmAutoTagCreateRule(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.AutoTagRuleCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmAutoTag().CreateRule(ctx, req.Rule)
}

func crmAutoTagQueryRules(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status string `json:"status"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmAutoTag().QueryRules(ctx, req.Query.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAutoTagDeleteRules(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmAutoTag().DeleteRules(ctx, req.IDs)
}

func crmAutoTagApplyToCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCrmAutoTag().ApplyToCustomer(ctx, req.CustomerID)
}

// ---------- 销售话术 ----------

func crmScriptCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.SalesScriptCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSalesScript().Create(ctx, req.Script)
}

func crmScriptUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
		services.SalesScriptCreateReq
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSalesScript().Update(ctx, req.ID, req.Script)
}

func crmScriptQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Stage    string `json:"stage"`
			Industry string `json:"industry"`
			Scene    string `json:"scene"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSalesScript().Query(ctx, req.Query.Stage, req.Query.Industry, req.Query.Scene, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmScriptDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSalesScript().Delete(ctx, req.IDs)
}

func crmScriptRecommend(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSalesScript().Recommend(ctx, req.CustomerID)
}

// _ 确保 identity 包被引用
var _ = identity.UserID
