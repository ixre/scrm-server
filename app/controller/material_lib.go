package controller

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"core/app/requests"
	"core/app/services"
	"core/common/app"
	"core/common/log"
)

type MaterialLib struct {
	Base
	srv *services.Material
}

func NewMaterialLib() *MaterialLib {
	return &MaterialLib{srv: services.NewMaterial()}
}

// Create
// @tags 素材库
// @Summary 创建素材库素材
// @Produce  json
// @Accept json
// @Param params body requests.UploadMaterialReq true "创建素材库素材请求"
// @Success 200 {object} app.JSONResult{data=models.Material} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/material/lib [post]
func (o MaterialLib) Create(c *gin.Context) {
	req := requests.UploadMaterialReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	item, err := o.srv.Create(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), req, consoleStaff.ExtCorpID, consoleStaff.ExtID)
	if err != nil {
		err = errors.Wrap(err, "Create failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(item)

}

// Delete
// @tags 素材库
// @Summary 删除素材库
// @Produce  json
// @Accept json
// @Param params body requests.CommonDeleteReq true "删除素材库"
// @Success 200 {object} app.JSONResult{data=bool} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/material/lib/action/delete [post]
func (o MaterialLib) Delete(c *gin.Context) {
	req := requests.CommonDeleteReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	total, err := o.srv.Delete(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), req.IDs,consoleStaff.ExtCorpID)

	if err != nil {
		err = errors.Wrap(err, "Delete failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(total)
}

// Update
// @tags 素材库
// @Summary 更新素材库
// @Produce  json
// @Accept json
// @Param id path string true "素材库ID"
// @Param params body requests.UpdateMaterialReq true "更新素材库请求"
// @Success 200 {object} app.JSONResult{data=models.Material} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/material/lib/{id} [put]
func (o MaterialLib) Update(c *gin.Context) {
	req := requests.UpdateMaterialReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	id, err := handler.GetIDParam()
	if err != nil {
		err = errors.Wrap(err, "handler.GetIDParam failed")
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	err = o.srv.Update(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), id, req, consoleStaff.ExtCorpID)
	if err != nil {
		err = errors.Wrap(err, "Update failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(nil)
}

// Query
// @tags 素材库
// @Summary 查询素材库列表
// @Produce  json
// @Accept json
// @Param params body requests.QueryMaterialReq true "查询素材库列表请求"
// @Success 200 {object} app.JSONResult{data=app.ItemsData{items=[]models.Material}} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/material/lib [get]
func (o MaterialLib) Query(c *gin.Context) {
	req := requests.QueryMaterialReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	items, total, err := o.srv.Query(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), req,consoleStaff.ExtCorpID)

	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItems(items, total)
}

//-------------------------侧边栏--------------------

// GetSidebarStatus
// @tags 素材库
// @Summary 获取素材库-侧边栏开关状态
// @Produce  json
// @Success 200 {object} app.JSONResult{data=constants.Boolean} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/material/lib/sidebar-status [get]
func (o MaterialLib) GetSidebarStatus(c *gin.Context) {
	handler := app.NewHandler(c)

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	status, err := o.srv.GetSidebarStatus(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), consoleStaff.ExtCorpID)

	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(status)
}

// UpdateSidebarStatus
// @tags 素材库
// @Summary 更新素材库-侧边栏开关状态
// @Produce  json
// @Accept json
// @Param params body requests.UpdateGetSidebarStatusReq true "更新素材库-侧边栏开关状态请求"
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/material/lib/sidebar-status [put]
func (o MaterialLib) UpdateSidebarStatus(c *gin.Context) {
	req := requests.UpdateGetSidebarStatusReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	err = o.srv.UpdateGetSidebarStatus(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), req, consoleStaff.ExtCorpID)
	if err != nil {
		err = errors.Wrap(err, "Update failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(nil)
}
