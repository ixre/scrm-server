package services

import (
	"context"
	"errors"

	"core/app/models"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Sales records use the same owner/manager boundary as customers. Pool leads are readable before claiming.
func salesAccessDB(ctx context.Context, db *gorm.DB, includePool bool) (*gorm.DB, error) {
	actor, manager, err := customerActor(ctx, db)
	if err != nil {
		return nil, err
	}
	q := db.WithContext(ctx).Where("tenant_id = ?", actor.TenantID)
	if !manager {
		if includePool {
			q = q.Where("(owner_user_id = ? OR pool_status = 1)", actor.ID)
		} else {
			q = q.Where("owner_user_id = ?", actor.ID)
		}
	}
	return q, nil
}

func lockSalesRecord[T any](ctx context.Context, tx *gorm.DB, id string, includePool bool) (T, error) {
	var row T
	if id == "" {
		return row, errors.New("id 不能为空")
	}
	db, err := salesAccessDB(ctx, tx, includePool)
	if err != nil {
		return row, err
	}
	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = errors.New("记录不存在或无权操作")
	}
	return row, err
}

func activeSalesCustomer(ctx context.Context, tx *gorm.DB, id string) (models.CrmCustomer, error) {
	var c models.CrmCustomer
	if id == "" {
		return c, errors.New("请选择关联客户")
	}
	db, err := customerAccessDB(ctx, tx, true)
	if err != nil {
		return c, err
	}
	if err = db.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", id).First(&c).Error; err != nil {
		return c, ErrCustomerAccess
	}
	if c.PoolStatus == 1 || c.OwnerUserID == "" {
		return c, errors.New("请先领取客户")
	}
	if c.ReassignStatus == 1 {
		return c, errors.New("请先接收转交客户")
	}
	return c, nil
}

// Advisory locks serialize logical keys that may not have a row yet (mobile, document number).
func lockSalesKey(ctx context.Context, tx *gorm.DB, kind, key string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", kind+":"+identity.TenantID(ctx)+":"+key).Error
}
