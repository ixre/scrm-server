package services

import (
	"core/common/identity"
	"slices"

	"context"

	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// ---------- A1 敏感词预警 ----------

type CrmSensitiveWordService struct{}

func NewCrmSensitiveWord() *CrmSensitiveWordService { return &CrmSensitiveWordService{} }

func (s CrmSensitiveWordService) List(ctx context.Context, category string, p *app.Pager) ([]models.CrmSensitiveWord, int64, error) {

	db := models.MustFor(ctx)
	if category != "" {
		db = db.Where("category = ?", category)
	}
	var total int64
	if err := db.Model(&models.CrmSensitiveWord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSensitiveWord
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmSensitiveWordService) Create(ctx context.Context, req models.CrmSensitiveWord) (models.CrmSensitiveWord, error) {
	tenantID := identity.TenantID(ctx)

	if strings.TrimSpace(req.Word) == "" {
		return req, errors.New("敏感词不能为空")
	}
	// 校验动作合法性
	if req.Action != "" && req.Action != "warn" && req.Action != "block" {
		return req, errors.New("action 只能为 warn 或 block")
	}
	// 校验分类合法性
	validCategories := map[string]bool{"fly_single": true, "abuse": true, "privacy": true, "other": true}
	if req.Category != "" && !validCategories[req.Category] {
		return req, errors.New("category 不合法")
	}
	// 检查重复
	var count int64
	models.DB.Model(&models.CrmSensitiveWord{}).Scopes(models.TenantScope(tenantID)).Where("word = ?", req.Word).Count(&count)
	if count > 0 {
		return req, fmt.Errorf("敏感词「%s」已存在", req.Word)
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	req.Word = strings.TrimSpace(req.Word)
	if req.Category == "" {
		req.Category = "other"
	}
	if req.Action == "" {
		req.Action = "warn"
	}
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmSensitiveWordService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmSensitiveWord{})
	return res.RowsAffected, res.Error
}

// Check 检查文本是否命中敏感词，命中则产生告警。
func (s CrmSensitiveWordService) Check(ctx context.Context, userID, customerID, content string) ([]models.CrmSensitiveAlert, error) {
	tenantID := identity.TenantID(ctx)

	if content == "" {
		return nil, nil
	}
	var words []models.CrmSensitiveWord
	if err := models.MustFor(ctx).Where("status = ?", "active").Find(&words).Error; err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, nil
	}
	alerts := make([]models.CrmSensitiveAlert, 0)
	for _, w := range words {
		if strings.Contains(content, w.Word) {
			alert := models.CrmSensitiveAlert{
				TenantID:   tenantID,
				UserID:     userID,
				CustomerID: customerID,
				Word:       w.Word,
				Category:   w.Category,
				Content:    truncate(content, 500),
				Action:     w.Action,
				Status:     "pending",
			}
			alert.ID = id_generator.StringID()
			if err := createCRM(models.DB, &alert); err != nil {
				return nil, err
			}
			alerts = append(alerts, alert)
		}
	}
	return alerts, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// ---------- A1 敏感词告警 ----------

type CrmSensitiveAlertService struct{}

func NewCrmSensitiveAlert() *CrmSensitiveAlertService { return &CrmSensitiveAlertService{} }

func (s CrmSensitiveAlertService) List(ctx context.Context, status string, p *app.Pager) ([]models.CrmSensitiveAlert, int64, error) {

	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmSensitiveAlert{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSensitiveAlert
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmSensitiveAlertService) Handle(ctx context.Context, ids []string, status string) (int64, error) {
	return handlePendingRecords(ctx, &models.CrmSensitiveAlert{}, ids, status, "handled", "ignored")
}

// handlePendingRecords 将 pending 记录批量更新为指定状态（allowed 为合法目标状态白名单）。
func handlePendingRecords(ctx context.Context, model any, ids []string, status string, allowed ...string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	if !slices.Contains(allowed, status) {
		return 0, fmt.Errorf("status 只能为 %s", strings.Join(allowed, " 或 "))
	}
	res := models.MustFor(ctx).Model(model).Where("id IN ? AND status = ?", ids, "pending").Update("status", status)
	return res.RowsAffected, res.Error
}

// ---------- A2 客户继承 ----------

type CrmInheritanceService struct{}

func NewCrmInheritance() *CrmInheritanceService { return &CrmInheritanceService{} }

// Execute 执行客户继承：把 fromUser 的客户批量转移给 toUser，并记录继承日志。
// 同时转移客户归属、跟进记录、商机归属。
func (s CrmInheritanceService) Execute(ctx context.Context, fromUserID, toUserID, inheritType, remark string) (models.CrmInheritance, error) {
	tenantID := identity.TenantID(ctx)

	if fromUserID == "" || toUserID == "" {
		return models.CrmInheritance{}, errors.New("from_user_id 和 to_user_id 不能为空")
	}
	if fromUserID == toUserID {
		return models.CrmInheritance{}, errors.New("不能继承给自己")
	}
	if inheritType == "" {
		inheritType = "transfer"
	}
	if inheritType != "transfer" && inheritType != "resign" {
		return models.CrmInheritance{}, errors.New("type 只能为 transfer 或 resign")
	}
	// 查询原归属客户
	var customers []models.CrmCustomer
	if err := models.MustFor(ctx).Where("owner_user_id = ? AND deleted_at IS NULL", fromUserID).Find(&customers).Error; err != nil {
		return models.CrmInheritance{}, err
	}
	if len(customers) == 0 {
		return models.CrmInheritance{}, errors.New("该员工名下无客户，无需继承")
	}
	customerIDs := make(constants.StringArrayField, 0, len(customers))
	for _, c := range customers {
		customerIDs = append(customerIDs, c.ID)
	}
	// 事务：批量转移客户+跟进+商机 + 记日志
	rec := models.CrmInheritance{
		TenantID:    tenantID,
		FromUserID:  fromUserID,
		ToUserID:    toUserID,
		Type:        inheritType,
		CustomerIDs: customerIDs,
		CustomerCnt: len(customers),
		Status:      "done",
		Remark:      remark,
	}
	rec.ID = id_generator.StringID()
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		// 转移客户归属
		if err := tx.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ? AND id IN ?", fromUserID, customerIDs).Update("owner_user_id", toUserID).Error; err != nil {
			return err
		}
		// 转移跟进记录归属
		if err := tx.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND customer_id IN ?", fromUserID, customerIDs).Update("user_id", toUserID).Error; err != nil {
			return err
		}
		// 转移商机归属
		if err := tx.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ? AND customer_id IN ?", fromUserID, customerIDs).Update("owner_user_id", toUserID).Error; err != nil {
			return err
		}
		return createCRM(tx, &rec)
	})
	return rec, err
}

func (s CrmInheritanceService) List(ctx context.Context, p *app.Pager) ([]models.CrmInheritance, int64, error) {

	db := models.MustFor(ctx)
	var total int64
	if err := db.Model(&models.CrmInheritance{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmInheritance
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- A3 客户流失提醒 ----------

type CrmCustomerLossService struct{}

func NewCrmCustomerLoss() *CrmCustomerLossService { return &CrmCustomerLossService{} }

func (s CrmCustomerLossService) List(ctx context.Context, lossType, status string, p *app.Pager) ([]models.CrmCustomerLoss, int64, error) {

	db := models.MustFor(ctx)
	if lossType != "" {
		db = db.Where("loss_type = ?", lossType)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmCustomerLoss{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomerLoss
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmCustomerLossService) Handle(ctx context.Context, ids []string, status string) (int64, error) {
	return handlePendingRecords(ctx, &models.CrmCustomerLoss{}, ids, status, "handled", "ignored")
}

// RecordLoss 记录客户流失事件（供回调/定时任务调用）。
// 去重：同一客户同一类型且状态为 pending 时不重复记录。
func (s CrmCustomerLossService) RecordLoss(ctx context.Context, customerID, customerName, userID, lossType string, days int) error {
	tenantID := identity.TenantID(ctx)

	if tenantID == "" || customerID == "" {
		return errors.New("tenant_id 和 customer_id 不能为空")
	}
	// 去重检查
	var exist int64
	models.DB.Model(&models.CrmCustomerLoss{}).Scopes(models.TenantScope(tenantID)).Where("customer_id = ? AND loss_type = ? AND status = ?", customerID, lossType, "pending").
		Count(&exist)
	if exist > 0 {
		return nil // 已有待处理流失记录，不重复
	}
	rec := models.CrmCustomerLoss{
		TenantID:     tenantID,
		CustomerID:   customerID,
		CustomerName: customerName,
		UserID:       userID,
		LossType:     lossType,
		Days:         days,
		Status:       "pending",
	}
	rec.ID = id_generator.StringID()
	return createCRM(models.DB, &rec)
}

// ---------- A4 超时未回复提醒 ----------

type CrmReplyTimeoutService struct{}

func NewCrmReplyTimeout() *CrmReplyTimeoutService { return &CrmReplyTimeoutService{} }

func (s CrmReplyTimeoutService) List(ctx context.Context, status string, p *app.Pager) ([]models.CrmReplyTimeout, int64, error) {

	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmReplyTimeout{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmReplyTimeout
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmReplyTimeoutService) Handle(ctx context.Context, ids []string, status string) (int64, error) {
	return handlePendingRecords(ctx, &models.CrmReplyTimeout{}, ids, status, "replied", "escalated")
}

// RecordTimeout 记录超时未回复事件。
// 去重：同一客户同一渠道且状态为 pending 时不重复记录。
func (s CrmReplyTimeoutService) RecordTimeout(ctx context.Context, userID, customerID, channel string, lastMsgAt time.Time, timeoutMins int) error {
	tenantID := identity.TenantID(ctx)

	if tenantID == "" || customerID == "" {
		return errors.New("tenant_id 和 customer_id 不能为空")
	}
	// 去重检查
	var exist int64
	models.DB.Model(&models.CrmReplyTimeout{}).Scopes(models.TenantScope(tenantID)).Where("customer_id = ? AND channel = ? AND status = ?", customerID, channel, "pending").
		Count(&exist)
	if exist > 0 {
		return nil // 已有待处理超时记录，不重复
	}
	rec := models.CrmReplyTimeout{
		TenantID:    tenantID,
		UserID:      userID,
		CustomerID:  customerID,
		Channel:     channel,
		LastMsgAt:   &lastMsgAt,
		TimeoutMins: timeoutMins,
		Status:      "pending",
	}
	rec.ID = id_generator.StringID()
	return createCRM(models.DB, &rec)
}

// ---------- A5 群SOP ----------

var validTriggerTypes = map[string]bool{"daily": true, "new_member": true, "keyword": true}
var validActionTypes = map[string]bool{"send_message": true, "send_material": true}

type CrmGroupSopService struct{}

func NewCrmGroupSop() *CrmGroupSopService { return &CrmGroupSopService{} }

func (s CrmGroupSopService) List(ctx context.Context, p *app.Pager) ([]models.CrmGroupSop, int64, error) {

	db := models.MustFor(ctx)
	var total int64
	if err := db.Model(&models.CrmGroupSop{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmGroupSop
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmGroupSopService) Get(ctx context.Context, id string) (models.CrmGroupSop, error) {

	var item models.CrmGroupSop
	err := models.MustFor(ctx).Where("id = ?", id).First(&item).Error
	return item, err
}

func (s CrmGroupSopService) Create(ctx context.Context, req models.CrmGroupSop) (models.CrmGroupSop, error) {
	tenantID := identity.TenantID(ctx)

	if strings.TrimSpace(req.Name) == "" {
		return req, errors.New("规则名称不能为空")
	}
	if !validTriggerTypes[req.TriggerType] {
		return req, errors.New("trigger_type 只能为 daily/new_member/keyword")
	}
	if !validActionTypes[req.ActionType] {
		return req, errors.New("action_type 只能为 send_message/send_material")
	}
	if strings.TrimSpace(req.ActionData) == "" {
		return req, errors.New("action_data 不能为空")
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	req.Name = strings.TrimSpace(req.Name)
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmGroupSopService) Update(ctx context.Context, id string, req models.CrmGroupSop) (models.CrmGroupSop, error) {

	if !validTriggerTypes[req.TriggerType] {
		return req, errors.New("trigger_type 只能为 daily/new_member/keyword")
	}
	if !validActionTypes[req.ActionType] {
		return req, errors.New("action_type 只能为 send_message/send_material")
	}
	if err := models.MustFor(ctx).Model(&models.CrmGroupSop{}).Where("id = ?", id).Updates(map[string]any{
		"name":         req.Name,
		"group_ids":    req.GroupIDs,
		"trigger_type": req.TriggerType,
		"trigger_data": req.TriggerData,
		"action_type":  req.ActionType,
		"action_data":  req.ActionData,
		"status":       req.Status,
	}).Error; err != nil {
		return req, err
	}
	return s.Get(ctx, id)
}

func (s CrmGroupSopService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmGroupSop{})
	return res.RowsAffected, res.Error
}

func (s CrmGroupSopService) ListTasks(ctx context.Context, status string, p *app.Pager) ([]models.CrmGroupSopTask, int64, error) {

	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmGroupSopTask{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmGroupSopTask
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmGroupSopService) DoneTask(ctx context.Context, id string) error {

	// 校验状态：只允许 pending 标记为 done
	now := time.Now()
	res := models.MustFor(ctx).Model(&models.CrmGroupSopTask{}).Where("id = ? AND status = ?", id, "pending").
		Updates(map[string]any{"status": "done", "done_at": &now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("任务不存在或已完成")
	}
	return nil
}
