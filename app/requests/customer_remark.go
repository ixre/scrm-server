package requests

type DeleteCustomerRemarkReq struct {
	IDs []string `json:"ids" validate:"gt=0"`
}

type DeleteRemarkOptionReq struct {
	IDs []string `json:"ids" form:"ids" validate:"required,gt=0"`
}
