package dto

import (
	"core/app/constants"
	"core/common/app"
)

// CreateContactWayGroupReq 创建渠道码分组请求参数
type CreateContactWayGroupReq struct {
	// Name 分组名称
	Name string `json:"name" validate:"required"`
	// SortWeight 分组排序
	SortWeight int `json:"sort_weight" validate:"gte=0"`
}

// UpdateContactWayGroupReq 更新渠道码分组请求参数
type UpdateContactWayGroupReq struct {
	// Name 分组名称
	Name string `json:"name"`
	// SortWeight 分组排序
	SortWeight int `json:"sort_weight" validate:"gte=0"`
}

// QueryContactWayGroupReq 查询渠道码分组列表请求参数
type QueryContactWayGroupReq struct {
	app.Pager
	app.Sorter
	// ID 渠道码分组ID
	ID string `form:"id" json:"id" validate:"omitempty,int64" `
	// Name 渠道码分组名称
	Name string `form:"name" json:"name"`
	// IsDefault 是否为默认分组
	IsDefault constants.Boolean `form:"is_default" json:"is_default"  validate:"omitempty,oneof=1 2" `
}
