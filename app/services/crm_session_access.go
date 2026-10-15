package services

import (
	"context"
	"errors"
	"time"

	"core/app/models"
	"core/common/identity"
)

// ValidateBusinessSession rechecks account state on every authenticated invocation.
// JWT validity alone cannot revoke disabled accounts or tenants.
func ValidateBusinessSession(ctx context.Context, allowExpired bool) (int32, error) {
	if identity.IsPlatform(ctx) {
		var admin models.PlatformAdmin
		if identity.PlatformID(ctx) == "" || models.DB.WithContext(ctx).Where("id = ? AND status = 1", identity.PlatformID(ctx)).First(&admin).Error != nil {
			return 401, errors.New("平台账号已停用或登录已失效")
		}
		return 0, nil
	}
	if identity.TenantID(ctx) == "" || identity.UserID(ctx) == "" {
		return 401, errors.New("请重新登录")
	}
	var user models.User
	if err := models.DB.WithContext(ctx).Where("tenant_id = ? AND id = ? AND status = 1", identity.TenantID(ctx), identity.UserID(ctx)).First(&user).Error; err != nil {
		return 401, errors.New("账号已停用或登录已失效")
	}
	var tenant models.Tenant
	if err := models.DB.WithContext(ctx).Where("id = ? AND status = 1", user.TenantID).First(&tenant).Error; err != nil {
		return 403, errors.New("企业已停用，请联系管理员")
	}
	if !allowExpired && tenant.ExpireAt != nil && !tenant.ExpireAt.After(time.Now()) {
		return 403, errors.New("企业套餐已到期，请先续费")
	}
	return 0, nil
}
