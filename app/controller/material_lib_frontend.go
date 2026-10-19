package controller

import (
	"context"
	"core/app/dto"
	"core/app/services"
	"core/common/app"
	"core/common/log"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type MaterialLibFrontend struct {
	Base
	srv *services.Material
}

func NewMaterialLibFrontend() *MaterialLibFrontend {
	return &MaterialLibFrontend{srv: services.NewMaterial()}
}

// Query
// @tags 素材库-H5
// @Summary 查询素材库列表
// @Produce  json
// @Accept json
// @Param params body dto.QueryMaterialReq true "查询素材库列表请求"
// @Success 200 {object} app.JSONResult{data=app.ItemsData{items=[]models.Material}} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/staff-frontend/material/lib [get]
func (o MaterialLibFrontend) Query(c *gin.Context) {
	req := dto.QueryMaterialReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	staffInfo, err := o.GetStaffInfo(handler)
	if err != nil {
		log.TracedError("GetStaffInfo failed", err)
		return
	}

	items, total, err := o.srv.Query(services.SyncTenantCtx(context.Background(), staffInfo.ExtCorpID), req, staffInfo.ExtCorpID)

	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItems(items, total)
}
