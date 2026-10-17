package services

import (
	"context"
	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"core/common/log"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

const poolRecycleBatchSize = 200
const recyclableCustomers = "pool_status = 0 AND owner_user_id IS NOT NULL AND is_locked = false AND reassign_status = 0"

// recycleEligible applies tenant-scoped existence checks; ids restrict the locked-batch recheck.
func recycleEligible(tx *gorm.DB, tenant string, ids []string, rule models.CrmPoolRule, now time.Time) *gorm.DB {
	q := tx.Model(&models.CrmCustomer{}).Where("tenant_id = ?", tenant).Where(recyclableCustomers)
	if len(ids) > 0 {
		q = q.Where("id IN ?", ids)
	}
	if rule.ProtectDays > 0 {
		q = q.Where("claimed_at IS NULL OR claimed_at <= ?", now.AddDate(0, 0, -rule.ProtectDays))
	}
	conditions := tx.Session(&gorm.Session{NewDB: true})
	if rule.RecycleDays > 0 {
		cutoff := now.AddDate(0, 0, -rule.RecycleDays)
		conditions = conditions.Or(`GREATEST(crm_customer.created_at, crm_customer.claimed_at) < ? AND NOT EXISTS (SELECT 1 FROM crm_followup f WHERE f.tenant_id = crm_customer.tenant_id AND f.customer_id = crm_customer.id AND f.deleted_at IS NULL AND f.created_at >= ?)`, cutoff, cutoff)
	}
	if rule.NoDealDays > 0 {
		conditions = conditions.Or(`GREATEST(crm_customer.created_at, crm_customer.claimed_at) < ? AND NOT EXISTS (SELECT 1 FROM crm_opportunity o WHERE o.tenant_id = crm_customer.tenant_id AND o.customer_id = crm_customer.id AND o.deleted_at IS NULL AND o.status NOT IN ('won','lost')) AND NOT EXISTS (SELECT 1 FROM crm_order o WHERE o.tenant_id = crm_customer.tenant_id AND o.customer_id = crm_customer.id AND o.deleted_at IS NULL AND o.status IN ('signed','completed'))`, now.AddDate(0, 0, -rule.NoDealDays))
	}
	return q.Where(conditions)
}

// AutoRecycle commits bounded batches. An error returns the number already committed.
// Locked rows are skipped and will be reconsidered by the next scheduled run.
func (s CrmPoolService) AutoRecycle(ctx context.Context) (int64, error) {
	tenant := identity.TenantID(ctx)
	if tenant == "" {
		return 0, errors.New("缺少租户")
	}
	if identity.UserID(ctx) != "" {
		if err := requireCRMManager(ctx, models.DB); err != nil {
			return 0, err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	var upper string
	if err := models.DB.WithContext(ctx).Model(&models.CrmCustomer{}).Select("COALESCE(MAX(id),0)::text").Where("tenant_id = ?", tenant).Where(recyclableCustomers).Scan(&upper).Error; err != nil {
		return 0, err
	}
	cursor := "0"
	var count int64
	for {
		var ids []string
		var changed int64
		stop := false
		err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var rule models.CrmPoolRule
			err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ?", tenant).First(&rule).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				stop = true
				return nil
			}
			if err != nil {
				return err
			}
			if !rule.Enabled {
				stop = true
				return nil
			}
			if err := validatePoolRule(rule); err != nil {
				return err
			}
			// Only one bounded batch is held. The ID boundary prevents chasing new inserts forever.
			if err := recycleEligible(tx, tenant, nil, rule, time.Now()).Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Select("id").Where("id > ? AND id <= ?", cursor, upper).Order("id").Limit(poolRecycleBatchSize).Find(&ids).Error; err != nil {
				return err
			}
			if len(ids) == 0 {
				stop = true
				return nil
			}
			// A fresh statement after acquiring locks sees followups committed before the lock.
			var customers []models.CrmCustomer
			now := time.Now()
			if err := recycleEligible(tx, tenant, ids, rule, now).Select("id, name").Find(&customers).Error; err != nil {
				return err
			}
			if len(customers) == 0 {
				return nil
			}
			logs := make([]models.CrmPoolLog, 0, len(customers))
			selected := make([]string, 0, len(customers))
			for _, c := range customers {
				selected = append(selected, c.ID)
				logs = append(logs, models.CrmPoolLog{Model: models.Model{ID: id_generator.StringID()}, TenantID: tenant, CustomerID: c.ID, CustomerName: c.Name, Action: "recycle", OperatorName: "系统自动回收", Remark: "按公海规则自动回收"})
			}
			res := tx.Model(&models.CrmCustomer{}).Where("tenant_id = ? AND id IN ?", tenant, selected).Updates(map[string]any{"pool_status": 1, "pool_at": now, "prev_owner_user_id": gorm.Expr("owner_user_id"), "owner_user_id": nil, "claimed_at": nil, "reassign_status": 0, "reassign_from": nil, "shared_staff_ids": constants.StringArrayField{}})
			if res.Error != nil {
				return res.Error
			}
			if err := tx.Create(&logs).Error; err != nil {
				return err
			}
			changed = res.RowsAffected
			return nil
		})
		if err != nil {
			return count, err
		}
		count += changed
		if stop {
			return count, nil
		}
		cursor = ids[len(ids)-1]
	}
}

// AutoRecycleAll bounds tenant reads and isolates each tenant's execution deadline.
func (s CrmPoolService) AutoRecycleAll() {
	cursor := "0"
	for {
		var tenants []models.Tenant
		now := time.Now()
		if err := models.DB.Where("id > ? AND status = 1 AND (expire_at IS NULL OR expire_at > ?)", cursor, now).Order("id").Limit(100).Find(&tenants).Error; err != nil {
			log.Sugar.Errorw("pool auto recycle: list tenants failed", "err", err)
			return
		}
		if len(tenants) == 0 {
			return
		}
		for _, tenant := range tenants {
			n, err := s.AutoRecycle(identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenant.ID}))
			if err != nil {
				log.Sugar.Errorw("pool auto recycle failed", "tenant", tenant.ID, "committed", n, "err", err)
			} else if n > 0 {
				log.Sugar.Infow("pool auto recycled", "tenant", tenant.ID, "count", n)
			}
		}
		cursor = tenants[len(tenants)-1].ID
	}
}
