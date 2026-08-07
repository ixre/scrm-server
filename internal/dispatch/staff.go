package dispatch

import (
	"context"
	"encoding/json"

	"core/app/entities"
	"core/app/requests"
	"core/app/services"
)

// 员工（staff）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "staff"
	Reg.Register(s, "Enable", staffEnable)
	Reg.Register(s, "GetDetail", staffGetDetail)
	Reg.Register(s, "StaffInfoStatistics", staffStaffInfoStatistics)
	Reg.Register(s, "UpdateCustomerTag", staffUpdateCustomerTag)
	Reg.Register(s, "QueryMainInfo", staffQueryMainInfo)
	Reg.Register(s, "GetCustomerSummary", staffGetCustomerSummary)
	Reg.Register(s, "UpdateCustomerInternalTag", staffUpdateCustomerInternalTag)

	p := "permission"
	Reg.Register(p, "Query", permissionQuery)
	Reg.Register(p, "Get", permissionGet)

	l := "login"
	Reg.Register(l, "StaffLogin", loginStaffLogin)
	Reg.Register(l, "StaffAdminLogin", loginStaffAdminLogin)

	j := "js_api"
	Reg.Register(j, "GetJSConfig", jsApiGetJSConfig)
	Reg.Register(j, "GetJSAgentConfig", jsApiGetJSAgentConfig)

	u := "util"
	Reg.Register(u, "ParseLink", utilParseLink)

	m := "msg_arch"
	Reg.Register(m, "QueryMsgs", msgArchQueryMsgs)
	Reg.Register(m, "QuerySessions", msgArchQuerySessions)
	Reg.Register(m, "SearchMsgs", msgArchSearchMsgs)
}

func staffEnable(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.EnableStaffs
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewStaffService().Enable(req, corpID)
}

func staffGetDetail(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewStaffService().GetDetail(req.ID, corpID)
}

func staffStaffInfoStatistics(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewStaffService().StaffInfoStatistics(req.ID, corpID)
}

func staffUpdateCustomerTag(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateCustomerTagsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewStaffService().UpdateCustomerTag(req, req.ExtStaffID, corpID)
}

func staffQueryMainInfo(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QueryMainStaffInfoReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewStaffService().QueryMainInfo(req, corpID, &req.Pager)
}

func staffGetCustomerSummary(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewStaffService().GetCustomerSummary(req.ID, corpID)
}

func staffUpdateCustomerInternalTag(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateCustomerInternalTagsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewStaffService().UpdateCustomerInternalTag(req, corpID)
}

func permissionQuery(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[entities.QueryPermissionReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewDefaultPermission().Query(p.Query, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func permissionGet(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultPermission().Get(req.ID)
}

func loginStaffLogin(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		State      string `json:"state"`
		SourceURL  string `json:"source_url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultLogin().StaffLogin(corpID, req.State, req.SourceURL)
}

func loginStaffAdminLogin(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		State     string `json:"state"`
		SourceURL string `json:"source_url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultLogin().StaffAdminLogin(corpID, req.State, req.SourceURL)
}

func jsApiGetJSConfig(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		URL string `json:"url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewJsApiService().GetJSConfig(req.URL, corpID)
}

func jsApiGetJSAgentConfig(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		URL string `json:"url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewJsApiService().GetJSAgentConfig(req.URL, corpID)
}

func utilParseLink(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.ParseLinkReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return (&services.Util{}).ParseLink(req)
}

func msgArchQueryMsgs(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QueryChatMsgReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewMsgArch().QueryMsgs(req, corpID)
}

func msgArchQuerySessions(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QuerySessionReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewMsgArch().QuerySessions(req, corpID)
}

func msgArchSearchMsgs(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.SearchMsgReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewMsgArch().SearchMsgs(req, corpID)
}
