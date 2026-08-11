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
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmSeckill
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSeckill().Create(t, req)
}

func crmSeckillQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmSeckill().Query(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSeckillGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSeckill().Get(t, req.ID)
}

func crmSeckillDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSeckill().Delete(t, req.IDs)
}

func crmSeckillGrab(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	return services.NewCrmSeckill().Grab(t, req.SeckillID, req.CustomerID)
}

func crmSeckillListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmSeckill().ListRecords(t, req.SeckillID, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 抽奖 ----

func crmLotteryCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmLottery
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLottery().Create(t, req)
}

func crmLotteryQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmLottery().Query(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmLotteryGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLottery().Get(t, req.ID)
}

func crmLotteryDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmLottery().Delete(t, req.IDs)
}

func crmLotteryDraw(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	return services.NewCrmLottery().Draw(t, req.LotteryID, req.CustomerID)
}

func crmLotteryListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmLottery().ListRecords(t, req.LotteryID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- Phase 6 报表扩展 ----

func crmReportSalesRanking(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		YearMonth string `json:"year_month"`
	}
	_ = Bind(params, &req)
	return services.NewCrmReport().SalesRanking(t, req.YearMonth)
}

func crmReportCustomerLifecycle(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().CustomerLifecycle(t)
}

func crmReportChannelCAC(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().ChannelCAC(t)
}

// ---- Phase 6 客户导入导出 ----

func crmCustomerExport(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	buf, filename, err := services.NewCrmCustomer().Export(t)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"filename": filename,
		"content":  buf.Bytes(),
	}, nil
}

func crmCustomerBatchImport(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	success, fail, err := services.NewCrmCustomer().BatchImport(t, uid, req.Rows)
	if err != nil {
		return nil, err
	}
	return map[string]any{"success": success, "fail": fail}, nil
}
