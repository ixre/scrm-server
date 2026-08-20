package services

import (
	"core/app/models"
	"core/common/log"
	"core/conf"
)

// Syncs 同步所有企业的全部信息
func Syncs() {
	var err error
	departmentService := NewDepartment()
	groupChatService := NewGroupChatService()
	staffService := NewStaffService()
	//customerService := NewCustomer()
	//tagService := NewTag()

	// 同步任务无 ctx：用单一 corp 的 tenant_id 注入（来自 conf.Settings.WeWork.ExtCorpID 反查 channel_link）。
	// 单租户演示场景下没有多 corp 映射，syncTenantIDFromCorp(conf.Settings.WeWork.ExtCorpID) 可能返回空，
	// 此时 Sync 会写入 NULL tenant_id（与 0005 回填兼容），等下一轮回填。
	syncCtx := syncTenantCtx(conf.Settings.WeWork.ExtCorpID)

	err = departmentService.Sync(syncCtx, conf.Settings.WeWork.ExtCorpID)
	if err != nil {
		log.Errorf("[ Wechat][ Error]: sync departments failed: %v", err)
		return
	}

	err = staffService.Sync(syncCtx, conf.Settings.WeWork.ExtCorpID)
	if err != nil {
		log.Errorf("[ Wechat][ Error]: sync staff failed: %v", err)
		return
	}

	err = groupChatService.SyncAll(syncCtx, conf.Settings.WeWork.ExtCorpID)
	if err != nil {
		log.Errorf("[ Wechat][ Error]: sync group chats failed: %v", err)
		return
	}

	//err = customerService.Sync(conf.Settings.WeWork.ExtCorpID)
	//if err != nil {
	//	panic(err)
	//}

	//err = tagService.Sync(conf.Settings.WeWork.ExtCorpID)
	//if err != nil {
	//	panic(err)
	//}

	models.SetupStaffRole() // 初始化超级管理员权限
}
