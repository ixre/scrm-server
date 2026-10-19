package requests

// DeleteContactWayReq 删除渠道码请求参数
type DeleteContactWayReq struct {
	// 渠道码ID
	IDs []string `json:"ids" validate:"gt=0,dive,int64"`
}
