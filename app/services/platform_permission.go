package services

import (
	"context"
	"core/app/constants"
	"core/app/models"
	"core/common/identity"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"strings"
)

func HasPlatformPermission(perms []string, point string) bool {
	parts := strings.Split(point, ":")
	if len(parts) != 3 || parts[0] != "platform" {
		return false
	}
	for _, grant := range perms {
		if grant == point || grant == "platform:*:full" || grant == "platform:"+parts[1]+":full" || grant == "platform:*:"+parts[2] {
			return true
		}
	}
	return false
}

func RequirePlatformPermission(ctx context.Context, point string) (string, error) {
	id := identity.PlatformID(ctx)
	if !identity.IsPlatform(ctx) || id == "" {
		return "", ErrPermissionDenied
	}
	var admin models.PlatformAdmin
	if err := models.DB.WithContext(ctx).Where("id=? AND status=1", id).First(&admin).Error; err != nil {
		return "", ErrPermissionDenied
	}
	if !HasPlatformPermission(admin.Perms, point) {
		parts := strings.Split(point, ":")
		mode := "write"
		if len(parts) == 3 && (strings.HasPrefix(parts[2], "list") || strings.HasPrefix(parts[2], "get") || parts[2] == "overview" || strings.Contains(parts[2], "usage")) {
			mode = "read"
		}
		if len(parts) != 3 || !HasPlatformPermission(admin.Perms, "platform:"+parts[1]+":"+mode) {
			return "", ErrPermissionDenied
		}
	}
	return id, nil
}

func platformPresetPermissions(role string) constants.StringArrayField {
	return append(constants.StringArrayField{}, platformPermissionPresets[role]...)
}

var platformPermissionPattern = regexp.MustCompile(`^platform:(\*|[a-z][a-z0-9_]*):([a-z][a-z0-9_]*)$`)

func platformActor(ctx context.Context) (models.PlatformAdmin, error) {
	var actor models.PlatformAdmin
	if !identity.IsPlatform(ctx) || identity.PlatformID(ctx) == "" {
		return actor, ErrPermissionDenied
	}
	if err := models.DB.WithContext(ctx).Where("id=? AND status=1", identity.PlatformID(ctx)).First(&actor).Error; err != nil {
		return actor, ErrPermissionDenied
	}
	return actor, nil
}

func grantablePlatformPermissions(ctx context.Context, role string, requested *[]string) (constants.StringArrayField, error) {
	actor, err := platformActor(ctx)
	if err != nil {
		return nil, err
	}
	if !crmRoleCodePattern.MatchString(role) {
		return nil, errors.New("平台角色编码无效")
	}
	grants := platformPresetPermissions(role)
	if requested != nil {
		grants = append(constants.StringArrayField{}, (*requested)...)
	} else if _, ok := platformPermissionPresets[role]; !ok {
		return nil, errors.New("自定义平台角色必须指定权限点")
	}
	if len(grants) > 500 {
		return nil, errors.New("权限点过多")
	}
	for _, point := range grants {
		if !platformPermissionPattern.MatchString(point) || !HasPlatformPermission(actor.Perms, point) {
			return nil, errors.New("不能授予超出自身范围的平台权限")
		}
	}
	return grants, nil
}

func manageablePlatformAdmin(ctx context.Context, id string) (models.PlatformAdmin, error) {
	actor, err := platformActor(ctx)
	if err != nil {
		return actor, err
	}
	var target models.PlatformAdmin
	if err := models.DB.WithContext(ctx).Where("id=?", id).First(&target).Error; err != nil {
		return target, errors.New("账号不存在")
	}
	for _, point := range target.Perms {
		if !HasPlatformPermission(actor.Perms, point) {
			return target, errors.New("不能管理权限高于自己的平台账号")
		}
	}
	return target, nil
}

// Serialize root-account changes so two administrators cannot concurrently
// remove the last remaining root. Root capability is a permission, not a name.
func mutatePlatformAdmin(ctx context.Context, id string, changes map[string]any, remove bool) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext('platform-root-accounts'))").Error; err != nil {
			return err
		}
		var target models.PlatformAdmin
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id=?", id).First(&target).Error; err != nil {
			return err
		}
		losingRoot := remove
		if status, ok := changes["status"].(int); ok && status != 1 {
			losingRoot = true
		}
		if perms, ok := changes["perms"].(constants.StringArrayField); ok && !HasPlatformPermission(perms, "platform:*:full") {
			losingRoot = true
		}
		if target.Status == 1 && HasPlatformPermission(target.Perms, "platform:*:full") && losingRoot {
			var count int64
			if err := tx.Model(&models.PlatformAdmin{}).Where("id<>? AND status=1 AND perms @> ?::jsonb", id, `["platform:*:full"]`).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return errors.New("不能删除、停用或降权最后一个平台管理员")
			}
		}
		if remove {
			return tx.Delete(&target).Error
		}
		return tx.Model(&target).Updates(changes).Error
	})
}
