package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"core/app/dto"
	"core/app/services"
	"core/common/app"
	"core/common/log"
)

type MsgArch struct {
	Base
	srv *services.MsgArch
}

func NewMsgArch() *MsgArch {
	return &MsgArch{srv: services.NewMsgArch()}
}

// QuerySessions
//@tags 会话存档
//@Summary 查询会话列表
//@Produce  json
//@Accept json
//@Param params body dto.QueryChatMsgReq true "查询会话列表请求"
//@Success 200 {object} app.JSONResult{} "成功"
//@Failure 400 {object} app.JSONResult{} "非法请求"
//@Failure 500 {object} app.JSONResult{} "内部错误"
//@Router /api/v1/console/customers/chat-sessions [get]
func (o MsgArch) QuerySessions(c *gin.Context) {
	req := dto.QuerySessionReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		handler.ResponseError(err)
		return
	}
	items, err := o.srv.QuerySessions(req, consoleStaff.ExtCorpID)
	if err != nil {
		log.TracedError("QuerySessions failed", err)
		handler.ResponseError(err)
		return
	}

	handler.ResponseItem(items)
}

// Sync
// @tags 会话存档
// @Summary 同步会话数据
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/chat-msg/sync [post]
func (o MsgArch) Sync(c *gin.Context) {
	handler := app.NewHandler(c)
	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}
	err = o.srv.Sync(consoleStaff.ExtCorpID)
	if err != nil {
		log.TracedError("QuerySessions failed", err)
		handler.ResponseError(err)
		return
	}

	handler.ResponseItem(nil)
}

// QuerySessionMsgs
//@tags 会话存档
//@Summary 会话中的消息列表
//@Produce  json
//@Accept json
//@Param params body dto.QueryChatMsgReq true "查询会话中的消息列表请求"
//@Success 200 {object} app.JSONResult{} "成功"
//@Failure 400 {object} app.JSONResult{} "非法请求"
//@Failure 500 {object} app.JSONResult{} "内部错误"
//@Router /api/v1/console/customer/sessions-msgs [get]
func (o MsgArch) QuerySessionMsgs(c *gin.Context) {
	req := dto.QueryChatMsgReq{}
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
	res, err := o.srv.QueryMsgs(req, consoleStaff.ExtCorpID)
	if err != nil {
		log.TracedError("QuerySessions failed", err)
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(res)
}

// SearchMsgs
//@tags 会话存档
//@Summary 搜索消息
//@Produce  json
//@Accept json
//@Param params body dto.QueryChatMsgReq true "搜索消息请求"
//@Success 200 {object} app.JSONResult{} "成功"
//@Failure 400 {object} app.JSONResult{} "非法请求"
//@Failure 500 {object} app.JSONResult{} "内部错误"
//@Router /api/v1/console/customer/sessions-msg/action/search [post]
func (o MsgArch) SearchMsgs(c *gin.Context) {
	req := dto.SearchMsgReq{}
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
	res, err := o.srv.SearchMsgs(req, consoleStaff.ExtCorpID)
	if err != nil {
		log.TracedError("QuerySessions failed", err)
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(res)
}
