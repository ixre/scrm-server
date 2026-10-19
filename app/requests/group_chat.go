package requests

import "core/common/app"

type GetAllGroupChatReq struct {
	app.Pager
	app.Sorter
}
