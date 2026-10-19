package dto

import "core/app/constants"

// UpdateCustomerDeleteStaffNotifierReq 流失提醒开关
type UpdateCustomerDeleteStaffNotifierReq struct {
	// 是否通知员工，员工被客户删除时提醒,1-打开 2-关闭
	IsNotifyStaff constants.EventNotifyStatus `json:"is_notify_staff" form:"is_notify_staff" validate:"oneof=1 2"`
}

// UpdateCustomerDeleteStaffNotifierResp 流失提醒开关
type UpdateCustomerDeleteStaffNotifierResp struct {
	// 是否通知员工，员工被客户删除时提醒,1-打开 2-关闭
	IsNotifyStaff constants.EventNotifyStatus `json:"is_notify_staff" form:"is_notify_staff"`
}
