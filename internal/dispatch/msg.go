package dispatch

import (
	"context"
	"encoding/json"

	"core/app/entities"
	"core/app/models"
	"core/app/requests"
	"core/app/services"
)

// 群发（mass_msg）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "mass_msg"
	Reg.Register(s, "Create", massMsgCreate)
	Reg.Register(s, "Update", massMsgUpdate)
	Reg.Register(s, "Query", massMsgQuery)
	Reg.Register(s, "Get", massMsgGet)
	Reg.Register(s, "Delete", massMsgDelete)
}

func massMsgCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		Req        requests.SendMassMsgReq `json:"req"`
		ExtStaffID string                  `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultMassMsgService().Create(req.Req, req.ExtStaffID, corpID)
}

func massMsgUpdate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                   `json:"id"`
		Req requests.UpdateMassMsgReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultMassMsgService().UpdateMassMsg(req.Req, req.ID, staffID, corpID)
}

func massMsgQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[struct{}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewDefaultMassMsgService().Query(corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func massMsgGet(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewDefaultMassMsgService().Get(req.ID, corpID)
}

func massMsgDelete(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewDefaultMassMsgService().DeleteTimedMassMsg(req.IDs)
}

// 欢迎语（welcome_msg）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "welcome_msg"
	Reg.Register(s, "Create", welcomeMsgCreate)
	Reg.Register(s, "Update", welcomeMsgUpdate)
	Reg.Register(s, "Query", welcomeMsgQuery)
	Reg.Register(s, "Get", welcomeMsgGet)
	Reg.Register(s, "Delete", welcomeMsgDelete)
}

func welcomeMsgCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		Req        requests.CreateWelcomeMsgReq `json:"req"`
		ExtStaffID string                       `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewWelcomeMsgService().Create(req.Req, corpID, req.ExtStaffID)
}

func welcomeMsgUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                      `json:"id"`
		Req requests.UpdateWelcomeMsgReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewWelcomeMsgService().Update(req.ID, req.Req, corpID)
}

func welcomeMsgQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryWelcomeMsgReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewWelcomeMsgService().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func welcomeMsgGet(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewWelcomeMsgService().Get(req.ID, corpID)
}

func welcomeMsgDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewWelcomeMsgService().Delete(req.IDs, corpID)
}

// 快捷回复（quick_reply）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "quick_reply"
	Reg.Register(s, "Create", quickReplyCreate)
	Reg.Register(s, "Update", quickReplyUpdate)
	Reg.Register(s, "Query", quickReplyQuery)
	Reg.Register(s, "Delete", quickReplyDelete)
}

func quickReplyCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		Req        entities.CreateQuickReplyReq `json:"req"`
		ExtStaffID string                       `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	staff := models.Staff{ExtCorpID: corpID, ExtID: req.ExtStaffID}
	return services.NewQuickReply().Create(req.Req, staff)
}

func quickReplyUpdate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		Req        entities.UpdateQuickReplyReq `json:"req"`
		ExtStaffID string                       `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	staff := models.Staff{ExtCorpID: corpID, ExtID: req.ExtStaffID}
	return services.NewQuickReply().Update(req.Req, staff)
}

func quickReplyQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryQuickReplyReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewQuickReply().QueryQuickReply(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func quickReplyDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewQuickReply().Delete(req.IDs, corpID)
}

// 快捷回复分组（quick_reply_group）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "quick_reply_group"
	Reg.Register(s, "Create", quickReplyGroupCreate)
	Reg.Register(s, "Update", quickReplyGroupUpdate)
	Reg.Register(s, "Delete", quickReplyGroupDelete)
	Reg.Register(s, "Query", quickReplyGroupQuery)
	Reg.Register(s, "QueryQuickReply", quickReplyGroupQueryQuickReply)
}

func quickReplyGroupQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[struct{}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewQuickReplyGroup().Query(corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func quickReplyGroupQueryQuickReply(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryQuickReplyReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewQuickReplyGroup().QueryQuickReply(p.Query, corpID, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func quickReplyGroupCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		Req        entities.CreateQuickReplyGroupReq `json:"req"`
		ExtStaffID string                            `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewQuickReplyGroup().Create(req.Req, corpID, req.ExtStaffID)
}

func quickReplyGroupUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.UpdateQuickReplyGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewQuickReplyGroup().Update(req, corpID)
}

func quickReplyGroupDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewQuickReplyGroup().Delete(req.IDs, corpID)
}

// 快捷回复前端（quick_reply_frontend）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "quick_reply_frontend"
	Reg.Register(s, "Query", quickReplyFrontendQuery)
}

func quickReplyFrontendQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryQuickReplyReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewQuickReplyFrontend().QueryQuickReply(p.Query, corpID, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// 快捷回复分组前端（quick_reply_group_frontend）域适配器 —— 通用 JSON 透传分发。
func init() {
	s := "quick_reply_group_frontend"
	Reg.Register(s, "Update", quickReplyGroupFrontendUpdate)
	Reg.Register(s, "Delete", quickReplyGroupFrontendDelete)
}

func quickReplyGroupFrontendUpdate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		Req        entities.UpdateQuickReplyGroupReq `json:"req"`
		ExtStaffID string                            `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewQuickReplyGroupFrontend().Update(req.Req, corpID, req.ExtStaffID)
}

func quickReplyGroupFrontendDelete(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		IDs        []string `json:"ids"`
		ExtStaffID string   `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewQuickReplyGroupFrontend().Delete(req.IDs, corpID, req.ExtStaffID)
}
