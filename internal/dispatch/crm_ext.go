package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 客户池扩展 dispatch handlers ----------

func crmCustomerLock(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs    []string `json:"ids"`
		Reason string   `json:"reason"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmCustomerExt().Lock(ctx, req.IDs, req.Reason)
}

func crmCustomerUnlock(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmCustomerExt().Unlock(ctx, req.IDs)
}

func crmCustomerBlacklist(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs    []string `json:"ids"`
		Reason string   `json:"reason"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmCustomerExt().Blacklist(ctx, req.IDs, req.Reason)
}

func crmCustomerRemoveBlacklist(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmCustomerExt().RemoveBlacklist(ctx, req.IDs)
}

func crmCustomerSetKey(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs   []string `json:"ids"`
		IsKey bool     `json:"is_key"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmCustomerExt().SetKey(ctx, req.IDs, req.IsKey)
}

func crmCustomerShare(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string   `json:"customer_id"`
		StaffIDs   []string `json:"staff_ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCustomerExt().Share(ctx, req.CustomerID, req.StaffIDs)
}

func crmCustomerListBlacklist(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.CustomerPoolQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmCustomerExt().ListBlacklist(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCustomerListKey(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.CustomerPoolQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmCustomerExt().ListKey(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCustomerListShared(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.CustomerPoolQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmCustomerExt().ListShared(ctx, u, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCustomerListUnreceive(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.CustomerPoolQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmCustomerExt().ListUnreceive(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCustomerListReassign(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.CustomerPoolQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmCustomerExt().ListReassign(ctx, u, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCustomerAcceptReassign(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return ok, services.NewCrmCustomerExt().AcceptReassign(ctx, u, id)
}

func crmCustomerRejectReassign(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmCustomerExt().RejectReassign(ctx, id)
}

// ---------- 线索池 dispatch handlers ----------

func crmLeadMoveToPool(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmLeadExt().MoveToPool(ctx, req.IDs)
}

func crmLeadReceiveFromPool(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return ok, services.NewCrmLeadExt().ReceiveFromPool(ctx, u, id)
}

func crmLeadListPool(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.LeadPoolQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmLeadExt().ListPool(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmLeadTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID       string `json:"id"`
		ToUserID string `json:"to_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeadExt().Transfer(ctx, req.ID, req.ToUserID)
}

func crmLeadConvertToCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ID     string `json:"id"`
		Remark string `json:"remark"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	c, err := services.NewCrmLeadExt().ConvertToCustomer(ctx, u, req.ID, req.Remark)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// ---------- 回收站 dispatch handlers ----------

func crmRecycleList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		EntityType string `json:"entity_type"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.RecycleQueryReq{Page: req.Page, PageSize: req.PageSize}
	items, total, err := services.NewCrmRecycle().List(ctx, req.EntityType, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmRecycleRestore(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmRecycle().Restore(ctx, id)
}

func crmRecycleClear(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmRecycle().Clear(ctx, id)
}

func crmRecycleDeleteRecord(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	n, e := services.NewCrmRecycle().DeleteRecord(ctx, req.IDs)
	return map[string]any{"ok": true, "count": n}, e
}

// ---------- 批量操作 dispatch handlers ----------

func crmBatchExecute(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req services.BatchExecReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmBatch().Execute(ctx, u, req)
}
