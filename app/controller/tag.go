package controller

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"core/app/dto"
	"core/app/services"
	"core/common/app"
	"core/common/log"
)

type Tag struct {
	Base
	tagSvc *services.TagService
}

func NewTag(tagSvc *services.TagService) *Tag {
	return &Tag{tagSvc: tagSvc}
}

// Sync
// @tags 客户标签
// @Summary 同步企微客户标签
// @Produce json
// @Success 200 {object} app.JSONResult{data=models.TagGroupSwagger} "成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer/action/sync [post]
func (t Tag) Sync(c *gin.Context) {
	handler := app.NewHandler(c)
	staffInfo, err := t.GetConsoleInfo(handler)
	if err != nil {
		handler.ResponseBadRequestError(err)
		return
	}

	err = t.tagSvc.Sync(staffInfo.ExtCorpID)
	if err != nil {
		log.Sugar.Error("svc.Syncs err: ", err)
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(nil)
}

// Create
// @tags 客户标签
// @Param params body dto.CreateTagReq true "添加标签"
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/customer/tag [post]
func (t Tag) Create(c *gin.Context) {
	req := &dto.CreateTagReq{}
	handler := app.NewHandler(c)
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}
	adminInfo, err := t.GetConsoleInfo(handler)
	if err != nil {
		handler.ResponseError(err)
		return
	}
	tags, err := t.tagSvc.Create(req, adminInfo.ExtCorpID)
	if err != nil {
		handler.ResponseError(err)
		return
	}

	handler.ResponseItem(tags)
}
