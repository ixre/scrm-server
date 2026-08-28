package models

import (
	"context"
	"errors"

	"core/common/identity"

	"gorm.io/gorm"
)

// TenantScope 强制追加 tenant_id 过滤。
//
// 行为约定（fail closed）：
//   - tenantID 非空：追加 WHERE tenant_id = ?
//   - tenantID 空字符串：语句报 ErrMissingTenant，不会退化为全表访问
//
// 适用场景：service 层接收形参 tenantID 时，用 Scopes 替代手写 Where("tenant_id = ?", t)。
//
// 注意：本文件定义的 For(ctx) / MustFor(ctx) 只对"经 BusinessServer.Invoke 注入本地身份的 ctx"安全。
// 确需跨租户的路径（seed / tasks / consumers / callback 反查租户 / 平台超管）必须显式使用
// CrossTenant() / CrossTenantDB(tx)，否则会被租户守卫（tenant_guard.go）拒绝。
func TenantScope(tenantID string) func(*gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if tenantID == "" {
			_ = db.AddError(ErrMissingTenant)
			return db
		}
		return db.Where("tenant_id = ?", tenantID)
	}
}

// ErrMissingTenant ctx 中未注入本地身份时返回。
var ErrMissingTenant = errors.New("missing tenant identity in context (For ctx requires gRPC entry injection)")

// For 返回绑定 ctx 的 *gorm.DB，自动从 ctx 注入 tenant 过滤。
//
// 安全范围：ctx 由 internal/grpcserver/business.go 的 BusinessServer.Invoke 注入。
// 禁止在 seed/tasks/consumers/callback 等无 ctx 路径调用。
func For(ctx context.Context) (*gorm.DB, error) {
	tID := identity.TenantID(ctx)
	if tID == "" {
		return nil, ErrMissingTenant
	}
	return DB.WithContext(ctx).Scopes(TenantScope(tID)), nil
}

// MustFor 同 For 但 panic —— 用于已确认 ctx 必有租户的入口。
func MustFor(ctx context.Context) *gorm.DB {
	db, err := For(ctx)
	if err != nil {
		panic(err)
	}
	return db
}
