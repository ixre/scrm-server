package requests

type DeleteQuickReplyReq struct {
	// 话术id
	IDs []string `json:"ids" validate:"gt=0,dive,int64"`
}
