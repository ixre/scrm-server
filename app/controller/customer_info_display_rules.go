package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"core/app/dto"
	"core/app/services"
	"core/common/app"
	"core/common/log"
)

type CustomerInfoDisplayRulesHandler struct {
	Base
	srv *services.CustomerInfoDisplayRules
}

func NewCustomerInfoDisplayRules() *CustomerInfoDisplayRulesHandler {
	return &CustomerInfoDisplayRulesHandler{srv: services.NewCustomerInfoDisplayRules()}
}

// Get
// @tags  客户管理
// @Summary 获取客户信息展示规则
// @Produce json
// @Success 200 {object} app.JSONResult{data=models.CustomerInfoDisplayRule} "成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer/info/displays [get]
func (ch CustomerInfoDisplayRulesHandler) Get(c *gin.Context) {
	handler := app.NewHandler(c)
	adminInfo, err := ch.GetConsoleInfo(handler)
	if err != nil {
		handler.ResponseError(err)
		return
	}
	displayRule, err := ch.srv.Get(adminInfo.ExtCorpID)
	if err != nil {
		err = errors.Wrap(err, "GetCustomerInfoDisplay failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(displayRule)
	return
}

// Update
// @tags  客户管理
// @Summary 更新客户基本信息展示规则
// @Param params body dto.UpdateCustomerInfoDisplayRulesReq true "更新客户基本信息展示规则请求"
// @Produce json
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer/info/displays [put]
func (ch CustomerInfoDisplayRulesHandler) Update(c *gin.Context) {
	var req dto.UpdateCustomerInfoDisplayRulesReq
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}
	consoleStaffInfo, err := ch.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}
	err = ch.srv.Update(consoleStaffInfo.ExtCorpID, &req)
	if err != nil {
		log.Sugar.Errorf("app.GetCustomerInfoDisplay errs: %v", err)
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(nil)
	return
}
