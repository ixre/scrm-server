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
	Reg.Register(s, "Sync", staffSync)
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
	Reg.Register(l, "ConsoleLogin", loginConsoleLogin)
	Reg.Register(l, "ConsoleForceLogin", loginConsoleForceLogin)
	Reg.Register(l, "ConsoleLoginCallback", func(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
		var req struct {
			CorpID string `json:"ext_corp_id"`
			Code   string `json:"code"`
		}
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
		return services.NewDefaultLogin().ConsoleLoginCallback(req.CorpID, req.Code)
	})
	Reg.Register(l, "StaffLoginCallback", func(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
		var req struct {
			CorpID string `json:"ext_corp_id"`
			Code   string `json:"code"`
		}
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
		return services.NewDefaultLogin().StaffLoginCallback(req.CorpID, req.Code)
	})

	j := "js_api"
	Reg.Register(j, "GetJSConfig", jsApiGetJSConfig)
	Reg.Register(j, "GetJSAgentConfig", jsApiGetJSAgentConfig)

	u := "util"
	Reg.Register(u, "ParseLink", utilParseLink)
	Reg.Register(u, "GetSignedURL", utilGetSignedURL)

	m := "msg_arch"
	Reg.Register(m, "QueryMsgs", msgArchQueryMsgs)
	Reg.Register(m, "QuerySessions", msgArchQuerySessions)
	Reg.Register(m, "SearchMsgs", msgArchSearchMsgs)
}

func staffEnable(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.EnableStaffs
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return nil, services.NewStaffService().Enable(ctx, req, corpID)
}

func staffGetDetail(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewStaffService().GetDetail(ctx, req.ID, corpID)
}

func staffStaffInfoStatistics(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewStaffService().StaffInfoStatistics(ctx, req.ID, corpID)
}

func staffUpdateCustomerTag(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateCustomerTagsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return nil, services.NewStaffService().UpdateCustomerTag(ctx, req, req.ExtStaffID, corpID)
}

func staffQueryMainInfo(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QueryMainStaffInfoReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewStaffService().QueryMainInfo(ctx, req, corpID, &req.Pager)
}

func staffGetCustomerSummary(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewStaffService().GetCustomerSummary(ctx, req.ID, corpID)
}

func staffUpdateCustomerInternalTag(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateCustomerInternalTagsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return nil, services.NewStaffService().UpdateCustomerInternalTag(ctx, req, corpID)
}

func permissionQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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

func permissionGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultPermission().Get(req.ID)
}

func loginStaffLogin(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtCorpID string `json:"ext_corp_id"`
		State     string `json:"state"`
		SourceURL string `json:"source_url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if corpID == "" {
		corpID = req.ExtCorpID
	}
	return services.NewDefaultLogin().StaffLogin(corpID, req.State, req.SourceURL)
}

func loginConsoleLogin(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtCorpID string `json:"ext_corp_id"`
		State     string `json:"state"`
		SourceURL string `json:"source_url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if corpID == "" {
		corpID = req.ExtCorpID
	}
	return services.NewDefaultLogin().ConsoleLogin(corpID, req.State, req.SourceURL)
}

func loginConsoleForceLogin(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ExtCorpID  string `json:"ext_corp_id"`
		ExtStaffID string `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultLogin().ForceConsoleLogin(req.ExtCorpID, req.ExtStaffID)
}

func jsApiGetJSConfig(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		URL string `json:"url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewJsApiService().GetJSConfig(req.URL, corpID)
}

func jsApiGetJSAgentConfig(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		URL string `json:"url"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewJsApiService().GetJSAgentConfig(req.URL, corpID)
}

func utilParseLink(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.ParseLinkReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return (&services.Util{}).ParseLink(req)
}

func utilGetSignedURL(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		FileName string `json:"file_name"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return (&services.Util{}).GetSignedURL(req.FileName)
}

func msgArchQueryMsgs(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QueryChatMsgReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewMsgArch().QueryMsgs(req, corpID)
}

func msgArchQuerySessions(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.QuerySessionReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewMsgArch().QuerySessions(req, corpID)
}

func msgArchSearchMsgs(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.SearchMsgReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewMsgArch().SearchMsgs(req, corpID)
}

func staffSync(ctx context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	ctx = corpCtx(ctx, corpID)
	return nil, services.NewStaffService().Sync(ctx, corpID)
}
