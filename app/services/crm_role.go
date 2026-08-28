package services

import (
	"context"
	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"regexp"
	"strings"
)

type CRMRoleInput struct {
	Code  string   `json:"code"`
	Name  string   `json:"name"`
	Perms []string `json:"perms"`
}

var crmPermissionPattern = regexp.MustCompile(`^crm:(\*|[a-z][a-z0-9_]*):([a-z][a-z0-9_]*)$`)
var crmRoleCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func CRMRoleOptions(ctx context.Context) ([]map[string]string, error) {
	if err := RequireCRMPermission(ctx, "crm:user:read"); err != nil {
		return nil, err
	}
	perms, err := crmActorPermissions(ctx, models.DB)
	if err != nil {
		return nil, err
	}
	var roles []models.CrmRole
	if err := models.MustFor(ctx).Order("id").Find(&roles).Error; err != nil {
		return nil, err
	}
	options := []map[string]string{}
	for _, role := range roles {
		allowed := true
		for _, point := range role.Perms {
			if !HasCRMPermission(perms, point) {
				allowed = false
				break
			}
		}
		if allowed {
			options = append(options, map[string]string{"id": role.ID, "code": role.Code, "name": role.Name})
		}
	}
	return options, nil
}

func ListCRMRoles(ctx context.Context) ([]models.CrmRole, error) {
	if err := RequireCRMPermission(ctx, "crm:role:read"); err != nil {
		return nil, err
	}
	var rows []models.CrmRole
	err := models.MustFor(ctx).Order("id").Find(&rows).Error
	return rows, err
}

func SaveCRMRole(ctx context.Context, id string, input CRMRoleInput) (models.CrmRole, error) {
	var row models.CrmRole
	if err := RequireCRMPermission(ctx, "crm:role:manage"); err != nil {
		return row, err
	}
	input.Code, input.Name = strings.TrimSpace(input.Code), strings.TrimSpace(input.Name)
	if !crmRoleCodePattern.MatchString(input.Code) || input.Name == "" || len(input.Name) > 128 || len(input.Perms) > 500 {
		return row, errors.New("角色编码、名称或权限点无效")
	}
	perms, err := crmActorPermissions(ctx, models.DB)
	if err != nil {
		return row, err
	}
	seen := map[string]bool{}
	grants := constants.StringArrayField{}
	for _, point := range input.Perms {
		if !crmPermissionPattern.MatchString(point) || !HasCRMPermission(perms, point) {
			return row, errors.New("不能授予无效或超出自身范围的权限点")
		}
		if !seen[point] {
			grants = append(grants, point)
			seen[point] = true
		}
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if id == "" {
			row = models.CrmRole{Model: models.Model{ID: id_generator.StringID()}, TenantID: identity.TenantID(ctx), Code: input.Code, Name: input.Name, Perms: grants}
			return tx.Create(&row).Error
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND id=?", identity.TenantID(ctx), id).First(&row).Error; err != nil {
			return err
		}
		var actor models.User
		if err := tx.Where("tenant_id=? AND id=?", identity.TenantID(ctx), identity.UserID(ctx)).First(&actor).Error; err != nil {
			return err
		}
		for _, point := range row.Perms {
			if !HasCRMPermission(perms, point) {
				return errors.New("不能修改超出自身权限的角色")
			}
		}
		if actor.RoleID == row.ID {
			return errors.New("不能修改自己正在使用的角色，请由另一位授权管理员操作")
		}
		if row.Code != input.Code {
			return errors.New("角色编码不可修改")
		}
		row.Name, row.Perms = input.Name, grants
		return tx.Model(&row).Updates(map[string]any{"name": row.Name, "perms": row.Perms}).Error
	})
	return row, err
}

func DeleteCRMRole(ctx context.Context, id string) error {
	if err := RequireCRMPermission(ctx, "crm:role:manage"); err != nil {
		return err
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var role models.CrmRole
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND id=?", identity.TenantID(ctx), id).First(&role).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.User{}).Where("tenant_id=? AND role_id=?", role.TenantID, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("角色仍有账号使用，不能删除")
		}
		return tx.Delete(&role).Error
	})
}
