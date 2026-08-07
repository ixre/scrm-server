package responses

import (
	"core/app/models"
)

type ContactWayGroup struct {
	models.ContactWayGroup
	// 渠道码数量
	Count int64 `json:"count"`
}
