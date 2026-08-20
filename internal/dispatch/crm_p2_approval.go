package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/identity"
)

// ---------- P2: 审批引擎 dispatch handlers ----------

// ---- 审批流定义 ----

func crmApprovalFlowList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		BizType string `json:"biz_type"`
	}
	if len(params) > 0 {
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
	}
	items, err := services.NewCrmApproval().ListFlows(ctx, req.BizType)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmApprovalFlowCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var f models.CrmApprovalFlow
	if err := Bind(params, &f); err != nil {
		return nil, err
	}
	return ok, services.NewCrmApproval().CreateFlow(ctx, &f)
}

func crmApprovalFlowUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var f models.CrmApprovalFlow
	if err := Bind(params, &f); err != nil {
		return nil, err
	}
	return ok, services.NewCrmApproval().UpdateFlow(ctx, id, &f)
}

func crmApprovalFlowDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmApproval().DeleteFlow(ctx, req.IDs)
		return ok, e
	}
}

// ---- 审批实例 ----

func crmApprovalSubmit(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		BizType string `json:"biz_type"`
		BizID   string `json:"biz_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	inst, err := services.NewCrmApproval().Submit(ctx, req.BizType, req.BizID, u)
	if err != nil {
		return nil, err
	}
	return inst, nil
}

func crmApprovalInstanceList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		BizType     string `json:"biz_type"`
		Status      string `json:"status"`
		ApplicantID string `json:"applicant_id"`
		Page        int    `json:"page"`
		PageSize    int    `json:"page_size"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.ApprovalInstanceQuery{
		Page:        req.Page,
		PageSize:    req.PageSize,
		BizType:     req.BizType,
		Status:      req.Status,
		ApplicantID: req.ApplicantID,
	}
	items, total, err := services.NewCrmApproval().ListInstances(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmApprovalApprove(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		Comment string `json:"comment"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmApproval().Approve(ctx, id, u, req.Comment)
}

func crmApprovalReject(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		Comment string `json:"comment"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmApproval().Reject(ctx, id, u, req.Comment)
}

func crmApprovalVoid(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return ok, services.NewCrmApproval().Void(ctx, id, u)
}

func crmApprovalRecordList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		InstanceID string `json:"instance_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, err := services.NewCrmApproval().ListRecords(ctx, req.InstanceID)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}
