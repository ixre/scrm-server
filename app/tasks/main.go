package tasks

import (
	"core/common/log"

	"github.com/gogf/gf/os/gcron"
)

// Start 用于运行定时任务
func Start() {
	var err error
	defer func() {
		if err := recover(); err != nil {
			log.Sugar.Errorw("task panic error", "err", err)
		}
	}()

	_, err = gcron.AddSingleton("@daily", (ContactWay{}).DailyClean, "ContactWayDailyClean")
	if err != nil {
		log.Sugar.Errorw("AddSingleton failed", "err", err)
	}

	_, err = gcron.AddSingleton("@hourly", (Staff{}).UpdateMsgArchStatus, "UpdateStaffMsgArchStatus")
	if err != nil {
		log.Sugar.Errorw("AddSingleton failed", "err", err)
	}

	_, err = gcron.AddSingleton("@hourly", (MassMsg{}).UpdateMassMsgStatus, "UpdateMassMsgStatus")
	if err != nil {
		log.Sugar.Errorw("AddSingleton failed", "err", err)
	}

	_, err = gcron.AddSingleton("@daily", (GroupChat{}).CleanGroupChatIncrement, "CleanGroupChatIncrement")
	if err != nil {
		log.Sugar.Errorw("AddSingleton failed", "err", err)
	}

	// Phase 1：公海自动回收（每日凌晨执行）
	_, err = gcron.AddSingleton("0 0 2 * * *", (CrmPool{}).AutoRecycle, "CrmPoolAutoRecycle")
	if err != nil {
		log.Sugar.Errorw("AddSingleton failed", "err", err)
	}

	// Phase 4：自动化定时任务（SOP执行/客户评分/自动打标签）
	RegisterCrmAutomationTasks()

	log.Sugar.Infow("Tasks Running")
}
