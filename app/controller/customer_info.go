package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"core/app/entities"
	"core/app/services"
	"core/common/app"
	"core/common/log"
)

type CustomerInfoHandler struct {
	Base
	customerInfoService *services.CustomerInfo
}

func NewCustomerInfoHandler() *CustomerInfoHandler {
	return &CustomerInfoHandler{customerInfoService: services.NewCustomerInfo()}
}

// Update
// @tags 客户管理
// @Summary 更新客户基本信息
// @Produce  json
// @Param params body entities.UpdateCustomerInfoReq true "客户基本信息更新请求"
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer/info [put]
func (ch CustomerInfoHandler) Update(c *gin.Context) {
	req := entities.UpdateCustomerInfoReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		log.Sugar.Debugw("BadRequest", "err", err)
		handler.ResponseBadRequestError(err)
		return
	}

	consoleStaff, err := ch.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	err = ch.customerInfoService.Update(&req, consoleStaff.ExtCorpID)
	if err != nil {
		log.Sugar.Errorf("ch.ch.Upsert errs: %v", err)
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(nil)
}

// Get
// @tags 客户管理
// @Summary 获取客户的基本信息
// @Produce  json
// @Param params body entities.GetCustomerInfoReq true "获取客户的基本信息请求"
// @Success 200 {object} app.JSONResult{}"成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer/info [get]
func (ch CustomerInfoHandler) Get(c *gin.Context) {
	req := entities.GetCustomerInfoReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		log.Sugar.Debugw("BadRequest", "err", err)
		handler.ResponseBadRequestError(err)
		return
	}

	consoleStaff, err := ch.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	info, err := ch.customerInfoService.Get(req, consoleStaff.ExtCorpID)
	if err != nil {
		err = errors.Wrap(err, "Upsert failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(info)
}
