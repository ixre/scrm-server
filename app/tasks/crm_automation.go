package tasks

import (
	"context"
	"core/app/models"
	"core/app/services"
	"core/common/id_generator"
	"core/common/identity"
	"core/common/log"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/gogf/gf/os/gcron"
)

// CrmAutomation Phase 4 自动化定时任务。
type CrmAutomation struct{}

// ExecuteSopTasks 执行到期的 SOP 任务（每小时）。
func (CrmAutomation) ExecuteSopTasks() {
	services.NewCrmSop().ExecutePendingTasks()
	log.Sugar.Infow("sop tasks executed")
}

// RecalculateScores 重算客户评分（每日凌晨）。
func (CrmAutomation) RecalculateScores() {
	// 查所有租户ID
	var tenantIDs []string
	models.DB.Model(&models.Tenant{}).Pluck("id", &tenantIDs)
	for _, tid := range tenantIDs {
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tid})
		services.NewCrmScore().RecalculateAll(ctx)
	}
}

// ApplyAutoTags 对所有客户应用自动打标签规则（每日）。
func (CrmAutomation) ApplyAutoTags() {
	var tenantIDs []string
	models.DB.Model(&models.Tenant{}).Pluck("id", &tenantIDs)
	for _, tid := range tenantIDs {
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tid})
		services.NewCrmAutoTag().ApplyAll(ctx)
	}
}

// RegisterCrmAutomationTasks 注册 Phase 4 + Phase 7 自动化定时任务。
func RegisterCrmAutomationTasks() {
	if _, err := gcron.AddSingleton("@every 5m", (CrmAutomation{}).GenerateReminders, "CrmGenerateReminders"); err != nil {
		log.Sugar.Errorw("RegisterCrmGenerateReminders failed", "err", err)
	}
	_, err := gcron.AddSingleton("@hourly", (CrmAutomation{}).ExecuteSopTasks, "CrmExecuteSopTasks")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmExecuteSopTasks failed", "err", err)
	}
	_, err = gcron.AddSingleton("0 0 3 * * *", (CrmAutomation{}).RecalculateScores, "CrmRecalculateScores")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmRecalculateScores failed", "err", err)
	}
	_, err = gcron.AddSingleton("0 0 4 * * *", (CrmAutomation{}).ApplyAutoTags, "CrmApplyAutoTags")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmApplyAutoTags failed", "err", err)
	}
	// Phase 7：超时未回复检测（每30分钟）
	_, err = gcron.AddSingleton("@every 30m", (CrmAutomation{}).CheckReplyTimeout, "CrmCheckReplyTimeout")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmCheckReplyTimeout failed", "err", err)
	}
	// SOP：跟进逾期检查（每日凌晨6点）
	_, err = gcron.AddSingleton("0 0 6 * * *", (CrmAutomation{}).CheckFollowupOverdue, "CrmCheckFollowupOverdue")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmCheckFollowupOverdue failed", "err", err)
	}
	// Phase 7：群SOP任务生成（每小时）
	_, err = gcron.AddSingleton("@hourly", (CrmAutomation{}).GenerateGroupSopTasks, "CrmGenerateGroupSopTasks")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmGenerateGroupSopTasks failed", "err", err)
	}
	// Phase 7：长期未跟进客户流失检测（每日凌晨5点）
	_, err = gcron.AddSingleton("0 0 5 * * *", (CrmAutomation{}).CheckUnfollowedLoss, "CrmCheckUnfollowedLoss")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmCheckUnfollowedLoss failed", "err", err)
	}
	// Phase 7：渠道引流任务超时扫描（每小时）
	_, err = gcron.AddSingleton("@hourly", (CrmAutomation{}).ScanChannelTasks, "CrmScanChannelTasks")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmScanChannelTasks failed", "err", err)
	}
	// Phase 7：拼团超时失败处理（每5分钟）
	_, err = gcron.AddSingleton("@every 5m", (CrmAutomation{}).ExpireGroupBuyRecords, "CrmExpireGroupBuyRecords")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmExpireGroupBuyRecords failed", "err", err)
	}
	// Phase 7：砍价超时失效处理（每5分钟）
	_, err = gcron.AddSingleton("@every 5m", (CrmAutomation{}).ExpireBargainRecords, "CrmExpireBargainRecords")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmExpireBargainRecords failed", "err", err)
	}
	// Phase 7：超时未回复自动升级（每小时，超过2小时升级为 escalated）
	_, err = gcron.AddSingleton("@hourly", (CrmAutomation{}).EscalateReplyTimeout, "CrmEscalateReplyTimeout")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmEscalateReplyTimeout failed", "err", err)
	}
	// 客户关怀：生日提醒（每日早上8点）
	_, err = gcron.AddSingleton("0 0 8 * * *", (CrmAutomation{}).GenerateBirthdayCare, "CrmGenerateBirthdayCare")
	if err != nil {
		log.Sugar.Errorw("RegisterCrmGenerateBirthdayCare failed", "err", err)
	}
}

// CheckReplyTimeout 扫描渠道互动中超时未回复的消息。
func (CrmAutomation) CheckReplyTimeout() {
	var tenantIDs []string
	models.DB.Model(&models.Tenant{}).Pluck("id", &tenantIDs)
	for _, tid := range tenantIDs {
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tid})
		// 查最近60分钟内的客户消息（action_type 含 customer/dm），检查是否有跟进回复
		var interactions []models.CrmChannelInteraction
		models.DB.Scopes(models.TenantScope(tid)).Where("action_type IN ? AND created_at > NOW() - INTERVAL '60 minutes'", []string{"lead", "comment", "dm"}).Find(&interactions)
		for _, inter := range interactions {
			if inter.CustomerID == "" {
				continue
			}
			// 检查该客户在互动后是否有跟进记录
			var followupCount int64
			models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tid)).Where("customer_id = ? AND created_at > ?", inter.CustomerID, inter.CreatedAt).
				Count(&followupCount)
			if followupCount == 0 {
				_ = services.NewCrmReplyTimeout().RecordTimeout(ctx, "", inter.CustomerID, inter.Channel, inter.CreatedAt, 30)
			}
		}
	}
	log.Sugar.Infow("reply timeout checked")
}

// CheckFollowupOverdue 检查跟进逾期客户并触发 SOP（每日）。
// 对有归属人且超过3天未跟进的客户，触发 followup_overdue SOP 规则。
func (CrmAutomation) CheckFollowupOverdue() {
	var tenantIDs []string
	models.DB.Model(&models.Tenant{}).Pluck("id", &tenantIDs)
	for _, tid := range tenantIDs {
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tid})
		// 查非公海、有归属人、3天未跟进的客户
		var customers []models.CrmCustomer
		models.DB.Scopes(models.TenantScope(tid)).
			Where("pool_status = 0 AND owner_user_id != ''").
			Where("id NOT IN (SELECT DISTINCT customer_id FROM crm_followup WHERE tenant_id = ? AND created_at > NOW() - INTERVAL '3 days' AND deleted_at IS NULL)", tid).
			Limit(200).Find(&customers)
		sop := services.NewCrmSop()
		for _, c := range customers {
			sop.Trigger(ctx, "followup_overdue", map[string]string{
				"customer_id": c.ID,
			})
		}
	}
	log.Sugar.Infow("followup overdue checked")
}

// GenerateGroupSopTasks 根据群SOP规则生成执行任务。
func (CrmAutomation) GenerateGroupSopTasks() {
	var tenantIDs []string
	models.DB.Model(&models.Tenant{}).Pluck("id", &tenantIDs)
	now := time.Now()
	for _, tid := range tenantIDs {
		var rules []models.CrmGroupSop
		models.DB.Scopes(models.TenantScope(tid)).Where("status = ?", "active").Find(&rules)
		for _, rule := range rules {
			if rule.TriggerType == "daily" {
				// 解析每日执行时间，默认当天09:00
				executeAt := time.Date(now.Year(), now.Month(), now.Day(), 9, 0, 0, 0, now.Location())
				if rule.TriggerData != "" {
					var td struct {
						Time string `json:"time"`
					}
					if json.Unmarshal([]byte(rule.TriggerData), &td) == nil && td.Time != "" {
						if parts := strings.Split(td.Time, ":"); len(parts) >= 2 {
							h, _ := strconv.Atoi(parts[0])
							m, _ := strconv.Atoi(parts[1])
							executeAt = time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
						}
					}
				}
				// 避免重复生成当天任务
				var exist int64
				models.DB.Model(&models.CrmGroupSopTask{}).Scopes(models.TenantScope(tid)).Where("rule_id = ? AND execute_at >= ? AND execute_at < ?", rule.ID, time.Date(executeAt.Year(), executeAt.Month(), executeAt.Day(), 0, 0, 0, 0, executeAt.Location()), time.Date(executeAt.Year(), executeAt.Month(), executeAt.Day(), 23, 59, 59, 0, executeAt.Location())).
					Count(&exist)
				if exist > 0 {
					continue
				}
				task := models.CrmGroupSopTask{
					TenantID:   tid,
					RuleID:     rule.ID,
					ActionType: rule.ActionType,
					ActionData: rule.ActionData,
					ExecuteAt:  &executeAt,
					Status:     "pending",
				}
				task.ID = id_generator.StringID()
				models.DB.Create(&task)
			}
		}
	}
	log.Sugar.Infow("group sop tasks generated")
}

// CheckUnfollowedLoss 检测长期未跟进客户，记录流失提醒。
func (CrmAutomation) CheckUnfollowedLoss() {
	var tenantIDs []string
	models.DB.Model(&models.Tenant{}).Pluck("id", &tenantIDs)
	for _, tid := range tenantIDs {
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tid})
		// 查超过30天未跟进的客户
		var customers []models.CrmCustomer
		models.DB.Scopes(models.TenantScope(tid)).Where("deleted_at IS NULL AND owner_user_id != '' AND updated_at < NOW() - INTERVAL '30 days'").Find(&customers)
		for _, c := range customers {
			// 检查是否已有流失记录（避免重复）
			var exist int64
			models.DB.Model(&models.CrmCustomerLoss{}).Scopes(models.TenantScope(tid)).Where("customer_id = ? AND loss_type = ? AND status = ?", c.ID, "unfollowed", "pending").Count(&exist)
			if exist > 0 {
				continue
			}
			days := int(time.Since(c.UpdatedAt).Hours() / 24)
			_ = services.NewCrmCustomerLoss().RecordLoss(ctx, c.ID, c.Name, c.OwnerUserID, "unfollowed", days)
		}
	}
	log.Sugar.Infow("unfollowed loss checked")
}

// ScanChannelTasks 扫描渠道引流任务，标记超时未处理的任务。
func (CrmAutomation) ScanChannelTasks() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := services.NewCrmChannelTask().Expire(ctx); err != nil {
		log.Sugar.Errorw("channel task scan failed", "err", err)
	}
}

// ExpireGroupBuyRecords 扫描超时未成团的拼团记录，标记为失败。
func (CrmAutomation) ExpireGroupBuyRecords() {
	now := time.Now()
	res := models.DB.Model(&models.CrmGroupBuyRecord{}).
		Where("status = ? AND expire_at < ?", "open", now).
		Update("status", "failed")
	log.Sugar.Infow("group buy records expired", "count", res.RowsAffected)
}

// ExpireBargainRecords 扫描超时未砍到底价的砍价记录，标记为过期。
func (CrmAutomation) ExpireBargainRecords() {
	now := time.Now()
	res := models.DB.Model(&models.CrmBargainRecord{}).
		Where("status = ? AND expire_at < ?", "active", now).
		Update("status", "expired")
	log.Sugar.Infow("bargain records expired", "count", res.RowsAffected)
}

// EscalateReplyTimeout 超时未回复自动升级（超过2小时未回复的 pending 升级为 escalated）。
func (CrmAutomation) EscalateReplyTimeout() {
	res := models.DB.Model(&models.CrmReplyTimeout{}).
		Where("status = ? AND created_at < NOW() - INTERVAL '2 hours'", "pending").
		Update("status", "escalated")
	log.Sugar.Infow("reply timeout escalated", "count", res.RowsAffected)
}

// GenerateBirthdayCare 生日关怀待办生成（每日）。
func (CrmAutomation) GenerateBirthdayCare() {
	services.NewCrmCare().GenerateBirthdayCareTodos()
}

// GenerateReminders pages tenants and sources instead of loading all tenant IDs.
func (CrmAutomation) GenerateReminders() {
	ctx := context.Background()
	lastID := "0"
	for {
		var tenants []models.Tenant
		if err := models.DB.WithContext(ctx).Select("id").Where("id > ? AND status = 1 AND (expire_at IS NULL OR expire_at > ?)", lastID, time.Now()).Order("id").Limit(100).Find(&tenants).Error; err != nil {
			log.Sugar.Errorw("reminder tenant scan failed", "err", err)
			return
		}
		if len(tenants) == 0 {
			return
		}
		for _, tenant := range tenants {
			bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
			tenantCtx := identity.WithIdentity(bounded, identity.Identity{TenantID: tenant.ID})
			if err := services.NewCrmTodo().GenerateFollowupTodos(tenantCtx); err != nil {
				log.Sugar.Errorw("followup reminders failed", "tenant", tenant.ID, "err", err)
			}
			cancel()
			bounded, cancel = context.WithTimeout(ctx, 30*time.Second)
			tenantCtx = identity.WithIdentity(bounded, identity.Identity{TenantID: tenant.ID})
			if err := services.NewCrmTodo().GenerateExpireTodos(tenantCtx); err != nil {
				log.Sugar.Errorw("receivables reminders failed", "tenant", tenant.ID, "err", err)
			}
			cancel()
		}
		lastID = tenants[len(tenants)-1].ID
	}
}
