package requests

type DeleteGroupChatAutoJoinReq struct {
	// 自动拉群码ID
	IDs []string `json:"ids" validate:"gt=0,dive,int64"`
}
