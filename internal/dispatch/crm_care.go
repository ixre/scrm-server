package dispatch

import (
	"context"
	"encoding/json"
	"errors"

	"core/app/models"
	"core/app/services"
	"core/common/app"
)

func init() {
	// 客户关怀模板
	Reg.Register("crm_care", "List", crmCareList)
	Reg.Register("crm_care", "Create", crmCareCreate)
	Reg.Register("crm_care", "Update", crmCareUpdate)
	Reg.Register("crm_care", "Delete", crmCareDelete)

	// 客户分群
	Reg.Register("crm_segment", "List", crmSegmentList)
	Reg.Register("crm_segment", "Create", crmSegmentCreate)
	Reg.Register("crm_segment", "Update", crmSegmentUpdate)
	Reg.Register("crm_segment", "Delete", crmSegmentDelete)
	Reg.Register("crm_segment", "Preview", crmSegmentPreview)

	// 跟进统计
	Reg.Register("crm_followup_stats", "Stats", crmFollowupStats)
}

// ---- 客户关怀 ----

func crmCareList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		Type string `json:"type"`
	}
	_ = Bind(params, &req)
	items, err := services.NewCrmCare().List(ctx, req.Type)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmCareCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var item models.CrmCareTemplate
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCare().Create(ctx, &item)
}

func crmCareUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var item models.CrmCareTemplate
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	var idHolder struct {
		ID string `json:"id"`
	}
	_ = Bind(params, &idHolder)
	if idHolder.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return ok, services.NewCrmCare().Update(ctx, idHolder.ID, &item)
}

func crmCareDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errors.New("ids 不能为空")
	}
	_, e := services.NewCrmCare().Delete(ctx, req.IDs)
	return ok, e
}

// ---- 客户分群 ----

func crmSegmentList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var pager app.Pager
	_ = Bind(params, &pager)
	items, total, err := services.NewCrmSegment().List(ctx, &pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSegmentCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var item models.CrmCustomerSegment
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSegment().Create(ctx, &item)
}

func crmSegmentUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var item models.CrmCustomerSegment
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	var idHolder struct {
		ID string `json:"id"`
	}
	_ = Bind(params, &idHolder)
	if idHolder.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return ok, services.NewCrmSegment().Update(ctx, idHolder.ID, &item)
}

func crmSegmentDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errors.New("ids 不能为空")
	}
	_, e := services.NewCrmSegment().Delete(ctx, req.IDs)
	return ok, e
}

func crmSegmentPreview(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		Conditions string `json:"conditions"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	count, err := services.NewCrmSegment().Preview(ctx, req.Conditions)
	if err != nil {
		return nil, err
	}
	return map[string]any{"count": count}, nil
}

// ---- 跟进统计 ----

func crmFollowupStats(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	return services.NewCrmFollowupStats().Stats(ctx)
}
