package dispatch

import (
	"context"
	"encoding/json"

	"core/app/entities"
	"core/app/requests"
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

func customerGetFullCustomerInfo(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtCustomerID string `json:"ext_customer_id"`
		ExtStaffID    string `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomer().GetFullCustomerInfo(req.ExtCustomerID, req.ExtStaffID, corpID)
}

func customerGetCustomerLosses(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryCustomerLossesReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewCustomer().GetCustomerLosses(p.Query, corpID, p.Pager, p.Sorter)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func customerStatistic(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QueryCustomerStatisticReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomer().CustomerStatistic(req, corpID)
}

func customerUpdateDeleteEventNotify(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.UpdateCustomerDeleteStaffNotifierReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomer().UpdateDeleteEventNotify(req, corpID)
}

func customerGetNotifyStaffRule(_ context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return services.NewCustomer().GetNotifyStaffRule(corpID)
}

func customerUpsertCustomerPortrait(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtCustomerID string `json:"ext_customer_id"`
		ExtStaffID    string `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomer().UpsertCustomerPortrait(req.ExtCustomerID, req.ExtStaffID)
}

func customerSyncSingleCustomerData(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtStaffID    string `json:"ext_staff_id"`
		ExtCustomerID string `json:"ext_customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomer().SyncSingleCustomerData(req.ExtStaffID, req.ExtCustomerID)
}

// ---------- customer_event ----------

func customerEventQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryEventListReq]
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

func customerInfoGet(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.GetCustomerInfoReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerInfo().Get(req, corpID)
}

func customerInfoUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.UpdateCustomerInfoReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerInfo().Update(&req, corpID)
}

// ---------- customer_info_display_rules ----------

func customerInfoDisplayRulesGet(_ context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return services.NewCustomerInfoDisplayRules().Get(corpID)
}

func customerInfoDisplayRulesUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.UpdateCustomerInfoDisplayRulesReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerInfoDisplayRules().Update(corpID, &req)
}

// ---------- customer_remark ----------

func customerRemarkGet(_ context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return services.NewCustomerRemarkService().Get(corpID)
}

func customerRemarkCreate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.AddCustomerRemarkReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerRemarkService().Create(&req, corpID)
}

func customerRemarkUpdate(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateRemarkReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerRemarkService().Update(&req)
}

func customerRemarkDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().Delete(req.IDs, corpID)
}

func customerRemarkAddOption(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.AddRemarkOptionReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().AddRemarkOption(&req)
}

func customerRemarkUpdateOption(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateRemarkOptionReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().UpdateRemarkOption(&req)
}

func customerRemarkDeleteOption(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().DeleteRemarkOption(req.IDs)
}

func customerRemarkExchangeOrder(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.ExchangeOrderReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCustomerRemarkService().ExchangeOrder(&req)
}

// ---------- clue_manual ----------

func clueManualCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateClueManualReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewClueManual().Create(req, corpID, staffID)
}

func clueManualUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                       `json:"id"`
		Req requests.UpdateClueManualReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewClueManual().Update(req.ID, req.Req, corpID)
}

func clueManualDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewClueManual().Delete(req.IDs, corpID)
}

// ---------- remainder ----------

func remainderCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateRemainderReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRemainder().Create(req, corpID, staffID)
}

func remainderUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                      `json:"id"`
		Req requests.UpdateRemainderReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRemainder().Update(req.ID, req.Req, corpID)
}

func remainderDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRemainder().Delete(req.ID, corpID)
}
