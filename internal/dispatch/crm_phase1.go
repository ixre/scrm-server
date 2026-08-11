package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 公海池 ----------

func crmPoolList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmPool().ListPool(t, req.Query.Keyword, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmPoolClaim(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmPool().Claim(t, identity.UserID(ctx), req.IDs)
}

func crmPoolRecycle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmPool().Recycle(t, req.IDs)
}

func crmPoolGetRule(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmPool().GetRule(t)
}

func crmPoolSetRule(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	var req struct {
		RecycleDays int  `json:"recycle_days"`
		Enabled     bool `json:"enabled"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmPool().SetRule(t, req.RecycleDays, req.Enabled)
}

// ---------- 客户转移 ----------

func crmCustomerTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireAnyRole(ctx, "admin", "manager"); err != nil {
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
	return services.NewCrmTransfer().Transfer(t, identity.UserID(ctx), req.IDs, req.ToUserID, req.Reason)
}

func crmCustomerListTransferLog(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmTransfer().ListTransferLog(t, req.CustomerID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- 跟进提醒 ----------

func crmFollowupListReminders(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().ListReminders(t, identity.UserID(ctx))
}

func crmFollowupListOverdue(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().ListOverdue(t, identity.UserID(ctx))
}

// ---------- 销售规则 ----------

func crmSalesRuleGet(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmSalesRule().GetRule(t)
}

func crmSalesRuleSet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	var req struct {
		RequireFollowup bool `json:"require_followup"`
		FollowupDays    int  `json:"followup_days"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSalesRule().SetRule(t, req.RequireFollowup, req.FollowupDays)
}

// ---------- 业绩目标 ----------

func crmQuotaSet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireAnyRole(ctx, "admin", "manager"); err != nil {
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
	return services.NewCrmQuota().Set(t, req.UserID, req.YearMonth, req.TargetAmount)
}

func crmQuotaGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	return services.NewCrmQuota().Get(t, req.UserID, req.YearMonth)
}

func crmQuotaList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	return services.NewCrmQuota().List(t, req.YearMonth)
}

func crmQuotaAchievement(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	return services.NewCrmQuota().Achievement(t, req.UserID, req.YearMonth)
}

func crmQuotaTeamAchievement(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	return services.NewCrmQuota().TeamAchievement(t, req.YearMonth)
}

// ---------- 客户去重 ----------

func crmDedupFindDuplicates(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Mobile string `json:"mobile"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmDedup().FindDuplicates(t, req.Mobile)
}

func crmDedupMerge(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireAnyRole(ctx, "admin", "manager"); err != nil {
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
	return nil, services.NewCrmDedup().Merge(t, req.PrimaryID, req.DuplicateIDs)
}
