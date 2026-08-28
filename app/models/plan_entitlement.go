package models

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ErrTenantExpired 套餐到期哨兵错误：grpcserver 据此把业务码改写为 402，
// 与其他 403 拒绝区分，前端全局拦截后引导续费。
var ErrTenantExpired = errors.New("企业套餐已到期，请先续费")

// AutoDowngradeExpired 将已到期的试用/限期免费租户自动降级为默认永久免费版
// （飞书/Notion 等主流 freemium 行为：试用结束回落免费版而非锁定）。
// 付费套餐到期不降级，仍由续费拦截（402）处理，避免擅自降低已购权益。
// 幂等：以 (id, plan, 仍过期) 为条件原子更新；并发下降级只生效一次。
// 返回 true 表示租户当前未过期（本次或并发请求已完成降级）。
func AutoDowngradeExpired(db *gorm.DB, tenant *Tenant) (bool, error) {
	var current Plan
	if err := db.Session(&gorm.Session{NewDB: true}).Where("code = ?", tenant.Plan).First(&current).Error; err != nil {
		return false, err
	}
	limitedFree := current.IsFree == 1 && current.FreeDays > 0
	if current.IsTrial != 1 && !limitedFree {
		return false, nil
	}
	var free Plan
	if err := db.Session(&gorm.Session{NewDB: true}).
		Where("is_free = 1 AND free_days = 0 AND status = 1").
		Order("is_default DESC, sort_order ASC").First(&free).Error; err != nil {
		return false, err
	}
	res := db.Model(&Tenant{}).
		Where("id = ? AND plan = ? AND expire_at IS NOT NULL AND expire_at <= NOW()", tenant.ID, current.Code).
		Updates(map[string]any{
			"plan":          free.Code,
			"expire_at":     nil,
			"max_users":     free.MaxUsers,
			"max_customers": free.MaxCustomers,
		})
	if res.Error != nil {
		return false, res.Error
	}
	if res.RowsAffected == 0 {
		// 并发请求可能已完成降级：重读确认当前状态。
		var latest Tenant
		if err := db.Session(&gorm.Session{NewDB: true}).Where("id = ?", tenant.ID).First(&latest).Error; err != nil {
			return false, err
		}
		*tenant = latest
		return latest.ExpireAt == nil || latest.ExpireAt.After(time.Now()), nil
	}
	tenant.Plan = free.Code
	tenant.ExpireAt = nil
	tenant.MaxUsers = free.MaxUsers
	tenant.MaxCustomers = free.MaxCustomers
	return true, nil
}

// PlanEntitlement is resolved from persisted state on every request. Disabling a
// plan removes it from sale; it does not cancel subscriptions already paid for.
type PlanEntitlement struct {
	Modules      []string `json:"modules"`
	MaxUsers     int      `json:"max_users"`
	MaxCustomers int      `json:"max_customers"`
}

func PlanLimit(plan, tenant int) int {
	if tenant > 0 && (plan == 0 || tenant < plan) {
		return tenant
	}
	return plan
}

func TenantEntitlement(db *gorm.DB, tenantID string) (PlanEntitlement, error) {
	var result PlanEntitlement
	if tenantID == "" {
		return result, ErrMissingTenant
	}
	var tenant Tenant
	if err := db.Session(&gorm.Session{NewDB: true}).Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		return result, err
	}
	if tenant.Status != 1 {
		return result, errors.New("企业已停用")
	}
	if tenant.ExpireAt != nil && !tenant.ExpireAt.After(time.Now()) {
		ok, err := AutoDowngradeExpired(db, &tenant)
		if err != nil {
			return result, err
		}
		if !ok {
			return result, ErrTenantExpired
		}
	}
	var plan Plan
	if err := db.Session(&gorm.Session{NewDB: true}).Where("code = ?", tenant.Plan).First(&plan).Error; err != nil {
		return result, errors.New("企业套餐不存在，请联系平台管理员")
	}
	if err := json.Unmarshal([]byte(plan.Modules), &result.Modules); err != nil {
		return result, errors.New("套餐模块配置无效")
	}
	if plan.MaxUsers < 0 || plan.MaxCustomers < 0 || tenant.MaxUsers < 0 || tenant.MaxCustomers < 0 {
		return result, errors.New("套餐配额配置无效")
	}
	result.MaxUsers = PlanLimit(plan.MaxUsers, tenant.MaxUsers)
	result.MaxCustomers = PlanLimit(plan.MaxCustomers, tenant.MaxCustomers)
	return result, nil
}

func RequirePlanModules(db *gorm.DB, tenantID string, modules ...string) error {
	entitlement, err := TenantEntitlement(db, tenantID)
	if err != nil {
		return err
	}
	for _, required := range modules {
		found := false
		for _, enabled := range entitlement.Modules {
			if enabled == required {
				found = true
				break
			}
		}
		if !found {
			name := required
			for _, def := range PlanModuleDefs {
				if def.Key == required {
					name = def.Name
					break
				}
			}
			return fmt.Errorf("当前套餐未包含「%s」，请升级套餐", name)
		}
	}
	return nil
}
