package tasks

import (
	"time"

	"core/app/services"
	"core/common/log"
)

type CrmPool struct {
	Base
}

// AutoRecycle 公海自动回收定时任务：遍历所有租户，按各自公海规则回收超时未跟进客户。
func (o CrmPool) AutoRecycle() {
	taskKey := "CrmPoolAutoRecycle"
	ok, err := o.Lock(taskKey, time.Minute*5)
	if err != nil {
		log.Sugar.Errorw("Lock failed", "err", err)
		return
	}
	if ok {
		defer o.Unlock(taskKey)
	} else {
		return // 其他实例正在执行
	}
	services.NewCrmPool().AutoRecycleAll()
}
