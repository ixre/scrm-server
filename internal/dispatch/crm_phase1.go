package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 公海池 ----------

func crmPoolList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Keyword string `json:"keyword"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmPool().ListPool(ctx, req.Query.Keyword, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmPoolGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return services.NewCrmPool().Get(ctx, req.ID)
}

func crmPoolClaim(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmPool().Claim(ctx, identity.UserID(ctx), req.IDs)
}

func crmPoolRecycle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmPool().Recycle(ctx, req.IDs)
}

func crmPoolGetRule(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmPool().GetRule(ctx)
}

func crmPoolSetRule(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := services.RequireCRMPermission(ctx, "crm:pool:manage"); err != nil {
		return nil, err
	}
	var req models.CrmPoolRule
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmPool().SetRule(ctx, req)
}

func crmPoolAssign(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := services.RequireCRMPermission(ctx, "crm:pool:assign"); err != nil {
		return nil, err
	}
	var req struct {
		IDs          []string `json:"ids"`
		TargetUserID string   `json:"target_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errors.New("ids 不能为空")
	}
	if req.TargetUserID == "" {
		return nil, errors.New("target_user_id 不能为空")
	}
	return services.NewCrmPool().Assign(ctx, identity.UserID(ctx), req.TargetUserID, req.IDs)
}

func crmPoolListLogs(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string     `json:"customer_id"`
		Pager      *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmPool().ListLogs(ctx, req.CustomerID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmPoolGetStats(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmPool().GetStats(ctx)
}

// ---------- 客户转移 ----------

func crmCustomerTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := services.RequireCRMPermission(ctx, "crm:customer:transfer"); err != nil {
		return nil, err
	}
	var req struct {
		IDs      []string `json:"ids"`
		ToUserID string   `json:"to_user_id"`
		Reason   string   `json:"reason"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmTransfer().Transfer(ctx, identity.UserID(ctx), req.IDs, req.ToUserID, req.Reason)
}

func crmCustomerListTransferLog(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string     `json:"customer_id"`
		Pager      *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmTransfer().ListTransferLog(ctx, req.CustomerID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- 跟进提醒 ----------

func crmFollowupListReminders(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().ListReminders(ctx, identity.UserID(ctx))
}

func crmFollowupListOverdue(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().ListOverdue(ctx, identity.UserID(ctx))
}

// ---------- 销售规则 ----------

func crmSalesRuleGet(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmSalesRule().GetRule(ctx)
}

func crmSalesRuleSet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := services.RequireCRMPermission(ctx, "crm:sales_rule:manage"); err != nil {
		return nil, err
	}
	var req struct {
		RequireFollowup bool `json:"require_followup"`
		FollowupDays    int  `json:"followup_days"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSalesRule().SetRule(ctx, req.RequireFollowup, req.FollowupDays)
}

// ---------- 业绩目标 ----------

func crmQuotaSet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := services.RequireCRMPermission(ctx, "crm:quota:manage"); err != nil {
		return nil, err
	}
	var req struct {
		UserID       string  `json:"user_id"`
		YearMonth    string  `json:"year_month"`
		TargetAmount float64 `json:"target_amount"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.YearMonth == "" {
		req.YearMonth = time.Now().Format("2006-01")
	}
	return services.NewCrmQuota().Set(ctx, req.UserID, req.YearMonth, req.TargetAmount)
}

func crmQuotaGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID    string `json:"user_id"`
		YearMonth string `json:"year_month"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.UserID == "" {
		req.UserID = identity.UserID(ctx)
	}
	if req.YearMonth == "" {
		req.YearMonth = time.Now().Format("2006-01")
	}
	return services.NewCrmQuota().Get(ctx, req.UserID, req.YearMonth)
}

func crmQuotaList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		YearMonth string `json:"year_month"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.YearMonth == "" {
		req.YearMonth = time.Now().Format("2006-01")
	}
	return services.NewCrmQuota().List(ctx, req.YearMonth)
}

func crmQuotaAchievement(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID    string `json:"user_id"`
		YearMonth string `json:"year_month"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.UserID == "" {
		req.UserID = identity.UserID(ctx)
	}
	if req.YearMonth == "" {
		req.YearMonth = time.Now().Format("2006-01")
	}
	return services.NewCrmQuota().Achievement(ctx, req.UserID, req.YearMonth)
}

func crmQuotaTeamAchievement(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		YearMonth string `json:"year_month"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.YearMonth == "" {
		req.YearMonth = time.Now().Format("2006-01")
	}
	return services.NewCrmQuota().TeamAchievement(ctx, req.YearMonth)
}

// ---------- 客户去重 ----------

func crmDedupFindDuplicates(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Mobile string `json:"mobile"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmDedup().FindDuplicates(ctx, req.Mobile)
}

func crmDedupMerge(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := services.RequireCRMPermission(ctx, "crm:dedup:manage"); err != nil {
		return nil, err
	}
	var req struct {
		PrimaryID    string   `json:"primary_id"`
		DuplicateIDs []string `json:"duplicate_ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.PrimaryID == "" {
		return nil, errors.New("primary_id 不能为空")
	}
	return nil, services.NewCrmDedup().Merge(ctx, req.PrimaryID, req.DuplicateIDs)
}
