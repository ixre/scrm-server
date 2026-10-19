package requests

type DeleteInternalTagReq struct {
	IDs []string `json:"ids" form:"ids" validate:"required,gt=0"`
}
