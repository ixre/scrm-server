package dto

import "core/common/app"

// QueryPermissionReq 查询权限列表请求参数
type QueryPermissionReq struct {
	app.Pager
	app.Sorter
	// ID 权限ID
	ID string `form:"id" json:"id" validate:"omitempty,int64"`
	// Name 权限名称
	Name string `json:"name"`
	// BizIdentity 业务标识
	BizIdentity string `json:"biz_identity" validate:"omitempty,alphanum"`
	// Operation 操作
	Operation string `json:"operation" validate:"omitempty,oneof=read full"`
	// Identity 权限标识
	Identity string `json:"identity" validate:"omitempty,alphanum,max=50"`
}
