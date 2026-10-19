package requests

type ConsoleLoginReq struct {
	// ExtCorpID 外部企业ID
	ExtCorpID string `form:"ext_corp_id" json:"ext_corp_id" validate:"omitempty,corp_id"`
	// SourceURL 登录来源页面URL，登录成功自动跳转过去
	SourceURL string `form:"source_url" json:"source_url" validate:"required"`
}

type ConsoleLoginCallbackReq struct {
	// AppID 外部企业ID
	AppID string `form:"appid" json:"appid" validate:"required,corp_id"`
	// Code
	Code string `form:"code" json:"Code" validate:"required"`
	// State
	State string `form:"state" json:"state"`
}

type ConsoleForceLoginReq struct {
	// ExtCorpID 外部企业ID
	ExtCorpID string `form:"ext_corp_id" json:"ext_corp_id" validate:"omitempty,corp_id"`
	// ExtStaffID 外部员工ID
	ExtStaffID string `form:"ext_staff_id" json:"ext_staff_id"`
}

type StaffForceLoginReq struct {
	// ExtCorpID 外部企业ID
	ExtCorpID string `form:"ext_corp_id" json:"ext_corp_id" validate:"omitempty,corp_id"`
	// ExtStaffID 外部员工ID
	ExtStaffID string `form:"ext_staff_id" json:"ext_staff_id" validate:"required"`
}

type StaffLoginReq struct {
	// ExtCorpID 外部企业ID
	ExtCorpID string `form:"ext_corp_id" json:"ext_corp_id" validate:"omitempty,corp_id"`
	// SourceURL 登录来源页面URL，登录成功自动跳转过去
	SourceURL string `form:"source_url" json:"source_url" validate:"required"`
}

type StaffLoginCallbackReq struct {
	// AppID 外部企业ID
	AppID string `form:"appid" json:"appid" validate:"required,corp_id"`
	// Code
	Code string `form:"code" json:"Code" validate:"required"`
	// SourceURL 登录来源页面URL，登录成功自动跳转过去
	SourceURL string `form:"source_url" json:"source_url" validate:"required,url"`
	// State
	State string `form:"state" json:"state"`
}

type CustomerLoginReq struct {
	// ExtCorpID 外部企业ID
	ExtCorpID string `form:"ext_corp_id" json:"ext_corp_id" validate:"omitempty,corp_id"`
	// SourceURL 登录来源页面URL，登录成功自动跳转过去
	SourceURL string `form:"source_url" json:"source_url" validate:"required"`
}

type CustomerLoginCallbackReq struct {
	AppID string `form:"appid" json:"appid" validate:"required,corp_id"`
	// Code
	Code string `form:"code" json:"Code" validate:"required"`
	// SourceURL 登录来源页面URL，登录成功自动跳转过去
	SourceURL string `form:"source_url" json:"source_url" validate:"required,url"`
	// State
	State string `form:"state" json:"state"`
}
