package dispatch

import (
	"context"
	"encoding/json"

	"core/app/dto"
	"core/app/services"
)

// 客户/CRM/提醒/线索/导出 域适配器 —— 通用 JSON 透传分发。
// 本批统一写在 customer.go，服务 key 按业务域划分。
func init() {
	// customer
	Reg.Register("customer", "GetFullCustomerInfo", customerGetFullCustomerInfo)
	Reg.Register("customer", "GetCustomerLosses", customerGetCustomerLosses)
	Reg.Register("customer", "CustomerStatistic", customerStatistic)
	Reg.Register("customer", "UpdateDeleteEventNotify", customerUpdateDeleteEventNotify)
	Reg.Register("customer", "GetNotifyStaffRule", customerGetNotifyStaffRule)
	Reg.Register("customer", "UpsertCustomerPortrait", customerUpsertCustomerPortrait)
	Reg.Register("customer", "SyncSingleCustomerData", customerSyncSingleCustomerData)

	// customer_event
	Reg.Register("customer_event", "Query", customerEventQuery)

	// customer_info
	Reg.Register("customer_info", "Get", customerInfoGet)
	Reg.Register("customer_info", "Update", customerInfoUpdate)

	// customer_info_display_rules
	Reg.Register("customer_info_display_rules", "Get", customerInfoDisplayRulesGet)
	Reg.Register("customer_info_display_rules", "Update", customerInfoDisplayRulesUpdate)

	// customer_remark
	Reg.Register("customer_remark", "Get", customerRemarkGet)
	Reg.Register("customer_remark", "Create", customerRemarkCreate)
	Reg.Register("customer_remark", "Update", customerRemarkUpdate)
	Reg.Register("customer_remark", "Delete", customerRemarkDelete)
	Reg.Register("customer_remark", "AddRemarkOption", customerRemarkAddOption)
	Reg.Register("customer_remark", "UpdateRemarkOption", customerRemarkUpdateOption)
	Reg.Register("customer_remark", "DeleteRemarkOption", customerRemarkDeleteOption)
	Reg.Register("customer_remark", "ExchangeOrder", customerRemarkExchangeOrder)

	// clue_manual
	Reg.Register("clue_manual", "Create", clueManualCreate)
	Reg.Register("clue_manual", "Update", clueManualUpdate)
	Reg.Register("clue_manual", "Delete", clueManualDelete)

	// remainder
	Reg.Register("remainder", "Create", remainderCreate)
	Reg.Register("remainder", "Update", remainderUpdate)
	Reg.Register("remainder", "Delete", remainderDelete)
}

// ---------- customer ----------

func customerGetFullCustomerInfo(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtCustomerID string `json:"ext_customer_id"`
		ExtStaffID    string `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewCustomer().GetFullCustomerInfo(ctx, req.ExtCustomerID, req.ExtStaffID, corpID)
}

func customerGetCustomerLosses(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[dto.QueryCustomerLossesReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	items, total, err := services.NewCustomer().GetCustomerLosses(ctx, p.Query, corpID, p.Pager, p.Sorter)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func customerStatistic(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.QueryCustomerStatisticReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewCustomer().CustomerStatistic(ctx, req, corpID)
}

func customerUpdateDeleteEventNotify(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.UpdateCustomerDeleteStaffNotifierReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return nil, services.NewCustomer().UpdateDeleteEventNotify(ctx, req, corpID)
}

func customerGetNotifyStaffRule(ctx context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	ctx = corpCtx(ctx, corpID)
	return services.NewCustomer().GetNotifyStaffRule(ctx, corpID)
}

func customerUpsertCustomerPortrait(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtCustomerID string `json:"ext_customer_id"`
		ExtStaffID    string `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomer().UpsertCustomerPortrait(ctx, req.ExtCustomerID, req.ExtStaffID)
}

func customerSyncSingleCustomerData(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtStaffID    string `json:"ext_staff_id"`
		ExtCustomerID string `json:"ext_customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomer().SyncSingleCustomerData(ctx, req.ExtStaffID, req.ExtCustomerID)
}

// ---------- customer_event ----------

func customerEventQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[dto.QueryEventListReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewCustomerEvent().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- customer_info ----------

func customerInfoGet(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.GetCustomerInfoReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerInfo().Get(req, corpID)
}

func customerInfoUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.UpdateCustomerInfoReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerInfo().Update(&req, corpID)
}

// ---------- customer_info_display_rules ----------

func customerInfoDisplayRulesGet(ctx context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return services.NewCustomerInfoDisplayRules().Get(corpID)
}

func customerInfoDisplayRulesUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.UpdateCustomerInfoDisplayRulesReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerInfoDisplayRules().Update(corpID, &req)
}

// ---------- customer_remark ----------

func customerRemarkGet(ctx context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return services.NewCustomerRemarkService().Get(corpID)
}

func customerRemarkCreate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req dto.AddCustomerRemarkReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerRemarkService().Create(&req, corpID)
}

func customerRemarkUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req dto.UpdateRemarkReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerRemarkService().Update(&req)
}

func customerRemarkDelete(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().Delete(req.IDs, corpID)
}

func customerRemarkAddOption(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req dto.AddRemarkOptionReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().AddRemarkOption(&req)
}

func customerRemarkUpdateOption(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req dto.UpdateRemarkOptionReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().UpdateRemarkOption(&req)
}

func customerRemarkDeleteOption(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().DeleteRemarkOption(req.IDs)
}

func customerRemarkExchangeOrder(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req dto.ExchangeOrderReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().ExchangeOrder(&req)
}

// ---------- clue_manual ----------

func clueManualCreate(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req dto.CreateClueManualReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewClueManual().Create(req, corpID, staffID)
}

func clueManualUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                  `json:"id"`
		Req dto.UpdateClueManualReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewClueManual().Update(req.ID, req.Req, corpID)
}

func clueManualDelete(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewClueManual().Delete(req.IDs, corpID)
}

// ---------- remainder ----------

func remainderCreate(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req dto.CreateRemainderReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRemainder().Create(req, corpID, staffID)
}

func remainderUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                 `json:"id"`
		Req dto.UpdateRemainderReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRemainder().Update(req.ID, req.Req, corpID)
}

func remainderDelete(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRemainder().Delete(req.ID, corpID)
}
