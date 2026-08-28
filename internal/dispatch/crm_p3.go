package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- P3: 大模块扩展 dispatch handlers ----------

// ---- 项目 ----

func crmProjectList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status     string `json:"status"`
		CustomerID string `json:"customer_id"`
		Keyword    string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.ProjectQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status, CustomerID: req.CustomerID, Keyword: req.Keyword}
	items, total, err := services.NewCrmProject().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmProjectCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p models.CrmProject
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProject().Create(ctx, &p)
}

func crmProjectUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var p models.CrmProject
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProject().Update(ctx, id, &p)
}

func crmProjectDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmProject().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmProjectChangeStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProject().ChangeStatus(ctx, id, req.Status)
}

// ---- 项目任务 ----

func crmProjectTaskList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ProjectID string `json:"project_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmProjectTask().ListByProject(ctx, req.ProjectID)
}

func crmProjectTaskCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var task models.CrmProjectTask
	if err := Bind(params, &task); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectTask().Create(ctx, &task)
}

func crmProjectTaskUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var task models.CrmProjectTask
	if err := Bind(params, &task); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectTask().Update(ctx, id, &task)
}

func crmProjectTaskDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmProjectTask().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmProjectTaskUpdateStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectTask().UpdateStatus(ctx, id, req.Status)
}

// ---- 财务账户 ----

func crmAccountList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Type    string `json:"type"`
		Status  string `json:"status"`
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.AccountQueryReq{Page: req.Page, PageSize: req.PageSize, Type: req.Type, Status: req.Status, Keyword: req.Keyword}
	items, total, err := services.NewCrmAccount().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAccountCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var a models.CrmAccount
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAccount().Create(ctx, &a)
}

func crmAccountUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var a models.CrmAccount
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAccount().Update(ctx, id, &a)
}

func crmAccountDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmAccount().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---- 财务收支记录 ----

func crmFinanceRecordList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Type      string `json:"type"`
		Category  string `json:"category"`
		AccountID string `json:"account_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.FinanceRecordQueryReq{Page: req.Page, PageSize: req.PageSize, Type: req.Type, Category: req.Category, AccountID: req.AccountID}
	items, total, err := services.NewCrmFinanceRecord().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFinanceRecordCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmFinanceRecord
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmFinanceRecord().Create(ctx, &r)
}

func crmFinanceRecordUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var r models.CrmFinanceRecord
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmFinanceRecord().Update(ctx, id, &r)
}

func crmFinanceRecordDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmFinanceRecord().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmFinanceRecordVoid(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmFinanceRecord().Void(ctx, id)
}

// ---- 发票 ----

func crmInvoiceList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	q := services.InvoiceQueryReq{Page: req.Page, PageSize: req.PageSize, CustomerID: req.CustomerID, Status: req.Status, Keyword: req.Keyword}
	items, total, err := services.NewCrmInvoice().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmInvoiceCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var i models.CrmInvoice
	if err := Bind(params, &i); err != nil {
		return nil, err
	}
	return ok, services.NewCrmInvoice().Create(ctx, &i)
}

func crmInvoiceUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var i models.CrmInvoice
	if err := Bind(params, &i); err != nil {
		return nil, err
	}
	return ok, services.NewCrmInvoice().Update(ctx, id, &i)
}

func crmInvoiceDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmInvoice().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmInvoiceVoid(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmInvoice().Void(ctx, id)
}

// ---- 产品 ----

func crmProductList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Category string `json:"category"`
		Status   string `json:"status"`
		Keyword  string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.ProductQueryReq{Page: req.Page, PageSize: req.PageSize, Category: req.Category, Status: req.Status, Keyword: req.Keyword}
	items, total, err := services.NewCrmProduct().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmProductCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p models.CrmProduct
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProduct().Create(ctx, &p)
}

func crmProductUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var p models.CrmProduct
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProduct().Update(ctx, id, &p)
}

func crmProductDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmProduct().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---- 仓库 ----

func crmWarehouseList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status  string `json:"status"`
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.WarehouseQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status, Keyword: req.Keyword}
	items, total, err := services.NewCrmWarehouse().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmWarehouseCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var w models.CrmWarehouse
	if err := Bind(params, &w); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWarehouse().Create(ctx, &w)
}

func crmWarehouseUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var w models.CrmWarehouse
	if err := Bind(params, &w); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWarehouse().Update(ctx, id, &w)
}

func crmWarehouseDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmWarehouse().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---- 库存流水 ----

func crmStockRecordList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		WarehouseID string `json:"warehouse_id"`
		ProductID   string `json:"product_id"`
		Type        string `json:"type"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.StockRecordQueryReq{Page: req.Page, PageSize: req.PageSize, WarehouseID: req.WarehouseID, ProductID: req.ProductID, Type: req.Type}
	items, total, err := services.NewCrmStockRecord().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmStockRecordCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmStockRecord
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmStockRecord().Create(ctx, &r)
}

func crmStockRecordDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmStockRecord().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmProductStockBalance(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Keyword     string `json:"keyword"`
		WarningOnly bool   `json:"warning_only"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmProduct().StockBalance(ctx, req.Keyword, req.WarningOnly, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmProductStockWarning(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	items, err := services.NewCrmProduct().StockWarning(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]any{"items": items, "total": len(items)}, nil
}

// ---- 工单 ----

func crmWorkorderList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status     string `json:"status"`
		CustomerID string `json:"customer_id"`
		Type       string `json:"type"`
		Priority   string `json:"priority"`
		Keyword    string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.WorkorderQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status, CustomerID: req.CustomerID, Type: req.Type, Priority: req.Priority, Keyword: req.Keyword}
	items, total, err := services.NewCrmWorkorder().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmWorkorderCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var w models.CrmWorkorder
	if err := Bind(params, &w); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkorder().Create(ctx, &w)
}

func crmWorkorderUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var w models.CrmWorkorder
	if err := Bind(params, &w); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkorder().Update(ctx, id, &w)
}

func crmWorkorderDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmWorkorder().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmWorkorderChangeStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkorder().ChangeStatus(ctx, id, req.Status)
}

func crmWorkorderTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var req struct {
		ToUserID string `json:"to_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkorder().Transfer(ctx, id, req.ToUserID)
}

// ---- 通话记录 ----

func crmCallRecordList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID     string `json:"user_id"`
		CustomerID string `json:"customer_id"`
		Direction  string `json:"direction"`
		Status     string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.CallRecordQueryReq{Page: req.Page, PageSize: req.PageSize, UserID: req.UserID, CustomerID: req.CustomerID, Direction: req.Direction, Status: req.Status}
	items, total, err := services.NewCrmCallRecord().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCallRecordCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmCallRecord
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCallRecord().Create(ctx, &r)
}

func crmCallRecordUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var r models.CrmCallRecord
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCallRecord().Update(ctx, id, &r)
}

func crmCallRecordDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmCallRecord().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmCallRecordStats(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmCallRecord().Stats(ctx)
}

// ---- 知识库 ----

func crmKnowledgeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		KnowledgeTypeID string `json:"knowledge_type_id"`
		Status          string `json:"status"`
		Keyword         string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.KnowledgeQueryReq{Page: req.Page, PageSize: req.PageSize, KnowledgeTypeID: req.KnowledgeTypeID, Status: req.Status, Keyword: req.Keyword}
	items, total, err := services.NewCrmKnowledge().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmKnowledgeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var k models.CrmKnowledge
	if err := Bind(params, &k); err != nil {
		return nil, err
	}
	return ok, services.NewCrmKnowledge().Create(ctx, &k)
}

func crmKnowledgeUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var k models.CrmKnowledge
	if err := Bind(params, &k); err != nil {
		return nil, err
	}
	return ok, services.NewCrmKnowledge().Update(ctx, id, &k)
}

func crmKnowledgeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmKnowledge().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---- 公告 ----

func crmNoticeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status  string `json:"status"`
		Type    string `json:"type"`
		Keyword string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.NoticeQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status, Type: req.Type, Keyword: req.Keyword}
	items, total, err := services.NewCrmNotice().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmNoticeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var n models.CrmNotice
	if err := Bind(params, &n); err != nil {
		return nil, err
	}
	return ok, services.NewCrmNotice().Create(ctx, &n)
}

func crmNoticeUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var n models.CrmNotice
	if err := Bind(params, &n); err != nil {
		return nil, err
	}
	return ok, services.NewCrmNotice().Update(ctx, id, &n)
}

func crmNoticeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmNotice().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---- 工作报告 ----

func crmWorkReportList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID string `json:"user_id"`
		Type   string `json:"type"`
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.WorkReportQueryReq{Page: req.Page, PageSize: req.PageSize, UserID: req.UserID, Type: req.Type, Status: req.Status}
	items, total, err := services.NewCrmWorkReport().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmWorkReportCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var r models.CrmWorkReport
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkReport().Create(ctx, &r)
}

func crmWorkReportUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var r models.CrmWorkReport
	if err := Bind(params, &r); err != nil {
		return nil, err
	}
	return ok, services.NewCrmWorkReport().Update(ctx, id, &r)
}

func crmWorkReportDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmWorkReport().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmWorkReportReview(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return ok, services.NewCrmWorkReport().Review(ctx, id, u)
}

// ---- AI分析 ----

func crmAiAnalysisList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
		Type       string `json:"type"`
		Status     string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.AiQueryReq{Page: req.Page, PageSize: req.PageSize, CustomerID: req.CustomerID, Type: req.Type, Status: req.Status}
	items, total, err := services.NewCrmAiAnalysis().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAiAnalysisCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var a models.CrmAiAnalysis
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAiAnalysis().Create(ctx, &a)
}

func crmAiAnalysisDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmAiAnalysis().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmAiAnalysisGetByCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmAiAnalysis().GetByCustomer(ctx, req.CustomerID)
}

func crmAiAnalysisAnalyzeCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAiAnalysis().AnalyzeCustomer(ctx, req.CustomerID)
}

func crmAiAnalysisStats(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmAiAnalysis().Stats(ctx)
}

// ---- 售后管理 ----

func crmAftersalesList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status     string `json:"status"`
		CustomerID string `json:"customer_id"`
		Type       string `json:"type"`
		Keyword    string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.AftersalesQueryReq{Page: req.Page, PageSize: req.PageSize, Status: req.Status, CustomerID: req.CustomerID, Type: req.Type, Keyword: req.Keyword}
	items, total, err := services.NewCrmAftersales().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAftersalesCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var a models.CrmAftersales
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAftersales().Create(ctx, &a)
}

func crmAftersalesUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var a models.CrmAftersales
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAftersales().Update(ctx, id, &a)
}

func crmAftersalesDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmAftersales().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmAftersalesChangeStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAftersales().ChangeStatus(ctx, id, req.Status)
}
