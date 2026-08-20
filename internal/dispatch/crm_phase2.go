package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- Phase 2 渠道对比看板 ----------

func crmReportChannelCompare(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().ChannelCompare(ctx)
}

// ---------- 引流任务 ----------

func crmChannelTaskList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	items, total, err := services.NewCrmChannelTask().List(ctx, req.Query.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmChannelTaskMarkDone(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return nil, services.NewCrmChannelTask().MarkDone(ctx, req.ID)
}

func crmChannelTaskMarkSkipped(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return nil, services.NewCrmChannelTask().MarkSkipped(ctx, req.ID)
}

func crmChannelTaskAssign(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireAnyRole(ctx, "admin", "manager"); err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		UserID string `json:"user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCrmChannelTask().Assign(ctx, req.ID, req.UserID)
}

// _ 确保 identity 包被引用（UserID 在其他 handler 中使用）
var _ = identity.UserID
