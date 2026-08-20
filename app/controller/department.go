package controller

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"core/app/entities"
	"core/app/requests"
	"core/app/services"
	"core/common/app"
	"core/common/log"
)

type Department struct {
	Base
	srv *services.Department
}

func NewDepartment() *Department {
	return &Department{srv: services.NewDepartment()}
}

// Query
// @tags 部门管理
// @Summary 查询部门信息
// @Produce json
// @Accept json
// @Param params body entities.GetSubDepartmentReq true "部门id列表"
// @Success 200 {object} app.JSONResult{data=app.ItemsData{items=[]models.Department}} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/departments [get]
func (d *Department) Query(c *gin.Context) {
	handler := app.NewHandler(c)
	req := entities.GetSubDepartmentReq{}
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := d.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}
	departments, total, err := d.srv.Query(context.Background(), req, consoleStaff.ExtCorpID)
	if err != nil {
		err = errors.Wrap(err, "Query departments failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItems(departments, total)
}

// Sync
// @tags 部门管理
// @Summary 同步企微部门数据
// @Success 200 {object} app.JSONResult{} "成功"
// @Failure 400 {object} app.JSONResult{} "请求错误"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/department [post]
func (d *Department) Sync(c *gin.Context) {
	handler := app.NewHandler(c)
	consoleStaffInfo, err := d.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}
	err = d.srv.Sync(context.Background(), consoleStaffInfo.ExtCorpID)
	if err != nil {
		err = errors.Wrap(err, "sync departments failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(nil)
}

// Get
// @tags 部门管理
// @Summary 获取根部门信息
// @Produce json
// @Accept json
// @Param params body requests.QueryDepartmentReq true "部门信息请求"
// @Success 200 {object} app.JSONResult{data=app.ItemsData{items=[]models.Department}} "成功"
// @Failure 400 {object} app.JSONResult{} "非法请求"
// @Failure 500 {object} app.JSONResult{} "内部错误"
// @Router /api/v1/console/department [get]
func (d *Department) Get(c *gin.Context) {
	handler := app.NewHandler(c)
	req := requests.QueryDepartmentReq{}
	ok, err := handler.BindAndValidateReq(&req)
	if !ok {
		handler.ResponseBadRequestError(errors.WithStack(err))
		return
	}

	consoleStaff, err := d.GetConsoleInfo(handler)
	if err != nil {
		log.TracedError("GetConsoleInfo failed", err)
		return
	}
	departments, err := d.srv.Get(context.Background(), req.ExtID, consoleStaff.ExtCorpID)
	if err != nil {
		err = errors.Wrap(err, "get departments failed")
		handler.ResponseError(err)
		return
	}
	handler.ResponseItem(departments)
}
