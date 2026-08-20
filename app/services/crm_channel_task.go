package services

import (
	"core/common/identity"
	
	"context"
	
	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 跨渠道引流任务 ----------

type CrmChannelTaskService struct{}

func NewCrmChannelTask() *CrmChannelTaskService { return &CrmChannelTaskService{} }

// CreateLeadToWecomTask 小红书留资 → 企微添加引流任务（幂等：同客户同 action pending 状态不重复建）。
func (s CrmChannelTaskService) CreateLeadToWecomTask(ctx context.Context, customerID string) error {
	tenantID := identity.TenantID(ctx)


	// 幂等：同客户已有 pending 的 add_wecom 任务则跳过
	var cnt int64
	models.DB.Model(&models.CrmChannelTask{}).Scopes(models.TenantScope(tenantID)).Where("customer_id = ? AND action = ? AND status = ?",  customerID,  "add_wecom",  "pending").Count(&cnt)
	if cnt > 0 {
		return nil
	}
	return s.create(ctx, customerID, constants.ChannelXiaohongshu, constants.ChannelWecom, "add_wecom", "", "")
}

func (s CrmChannelTaskService) create(ctx context.Context, customerID, fromCh, toCh, action, assignUserID, remark string) error {
	tenantID := identity.TenantID(ctx)


	t := models.CrmChannelTask{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		CustomerID:  customerID,
		FromChannel: fromCh,
		ToChannel:   toCh,
		Action:      action,
		Status:      "pending",
		AssignUserID: assignUserID,
		Remark:      remark,
	}
	if err := createCRM(models.DB, &t, "AssignUserID"); err != nil {
		log.Sugar.Errorw("channel task create failed", "err", err)
		return err
	}
	return nil
}

// List 列出引流任务（按租户+状态过滤）。
func (s CrmChannelTaskService) List(ctx context.Context, status string, pager *app.Pager) ([]models.CrmChannelTask, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmChannelTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmChannelTask
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("created_at desc").Find(&items).Error
	return items, total, err
}

// MarkDone 标记任务完成。
func (s CrmChannelTaskService) MarkDone(ctx context.Context, taskID string) error {
	tenantID := identity.TenantID(ctx)


	return models.DB.Model(&models.CrmChannelTask{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", taskID).
		Update("status", "done").Error
}

// MarkSkipped 标记任务跳过。
func (s CrmChannelTaskService) MarkSkipped(ctx context.Context, taskID string) error {
	tenantID := identity.TenantID(ctx)


	return models.DB.Model(&models.CrmChannelTask{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", taskID).
		Update("status", "skipped").Error
}

// Assign 分配任务给销售。
func (s CrmChannelTaskService) Assign(ctx context.Context, taskID, userID string) error {
	tenantID := identity.TenantID(ctx)


	return models.DB.Model(&models.CrmChannelTask{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", taskID).
		Update("assign_user_id", userID).Error
}

// PendingCountByUser 某销售的待处理引流任务数（工作台用）。
func (s CrmChannelTaskService) PendingCountByUser(ctx context.Context, userID string) int64 {
	tenantID := identity.TenantID(ctx)


	var cnt int64
	models.DB.Model(&models.CrmChannelTask{}).Scopes(models.TenantScope(tenantID)).Where("assign_user_id = ? AND status = ?",  userID,  "pending").Count(&cnt)
	return cnt
}

// _ 确保 gorm import 被使用（createCRM 内部用 gorm，但显式引用避免未使用）
var _ = gorm.ErrRecordNotFound
