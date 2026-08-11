package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 项目事件 ----------

func crmProjectEventList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ProjectID string `json:"project_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmProjectEvent().ListByProject(t, req.ProjectID)
}

func crmProjectEventCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var e models.CrmProjectEvent
	if err := Bind(params, &e); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectEvent().Create(t, &e)
}

func crmProjectEventUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var e models.CrmProjectEvent
	if err := Bind(params, &e); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectEvent().Update(t, id, &e)
}

func crmProjectEventDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmProjectEvent().Delete(t, req.IDs)
		return ok, e
	}
}

func crmProjectEventCompleteEvent(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectEvent().CompleteEvent(t, id)
}

// ---------- 项目文档 ----------

func crmProjectFileList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ProjectID string `json:"project_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmProjectFile().ListByProject(t, req.ProjectID)
}

func crmProjectFileCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var f models.CrmProjectFile
	if err := Bind(params, &f); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectFile().Create(t, u, &f)
}

func crmProjectFileDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmProjectFile().Delete(t, req.IDs)
		return ok, e
	}
}

// ---------- 供应商 ----------

func crmSupplierList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	q := services.SupplierQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmSupplier().Query(t, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSupplierCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var sup models.CrmSupplier
	if err := Bind(params, &sup); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSupplier().Create(t, &sup)
}

func crmSupplierUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var sup models.CrmSupplier
	if err := Bind(params, &sup); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSupplier().Update(t, id, &sup)
}

func crmSupplierDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmSupplier().Delete(t, req.IDs)
		return ok, e
	}
}

// ---------- 采购订单 ----------

func crmPurchaseOrderList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		SupplierID string `json:"supplier_id"`
		Status     string `json:"status"`
		Keyword    string `json:"keyword"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PurchaseOrderQueryReq{Page: req.Page, PageSize: req.PageSize, SupplierID: req.SupplierID, Status: req.Status, Keyword: req.Keyword}
	items, total, err := services.NewCrmPurchaseOrder().Query(t, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmPurchaseOrderCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var o models.CrmPurchaseOrder
	if err := Bind(params, &o); err != nil {
		return nil, err
	}
	return ok, services.NewCrmPurchaseOrder().Create(t, &o)
}

func crmPurchaseOrderUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var o models.CrmPurchaseOrder
	if err := Bind(params, &o); err != nil {
		return nil, err
	}
	return ok, services.NewCrmPurchaseOrder().Update(t, id, &o)
}

func crmPurchaseOrderDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmPurchaseOrder().Delete(t, req.IDs)
		return ok, e
	}
}

func crmPurchaseOrderChangeStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmPurchaseOrder().ChangeStatus(t, id, req.Status)
}

// ---------- 考勤 ----------

func crmAttendanceList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID string `json:"user_id"`
		Type   string `json:"type"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.AttendanceQueryReq{Page: req.Page, PageSize: req.PageSize, UserID: req.UserID, Type: req.Type}
	items, total, err := services.NewCrmAttendance().Query(t, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAttendanceCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var a models.CrmAttendance
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAttendance().Create(t, u, &a)
}

// ---------- 请假 ----------

func crmLeaveList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	q := services.LeaveQueryReq{Page: req.Page, PageSize: req.PageSize, UserID: req.UserID, Status: req.Status}
	items, total, err := services.NewCrmLeave().Query(t, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmLeaveCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var l models.CrmLeave
	if err := Bind(params, &l); err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Create(t, u, &l)
}

func crmLeaveUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var l models.CrmLeave
	if err := Bind(params, &l); err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Update(t, id, &l)
}

func crmLeaveDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmLeave().Delete(t, req.IDs)
		return ok, e
	}
}

func crmLeaveApprove(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Approve(t, id)
}

func crmLeaveReject(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Reject(t, id)
}

// ---------- 自动分配规则 ----------

func crmAutoAllocationList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	q := services.AutoAllocationQueryReq{Page: req.Page, PageSize: req.PageSize, EntityType: req.EntityType}
	items, total, err := services.NewCrmAutoAllocation().Query(t, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAutoAllocationCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var a models.CrmAutoAllocation
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAutoAllocation().Create(t, &a)
}

func crmAutoAllocationUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var a models.CrmAutoAllocation
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAutoAllocation().Update(t, id, &a)
}

func crmAutoAllocationDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmAutoAllocation().Delete(t, req.IDs)
		return ok, e
	}
}

func crmAutoAllocationToggleStatus(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAutoAllocation().ToggleStatus(t, id, req.Enabled)
}

// ---------- 公海类型 ----------

func crmSeasTypeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
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
	q := services.SeasTypeQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmSeasType().Query(t, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSeasTypeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var st models.CrmSeasType
	if err := Bind(params, &st); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSeasType().Create(t, &st)
}

func crmSeasTypeUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var st models.CrmSeasType
	if err := Bind(params, &st); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSeasType().Update(t, id, &st)
}

func crmSeasTypeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{
		_, e := services.NewCrmSeasType().Delete(t, req.IDs)
		return ok, e
	}
}

func crmSeasTypeSetDefault(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmSeasType().SetDefault(t, id)
}

// ---------- 客户设置 ----------

func crmCustomerSettingGet(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmCustomerSetting().Get(t)
}

func crmCustomerSettingSave(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var cs models.CrmCustomerSetting
	if err := Bind(params, &cs); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCustomerSetting().Save(t, &cs)
}
