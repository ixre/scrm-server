package tasks

import (
	"core/app/services"
 "context"
	"core/common/log"
	"time"
)

type MassMsg struct {
	Base
}

// UpdateMassMsgStatus 更新群发消息的状态
func (o MassMsg) UpdateMassMsgStatus() {
	taskKey := "UpdateMassMsgStatus"

	ok, err := o.Lock(taskKey, time.Minute)
	if err != nil {
		log.Sugar.Errorw("Lock failed", "err", err)
		return
	}
	if ok {
		defer o.Unlock(taskKey)
	}

	// 每小时更新群发状态
	err = services.EachWecomCorp(context.Background(), func(ctx context.Context, corpID string) error { return (services.GroupChatMassMsg{}).UpdateGroupMsgSentStatus(corpID) })
	if err != nil {
		log.Sugar.Errorw("UpdateMassMsgStatus failed", "err", err)
		return
	}
}
