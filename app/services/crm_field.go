package services

import (
	"context"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"errors"
	"gorm.io/gorm"
	"strings"
)

type CrmFieldService struct{}

func NewCrmField() *CrmFieldService { return &CrmFieldService{} }

func (s CrmFieldService) ListDefs(ctx context.Context, entityType string) ([]models.CrmFieldDef, error) {
	if err := requireEntityOperation(ctx, models.DB, entityType, "read", "read"); err != nil {
		return nil, err
	}
	var rows []models.CrmFieldDef
	err := models.MustFor(ctx).Where("entity_type=?", entityType).Order("sort asc,id desc").Find(&rows).Error
	return rows, err
}

func (s CrmFieldService) CreateDef(ctx context.Context, d *models.CrmFieldDef) error {
	if d == nil || strings.TrimSpace(d.FieldKey) == "" || strings.TrimSpace(d.FieldLabel) == "" {
		return errors.New("字段标识和名称不能为空")
	}
	if err := requireEntityModule(ctx, models.DB, d.EntityType); err != nil {
		return err
	}
	d.ID, d.TenantID = id_generator.StringID(), identity.TenantID(ctx)
	d.Timestamp = models.Timestamp{}
	d.FieldKey, d.FieldLabel = strings.TrimSpace(d.FieldKey), strings.TrimSpace(d.FieldLabel)
	if d.Status == "" {
		d.Status = "active"
	}
	return createCRM(models.DB.WithContext(ctx), d)
}

func (s CrmFieldService) UpdateDef(ctx context.Context, id string, d *models.CrmFieldDef) error {
	if d == nil || id == "" || strings.TrimSpace(d.FieldLabel) == "" {
		return errors.New("字段定义、ID 和名称不能为空")
	}
	var current models.CrmFieldDef
	if err := models.MustFor(ctx).Where("id=?", id).First(&current).Error; err != nil {
		return err
	}
	if err := requireEntityModule(ctx, models.DB, current.EntityType); err != nil {
		return err
	}
	if d.EntityType != "" && d.EntityType != current.EntityType || d.FieldKey != "" && d.FieldKey != current.FieldKey {
		return errors.New("字段所属业务和标识不可修改")
	}
	return models.MustFor(ctx).Model(&current).Updates(map[string]any{"field_label": strings.TrimSpace(d.FieldLabel), "field_type": d.FieldType, "options": d.Options, "is_required": d.IsRequired, "is_searchable": d.IsSearchable, "default_value": d.DefaultValue, "sort": d.Sort, "status": d.Status}).Error
}

func (s CrmFieldService) DeleteDef(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	var rows []models.CrmFieldDef
	if err := models.MustFor(ctx).Where("id IN ?", ids).Find(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) != len(ids) {
		return 0, ErrPermissionDenied
	}
	for _, row := range rows {
		if err := requireEntityModule(ctx, models.DB, row.EntityType); err != nil {
			return 0, err
		}
	}
	result := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmFieldDef{})
	return result.RowsAffected, result.Error
}

func (s CrmFieldService) GetValues(ctx context.Context, entityType, entityID string) ([]models.CrmFieldValue, error) {
	var rows []models.CrmFieldValue
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := fieldParentAccess(ctx, tx, entityType, entityID, false); err != nil {
			return err
		}
		return tx.Where("tenant_id=? AND entity_type=? AND entity_id=?", identity.TenantID(ctx), entityType, entityID).Find(&rows).Error
	})
	return rows, err
}

func (s CrmFieldService) SetValue(ctx context.Context, entityType, entityID, fieldKey, value string) error {
	if strings.TrimSpace(fieldKey) == "" {
		return errors.New("字段标识不能为空")
	}
	return s.BatchSetValues(ctx, entityType, entityID, map[string]string{fieldKey: value})
}

// Locking the parent serializes field writes, including creation of the first
// value, without allowing field endpoints to bypass parent ownership or rights.
func (s CrmFieldService) BatchSetValues(ctx context.Context, entityType, entityID string, values map[string]string) error {
	if entityID == "" {
		return errors.New("实体 ID 不能为空")
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := fieldParentAccess(ctx, tx, entityType, entityID, true); err != nil {
			return err
		}
		for key, value := range values {
			key = strings.TrimSpace(key)
			if key == "" {
				return errors.New("字段标识不能为空")
			}
			var row models.CrmFieldValue
			err := tx.Where("tenant_id=? AND entity_type=? AND entity_id=? AND field_key=?", identity.TenantID(ctx), entityType, entityID, key).First(&row).Error
			if err == nil {
				if err := tx.Model(&row).Update("value", value).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			row = models.CrmFieldValue{Model: models.Model{ID: id_generator.StringID()}, TenantID: identity.TenantID(ctx), EntityType: entityType, EntityID: entityID, FieldKey: key, Value: value}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
