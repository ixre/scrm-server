package tasks

import (
	"core/app/models"
 "core/app/services"
 "context"
	"core/common/log"
	"time"
)

type GroupChat struct {
	Base
}

// CleanGroupChatIncrement
// 每天删除客户群数量
func (o GroupChat) CleanGroupChatIncrement() {
	taskKey := "CleanCachedGroupChatNum"

	ok, err := o.Lock(taskKey, time.Minute)
	if err != nil {
		log.Sugar.Errorw("Lock failed", "err", err)
		return
	}
	if ok {
		defer o.Unlock(taskKey)
	}
	// 每天删除客户群数量
	err = services.EachWecomCorp(context.Background(), func(ctx context.Context, corpID string) error { return (models.GroupChat{}).CleanGroupChatIncrement(corpID) })
	if err != nil {
		log.Sugar.Errorw("UpdateMassMsgStatus failed", "err", err)
		return
	}
}
