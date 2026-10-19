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

type GroupChat struct {
	Base
	srv *services.GroupChatService
}

func NewGroupChat() *GroupChat {
	return &GroupChat{srv: services.NewGroupChatService()}
}

// Query
// @tags 客户群
// @Summary 客户群列表
// @Produce  json
// @Accept json
// @Param params body dto.QueryGroupChatReq true "查询客户群列表请求"
// @Success 200 {object} app.JSONResult{data=app.ItemsData{items=[]models.GroupChat}} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer-group [get]
func (o GroupChat) Query(c *gin.Context) {
	req := dto.QueryGroupChatReq{}
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

	items, total, err := o.srv.Query(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), req, consoleStaff.ExtCorpID, &req.Pager, &req.Sorter)
	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}

	handler.ResponseItems(items, total)
}

// UpdateTags
// @tags 客户群
// @Summary 客户群打标签
// @Produce  json
// @Accept json
// @Param params body dto.UpdateCustomerGroupReq true "客户群打标签请求"
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer-group/action/update-tags [post]
func (o GroupChat) UpdateTags(c *gin.Context) {
	req := dto.UpdateCustomerGroupReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	err = o.srv.UpdateTags(context.Background(), req)
	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(nil)
}

// Export
// @tags 客户群
// @Summary 查询客户群列表导出
// @Produce  json
// @Accept json
// @Param params body dto.QueryGroupChatReq true "查询客户群列表请求"
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer-groups/action/export [get]
func (o GroupChat) Export(c *gin.Context) {
	req := dto.QueryGroupChatReq{}
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

	buf, filename, err := o.srv.Export(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), req, consoleStaff.ExtCorpID)

	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseFile(buf, filename)
}

// GetAllOwners
// @tags 客户群
// @Summary 群主列表
// @Produce json
// @Accept json
// @Success 200 {object} app.JSONResult{data=[]models.StaffMainInfo} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer-group/tags [put]
func (o GroupChat) GetAllOwners(c *gin.Context) {
	handler := app.NewHandler(c)

	consoleStaff, err := o.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}

	items, err := o.srv.GetAllOwners(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), consoleStaff.ExtCorpID)

	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(items)
}

// GetAll
// @tags 客户群
// @Summary 全部列表
// @Produce json
// @Accept json
// @Param params body requests.GetAllGroupChatReq true "查询客户群列表请求"
// @Success 200 {object} app.JSONResult{data=[]models.StaffMainInfo} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer-group/action/get-all [post]
func (o GroupChat) GetAll(c *gin.Context) {
	req := dto.QueryGroupChatReq{}
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

	items, total, err := o.srv.GetAll(services.SyncTenantCtx(context.Background(), consoleStaff.ExtCorpID), req, consoleStaff.ExtCorpID)

	if err != nil {
		err = errors.Wrap(err, "Query failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItems(items, total)
}
