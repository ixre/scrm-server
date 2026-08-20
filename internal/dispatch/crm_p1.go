package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ok 是成功操作的统一返回值。
var ok = map[string]any{"ok": true}

// ---------- P1: CRM 核心补全 dispatch handlers ----------

// ---- 联系人 ----

func crmContactList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
		Keyword    string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.QueryReq{Page: req.Page, PageSize: req.PageSize, CustomerID: req.CustomerID, Keyword: req.Keyword}
	items, total, err := services.NewCrmContact().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmContactCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmContact
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmContact().Create(ctx, &c)
}

func crmContactUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmContact
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmContact().Update(ctx, id, &c)
}

func crmContactDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmContact().Delete(ctx, req.IDs); return ok, e }
}

// ---- 回款计划 ----

func crmReceivablesPlanList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		OrderID    string `json:"order_id"`
		CustomerID string `json:"customer_id"`
		Status     string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PlanQueryReq{Page: req.Page, PageSize: req.PageSize, OrderID: req.OrderID, CustomerID: req.CustomerID, Status: req.Status}
	items, total, err := services.NewCrmReceivablesPlan().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmReceivablesPlanCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p models.CrmReceivablesPlan
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivablesPlan().Create(ctx, &p)
}

func crmReceivablesPlanUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p models.CrmReceivablesPlan
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivablesPlan().Update(ctx, id, &p)
}

func crmReceivablesPlanDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmReceivablesPlan().Delete(ctx, req.IDs); return ok, e }
}

func crmReceivablesPlanBatchCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Plans []models.CrmReceivablesPlan `json:"plans"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivablesPlan().BatchCreate(ctx, req.Plans)
}

// ---- 回款登记 ----

func crmReceivablesList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		OrderID    string `json:"order_id"`
		CustomerID string `json:"customer_id"`
		PlanID     string `json:"plan_id"`
		Status     string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.RecvQueryReq{Page: req.Page, PageSize: req.PageSize, OrderID: req.OrderID, CustomerID: req.CustomerID, PlanID: req.PlanID, Status: req.Status}
	items, total, err := services.NewCrmReceivables().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmReceivablesCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmReceivables
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivables().Create(ctx, &r)
}

func crmReceivablesUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmReceivables
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivables().Update(ctx, id, &r)
}

func crmReceivablesDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmReceivables().Delete(ctx, req.IDs); return ok, e }
}

func crmReceivablesVoid(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivables().Void(ctx, id)
}

// ---- 报价单 ----

func crmQuoteList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
		Status     string `json:"status"`
		Keyword    string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.QuoteQueryReq{Page: req.Page, PageSize: req.PageSize, CustomerID: req.CustomerID, Status: req.Status, Keyword: req.Keyword}
	items, total, err := services.NewCrmQuote().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmQuoteCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var q models.CrmQuote
	if err := Bind(params, &q); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuote().Create(ctx, &q)
}

func crmQuoteUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var q models.CrmQuote
	if err := Bind(params, &q); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuote().Update(ctx, id, &q)
}

func crmQuoteDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmQuote().Delete(ctx, req.IDs); return ok, e }
}

func crmQuoteChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuote().ChangeStatus(ctx, id, req.Status)
}

func crmQuoteTransfer(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ToUserID string `json:"to_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuote().Transfer(ctx, id, req.ToUserID)
}

// ---- 待办 ----

func crmTodoList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.TodoQueryReq{Page: req.Page, PageSize: req.PageSize, Type: req.Type, Status: req.Status}
	items, total, err := services.NewCrmTodo().Query(ctx, u, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmTodoHandle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmTodo().Handle(ctx, req.IDs, req.Status)
}

func crmTodoCount(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return services.NewCrmTodo().Count(ctx, u)
}

// ---- 合同订单产品明细 ----

func crmOrderProductList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOrderProduct().ListByOrder(ctx, req.OrderID)
}

func crmOrderProductCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p models.CrmOrderProduct
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderProduct().Create(ctx, &p)
}

func crmOrderProductDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmOrderProduct().Delete(ctx, req.IDs); return ok, e }
}

// ---- 合同订单附件 ----

func crmOrderAttachmentList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOrderAttachment().ListByOrder(ctx, req.OrderID)
}

func crmOrderAttachmentCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var a models.CrmOrderAttachment
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderAttachment().Create(ctx, &a)
}

func crmOrderAttachmentDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmOrderAttachment().Delete(ctx, req.IDs); return ok, e }
}

// ---- 合同订单进度 ----

func crmOrderProgressList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOrderProgress().ListByOrder(ctx, req.OrderID)
}

func crmOrderProgressCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p models.CrmOrderProgress
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderProgress().Create(ctx, &p)
}

func crmOrderProgressUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p models.CrmOrderProgress
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderProgress().Update(ctx, id, &p)
}

func crmOrderProgressDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmOrderProgress().Delete(ctx, req.IDs); return ok, e }
}

func crmOrderProgressUpdateStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderProgress().UpdateStatus(ctx, id, req.Status)
}

// ---- 快捷跟进 ----

func crmQuickFollowupList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return services.NewCrmQuickFollowupService().List(ctx, u)
}

func crmQuickFollowupCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var q models.CrmQuickFollowup
	if err := Bind(params, &q); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuickFollowupService().Create(ctx, u, &q)
}

func crmQuickFollowupDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{ _, e := services.NewCrmQuickFollowupService().Delete(ctx, u, req.IDs); return ok, e }
}

// ---- 拜访签到 ----

func crmSigninList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		CustomerID string `json:"customer_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.SigninQueryReq{Page: req.Page, PageSize: req.PageSize, CustomerID: req.CustomerID}
	items, total, err := services.NewCrmSigninService().Query(ctx, u, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSigninCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var s models.CrmSignin
	if err := Bind(params, &s); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSigninService().Create(ctx, u, &s)
}

func crmSigninDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{ _, e := services.NewCrmSigninService().Delete(ctx, u, req.IDs); return ok, e }
}
