package responses

import "core/app/models"

type MassMsgDetail struct {
	models.MassMsg
	Creator models.StaffMainInfo `json:"creator"`
}
