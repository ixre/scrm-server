package services

import (
	"context"
	"encoding/json"
	"errors"

	"core/app/models"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrCustomerAccess = errors.New("客户不存在或无权操作")

// customerActor 从数据库确认有效身份，不信任客户端提交的角色或归属人。
func customerActor(ctx context.Context, db *gorm.DB) (models.User, bool, error) {
	var user models.User
	tenant, uid := identity.TenantID(ctx), identity.UserID(ctx)
	if tenant == "" || uid == "" {
		return user, false, ErrCustomerAccess
	}
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND status = ?", tenant, uid, 1).First(&user).Error; err != nil {
		return user, false, ErrCustomerAccess
	}
	var role models.CrmRole
	if user.RoleID == "" {
		return user, false, nil
	}
	err := db.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenant, user.RoleID).First(&role).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return user, false, err
	}
	return user, err == nil && (role.Code == "admin" || role.Code == "manager"), nil
}

// customerAccessDB 统一客户列表、详情、导出和写操作的数据范围。
// 普通销售可读取自己的及共享客户，只有负责人可写；管理员/经理可管理本租户客户。
func customerAccessDB(ctx context.Context, db *gorm.DB, write bool) (*gorm.DB, error) {
	user, manager, err := customerActor(ctx, db)
	if err != nil {
		return nil, err
	}
	return customerAccessForActor(ctx, db, user, manager, write), nil
}

func customerAccessForActor(ctx context.Context, db *gorm.DB, user models.User, manager, write bool) *gorm.DB {
	q := db.WithContext(ctx).Where("tenant_id = ?", user.TenantID)
	if !manager {
		if write {
			q = q.Where("owner_user_id = ?", user.ID)
		} else {
			shared, _ := json.Marshal([]string{user.ID})
			q = q.Where("(owner_user_id = ? OR shared_staff_ids @> ?::jsonb)", user.ID, string(shared))
		}
	}
	return q
}

func validateCustomerOwner(ctx context.Context, db *gorm.DB, owner string) error {
	user, manager, err := customerActor(ctx, db)
	if err != nil {
		return err
	}
	if !manager && owner != "" && owner != user.ID {
		return ErrCustomerAccess
	}
	if owner == "" {
		return nil
	}
	var target models.User
	if err := db.WithContext(ctx).Where("tenant_id = ? AND id = ? AND status = ?", user.TenantID, owner, 1).First(&target).Error; err != nil {
		return ErrCustomerAccess
	}
	return nil
}

// updateCustomers 用于客户扩展操作：整个批次校验、加锁、更新，避免部分越权更新。
func updateCustomers(ctx context.Context, ids []string, changes map[string]any) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	var count int64
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		db, err := customerAccessDB(ctx, tx, true)
		if err != nil {
			return err
		}
		var rows []models.CrmCustomer
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		unique := make(map[string]bool)
		for _, id := range ids {
			unique[id] = true
		}
		if len(rows) != len(unique) {
			return ErrCustomerAccess
		}
		res := tx.Model(&models.CrmCustomer{}).Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Updates(changes)
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}
