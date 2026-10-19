package dto

import (
	"core/app/constants"
	"core/common/app"
)

// CreateRoleReq 创建角色请求参数
type CreateRoleReq struct {
	// Name 角色名称
	Name string `json:"name" validate:"required"`
	// Description 角色描述
	Description string `json:"description"`
	// SortWeight 角色排序权重
	SortWeight int `json:"sort_weight" validate:"gte=0"`
	// PermissionIDs 角色绑定的权限标识数组
	PermissionIDs constants.StringArrayField `json:"permission_ids" validate:"dive,word"`
}

// UpdateRoleReq 更新角色请求参数
type UpdateRoleReq struct {
	// Name 角色名称
	Name string `json:"name"`
	// Description 角色描述
	Description string `json:"description"`
	// SortWeight 角色排序权重
	SortWeight int `json:"sort_weight" validate:"gte=0"`
	// PermissionIDs 角色绑定的权限标识数组
	PermissionIDs constants.StringArrayField `json:"permission_ids" validate:"dive,word"`
}

// QueryRoleReq 查询角色列表请求参数
type QueryRoleReq struct {
	app.Pager
	app.Sorter
	// ID 角色ID
	ID string `form:"id" json:"id" validate:"omitempty,int64" `
	// Name 角色名称
	Name string `form:"name" json:"name"`
	// Type 角色类型
	Type string `json:"type" validate:"omitempty,oneof=Admin DepartmentAdmin Staff"`
	// IsDefault 是否为默认角色
	IsDefault constants.Boolean `form:"is_default" json:"is_default"  validate:"omitempty,oneof=1 2" `
}

// QueryRoleStaffsReq 查询授权员工列表请求参数
type QueryRoleStaffsReq struct {
	app.Pager
	app.Sorter
	// ID 员工ID
	StaffID string `form:"staff_id" json:"id" validate:"omitempty,int64"`
	// ExtStaffID 员工外部ID
	ExtStaffID string `form:"ext_staff_id"  json:"ext_staff_id" validate:"omitempty"`
	// Name 员工名称
	Name string `form:"name" json:"name"`
	// RoleID 角色ID
	RoleID string `form:"role_id" json:"role_id" validate:"omitempty,int64"`
	// RoleType 角色类型
	RoleType string `form:"role_type" json:"type" validate:"omitempty,oneof=Admin DepartmentAdmin Staff"`
}
