package requests

import "core/common/app"

type CommonDeleteReq struct {
	IDs []string `json:"ids" form:"ids" validate:"required,gt=0"`
}

type QueryCustomerGroupTagGroupReq struct {
	Name string `json:"name" form:"name" validate:"omitempty"`
	app.Sorter
	app.Pager
}
