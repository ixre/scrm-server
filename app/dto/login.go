package dto

// ConsoleLoginResp 企业员工后台扫码登录响应
type ConsoleLoginResp struct {
	// AppID 外部企业ID
	AppID string `json:"app_id"`
	// AgentID 授权应用AgentID
	AgentID int64 `json:"agent_id"`
	// RedirectURI 重定向地址，需要进行UrlEncode，授权回调地址
	RedirectURI string `json:"redirect_uri"`
	// State
	State string `json:"state"`
	// LocationURL 完整的跳转URL
	LocationURL string `json:"location_url"`
}

// StaffLoginResp 企业员工H5登录响应
type StaffLoginResp struct {
	// AppID 外部企业ID
	AppID string `json:"app_id"`
	// RedirectURI 重定向地址，需要进行UrlEncode，授权回调地址
	RedirectURI string `json:"redirect_uri"`
	// SourceURL 登录来源页面URL，登录成功自动跳转过去
	SourceURL string `form:"source_url" json:"source_url" validate:"required,url"`
	// State
	State string `json:"state"`
	// LocationURL 完整的跳转URL
	LocationURL string `json:"location_url"`
}

// CustomerLoginResp 客户H5登录响应
type CustomerLoginResp struct {
	// AppID 外部企业ID
	AppID string `json:"app_id"`
	// RedirectURI 重定向地址，需要进行UrlEncode，授权回调地址
	RedirectURI string `json:"redirect_uri"`
	// SourceURL 登录来源页面URL，登录成功自动跳转过去
	SourceURL string `form:"source_url" json:"source_url" validate:"required,url"`
	// State
	State string `json:"state"`
	// LocationURL 完整的跳转URL
	LocationURL string `json:"location_url"`
}
