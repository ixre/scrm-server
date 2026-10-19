package requests

// DeleteContactWayGroupReq 删除渠道码分组请求参数
type DeleteContactWayGroupReq struct {
	// 渠道码分组ID
	IDs []string `json:"ids" validate:"gt=0,dive,int64"`
}
