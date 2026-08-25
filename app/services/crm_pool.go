package services

import (
	"core/common/identity"
	"fmt"

	"context"

	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"github.com/pkg/errors"
	"gorm.io/gorm"
)

// ---------- 客户公海池 ----------

type CrmPoolService struct{}

func NewCrmPool() *CrmPoolService { return &CrmPoolService{} }

// ListPool 公海客户列表（pool_status=1），按租户隔离，支持关键字搜索。
func (s CrmPoolService) ListPool(ctx context.Context, keyword string, pager *app.Pager) ([]models.CrmCustomer, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx).Where("pool_status = ?", 1)
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var cs []models.CrmCustomer
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("pool_at desc").Find(&cs).Error
	return cs, total, err
}

// Claim 认领公海客户：pool_status→0，owner→认领人。仅公海客户可认领。
// 校验：领取上限、前负责人冷却期。记录操作日志。
func (s CrmPoolService) Claim(ctx context.Context, userID string, customerIDs []string) (int64, error) {
	tenantID := identity.TenantID(ctx)

	if len(customerIDs) == 0 {
		return 0, nil
	}

	// 读取公海规则
	rule, _ := s.GetRule(ctx)

	// 校验领取上限
	if rule.ClaimLimit > 0 {
		var currentCount int64
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).
			Where("owner_user_id = ? AND pool_status = ?", userID, 0).Count(&currentCount)
		if currentCount+int64(len(customerIDs)) > int64(rule.ClaimLimit) {
			return 0, errors.New(fmt.Sprintf("领取上限为 %d，当前已拥有 %d 个客户，无法领取 %d 个", rule.ClaimLimit, currentCount, len(customerIDs)))
		}
	}

	// 查询待认领客户（用于前负责人冷却校验和日志）
	var customers []models.CrmCustomer
	models.DB.Scopes(models.TenantScope(tenantID)).
		Where("id IN ? AND pool_status = ?", customerIDs, 1).Find(&customers)

	// 校验前负责人冷却期
	if rule.ClaimCooldownDays > 0 {
		cooldownThreshold := time.Now().AddDate(0, 0, -rule.ClaimCooldownDays)
		for _, c := range customers {
			if c.PrevOwnerUserID == userID && c.PoolAt != nil && c.PoolAt.After(cooldownThreshold) {
				return 0, errors.New(fmt.Sprintf("客户「%s」回收未满 %d 天，前负责人不可领取", c.Name, rule.ClaimCooldownDays))
			}
		}
	}

	now := time.Now()
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ? AND pool_status = ?", customerIDs, 1).
		Updates(map[string]any{"pool_status": 0, "pool_at": nil, "owner_user_id": userID, "claimed_at": now})
	if res.Error != nil {
		log.Sugar.Errorw("crm pool claim failed", "err", res.Error)
	}

	// 记录操作日志
	if res.RowsAffected > 0 {
		operatorName := identity.UserID(ctx)
		for _, c := range customers {
			s.writePoolLog(tenantID, c.ID, c.Name, "claim", userID, operatorName, "", "")
		}
		// Phase 4：公海认领触发 SOP
		sop := NewCrmSop()
		for _, cid := range customerIDs {
			sop.Trigger(ctx, "pool_claim", map[string]string{"customer_id": cid, "pool_type": "customer"})
		}
	}
	return res.RowsAffected, res.Error
}

// Assign 管理员分配公海客户给指定销售。
func (s CrmPoolService) Assign(ctx context.Context, operatorID string, targetUserID string, customerIDs []string) (int64, error) {
	tenantID := identity.TenantID(ctx)

	if len(customerIDs) == 0 {
		return 0, nil
	}
	if targetUserID == "" {
		return 0, errors.New("目标销售不能为空")
	}
	now := time.Now()

	// 查询待分配客户（用于日志）
	var customers []models.CrmCustomer
	models.DB.Scopes(models.TenantScope(tenantID)).
		Where("id IN ? AND pool_status = ?", customerIDs, 1).Find(&customers)

	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ? AND pool_status = ?", customerIDs, 1).
		Updates(map[string]any{"pool_status": 0, "pool_at": nil, "owner_user_id": targetUserID, "claimed_at": now})
	if res.Error != nil {
		log.Sugar.Errorw("crm pool assign failed", "err", res.Error)
	}

	if res.RowsAffected > 0 {
		operatorName := identity.UserID(ctx)
		for _, c := range customers {
			s.writePoolLog(tenantID, c.ID, c.Name, "assign", operatorID, operatorName, targetUserID, "")
		}
	}
	return res.RowsAffected, res.Error
}

// Recycle 手动回收客户进公海：pool_status→1，owner 清空，pool_at=now，记录前负责人。
func (s CrmPoolService) Recycle(ctx context.Context, customerIDs []string) (int64, error) {
	tenantID := identity.TenantID(ctx)

	if len(customerIDs) == 0 {
		return 0, nil
	}
	now := time.Now()

	// 查询待回收客户（用于记录前负责人和日志）
	var customers []models.CrmCustomer
	models.DB.Scopes(models.TenantScope(tenantID)).
		Where("id IN ? AND pool_status = ?", customerIDs, 0).Find(&customers)

	// 构建逐条更新（需要保留 prev_owner_user_id）
	var affected int64
	operatorID := identity.UserID(ctx)
	operatorName := identity.UserID(ctx)
	for _, c := range customers {
		res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).
			Where("id = ? AND pool_status = ?", c.ID, 0).
			Updates(map[string]any{"pool_status": 1, "pool_at": now, "owner_user_id": "", "prev_owner_user_id": c.OwnerUserID})
		if res.Error != nil {
			log.Sugar.Errorw("crm pool recycle failed", "err", res.Error, "customer", c.ID)
			continue
		}
		affected += res.RowsAffected
		s.writePoolLog(tenantID, c.ID, c.Name, "recycle", operatorID, operatorName, "", "手动回收")
	}
	return affected, nil
}

// AutoRecycle 自动回收：按租户公海规则，将超 N 天未跟进或超 N 天未成单的非公海客户回收。
// 保护期内的客户（ClaimedAt + ProtectDays > now）不被回收。锁定客户不被回收。
func (s CrmPoolService) AutoRecycle(ctx context.Context) (int64, error) {
	rule, err := s.GetRule(ctx)
	if err != nil || !rule.Enabled {
		return 0, nil
	}
	if rule.RecycleDays <= 0 && rule.NoDealDays <= 0 {
		return 0, nil
	}
	now := time.Now()
	followupThreshold := time.Now().AddDate(0, 0, -rule.RecycleDays)
	noDealThreshold := time.Now().AddDate(0, 0, -rule.NoDealDays)

	// 查所有非公海、有归属、未锁定的客户
	var cs []models.CrmCustomer
	if err := models.MustFor(ctx).Where("pool_status = ? AND owner_user_id <> '' AND is_locked = ?", 0, false).Find(&cs).Error; err != nil {
		return 0, err
	}
	var toRecycle []string
	for _, c := range cs {
		// 保护期内跳过
		if rule.ProtectDays > 0 && c.ClaimedAt != nil && c.ClaimedAt.AddDate(0, 0, rule.ProtectDays).After(now) {
			continue
		}
		shouldRec := false
		// 未跟进回收
		if rule.RecycleDays > 0 {
			lastFollowup := lastFollowupTime(ctx, c.ID)
			if lastFollowup.IsZero() {
				lastFollowup = c.CreatedAt
			}
			if shouldRecycle(lastFollowup, followupThreshold) {
				shouldRec = true
			}
		}
		// 未成单回收（有商机的不回收）
		if !shouldRec && rule.NoDealDays > 0 {
			if !hasActiveOpportunity(ctx, c.ID) && c.CreatedAt.Before(noDealThreshold) {
				shouldRec = true
			}
		}
		if shouldRec {
			toRecycle = append(toRecycle, c.ID)
		}
	}
	if len(toRecycle) == 0 {
		return 0, nil
	}
	return s.Recycle(ctx, toRecycle)
}

// lastFollowupTime 取某客户最新跟进时间，无跟进则返回零值。
func lastFollowupTime(ctx context.Context, customerID string) time.Time {
	var f models.CrmFollowup
	if err := models.MustFor(ctx).Where("customer_id = ?", customerID).
		Order("created_at desc").Limit(1).First(&f).Error; err != nil {
		return time.Time{}
	}
	return f.CreatedAt
}

// shouldRecycle 判断是否应回收：基准时间（最近跟进或创建时间）早于阈值。
// 纯函数，供单测。
func shouldRecycle(baseTime time.Time, threshold time.Time) bool {
	if baseTime.IsZero() {
		return false
	}
	return baseTime.Before(threshold)
}

// ---------- 公海规则 ----------

// GetRule 读取租户公海规则，不存在则返回默认（不自动回收）。
func (s CrmPoolService) GetRule(ctx context.Context) (models.CrmPoolRule, error) {

	var r models.CrmPoolRule
	err := models.MustFor(ctx).First(&r).Error
	if err == nil {
		return r, nil
	}
	if err == gorm.ErrRecordNotFound {
		return models.CrmPoolRule{}, nil
	}
	return models.CrmPoolRule{}, err
}

// SetRule 设置租户公海规则（upsert）。
func (s CrmPoolService) SetRule(ctx context.Context, rule models.CrmPoolRule) (models.CrmPoolRule, error) {
	tenantID := identity.TenantID(ctx)

	var r models.CrmPoolRule
	err := models.MustFor(ctx).First(&r).Error
	if err == nil {
		r.RecycleDays = rule.RecycleDays
		r.NoDealDays = rule.NoDealDays
		r.Enabled = rule.Enabled
		r.ClaimLimit = rule.ClaimLimit
		r.ClaimCooldownDays = rule.ClaimCooldownDays
		r.ProtectDays = rule.ProtectDays
		err = models.DB.Save(&r).Error
		return r, err
	}
	if err != gorm.ErrRecordNotFound {
		return r, err
	}
	r = models.CrmPoolRule{
		Model:             models.Model{ID: id_generator.StringID()},
		TenantID:          tenantID,
		RecycleDays:       rule.RecycleDays,
		NoDealDays:        rule.NoDealDays,
		Enabled:           rule.Enabled,
		ClaimLimit:        rule.ClaimLimit,
		ClaimCooldownDays: rule.ClaimCooldownDays,
		ProtectDays:       rule.ProtectDays,
	}
	err = models.DB.Create(&r).Error
	return r, err
}

// AutoRecycleAll 遍历所有租户执行自动回收（定时任务调用）。
func (s CrmPoolService) AutoRecycleAll() {
	var ts []models.Tenant
	if err := models.DB.Find(&ts).Error; err != nil {
		log.Sugar.Errorw("pool auto recycle: list tenants failed", "err", err)
		return
	}
	for _, t := range ts {
		n, err := s.AutoRecycle(identity.WithIdentity(context.Background(), identity.Identity{TenantID: t.ID}))
		if err != nil {
			log.Sugar.Errorw("pool auto recycle failed", "tenant", t.ID, "err", err)
			continue
		}
		if n > 0 {
			log.Sugar.Infow("pool auto recycled", "tenant", t.ID, "count", n)
		}
	}
}

// ---------- 公海操作记录 ----------

// ListLogs 查询公海操作记录，支持按客户ID筛选。
func (s CrmPoolService) ListLogs(ctx context.Context, customerID string, pager *app.Pager) ([]models.CrmPoolLog, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx).Model(&models.CrmPoolLog{})
	if customerID != "" {
		db = db.Where("customer_id = ?", customerID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmPoolLog
	err := db.Order("created_at desc").Offset(pager.GetOffset()).Limit(pager.GetLimit()).Find(&items).Error
	return items, total, err
}

// writePoolLog 写入公海操作记录。
func (s CrmPoolService) writePoolLog(tenantID, customerID, customerName, action, operatorID, operatorName, targetUserID, remark string) {
	logEntry := models.CrmPoolLog{
		Model:        models.Model{ID: id_generator.StringID()},
		TenantID:     tenantID,
		CustomerID:   customerID,
		CustomerName: customerName,
		Action:       action,
		OperatorID:   operatorID,
		OperatorName: operatorName,
		TargetUserID: targetUserID,
		Remark:       remark,
	}
	if err := models.DB.Create(&logEntry).Error; err != nil {
		log.Sugar.Errorw("write pool log failed", "err", err)
	}
}

// ---------- 公海统计 ----------

// PoolStats 公海池统计概览。
type PoolStats struct {
	TotalInPool   int64 `json:"total_in_pool"`  // 当前公海客户数
	ClaimedToday  int64 `json:"claimed_today"`  // 今日认领数
	RecycledToday int64 `json:"recycled_today"` // 今日回收数
	AssignedToday int64 `json:"assigned_today"` // 今日分配数
}

// GetStats 获取公海池统计概览。
func (s CrmPoolService) GetStats(ctx context.Context) (PoolStats, error) {
	tenantID := identity.TenantID(ctx)
	var stats PoolStats

	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).
		Where("pool_status = ?", 1).Count(&stats.TotalInPool)

	today := time.Now().Format("2006-01-02")
	models.DB.Model(&models.CrmPoolLog{}).Scopes(models.TenantScope(tenantID)).
		Where("action = ? AND created_at >= ?", "claim", today).Count(&stats.ClaimedToday)
	models.DB.Model(&models.CrmPoolLog{}).Scopes(models.TenantScope(tenantID)).
		Where("action = ? AND created_at >= ?", "recycle", today).Count(&stats.RecycledToday)
	models.DB.Model(&models.CrmPoolLog{}).Scopes(models.TenantScope(tenantID)).
		Where("action = ? AND created_at >= ?", "assign", today).Count(&stats.AssignedToday)

	return stats, nil
}

// ---------- 辅助函数 ----------

// hasActiveOpportunity 判断客户是否有进行中的商机。
func hasActiveOpportunity(ctx context.Context, customerID string) bool {
	var count int64
	models.MustFor(ctx).Model(&models.CrmOpportunity{}).
		Where("customer_id = ? AND status NOT IN ?", customerID, []string{"won", "lost"}).Count(&count)
	return count > 0
}
