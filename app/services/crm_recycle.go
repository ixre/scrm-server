package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"fmt"

	"core/app/models"
	"core/common/app"
	"core/common/log"

	"gorm.io/gorm"
)

// RecycleQueryReq 回收站查询参数。
type RecycleQueryReq struct {
	Page     int
	PageSize int
}

func (r RecycleQueryReq) pager() *app.Pager {
	p := &app.Pager{Page: r.Page, PageSize: r.PageSize}
	p.SetDefault()
	return p
}

type CrmRecycleService struct{}

func NewCrmRecycle() *CrmRecycleService { return &CrmRecycleService{} }

// List 按实体类型分页查询回收站记录。
func (s CrmRecycleService) List(ctx context.Context, entityType string, req RecycleQueryReq) ([]models.CrmRecycle, int64, error) {


	p := req.pager()
	db := models.MustFor(ctx)
	if entityType != "" {
		db = db.Where("entity_type = ?", entityType)
	}
	var total int64
	if err := db.Model(&models.CrmRecycle{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmRecycle
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("created_at desc").Find(&items).Error
	return items, total, err
}

// Restore 恢复：把对应实体的 deleted_at 清空，删除回收站记录。
func (s CrmRecycleService) Restore(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	var rec models.CrmRecycle
	if err := models.MustFor(ctx).Where("id = ?", id).First(&rec).Error; err != nil {
		return err
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		// 清空对应实体的 deleted_at（恢复软删除记录）。
		if err := restoreEntity(tx, tenantID, rec.EntityType, rec.EntityID); err != nil {
			return err
		}
		// 删除回收站记录。
		return tx.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Delete(&models.CrmRecycle{}).Error
	})
}

// Clear 彻底删除：物理删除对应实体，删除回收站记录。
func (s CrmRecycleService) Clear(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	var rec models.CrmRecycle
	if err := models.MustFor(ctx).Where("id = ?", id).First(&rec).Error; err != nil {
		return err
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		// 物理删除对应实体（含已软删除记录）。
		if err := hardDeleteEntity(tx, tenantID, rec.EntityType, rec.EntityID); err != nil {
			return err
		}
		// 删除回收站记录。
		return tx.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Delete(&models.CrmRecycle{}).Error
	})
}

// DeleteRecord 从回收站删除记录（仅删回收站记录，不影响实体）。
func (s CrmRecycleService) DeleteRecord(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmRecycle{})
	if res.Error != nil {
		log.Sugar.Errorw("crm recycle delete record failed", "err", res.Error)
	}
	return res.RowsAffected, res.Error
}

// restoreEntity 清空指定实体的 deleted_at（恢复软删除）。
func restoreEntity(tx *gorm.DB, tenantID, entityType, entityID string) error {
	table, err := entityTable(entityType)
	if err != nil {
		return err
	}
	return tx.Table(table).Scopes(models.TenantScope(tenantID)).Where("id = ?", entityID).
		Update("deleted_at", nil).Error
}

// hardDeleteEntity 物理删除指定实体（含已软删除记录）。
func hardDeleteEntity(tx *gorm.DB, tenantID, entityType, entityID string) error {
	table, err := entityTable(entityType)
	if err != nil {
		return err
	}
	return tx.Unscoped().Table(table).Scopes(models.TenantScope(tenantID)).Where("id = ?", entityID).
		Delete(nil).Error
}

// entityTypeTable 实体类型→表名映射。
var entityTypeTable = map[string]string{
	"customer":    "crm_customer",
	"lead":        "crm_lead",
	"opportunity": "crm_opportunity",
	"order":       "crm_order",
	"contact":     "crm_contact",
}

func entityTable(entityType string) (string, error) {
	if t, ok := entityTypeTable[entityType]; ok {
		return t, nil
	}
	return "", fmt.Errorf("不支持的实体类型: %s", entityType)
}
