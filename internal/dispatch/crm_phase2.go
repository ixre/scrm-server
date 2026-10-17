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
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		app.Pager
		Status string `json:"status"`
		Query  struct {
			Status string `json:"status"`
		} `json:"query"`
		LegacyPager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	p := &req.Pager
	if req.LegacyPager != nil {
		p = req.LegacyPager
	}
	status := req.Status
	if status == "" {
		status = req.Query.Status
	}
	rows, total, err := services.NewCrmChannelTask().List(ctx, status, p)
	if err != nil {
		return nil, err
	}
	return Result(rows, total), nil
}
func crmChannelTaskMarkDone(ctx context.Context, a, b string, params json.RawMessage) (any, error) {
	return resolveChannelTask(ctx, params, "done")
}
func crmChannelTaskMarkSkipped(ctx context.Context, a, b string, params json.RawMessage) (any, error) {
	return resolveChannelTask(ctx, params, "skipped")
}
func resolveChannelTask(ctx context.Context, params json.RawMessage, status string) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID      string `json:"id"`
		Note    string `json:"note"`
		Version int64  `json:"version"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmChannelTask().Resolve(ctx, req.ID, status, req.Note, req.Version)
}
func crmChannelTaskAssign(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID      string `json:"id"`
		UserID  string `json:"user_id"`
		Version int64  `json:"version"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmChannelTask().Assign(ctx, req.ID, req.UserID, req.Version)
}
func crmChannelTaskClaim(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmChannelTask().Claim(ctx, req.ID, req.Version)
}
func crmChannelTaskReopen(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID      string `json:"id"`
		Note    string `json:"note"`
		Version int64  `json:"version"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmChannelTask().Reopen(ctx, req.ID, req.Note, req.Version)
}
func crmChannelTaskLogs(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
		app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	rows, total, err := services.NewCrmChannelTask().Logs(ctx, req.ID, &req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(rows, total), nil
}

// _ 确保 identity 包被引用（UserID 在其他 handler 中使用）
var _ = identity.UserID
