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
	Reg.Register("group_chat", "GetAllOwners", groupChatGetAllOwners)
	Reg.Register("group_chat", "GetAll", groupChatGetAll)

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

func groupChatQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryGroupChatReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewGroupChatService().Query(ctx, p.Query, corpID, p.Pager, p.Sorter)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatUpdateTags(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateCustomerGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewGroupChatService().UpdateTags(ctx, req)
}

func groupChatGetAllOwners(ctx context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return services.NewGroupChatService().GetAllOwners(ctx, corpID)
}

func groupChatGetAll(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryGroupChatReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	MergePS(&p)
	items, total, err := services.NewGroupChatService().GetAll(ctx, p.Query, corpID)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatGroupQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
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

func groupChatGroupCreate(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req entities.CreateGroupChatGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatGroupService().Create(req, corpID, staffID)
}

func groupChatGroupUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                           `json:"id"`
		Req entities.UpdateGroupChatGroupReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatGroupService().Update(req.ID, req.Req, corpID)
}

func groupChatGroupDelete(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.DeleteGroupChatGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatGroupService().Delete(req, corpID)
}

func groupChatTagCreate(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateGroupChatTagsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerGroupTag().Create(req, corpID, staffID)
}

func groupChatTagUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateGroupChatTagReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerGroupTag().Update(req, corpID)
}

func groupChatTagDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCustomerGroupTag().Delete(req.IDs)
}

func groupChatTagGroupQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
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

func groupChatTagGroupCreate(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.CreateGroupChatTagGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatTagGroup().Create(&req, corpID, staffID)
}

func groupChatTagGroupUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateGroupChatTagGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatTagGroup().Update(&req, corpID)
}

func groupChatTagGroupDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatTagGroup().Delete(req.IDs)
}

func groupChatAutoJoinDelete(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatAutoJoin().Delete(req.IDs, corpID)
}

func groupChatAutoJoinBatchRegroup(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.BatchRegroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewGroupChatAutoJoin().BatchRegroup(req, corpID)
}

func groupChatAutoJoinCreate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.CreateGroupChatAutoJoinCodeReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatAutoJoin().Create(req, corpID)
}

func groupChatAutoJoinQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
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

func groupChatAutoJoinUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                                    `json:"id"`
		Req requests.UpdateGroupChatAutoJoinQrCodeReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatAutoJoin().Update(req.ID, req.Req, corpID)
}

func groupChatMassMsgQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
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

func groupChatMassMsgGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatMassMsg().Get(req.ID)
}

func groupChatMassMsgSendAsync(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.SendGroupChatMassMsgReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatMassMsg().SendMassMsgAsync(req, staffID, corpID)
}

func groupChatMassMsgDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewGroupChatMassMsg().DeleteTimedMassMsg(req.IDs)
}

func groupChatWelcomeMsgQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryGroupChatWelcomeMsgReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	MergePS(&p)
	staff := models.Staff{ExtCorpID: corpID}
	items, total, err := services.NewGroupChatWelcomeMsg().Query(p.Query, staff)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func groupChatWelcomeMsgCreate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
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

func groupChatWelcomeMsgUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
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

func groupChatWelcomeMsgDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewGroupChatWelcomeMsg().Delete(req.IDs)
}
