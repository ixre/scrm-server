package dispatch

import (
	"context"
	"encoding/json"

	"core/app/entities"
	"core/app/models"
	"core/app/requests"
	"core/app/services"
)

// 标签/素材（tag）业务域适配器 —— 通用 JSON 透传分发。
//
// 覆盖服务：tag（标签）、tag_group（标签组）、material_lib（素材库）、
// material_lib_tag（素材标签）、material_lib_tag_frontend（素材标签前端）、temp_material（临时素材）。
func init() {
	// tag
	Reg.Register("tag", "Create", tagCreate)
	Reg.Register("tag", "Sync", tagSync)
	Reg.Register("tag", "DeleteTagGroups", tagDeleteTagGroups)
	// tag_group
	Reg.Register("tag_group", "Query", tagGroupQuery)
	Reg.Register("tag_group", "Create", tagGroupCreate)
	Reg.Register("tag_group", "Update", tagGroupUpdate)
	Reg.Register("tag_group", "Delete", tagGroupDelete)
	Reg.Register("tag_group", "ExchangeOrder", tagGroupExchangeOrder)
	// material_lib
	Reg.Register("material_lib", "Query", materialQuery)
	Reg.Register("material_lib", "Create", materialCreate)
	Reg.Register("material_lib", "Update", materialUpdate)
	Reg.Register("material_lib", "Delete", materialDelete)
	Reg.Register("material_lib", "GetSidebarStatus", materialGetSidebarStatus)
	Reg.Register("material_lib", "UpdateGetSidebarStatus", materialUpdateGetSidebarStatus)
	// material_lib_tag
	Reg.Register("material_lib_tag", "Query", materialLibTagQuery)
	Reg.Register("material_lib_tag", "Create", materialLibTagCreate)
	Reg.Register("material_lib_tag", "Delete", materialLibTagDelete)
	// material_lib_tag_frontend
	Reg.Register("material_lib_tag_frontend", "Query", materialLibTagFrontendQuery)
	// temp_material
	Reg.Register("temp_material", "UploadMedia", tempMaterialUploadMedia)
}

// -------- tag（标签） --------

func tagCreate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.CreateTagReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTag().Create(&req, corpID)
}

func tagSync(ctx context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	return nil, services.NewTag().Sync(corpID)
}

func tagDeleteTagGroups(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.DeleteTagGroupsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTag().DeleteTagGroups(req)
}

// -------- tag_group（标签组） --------

func tagGroupQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.TagListReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	MergePS(&p)
	items, total, err := services.NewTagGroupService().Query(p.Query, corpID)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func tagGroupCreate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.CreateTagGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTagGroupService().Create(req, corpID)
}

func tagGroupUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateTagGroupReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTagGroupService().Update(req, corpID)
}

func tagGroupDelete(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.DeleteTagGroupsReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTagGroupService().Delete(req, corpID)
}

func tagGroupExchangeOrder(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req requests.ExchangeOrderReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewTagGroupService().ExchangeOrder(&req)
}

// -------- material_lib（素材库） --------

func materialQuery(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var p QueryParams[requests.QueryMaterialReq]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	MergePS(&p)
	ctx = corpCtx(ctx, corpID)
	items, total, err := services.NewMaterial().Query(ctx, p.Query, corpID)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func materialCreate(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req requests.UploadMaterialReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewMaterial().Create(ctx, req, corpID, staffID)
}

func materialUpdate(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ID  string                     `json:"id"`
		Req requests.UpdateMaterialReq `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return nil, services.NewMaterial().Update(ctx, req.ID, req.Req, corpID)
}

func materialDelete(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return services.NewMaterial().Delete(ctx, req.IDs, corpID)
}

func materialGetSidebarStatus(ctx context.Context, corpID, _ string, _ json.RawMessage) (any, error) {
	ctx = corpCtx(ctx, corpID)
	return services.NewMaterial().GetSidebarStatus(ctx, corpID)
}

func materialUpdateGetSidebarStatus(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req requests.UpdateGetSidebarStatusReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	ctx = corpCtx(ctx, corpID)
	return nil, services.NewMaterial().UpdateGetSidebarStatus(ctx, req, corpID)
}

// -------- material_lib_tag（素材标签） --------

func materialLibTagQuery(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var p QueryParams[struct {
		Name string `json:"name"`
	}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	creator := models.Staff{ExtCorpID: corpID, ExtID: staffID}
	items, total, err := services.NewMaterialLibTag().Query(p.Query.Name, creator, p.Pager, p.Sorter)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func materialLibTagCreate(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var req struct {
		Names []string `json:"names"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	creator := models.Staff{ExtCorpID: corpID, ExtID: staffID}
	return services.NewMaterialLibTag().Create(req.Names, creator)
}

func materialLibTagDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewMaterialLibTag().Delete(req.IDs)
}

// -------- material_lib_tag_frontend（素材标签前端） --------

func materialLibTagFrontendQuery(ctx context.Context, corpID, staffID string, params json.RawMessage) (any, error) {
	var p QueryParams[struct {
		Name string `json:"name"`
	}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	creator := models.Staff{ExtCorpID: corpID, ExtID: staffID}
	items, total, err := services.NewMaterialLibTagFrontend().Query(p.Query.Name, creator, p.Pager, p.Sorter)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// -------- temp_material（临时素材） --------

func tempMaterialUploadMedia(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
	var req entities.UploadMaterialReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTempMaterial().UploadMedia(req, corpID)
}
