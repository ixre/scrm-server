package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
)

// ---------- 仪表盘 ----------

// DashboardReq 仪表盘请求参数：year_month 为 "YYYY-MM"，空则默认当月。
type DashboardReq struct {
	YearMonth string `json:"year_month"`
}

func bindDashboardReq(params json.RawMessage) (string, error) {
	var req DashboardReq
	if err := Bind(params, &req); err != nil {
		return "", err
	}
	return req.YearMonth, nil
}

func crmDashboardBriefing(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	ym, err := bindDashboardReq(params)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Briefing(ctx, ym)
}

func crmDashboardSummary(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	ym, err := bindDashboardReq(params)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Summary(ctx, ym)
}

func crmDashboardTrend(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	ym, err := bindDashboardReq(params)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Trend(ctx, ym)
}

func crmDashboardRanking(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	ym, err := bindDashboardReq(params)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Ranking(ctx, ym)
}

func crmDashboardAchievement(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	ym, err := bindDashboardReq(params)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Achievement(ctx, ym)
}

// ---------- 统计报表 ----------

func crmStatsCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().CustomerAnalysis(ctx, req)
}

func crmStatsLead(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().LeadAnalysis(ctx, req)
}

func crmStatsContract(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().ContractStats(ctx, req)
}

func crmStatsWorkorder(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().WorkorderStats(ctx, req)
}

func crmStatsProduct(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().ProductStats(ctx, req)
}

func crmStatsCall(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().CallStats(ctx, req)
}
