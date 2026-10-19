package requests

import (
	"core/app/constants"
	"core/common/app"
)

// QueryMassMsgReq 查询群发消息列表请求参数
type QueryMassMsgReq struct {
	app.Pager
	app.Sorter
}

type MassMsgNotifyReq struct {
	//  群发消息ids
	IDs constants.StringArrayField `json:"ids" form:"ids" validate:"gt=0"`
}

type CountCustomer struct {
	// 筛选条件
	constants.ExtCustomerFilter
	// 是否有筛选条件
	ExtCustomerFilterEnable constants.Boolean `json:"ext_customer_filter_enable" form:"ext_customer_filter_enable"`
}
