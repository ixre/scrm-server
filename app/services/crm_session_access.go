package services

import (
	"context"
	"errors"
	"strconv"
	"time"

	"core/app/models"
	"core/common/identity"
)

// ValidateBusinessSession rechecks account state on every authenticated invocation.
// JWT validity alone cannot revoke disabled accounts or tenants.
func ValidateBusinessSession(ctx context.Context, allowExpired bool, sessionVersion ...string) (int32, error) {
	if identity.IsPlatform(ctx) {
		var admin models.PlatformAdmin
		if identity.PlatformID(ctx) == "" || models.DB.WithContext(ctx).Where("id = ? AND status = 1", identity.PlatformID(ctx)).First(&admin).Error != nil {
			return 401, errors.New("平台账号已停用或登录已失效")
		}
		if len(sessionVersion) > 0 && sessionVersion[0] != strconv.FormatInt(admin.AuthVersion, 10) {
			return 401, errors.New("登录已失效，请重新登录")
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
	if len(sessionVersion) > 0 && sessionVersion[0] != strconv.FormatInt(user.AuthVersion, 10) {
		return 401, errors.New("登录已失效，请重新登录")
	}
	var tenant models.Tenant
	if err := models.DB.WithContext(ctx).Where("id = ? AND status = 1", user.TenantID).First(&tenant).Error; err != nil {
		return 403, errors.New("企业已停用，请联系管理员")
	}
	if !allowExpired && tenant.ExpireAt != nil && !tenant.ExpireAt.After(time.Now()) {
		// 试用/限期免费到期自动降级免费版；付费套餐到期仍拦截续费。
		if ok, _ := models.AutoDowngradeExpired(models.DB.WithContext(ctx), &tenant); ok {
			return 0, nil
		}
		return 403, models.ErrTenantExpired
	}
	return 0, nil
}
