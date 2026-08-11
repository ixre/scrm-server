package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
)

// ---------- 仪表盘 ----------

func crmDashboardBriefing(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Briefing(t)
}

func crmDashboardSummary(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Summary(t)
}

func crmDashboardTrend(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Trend(t)
}

func crmDashboardRanking(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Ranking(t)
}

func crmDashboardAchievement(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Achievement(t)
}

// ---------- 统计报表 ----------

func crmStatsCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().CustomerAnalysis(t, req)
}

func crmStatsLead(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().LeadAnalysis(t, req)
}

func crmStatsContract(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().ContractStats(t, req)
}

func crmStatsWorkorder(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().WorkorderStats(t, req)
}

func crmStatsProduct(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().ProductStats(t, req)
}

func crmStatsCall(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.StatsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmStatistics().CallStats(t, req)
}
