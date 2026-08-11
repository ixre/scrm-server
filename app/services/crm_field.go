package services

import (
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CrmFieldService 动态字段/自定义表单服务。
// 字段定义(CrmFieldDef)按实体类型(customer/lead/opportunity/order)组织，
// 字段值(CrmFieldValue)按 (entity_type, entity_id, field_key) 唯一存储，支持 upsert。
type CrmFieldService struct{}

func NewCrmField() *CrmFieldService { return &CrmFieldService{} }

// ListDefs 按实体类型列出字段定义，按 sort 升序、id 降序排列。
func (s CrmFieldService) ListDefs(tenantID, entityType string) ([]models.CrmFieldDef, error) {
	var items []models.CrmFieldDef
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("entity_type = ?",  entityType).
		Order("sort asc, id desc").Find(&items).Error
	return items, err
}

// CreateDef 新建字段定义。
func (s CrmFieldService) CreateDef(tenantID string, d *models.CrmFieldDef) error {
	if d == nil {
		return errors.New("字段定义不能为空")
	}
	if strings.TrimSpace(d.FieldKey) == "" {
		return errors.New("字段标识不能为空")
	}
	if strings.TrimSpace(d.FieldLabel) == "" {
		return errors.New("字段名称不能为空")
	}
	if d.EntityType == "" {
		return errors.New("实体类型不能为空")
	}
	d.ID = id_generator.StringID()
	d.TenantID = tenantID
	d.FieldKey = strings.TrimSpace(d.FieldKey)
	d.FieldLabel = strings.TrimSpace(d.FieldLabel)
	if d.Status == "" {
		d.Status = "active"
	}
	return createCRM(models.DB, d)
}

// UpdateDef 更新字段定义。
func (s CrmFieldService) UpdateDef(tenantID, id string, d *models.CrmFieldDef) error {
	if d == nil {
		return errors.New("字段定义不能为空")
	}
	if id == "" {
		return errors.New("id 不能为空")
	}
	m := map[string]any{
		"entity_type":   d.EntityType,
		"field_key":     d.FieldKey,
		"field_label":   d.FieldLabel,
		"field_type":    d.FieldType,
		"options":       d.Options,
		"is_required":   d.IsRequired,
		"is_searchable": d.IsSearchable,
		"default_value": d.DefaultValue,
		"sort":          d.Sort,
		"status":        d.Status,
	}
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// DeleteDef 批量删除字段定义。
func (s CrmFieldService) DeleteDef(tenantID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmFieldDef{})
	return res.RowsAffected, res.Error
}

// GetValues 取某实体的全部动态字段值。
func (s CrmFieldService) GetValues(tenantID, entityType, entityID string) ([]models.CrmFieldValue, error) {
	var items []models.CrmFieldValue
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("entity_type = ? AND entity_id = ?",  entityType,  entityID).
		Find(&items).Error
	return items, err
}

// SetValue upsert 单个字段值（按 tenant+entity+field_key 唯一）。
func (s CrmFieldService) SetValue(tenantID, entityType, entityID, fieldKey, value string) error {
	if entityType == "" || entityID == "" || fieldKey == "" {
		return errors.New("entity_type/entity_id/field_key 不能为空")
	}
	var existing models.CrmFieldValue
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("entity_type = ? AND entity_id = ? AND field_key = ?",  entityType,  entityID,  fieldKey).First(&existing).Error
	if err == nil {
		return models.DB.Model(&existing).Update("value", value).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	v := &models.CrmFieldValue{
		TenantID:   tenantID,
		EntityType: entityType,
		EntityID:   entityID,
		FieldKey:   fieldKey,
		Value:      value,
	}
	v.ID = id_generator.StringID()
	return models.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(v).Error
}

// BatchSetValues 批量 upsert 字段值。
func (s CrmFieldService) BatchSetValues(tenantID, entityType, entityID string, values map[string]string) error {
	if entityType == "" || entityID == "" {
		return errors.New("entity_type/entity_id 不能为空")
	}
	if len(values) == 0 {
		return nil
	}
	tx := models.DB.Begin()
	for k, val := range values {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		var existing models.CrmFieldValue
		err := tx.Scopes(models.TenantScope(tenantID)).Where("entity_type = ? AND entity_id = ? AND field_key = ?",  entityType,  entityID,  k).First(&existing).Error
		if err == nil {
			if e := tx.Model(&existing).Update("value", val).Error; e != nil {
				tx.Rollback()
				return e
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			tx.Rollback()
			return err
		}
		v := &models.CrmFieldValue{
			TenantID:   tenantID,
			EntityType: entityType,
			EntityID:   entityID,
			FieldKey:   k,
			Value:      val,
		}
		v.ID = id_generator.StringID()
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(v).Error; e != nil {
			tx.Rollback()
			return e
		}
	}
	return tx.Commit().Error
}
