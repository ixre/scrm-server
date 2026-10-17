package dispatch

import (
	"context"
	"encoding/json"
	"errors"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 项目事件 ----------

func crmProjectEventList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmProjectEvent().ListByProject(ctx, req.ProjectID)
}

func crmProjectEventCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var e models.CrmProjectEvent
	if err := Bind(params, &e); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectEvent().Create(ctx, &e)
}

func crmProjectEventUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var e models.CrmProjectEvent
	if err := Bind(params, &e); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectEvent().Update(ctx, id, &e)
}

func crmProjectEventDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmProjectEvent().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmProjectEventCompleteEvent(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectEvent().CompleteEvent(ctx, id)
}

// ---------- 项目文档 ----------

func crmProjectFileList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmProjectFile().ListByProject(ctx, req.ProjectID)
}

func crmProjectFileCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var f models.CrmProjectFile
	if err := Bind(params, &f); err != nil {
		return nil, err
	}
	return ok, services.NewCrmProjectFile().Create(ctx, u, &f)
}

func crmProjectFileDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmProjectFile().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---------- 供应商 ----------

func crmSupplierList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	q := services.SupplierQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmSupplier().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSupplierCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var sup models.CrmSupplier
	if err := Bind(params, &sup); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSupplier().Create(ctx, &sup)
}

func crmSupplierUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var sup models.CrmSupplier
	if err := Bind(params, &sup); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSupplier().Update(ctx, id, &sup)
}

func crmSupplierDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmSupplier().Delete(ctx, req.IDs)
		return ok, e
	}
}

// ---------- 采购订单 ----------

func crmPurchaseOrderList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmPurchaseOrder().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmPurchaseOrderCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var o models.CrmPurchaseOrder
	if err := Bind(params, &o); err != nil {
		return nil, err
	}
	return ok, services.NewCrmPurchaseOrder().Create(ctx, &o)
}

func crmPurchaseOrderUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var o models.CrmPurchaseOrder
	if err := Bind(params, &o); err != nil {
		return nil, err
	}
	return ok, services.NewCrmPurchaseOrder().Update(ctx, id, &o)
}

func crmPurchaseOrderDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmPurchaseOrder().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmPurchaseOrderChangeStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmPurchaseOrder().ChangeStatus(ctx, id, req.Status)
}

// ---------- 考勤 ----------

func crmAttendanceList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmAttendance().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAttendanceCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var a models.CrmAttendance
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAttendance().Create(ctx, u, &a)
}

// ---------- 请假 ----------

func crmLeaveList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	q := services.LeaveQueryReq{Page: req.Page, PageSize: req.PageSize, UserID: req.UserID, Status: req.Status}
	items, total, err := services.NewCrmLeave().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmLeaveCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var l models.CrmLeave
	if err := Bind(params, &l); err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Create(ctx, u, &l)
}

func crmLeaveUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var l models.CrmLeave
	if err := Bind(params, &l); err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Update(ctx, id, &l)
}

func crmLeaveDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmLeave().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmLeaveApprove(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Approve(ctx, id)
}

func crmLeaveReject(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeave().Reject(ctx, id)
}

// ---------- 自动分配规则 ----------

func crmAutoAllocationList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		EntityType string `json:"entity_type"`
		app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.AutoAllocationQueryReq{Page: req.Page, PageSize: req.PageSize, EntityType: req.EntityType}
	items, total, err := services.NewCrmAutoAllocation().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmAutoAllocationCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var a models.CrmAutoAllocation
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAutoAllocation().Create(ctx, &a)
}

func crmAutoAllocationUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var a models.CrmAutoAllocation
	if err := Bind(params, &a); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAutoAllocation().Update(ctx, id, &a)
}

func crmAutoAllocationDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmAutoAllocation().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmAutoAllocationToggleStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmAutoAllocation().ToggleStatus(ctx, id, req.Enabled)
}

// ---------- 公海类型 ----------

func crmSeasTypeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	q := services.SeasTypeQueryReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmSeasType().Query(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSeasTypeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var st models.CrmSeasType
	if err := Bind(params, &st); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSeasType().Create(ctx, &st)
}

func crmSeasTypeUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	var st models.CrmSeasType
	if err := Bind(params, &st); err != nil {
		return nil, err
	}
	return ok, services.NewCrmSeasType().Update(ctx, id, &st)
}

func crmSeasTypeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		_, e := services.NewCrmSeasType().Delete(ctx, req.IDs)
		return ok, e
	}
}

func crmSeasTypeSetDefault(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmSeasType().SetDefault(ctx, id)
}

// ---------- 客户设置 ----------

func crmCustomerSettingGet(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmCustomerSetting().Get(ctx)
}

func crmCustomerSettingSave(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var cs models.CrmCustomerSetting
	if err := Bind(params, &cs); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCustomerSetting().Save(ctx, &cs)
}

// ---------- 线索来源 ----------

func crmLeadSourceList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	items, err := services.NewCrmLeadSource().List(ctx)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmLeadSourceCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var src models.CrmLeadSource
	if err := Bind(params, &src); err != nil {
		return nil, err
	}
	return ok, services.NewCrmLeadSource().Create(ctx, &src)
}

func crmLeadSourceUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var src models.CrmLeadSource
	if err := Bind(params, &src); err != nil {
		return nil, err
	}
	// id 在顶层
	var idHolder struct {
		ID string `json:"id"`
	}
	_ = Bind(params, &idHolder)
	if idHolder.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return ok, services.NewCrmLeadSource().Update(ctx, idHolder.ID, &src)
}

func crmLeadSourceDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		return nil, errNeedIdentity
	}
	_, e := services.NewCrmLeadSource().Delete(ctx, req.IDs)
	return ok, e
}

// ---------- 赢单/输单原因字典 ----------

// ---- 客户标签 ----

func crmCustomerTagList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		Group string `json:"group"`
	}
	_ = Bind(params, &req)
	items, err := services.NewCrmCustomerTag().List(ctx, req.Group)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmCustomerTagCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var item models.CrmCustomerTag
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	return ok, services.NewCrmCustomerTag().Create(ctx, &item)
}

func crmCustomerTagUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var item models.CrmCustomerTag
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
	return ok, services.NewCrmCustomerTag().Update(ctx, idHolder.ID, &item)
}

func crmCustomerTagDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	_, e := services.NewCrmCustomerTag().Delete(ctx, req.IDs)
	return ok, e
}

func crmCustomerTagBatchTag(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		Action      string   `json:"action"`
		CustomerIDs []string `json:"customer_ids"`
		TagIDs      []string `json:"tag_ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomerTag().BatchTag(ctx, req.Action, req.CustomerIDs, req.TagIDs)
}

func crmCustomerTagStats(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	items, err := services.NewCrmCustomerTag().Stats(ctx)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmReasonDictList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Category string `json:"category"`
		} `json:"query"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, err := services.NewCrmReasonDict().List(ctx, req.Query.Category)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmReasonDictCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var item models.CrmReasonDict
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReasonDict().Create(ctx, &item)
}

func crmReasonDictUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	// 前端发送 { id, name, sort, enabled }，直接绑定到 CrmReasonDict
	var item models.CrmReasonDict
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	// id 在顶层
	var idHolder struct {
		ID string `json:"id"`
	}
	_ = Bind(params, &idHolder)
	if idHolder.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return ok, services.NewCrmReasonDict().Update(ctx, idHolder.ID, &item)
}

func crmReasonDictDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
		return nil, errNeedIdentity
	}
	_, e := services.NewCrmReasonDict().Delete(ctx, req.IDs)
	return ok, e
}

// ---- 行业字典（平台级） ----

func crmIndustryDictList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	items, err := services.NewCrmIndustryDict().List(ctx)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmIndustryDictCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator); err != nil {
		return nil, err
	}
	var item models.CrmIndustryDict
	if err := Bind(params, &item); err != nil {
		return nil, err
	}
	return ok, services.NewCrmIndustryDict().Create(&item)
}

func crmIndustryDictUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator); err != nil {
		return nil, err
	}
	var item models.CrmIndustryDict
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
	return ok, services.NewCrmIndustryDict().Update(idHolder.ID, &item)
}

func crmIndustryDictDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator); err != nil {
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
	_, e := services.NewCrmIndustryDict().Delete(req.IDs)
	return ok, e
}

func crmAutoAllocationLogs(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		RuleID   string `json:"rule_id"`
		BeforeID string `json:"before_id"`
		PageSize int    `json:"page_size"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmAutoAllocation().Logs(ctx, req.RuleID, req.BeforeID, req.PageSize)
}
