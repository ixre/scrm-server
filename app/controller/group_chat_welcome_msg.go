package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"core/app/dto"
	"core/app/requests"
	"core/app/services"
	"core/common/app"
	"core/common/log"
)

type GroupChatWelcomeMsg struct {
	Base
	srv *services.GroupChatWelcomeMsg
}

func NewGroupChatWelcomeMsg() *GroupChatWelcomeMsg {
	return &GroupChatWelcomeMsg{srv: services.NewGroupChatWelcomeMsg()}
}

// Query
// @tags 客户群管理
// @Summary 查询入群欢迎语
// @Produce  json
// @Accept json
// @Param params body dto.QueryGroupChatWelcomeMsgReq true "查询入群欢迎语请求"
// @Success 200 {object} app.JSONResult{data=app.ItemsData{items=[]models.GroupChatWelcomeMsg}} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer-group/welcome-msgs [get]
func (o *GroupChatWelcomeMsg) Query(c *gin.Context) {
	req := dto.QueryGroupChatWelcomeMsgReq{}
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

	items, total, err := o.srv.Query(req, consoleStaff)
	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItems(items, total)
}

// Create
// @tags 客户群管理
// @Summary 创建入群欢迎语
// @produce  json
// @accept json
// @param params body dto.CreateGroupChatWelcomeMsgReq true "创建入群欢迎语请求"
// @success 200 {object} app.JSONResult{data=models.GroupChatWelcomeMsg} "成功"
// @failure 400 {object} app.JSONResult{} "非法请求"
// @failure 500 {object} app.JSONResult{} "内部错误"
// @router /api/v1/console/customer-group/welcome-msg [post]
func (o *GroupChatWelcomeMsg) Create(c *gin.Context) {
	req := dto.CreateGroupChatWelcomeMsgReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("get staff admin info failed", err)
		return
	}

	item, err := o.srv.Create(req, consoleStaff)
	if err != nil {
		err = errors.Wrap(err, "create failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(item)
}

// Update
// @tags 客户群管理
// @Summary 更新入群欢迎语
// @produce  json
// @accept json
// @param id path string true "更新入群欢迎语id"
// @param params body dto.UpdateGroupChatWelcomeMsgReq true "更新入群欢迎语请求"
// @success 200 {object} app.JSONResult{data=models.GroupChatWelcomeMsg} "成功"
// @failure 400 {object} app.JSONResult{} "非法请求"
// @failure 500 {object} app.JSONResult{} "内部错误"
// @router /api/v1/console/customer-group/welcome-msg/{id} [put]
func (o *GroupChatWelcomeMsg) Update(c *gin.Context) {
	req := dto.UpdateGroupChatWelcomeMsgReq{}
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

	item, err := o.srv.Update(req, id, consoleStaff)
	if err != nil {
		err = errors.Wrap(err, "update failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(item)
}

// Delete
// @tags 客户群管理
// @Summary 删除入群欢迎语
// @produce  json
// @accept json
// @param params body requests.CommonDeleteReq true "删除入群欢迎语请求"
// @success 200 {object} app.JSONResult{} "成功"
// @failure 400 {object} app.JSONResult{} "非法请求"
// @failure 500 {object} app.JSONResult{} "内部错误"
// @router /api/v1/console/customer-group/welcome-msg/action/delete [post]
func (o *GroupChatWelcomeMsg) Delete(c *gin.Context) {
	req := requests.CommonDeleteReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	_, err = o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	total, err := o.srv.Delete(req.IDs)
	if err != nil {
		err = errors.Wrap(err, "delete failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(total)
}
