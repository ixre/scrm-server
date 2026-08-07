package dispatch

import (
	"context"
	"encoding/json"

	"core/app/entities"
	"core/app/models"
	"core/app/requests"
	"core/app/services"
)

// 群聊（group_chat）域适配器 —— 通用 JSON 透传分发。
func init() {
	Reg.Register("group_chat", "Query", groupChatQuery)
	Reg.Register("group_chat", "UpdateTags", groupChatUpdateTags)

	Reg.Register("group_chat_group", "Query", groupChatGroupQuery)
	Reg.Register("group_chat_group", "Create", groupChatGroupCreate)
	Reg.Register("group_chat_group", "Update", groupChatGroupUpdate)
	Reg.Register("group_chat_group", "Delete", groupChatGroupDelete)

	Reg.Register("group_chat_tag", "Create", groupChatTagCreate)
	Reg.Register("group_chat_tag", "Update", groupChatTagUpdate)
	Reg.Register("group_chat_tag", "Delete", groupChatTagDelete)

	Reg.Register("group_chat_tag_group", "Query", groupChatTagGroupQuery)
	Reg.Register("group_chat_tag_group", "Create", groupChatTagGroupCreate)
	Reg.Register("group_chat_tag_group", "Update", groupChatTagGroupUpdate)
	Reg.Register("group_chat_tag_group", "Delete", groupChatTagGroupDelete)

	Reg.Register("group_chat_auto_join", "Create", groupChatAutoJoinCreate)
	Reg.Register("group_chat_auto_join", "Query", groupChatAutoJoinQuery)
	Reg.Register("group_chat_auto_join", "Update", groupChatAutoJoinUpdate)
	Reg.Register("group_chat_auto_join", "Delete", groupChatAutoJoinDelete)
	Reg.Register("group_chat_auto_join", "BatchRegroup", groupChatAutoJoinBatchRegroup)

	Reg.Register("group_chat_mass_msg", "Query", groupChatMassMsgQuery)
	Reg.Register("group_chat_mass_msg", "Get", groupChatMassMsgGet)
	Reg.Register("group_chat_mass_msg", "SendMassMsgAsync", groupChatMassMsgSendAsync)
	Reg.Register("group_chat_mass_msg", "Delete", groupChatMassMsgDelete)

	Reg.Register("group_chat_welcome_msg", "Query", groupChatWelcomeMsgQuery)
	Reg.Register("group_chat_welcome_msg", "Create", groupChatWelcomeMsgCreate)
	Reg.Register("group_chat_welcome_msg", "Update", groupChatWelcomeMsgUpdate)
	Reg.Register("group_chat_welcome_msg", "Delete", groupChatWelcomeMsgDelete)
}

func groupChatQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryGroupChatReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewGroupChatService().Query(p.Query, corpID, p.Pager, p.Sorter)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatUpdateTags(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateCustomerGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewGroupChatService().UpdateTags(req)
}

func groupChatGroupQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[entities.QueryGroupChatGroupReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewGroupChatGroupService().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatGroupCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req entities.CreateGroupChatGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatGroupService().Create(req, corpID, staffID)
}

func groupChatGroupUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                           `json:"id"`
		Req entities.UpdateGroupChatGroupReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatGroupService().Update(req.ID, req.Req, corpID)
}

func groupChatGroupDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.DeleteGroupChatGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatGroupService().Delete(req, corpID)
}

func groupChatTagCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateGroupChatTagsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerGroupTag().Create(req, corpID, staffID)
}

func groupChatTagUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateGroupChatTagReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerGroupTag().Update(req, corpID)
}

func groupChatTagDelete(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerGroupTag().Delete(req.IDs)
}

func groupChatTagGroupQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[string]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewGroupChatTagGroup().Query(p.Query, corpID, p.Pager, p.Sorter)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatTagGroupCreate(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateGroupChatTagGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatTagGroup().Create(&req, corpID, staffID)
}

func groupChatTagGroupUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateGroupChatTagGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatTagGroup().Update(&req, corpID)
}

func groupChatTagGroupDelete(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatTagGroup().Delete(req.IDs)
}

func groupChatAutoJoinDelete(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatAutoJoin().Delete(req.IDs, corpID)
}

func groupChatAutoJoinBatchRegroup(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.BatchRegroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewGroupChatAutoJoin().BatchRegroup(req, corpID)
}

func groupChatAutoJoinCreate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.CreateGroupChatAutoJoinCodeReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatAutoJoin().Create(req, corpID)
}

func groupChatAutoJoinQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryGroupChatAutoJoinReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewGroupChatAutoJoin().Query(p.Query, corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatAutoJoinUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                                   `json:"id"`
		Req requests.UpdateGroupChatAutoJoinQrCodeReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatAutoJoin().Update(req.ID, req.Req, corpID)
}

func groupChatMassMsgQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[struct{}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewGroupChatMassMsg().Query(corpID, p.Sorter, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatMassMsgGet(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatMassMsg().Get(req.ID)
}

func groupChatMassMsgSendAsync(_ context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.SendGroupChatMassMsgReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatMassMsg().SendMassMsgAsync(req, staffID, corpID)
}

func groupChatMassMsgDelete(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewGroupChatMassMsg().DeleteTimedMassMsg(req.IDs)
}

func groupChatWelcomeMsgQuery(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryGroupChatWelcomeMsgReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	staff := models.Staff{ExtCorpID: corpID}
	items, total, err := services.NewGroupChatWelcomeMsg().Query(p.Query, staff)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatWelcomeMsgCreate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		Req        requests.CreateGroupChatWelcomeMsgReq `json:"req"`
		ExtStaffID string                                `json:"ext_staff_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	staff := models.Staff{ExtCorpID: corpID, ExtID: req.ExtStaffID}
	return services.NewGroupChatWelcomeMsg().Create(req.Req, staff)
}

func groupChatWelcomeMsgUpdate(_ context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                                `json:"id"`
		Req requests.UpdateGroupChatWelcomeMsgReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	staff := models.Staff{ExtCorpID: corpID}
	return services.NewGroupChatWelcomeMsg().Update(req.Req, req.ID, staff)
}

func groupChatWelcomeMsgDelete(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatWelcomeMsg().Delete(req.IDs)
}
