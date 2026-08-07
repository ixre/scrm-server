package dispatch

import (
	"context"
	"encoding/json"

	"core/app/entities"
	"core/app/requests"
	"core/app/services"
)

// 渠道码（contact_way）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "contact_way"
	Reg.Register(s, "Query", contactWayQuery)
	Reg.Register(s, "Get", contactWayGet)
	Reg.Register(s, "Create", contactWayCreate)
	Reg.Register(s, "Update", contactWayUpdate)
	Reg.Register(s, "Delete", contactWayDelete)
	Reg.Register(s, "BatchUpdate", contactWayBatchUpdate)
}

func contactWayQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryContactWayReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewContactWay().Query(p.Query, corpID)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func contactWayGet(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWay().Get(req.ID, corpID)
}

func contactWayCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateContactWayReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWay().Create(req, corpID, staffID)
}

func contactWayUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                        `json:"id"`
		Req requests.UpdateContactWayReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWay().Update(req.ID, req.Req, corpID)
}

func contactWayDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWay().Delete(req.IDs, corpID)
}

func contactWayBatchUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.BatchUpdateContactWayReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWay().BatchUpdate(req, corpID)
}

// 渠道码分组（contact_way_group）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "contact_way_group"
	Reg.Register(s, "Query", contactWayGroupQuery)
	Reg.Register(s, "Get", contactWayGroupGet)
	Reg.Register(s, "Create", contactWayGroupCreate)
	Reg.Register(s, "Update", contactWayGroupUpdate)
	Reg.Register(s, "Delete", contactWayGroupDelete)
}

func contactWayGroupQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryContactWayGroupReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewContactWayGroup().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func contactWayGroupGet(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWayGroup().Get(req.ID, corpID)
}

func contactWayGroupCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateContactWayGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWayGroup().Create(req, corpID, staffID)
}

func contactWayGroupUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                        `json:"id"`
		Req requests.UpdateContactWayGroupReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWayGroup().Update(req.ID, req.Req, corpID)
}

func contactWayGroupDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewContactWayGroup().Delete(req.IDs, corpID)
}

// 部门（department）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "department"
	Reg.Register(s, "Query", departmentQuery)
	Reg.Register(s, "Get", departmentGet)
}

func departmentQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[entities.GetSubDepartmentReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewDepartment().Query(p.Query, corpID)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func departmentGet(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtDeptID int64 `json:"ext_dept_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDepartment().Get(req.ExtDeptID, corpID)
}

// 企业风控（corporation_risk_mgr）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "corporation_risk_mgr"
	Reg.Register(s, "Query", corpRiskMgrQuery)
	Reg.Register(s, "Get", corpRiskMgrGet)
	Reg.Register(s, "Upsert", corpRiskMgrUpsert)
}

func corpRiskMgrQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryStaffDeleteCustomerHistoryReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewCorpRiskMgrService().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func corpRiskMgrGet(_ context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return services.NewCorpRiskMgrService().Get(corpID)
}

func corpRiskMgrUpsert(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateStaffDeleteCustomerNotifierReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCorpRiskMgrService().Upsert(req, corpID)
}

// 内部标签（internal_tag）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "internal_tag"
	Reg.Register(s, "Query", internalTagQuery)
	Reg.Register(s, "Create", internalTagCreate)
	Reg.Register(s, "Delete", internalTagDelete)
}

func internalTagQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryInternalTagReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewInternalTag().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func internalTagCreate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.CreateInternalTagReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewInternalTag().Create(req, corpID)
}

func internalTagDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewInternalTag().Delete(req.IDs, corpID)
}
