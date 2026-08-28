package dispatch

import (
	"context"
	"encoding/json"

	"core/app/requests"
	"core/app/services"
)

// 角色（role）域适配器 —— 通用 JSON 透传分发模板。
func init() {
	s := "role"
	Reg.Register(s, "Create", roleCreate)
	Reg.Register(s, "Update", roleUpdate)
	Reg.Register(s, "Query", roleQuery)
	Reg.Register(s, "QueryStaffs", roleQueryStaffs)
	Reg.Register(s, "Get", roleGet)
	Reg.Register(s, "AssignToStaffs", roleAssignToStaffs)
}

func roleCreate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.CreateRoleReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRole().Create(req, corpID)
}

func roleUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                 `json:"id"`
		Req requests.UpdateRoleReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRole().Update(req.ID, req.Req, corpID)
}

func roleQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryRoleReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewRole().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func roleQueryStaffs(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryRoleStaffsReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewRole().QueryStaffs(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func roleGet(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRole().Get(req.ID, corpID)
}

func roleAssignToStaffs(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtStaffIDs []string `json:"ext_staff_ids"`
		RoleID      string   `json:"role_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewRole().AssignToStaffs(req.ExtStaffIDs, req.RoleID, corpID)
}
