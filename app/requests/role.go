package requests

// AssignToStaffsReq 授权角色给员工请求
type AssignToStaffsReq struct {
	// ExtStaffIDs 外部员工ID
	ExtStaffIDs []string `json:"ext_staff_ids" validate:"gt=0,dive,alphanum"`
	// RoleID 角色ID
	RoleID string `json:"role_id" validate:"required,int64"`
}
