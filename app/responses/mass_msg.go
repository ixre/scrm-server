package responses

import "core/app/models"

type GroupChatMassMsgDetail struct {
	models.GroupChatMassMsg
	Creator models.StaffMainInfo `json:"creator"`
}
