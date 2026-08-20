package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
)

// ---------- 仪表盘 ----------

func crmDashboardBriefing(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Briefing(ctx)
}

func crmDashboardSummary(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Summary(ctx)
}

func crmDashboardTrend(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Trend(ctx)
}

func crmDashboardRanking(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Ranking(ctx)
}

func crmDashboardAchievement(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmDashboard().Achievement(ctx)
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
