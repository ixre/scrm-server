package requests

import "core/common/app"

// QueryQuickReplyGroupReq 查询话术分组
// 企业话术无需查询条件
type QueryQuickReplyGroupReq struct {
	app.Pager
	app.Sorter
}

type DeleteQuickReplyGroupReq struct {
	// 话术组ID
	IDs []string `json:"ids" validate:"gt=0,dive,int64"`
}
