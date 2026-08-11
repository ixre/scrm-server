package services

import (
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 客户公海池 ----------

type CrmPoolService struct{}

func NewCrmPool() *CrmPoolService { return &CrmPoolService{} }

// ListPool 公海客户列表（pool_status=1），按租户隔离，支持关键字搜索。
func (s CrmPoolService) ListPool(tenantID, keyword string, pager *app.Pager) ([]models.CrmCustomer, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("pool_status = ?",  1)
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
func (s CrmPoolService) Claim(tenantID, userID string, customerIDs []string) (int64, error) {
	if len(customerIDs) == 0 {
		return 0, nil
	}
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ? AND pool_status = ?",  customerIDs,  1).
		Updates(map[string]any{"pool_status": 0, "pool_at": nil, "owner_user_id": userID})
	if res.Error != nil {
		log.Sugar.Errorw("crm pool claim failed", "err", res.Error)
	}
	// Phase 4：公海认领触发 SOP
	if res.RowsAffected > 0 {
		sop := NewCrmSop()
		for _, cid := range customerIDs {
			sop.Trigger(tenantID, "pool_claim", map[string]string{"customer_id": cid})
		}
	}
	return res.RowsAffected, res.Error
}

// Recycle 手动回收客户进公海：pool_status→1，owner 清空，pool_at=now。
func (s CrmPoolService) Recycle(tenantID string, customerIDs []string) (int64, error) {
	if len(customerIDs) == 0 {
		return 0, nil
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id IN ? AND pool_status = ?",  customerIDs,  0).
		Updates(map[string]any{"pool_status": 1, "pool_at": now, "owner_user_id": ""})
	if res.Error != nil {
		log.Sugar.Errorw("crm pool recycle failed", "err", res.Error)
	}
	return res.RowsAffected, res.Error
}

// AutoRecycle 自动回收：按租户公海规则，将超 N 天未跟进的非公海客户回收。
// 遍历该租户所有 owner 非空的客户，若最近一次跟进距今 > recycleDays 则回收。
func (s CrmPoolService) AutoRecycle(tenantID string) (int64, error) {
	rule, err := s.GetRule(tenantID)
	if err != nil || !rule.Enabled || rule.RecycleDays <= 0 {
		return 0, nil
	}
	threshold := time.Now().AddDate(0, 0, -rule.RecycleDays)

	// 查所有非公海、有归属的客户，取每客户最新跟进时间。
	var cs []models.CrmCustomer
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("pool_status = ? AND owner_user_id <> ''",  0).Find(&cs).Error; err != nil {
		return 0, err
	}
	var toRecycle []string
	for _, c := range cs {
		lastFollowup := lastFollowupTime(tenantID, c.ID)
		if shouldRecycle(lastFollowup, threshold) {
			toRecycle = append(toRecycle, c.ID)
		}
	}
	if len(toRecycle) == 0 {
		return 0, nil
	}
	return s.Recycle(tenantID, toRecycle)
}

// lastFollowupTime 取某客户最新跟进时间，无跟进则返回零值。
func lastFollowupTime(tenantID, customerID string) time.Time {
	var f models.CrmFollowup
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("customer_id = ?",  customerID).
		Order("created_at desc").Limit(1).First(&f).Error; err != nil {
		return time.Time{}
	}
	return f.CreatedAt
}

// shouldRecycle 判断是否应回收：最近跟进时间早于阈值（或无跟进且客户创建时间早于阈值）。
// 纯函数，供单测。
func shouldRecycle(lastFollowup time.Time, threshold time.Time) bool {
	if lastFollowup.IsZero() {
		return false // 无跟进记录不自动回收（靠手动回收或创建时进公海）
	}
	return lastFollowup.Before(threshold)
}

// ---------- 公海规则 ----------

// GetRule 读取租户公海规则，不存在则返回默认（不自动回收）。
func (s CrmPoolService) GetRule(tenantID string) (models.CrmPoolRule, error) {
	var r models.CrmPoolRule
	err := models.DB.Scopes(models.TenantScope(tenantID)).First(&r).Error
	if err == nil {
		return r, nil
	}
	if err == gorm.ErrRecordNotFound {
		return models.CrmPoolRule{}, nil
	}
	return models.CrmPoolRule{}, err
}

// SetRule 设置租户公海规则（upsert）。
func (s CrmPoolService) SetRule(tenantID string, recycleDays int, enabled bool) (models.CrmPoolRule, error) {
	var r models.CrmPoolRule
	err := models.DB.Scopes(models.TenantScope(tenantID)).First(&r).Error
	if err == nil {
		r.RecycleDays = recycleDays
		r.Enabled = enabled
		err = models.DB.Save(&r).Error
		return r, err
	}
	if err != gorm.ErrRecordNotFound {
		return r, err
	}
	r = models.CrmPoolRule{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		RecycleDays: recycleDays,
		Enabled:     enabled,
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
		n, err := s.AutoRecycle(t.ID)
		if err != nil {
			log.Sugar.Errorw("pool auto recycle failed", "tenant", t.ID, "err", err)
			continue
		}
		if n > 0 {
			log.Sugar.Infow("pool auto recycled", "tenant", t.ID, "count", n)
		}
	}
}
