package dto

import "core/common/app"

type CreateInternalTagReq struct {
	Names []string `json:"names" form:"names" validate:"required"`
}

type QueryInternalTagReq struct {
	ExtStaffID string `json:"ext_staff_id" form:"ext_staff_id" validate:"required"`
	app.Sorter
	app.Pager
}
