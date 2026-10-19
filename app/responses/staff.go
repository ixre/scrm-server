package responses

import "core/app/models"

type StaffDetail struct {
	models.Staff
	Role models.Role `json:"role,omitempty"`
}
