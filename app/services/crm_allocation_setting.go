package services

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validateAllocation(tx *gorm.DB, tid string, a *models.CrmAutoAllocation) error {
	if a == nil {
		return errors.New("规则不能为空")
	}
	a.Name = strings.TrimSpace(a.Name)
	if a.Name == "" || utf8.RuneCountInString(a.Name) > 128 {
		return errors.New("规则名称须为1至128字")
	}
	if a.EntityType != "customer" && a.EntityType != "lead" {
		return errors.New("请选择客户或线索")
	}
	if a.Strategy != "round_robin" && a.Strategy != "weighted" && a.Strategy != "random" {
		return errors.New("分配策略无效")
	}
	ids, err := workflowIDs(a.TargetIDs)
	if err != nil {
		return errors.New("请选择有效目标销售")
	}
	if len(ids) > 1000 {
		return errors.New("单条规则最多配置1000个目标销售")
	}
	var n int64
	if err = tx.Model(&models.User{}).Where("tenant_id = ? AND id IN ? AND status = 1", tid, ids).Count(&n).Error; err != nil {
		return err
	}
	if n != int64(len(ids)) {
		return errors.New("目标销售已停用、不存在或不属于本租户")
	}
	// Retain configured order for round-robin execution.
	seen := map[string]bool{}
	unique := constants.StringArrayField{}
	for _, id := range a.TargetIDs {
		if !seen[id] {
			unique = append(unique, id)
			seen[id] = true
		}
	}
	a.TargetIDs = unique
	if a.Sources == nil {
		a.Sources = constants.StringArrayField{}
	}
	if a.ExecutionEnabled {
		return validateAllocationExecution(*a)
	}
	return nil
}
func (s CrmAutoAllocationService) Create(ctx context.Context, a *models.CrmAutoAllocation) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		tid := identity.TenantID(ctx)
		if err := validateAllocation(tx, tid, a); err != nil {
			return err
		}
		a.LastTargetID = ""
		enabled := a.IsEnabled
		a.ID, a.TenantID, a.Timestamp = id_generator.StringID(), tid, models.Timestamp{}
		if err := createCRM(tx, a); err != nil {
			return err
		}
		// GORM's default:true would otherwise override an explicitly disabled rule.
		if !enabled {
			if err := tx.Model(a).Update("is_enabled", false).Error; err != nil {
				return err
			}
			a.IsEnabled = false
		}
		return nil
	})
}
func (s CrmAutoAllocationService) Update(ctx context.Context, id string, a *models.CrmAutoAllocation) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		tid := identity.TenantID(ctx)
		if a == nil {
			return errors.New("规则不能为空")
		}
		var existing models.CrmAutoAllocation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tid, id).First(&existing).Error; err != nil {
			return errors.New("规则不存在或无权操作")
		}
		if a.Conditions == nil {
			a.Conditions = existing.Conditions
		}
		if a.Sources == nil {
			a.Sources = existing.Sources
		}
		if err := validateAllocation(tx, tid, a); err != nil {
			return err
		}
		changes := map[string]any{"entity_type": a.EntityType, "name": a.Name, "strategy": a.Strategy, "target_ids": a.TargetIDs, "is_enabled": a.IsEnabled, "priority": a.Priority, "execution_enabled": a.ExecutionEnabled, "sources": a.Sources}
		if a.Conditions != nil {
			changes["conditions"] = a.Conditions
		}
		res := tx.Model(&models.CrmAutoAllocation{}).Where("tenant_id = ? AND id = ?", tid, id).Updates(changes)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != 1 {
			return errors.New("规则不存在或无权操作")
		}
		return nil
	})
}
func (s CrmAutoAllocationService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		var rows []models.CrmAutoAllocation
		tid := identity.TenantID(ctx)
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN ?", tid, ids).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return errors.New("规则不存在或无权操作")
		}
		return tx.Where("tenant_id = ? AND id IN ?", tid, ids).Delete(&models.CrmAutoAllocation{}).Error
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}
func (s CrmAutoAllocationService) Query(ctx context.Context, req AutoAllocationQueryReq) ([]models.CrmAutoAllocation, int64, error) {
	if err := requireCRMManager(ctx, models.DB); err != nil {
		return nil, 0, err
	}
	pager := (&app.Pager{Page: req.Page, PageSize: req.PageSize}).SetDefault()
	q := models.MustFor(ctx).Model(&models.CrmAutoAllocation{})
	if req.EntityType != "" {
		q = q.Where("entity_type = ?", req.EntityType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.CrmAutoAllocation
	err := q.Order("priority desc, id desc").Offset(pager.GetOffset()).Limit(pager.GetLimit()).Find(&rows).Error
	return rows, total, err
}
func (s CrmAutoAllocationService) ToggleStatus(ctx context.Context, id string, enabled bool) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		tid := identity.TenantID(ctx)
		var rule models.CrmAutoAllocation
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tid, id).First(&rule).Error; err != nil {
			return errors.New("规则不存在或无权操作")
		}
		if enabled {
			if err := validateAllocation(tx, tid, &rule); err != nil {
				return err
			}
		}
		return tx.Model(&rule).Update("is_enabled", enabled).Error
	})
}
