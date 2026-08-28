package services

import (
	"context"
	"core/app/models"
	"core/common/identity"
	"gorm.io/gorm"
	"strconv"
)

// Logging out must remain possible after subscription expiry or tenant disablement.
func ValidateLogoutSession(ctx context.Context, version string) (int32, error) {
	if identity.IsPlatform(ctx) {
		var admin models.PlatformAdmin
		if identity.PlatformID(ctx) == "" || models.DB.WithContext(ctx).Where("id=?", identity.PlatformID(ctx)).First(&admin).Error != nil || strconv.FormatInt(admin.AuthVersion, 10) != version {
			return 401, ErrPermissionDenied
		}
		return 0, nil
	}
	var user models.User
	if identity.TenantID(ctx) == "" || identity.UserID(ctx) == "" || models.DB.WithContext(ctx).Where("tenant_id=? AND id=?", identity.TenantID(ctx), identity.UserID(ctx)).First(&user).Error != nil || strconv.FormatInt(user.AuthVersion, 10) != version {
		return 401, ErrPermissionDenied
	}
	return 0, nil
}

// Logout revokes all sessions of this account across gateway replicas.
func Logout(ctx context.Context) error {
	if identity.IsPlatform(ctx) {
		return models.DB.WithContext(ctx).Model(&models.PlatformAdmin{}).Where("id=?", identity.PlatformID(ctx)).Update("auth_version", gorm.Expr("auth_version + 1")).Error
	}
	if identity.TenantID(ctx) == "" || identity.UserID(ctx) == "" {
		return ErrPermissionDenied
	}
	return models.MustFor(ctx).Model(&models.User{}).Where("id=?", identity.UserID(ctx)).Update("auth_version", gorm.Expr("auth_version + 1")).Error
}

func LogoutWecom(ctx context.Context, corpID, staffID string) error {
	return models.DB.WithContext(ctx).Model(&models.Staff{}).Where("ext_corp_id=? AND ext_id=?", corpID, staffID).Update("auth_version", gorm.Expr("auth_version + 1")).Error
}
