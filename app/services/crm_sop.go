package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- SOP 自动化 ----------

type CrmSopService struct{}

func NewCrmSop() *CrmSopService { return &CrmSopService{} }

// SopRuleCreateReq SOP 规则创建请求。
type SopRuleCreateReq struct {
	Rule models.CrmSopRule `json:"rule"`
}

// CreateRule 创建 SOP 规则。
func (s CrmSopService) CreateRule(ctx context.Context, req models.CrmSopRule) (models.CrmSopRule, error) {
	tenantID := identity.TenantID(ctx)


	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.TriggerType == "" || req.ActionType == "" {
		return req, errors.New("trigger_type/action_type 不能为空")
	}
	if err := createCRM(models.DB, &req); err != nil {
		log.Sugar.Errorw("sop rule create failed", "err", err)
		return req, err
	}
	return req, nil
}

// UpdateRule 更新 SOP 规则。
func (s CrmSopService) UpdateRule(ctx context.Context, id string, req models.CrmSopRule) (models.CrmSopRule, error) {
	tenantID := identity.TenantID(ctx)


	updates := map[string]any{
		"name": req.Name, "trigger_type": req.TriggerType, "trigger_cond": req.TriggerCond,
		"action_type": req.ActionType, "action_content": req.ActionContent,
		"followup_type": req.FollowupType, "delay_hours": req.DelayHours,
		"status": req.Status, "remark": req.Remark,
	}
	if err := models.DB.Model(&models.CrmSopRule{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(updates).Error; err != nil {
		return req, err
	}
	return req, nil
}

// QueryRules 列出 SOP 规则。
func (s CrmSopService) QueryRules(ctx context.Context, status string, pager *app.Pager) ([]models.CrmSopRule, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmSopRule{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSopRule
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// DeleteRules 删除 SOP 规则。
func (s CrmSopService) DeleteRules(ctx context.Context, ids []string) (int64, error) {


	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmSopRule{})
	return res.RowsAffected, res.Error
}

// ListTasks 列出 SOP 任务。
func (s CrmSopService) ListTasks(ctx context.Context, status, assignUserID string, pager *app.Pager) ([]models.CrmSopTask, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if assignUserID != "" {
		db = db.Where("assign_user_id = ?", assignUserID)
	}
	var total int64
	if err := db.Model(&models.CrmSopTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSopTask
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("execute_at desc").Find(&items).Error
	return items, total, err
}

// MarkTaskDone 标记 SOP 任务完成。
func (s CrmSopService) MarkTaskDone(ctx context.Context, taskID string) error {
	tenantID := identity.TenantID(ctx)


	now := time.Now()
	return models.DB.Model(&models.CrmSopTask{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", taskID).
		Updates(map[string]any{"status": "done", "done_at": &now}).Error
}

// MarkTaskSkipped 标记 SOP 任务跳过。
func (s CrmSopService) MarkTaskSkipped(ctx context.Context, taskID string) error {
	tenantID := identity.TenantID(ctx)


	return models.DB.Model(&models.CrmSopTask{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", taskID).
		Update("status", "skipped").Error
}

// Trigger 触发 SOP 规则：根据触发类型和条件，查找匹配的规则并创建任务。
// triggerData 包含 customer_id/opportunity_id/stage_id/tag_name 等。
func (s CrmSopService) Trigger(ctx context.Context, triggerType string, triggerData map[string]string) {


	var rules []models.CrmSopRule
	models.MustFor(ctx).Where("trigger_type = ? AND status = ?",  triggerType,  "active").Find(&rules)
	for _, rule := range rules {
		// 检查触发条件
		if !s.matchTriggerCond(rule, triggerData) {
			continue
		}
		customerID := triggerData["customer_id"]
		if customerID == "" {
			continue
		}
		s.createSopTask(ctx, rule, triggerData)
	}
}

// matchTriggerCond 检查触发条件是否匹配。
func (s CrmSopService) matchTriggerCond(rule models.CrmSopRule, data map[string]string) bool {
	if rule.TriggerCond == "" {
		return true // 无条件 = 全匹配
	}
	switch rule.TriggerType {
	case "stage_change":
		return data["stage_id"] == rule.TriggerCond
	case "tag_added":
		return data["tag_name"] == rule.TriggerCond
	default:
		return true
	}
}

// createSopTask 创建 SOP 任务实例（幂等：同规则+同客户 pending 状态不重复建）。
func (s CrmSopService) createSopTask(ctx context.Context, rule models.CrmSopRule, data map[string]string) {
	tenantID := identity.TenantID(ctx)


	customerID := data["customer_id"]
	// 幂等
	var cnt int64
	models.DB.Model(&models.CrmSopTask{}).Scopes(models.TenantScope(tenantID)).Where("rule_id = ? AND customer_id = ? AND status = ?",  rule.ID,  customerID,  "pending").Count(&cnt)
	if cnt > 0 {
		return
	}
	// 取客户 owner 作为分配人
	var cust models.CrmCustomer
	models.DB.Where("id = ?", customerID).First(&cust)
	executeAt := time.Now()
	if rule.DelayHours > 0 {
		executeAt = executeAt.Add(time.Duration(rule.DelayHours) * time.Hour)
	}
	task := models.CrmSopTask{
		Model:        models.Model{ID: id_generator.StringID()},
		TenantID:     tenantID,
		RuleID:       rule.ID,
		CustomerID:   customerID,
		OpportunityID: data["opportunity_id"],
		ActionType:   rule.ActionType,
		ActionContent: rule.ActionContent,
		Status:       "pending",
		ExecuteAt:    &executeAt,
		AssignUserID: cust.OwnerUserID,
	}
	if err := createCRM(models.DB, &task, "OpportunityID", "AssignUserID"); err != nil {
		log.Sugar.Errorw("sop task create failed", "err", err, "rule", rule.ID, "customer", customerID)
	}
}

// ExecutePendingTasks 执行到期的 SOP 任务（定时调用）。
// 对 create_followup 类型：自动创建跟进记录。
func (s CrmSopService) ExecutePendingTasks() {
	var tasks []models.CrmSopTask
	now := time.Now()
	models.DB.Where("status = ? AND execute_at <= ?", "pending", now).Limit(100).Find(&tasks)
	for _, task := range tasks {
		s.executeTask(task)
	}
}

// executeTask 执行单个 SOP 任务。
func (s CrmSopService) executeTask(task models.CrmSopTask) {
	switch task.ActionType {
	case "create_followup":
		// 自动创建跟进记录
		followup := models.CrmFollowup{
			Model:      models.Model{ID: id_generator.StringID()},
			TenantID:   task.TenantID,
			CustomerID: task.CustomerID,
			OpportunityID: task.OpportunityID,
			UserID:     task.AssignUserID,
			Type:       "other",
			Content:    fmt.Sprintf("[SOP自动] %s", task.ActionContent),
		}
		if err := createCRM(models.DB, &followup, "CustomerID", "OpportunityID", "UserID"); err != nil {
			log.Sugar.Errorw("sop execute: create followup failed", "err", err, "task", task.ID)
			return
		}
	case "send_message":
		// 仅记录，不发实际消息（需对接渠道发送能力）
		log.Sugar.Infow("sop send_message triggered", "task", task.ID, "customer", task.CustomerID, "content", task.ActionContent)
	}
	// 标记完成
	now := time.Now()
	models.DB.Model(&models.CrmSopTask{}).Where("id = ?", task.ID).
		Updates(map[string]any{"status": "done", "done_at": &now})
}

// PendingTaskCountByUser 某销售的待处理 SOP 任务数（工作台用）。
func (s CrmSopService) PendingTaskCountByUser(ctx context.Context, userID string) int64 {
	tenantID := identity.TenantID(ctx)


	var cnt int64
	models.DB.Model(&models.CrmSopTask{}).Scopes(models.TenantScope(tenantID)).Where("assign_user_id = ? AND status = ?",  userID,  "pending").Count(&cnt)
	return cnt
}

// _ 确保 gorm import 被使用
var _ = gorm.ErrRecordNotFound
