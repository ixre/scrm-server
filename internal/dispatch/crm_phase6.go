package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- Phase 6 营销活动深化 dispatch handlers ----------

// ---- 秒杀 ----

func crmSeckillCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmSeckill
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSeckill().Create(ctx, req)
}

func crmSeckillQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSeckill().Query(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSeckillGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSeckill().Get(ctx, req.ID)
}

func crmSeckillDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSeckill().Delete(ctx, req.IDs)
}

func crmSeckillGrab(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		SeckillID  string `json:"seckill_id"`
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSeckill().Grab(ctx, req.SeckillID, req.CustomerID)
}

func crmSeckillListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		SeckillID string `json:"seckill_id"`
		Status    string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSeckill().ListRecords(ctx, req.SeckillID, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 抽奖 ----

func crmLotteryCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmLottery
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLottery().Create(ctx, req)
}

func crmLotteryQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmLottery().Query(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmLotteryGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmLottery().Get(ctx, req.ID)
}

func crmLotteryDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmLottery().Delete(ctx, req.IDs)
}

func crmLotteryDraw(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		LotteryID  string `json:"lottery_id"`
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLottery().Draw(ctx, req.LotteryID, req.CustomerID)
}

func crmLotteryListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		LotteryID string `json:"lottery_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmLottery().ListRecords(ctx, req.LotteryID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- Phase 6 报表扩展 ----

func crmReportSalesRanking(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		YearMonth string `json:"year_month"`
	}
	_ = Bind(params, &req)
	return services.NewCrmReport().SalesRanking(ctx, req.YearMonth)
}

func crmReportCustomerLifecycle(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().CustomerLifecycle(ctx)
}

func crmReportChannelCAC(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().ChannelCAC(ctx)
}

// ---- 管理视角看板：业务健康度 / 员工效能 / 风险预警 ----

func crmReportHealth(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	return services.NewCrmReport().Health(ctx)
}

func crmReportStaffEfficiency(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	return services.NewCrmReport().StaffEfficiency(ctx)
}

func crmReportRiskAlerts(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	return services.NewCrmReport().RiskAlerts(ctx)
}

// ---- Phase 6 客户导入导出 ----

func crmCustomerExport(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	buf, filename, err := services.NewCrmCustomer().Export(ctx, EffectiveOwnerFilter(ctx, ""))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"filename": filename,
		"content":  buf.Bytes(),
	}, nil
}

func crmCustomerBatchImport(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	uid := identity.UserID(ctx)
	var req struct {
		Rows []map[string]string `json:"rows"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	success, fail, err := services.NewCrmCustomer().BatchImport(ctx, uid, req.Rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{"success": success, "fail": fail}, nil
}

func crmLeadBatchImport(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	uid := identity.UserID(ctx)
	var req struct {
		Rows []map[string]string `json:"rows"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	success, fail, err := services.NewCrmLead().BatchImport(ctx, uid, req.Rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{"success": success, "fail": fail}, nil
}

func crmLeadExport(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status      string `json:"status"`
		OwnerUserID string `json:"owner_user_id"`
	}
	_ = Bind(params, &req)
	buf, filename, err := services.NewCrmLeadExt().Export(ctx, req.Status, EffectiveOwnerFilter(ctx, req.OwnerUserID))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"filename": filename,
		"content":  buf.Bytes(),
	}, nil
}

func crmLeadBatchTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs      []string `json:"ids"`
		ToUserID string   `json:"to_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	n, err := services.NewCrmLeadExt().BatchTransfer(ctx, req.IDs, req.ToUserID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"affected": n}, nil
}

func crmOpportunityExport(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status      string `json:"status"`
		OwnerUserID string `json:"owner_user_id"`
	}
	_ = Bind(params, &req)
	buf, filename, err := services.NewCrmOpportunityExt().Export(ctx, req.Status, EffectiveOwnerFilter(ctx, req.OwnerUserID))
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"filename": filename,
		"content":  buf.Bytes(),
	}, nil
}

func crmOpportunityBatchTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs      []string `json:"ids"`
		ToUserID string   `json:"to_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	n, err := services.NewCrmOpportunityExt().BatchTransfer(ctx, req.IDs, req.ToUserID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"affected": n}, nil
}

func crmOpportunityBatchImport(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	uid := identity.UserID(ctx)
	var req struct {
		Rows []map[string]string `json:"rows"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	success, fail, err := services.NewCrmOpportunityExt().BatchImport(ctx, uid, req.Rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{"success": success, "fail": fail}, nil
}
