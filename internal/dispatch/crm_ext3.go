package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 财务扩展 dispatch handlers ----------

// ---- 收支类别 ----

func crmIncomeCategoryList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Type   string `json:"type"`
		Status int    `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.IncomeCategoryQueryReq{Page: req.Page, PageSize: req.PageSize, Type: req.Type, Status: req.Status}
	items, total, err := services.NewCrmIncomeCategory().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmIncomeCategoryCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmIncomeCategory
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmIncomeCategory().Create(ctx, &c)
}

func crmIncomeCategoryUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmIncomeCategory
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmIncomeCategory().Update(ctx, id, &c)
}

func crmIncomeCategoryDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmIncomeCategory().Delete(ctx, req.IDs); return ok, e }
}

// ---- 其他收入单 ----

func crmOtherIncomeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status     string `json:"status"`
		CategoryID string `json:"category_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.OtherIncomeQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status, CategoryID: req.CategoryID}
	items, total, err := services.NewCrmOtherIncome().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmOtherIncomeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmOtherIncome
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOtherIncome().Create(ctx, &r)
}

func crmOtherIncomeUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmOtherIncome
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOtherIncome().Update(ctx, id, &r)
}

func crmOtherIncomeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmOtherIncome().Delete(ctx, req.IDs); return ok, e }
}

func crmOtherIncomeChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmOtherIncome().ChangeStatus(ctx, id, req.Status)
}

// ---- 其他支出单 ----

func crmOtherExpenseList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status     string `json:"status"`
		CategoryID string `json:"category_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.OtherExpenseQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status, CategoryID: req.CategoryID}
	items, total, err := services.NewCrmOtherExpense().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmOtherExpenseCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmOtherExpense
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOtherExpense().Create(ctx, &r)
}

func crmOtherExpenseUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmOtherExpense
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOtherExpense().Update(ctx, id, &r)
}

func crmOtherExpenseDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmOtherExpense().Delete(ctx, req.IDs); return ok, e }
}

func crmOtherExpenseChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmOtherExpense().ChangeStatus(ctx, id, req.Status)
}

// ---- 资金转账单 ----

func crmFundTransferList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	q := services.FundTransferQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status}
	items, total, err := services.NewCrmFundTransfer().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFundTransferCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmFundTransfer
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmFundTransfer().Create(ctx, &r)
}

func crmFundTransferUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmFundTransfer
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmFundTransfer().Update(ctx, id, &r)
}

func crmFundTransferDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmFundTransfer().Delete(ctx, req.IDs); return ok, e }
}

func crmFundTransferChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmFundTransfer().ChangeStatus(ctx, id, req.Status)
}

// ---------- 进销存单据 dispatch handlers ----------

// ---- 入库单 ----

func crmStockInList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		WarehouseID string `json:"warehouse_id"`
		Type        string `json:"type"`
		Status      string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.StockInQueryReq{Page: req.Page, PageSize: req.PageSize, WarehouseID: req.WarehouseID, Type: req.Type, Status: req.Status}
	items, total, err := services.NewCrmStockIn().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmStockInCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockIn
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockIn().Create(ctx, &r)
}

func crmStockInUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockIn
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockIn().Update(ctx, id, &r)
}

func crmStockInDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmStockIn().Delete(ctx, req.IDs); return ok, e }
}

func crmStockInChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmStockIn().ChangeStatus(ctx, id, req.Status)
}

// ---- 出库单 ----

func crmStockOutList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		WarehouseID string `json:"warehouse_id"`
		Type        string `json:"type"`
		Status      string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.StockOutQueryReq{Page: req.Page, PageSize: req.PageSize, WarehouseID: req.WarehouseID, Type: req.Type, Status: req.Status}
	items, total, err := services.NewCrmStockOut().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmStockOutCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockOut
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockOut().Create(ctx, &r)
}

func crmStockOutUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockOut
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockOut().Update(ctx, id, &r)
}

func crmStockOutDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmStockOut().Delete(ctx, req.IDs); return ok, e }
}

func crmStockOutChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmStockOut().ChangeStatus(ctx, id, req.Status)
}

// ---- 盘点单 ----

func crmStockCheckList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		WarehouseID string `json:"warehouse_id"`
		Status      string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.StockCheckQueryReq{Page: req.Page, PageSize: req.PageSize, WarehouseID: req.WarehouseID, Status: req.Status}
	items, total, err := services.NewCrmStockCheck().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmStockCheckCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockCheck
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockCheck().Create(ctx, &r)
}

func crmStockCheckUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockCheck
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockCheck().Update(ctx, id, &r)
}

func crmStockCheckDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmStockCheck().Delete(ctx, req.IDs); return ok, e }
}

func crmStockCheckChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmStockCheck().ChangeStatus(ctx, id, req.Status)
}

// ---- 调拨单 ----

func crmStockTransferList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	q := services.StockTransferQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status}
	items, total, err := services.NewCrmStockTransfer().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmStockTransferCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockTransfer
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockTransfer().Create(ctx, &r)
}

func crmStockTransferUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockTransfer
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockTransfer().Update(ctx, id, &r)
}

func crmStockTransferDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmStockTransfer().Delete(ctx, req.IDs); return ok, e }
}

func crmStockTransferChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmStockTransfer().ChangeStatus(ctx, id, req.Status)
}

// ---------- 工单扩展 dispatch handlers ----------

// ---- 工单类型 ----

func crmWorkorderTypeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status int `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.WorkorderTypeQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status}
	items, total, err := services.NewCrmWorkorderType().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmWorkorderTypeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmWorkorderType
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkorderType().Create(ctx, &c)
}

func crmWorkorderTypeUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmWorkorderType
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkorderType().Update(ctx, id, &c)
}

func crmWorkorderTypeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmWorkorderType().Delete(ctx, req.IDs); return ok, e }
}

// ---- 工单池 ----

func crmWorkorderPoolList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.WorkorderPoolQueryReq{Page: req.Page, PageSize: req.PageSize}
	items, total, err := services.NewCrmWorkorderPool().ListPool(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmWorkorderPoolMoveToPool(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmWorkorderPool().MoveToPool(ctx, req.IDs)
}

func crmWorkorderPoolReceive(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return ok, services.NewCrmWorkorderPool().Receive(ctx, u, id)
}

func crmWorkorderPoolBatchReceive(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmWorkorderPool().BatchReceive(ctx, u, req.IDs)
}

// ---- 工单日志 ----

func crmWorkorderLogList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		WorkorderID string `json:"workorder_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmWorkorderLog().ListByWorkorder(ctx, req.WorkorderID)
}

func crmWorkorderLogAdd(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		WorkorderID string `json:"workorder_id"`
		Action      string `json:"action"`
		Content     string `json:"content"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkorderLog().Add(ctx, req.WorkorderID, u, req.Action, req.Content)
}

// ---------- 呼叫中心扩展 dispatch handlers ----------

func crmCallListList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID string `json:"user_id"`
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.CallListQueryReq{Page: req.Page, PageSize: req.PageSize, UserID: req.UserID, Status: req.Status}
	items, total, err := services.NewCrmCallList().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCallListCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmCallList
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCallList().Create(ctx, &c)
}

func crmCallListUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmCallList
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCallList().Update(ctx, id, &c)
}

func crmCallListDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmCallList().Delete(ctx, req.IDs); return ok, e }
}

func crmCallListBatchAssign(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID string   `json:"user_id"`
		IDs    []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmCallList().BatchAssign(ctx, req.UserID, req.IDs)
}

func crmCallListMarkDialed(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmCallList().MarkDialed(ctx, id)
}

func crmCallListMarkSkipped(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmCallList().MarkSkipped(ctx, id)
}
