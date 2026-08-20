package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// CrmSceneService 列表场景（自定义筛选条件集合）服务。
type CrmSceneService struct{}

func NewCrmScene() *CrmSceneService { return &CrmSceneService{} }

// List 列出当前用户自己的场景 + 团队共享的场景，按 sort 升序、id 降序。
func (s CrmSceneService) List(ctx context.Context, userID, entityType string) ([]models.CrmScene, error) {


	var items []models.CrmScene
	err := models.MustFor(ctx).Where("entity_type = ? AND (user_id = ? OR is_shared = ?)",  entityType,  userID,  true).
		Order("sort asc, id desc").Find(&items).Error
	return items, err
}

// Create 创建场景。
func (s CrmSceneService) Create(ctx context.Context, userID string, sc *models.CrmScene) error {
	tenantID := identity.TenantID(ctx)


	if sc == nil {
		return errors.New("场景不能为空")
	}
	if strings.TrimSpace(sc.Name) == "" {
		return errors.New("场景名称不能为空")
	}
	if sc.EntityType == "" {
		return errors.New("entity_type 不能为空")
	}
	sc.ID = id_generator.StringID()
	sc.TenantID = tenantID
	sc.UserID = userID
	sc.Name = strings.TrimSpace(sc.Name)
	// 若设为默认，先取消同 entity_type 的其他默认。
	if sc.IsDefault {
		if err := s.clearDefault(ctx, userID, sc.EntityType); err != nil {
			return err
		}
	}
	return models.DB.Create(sc).Error
}

// Update 更新场景（仅归属用户可改）。
func (s CrmSceneService) Update(ctx context.Context, userID, id string, sc *models.CrmScene) error {
	tenantID := identity.TenantID(ctx)


	if sc == nil {
		return errors.New("场景不能为空")
	}
	if strings.TrimSpace(sc.Name) == "" {
		return errors.New("场景名称不能为空")
	}
	// 仅归属用户可编辑。
	var exist models.CrmScene
	if err := models.MustFor(ctx).Where("id = ?",  id).First(&exist).Error; err != nil {
		return err
	}
	if exist.UserID != userID {
		return errors.New("无权限编辑他人场景")
	}
	m := map[string]any{
		"name":       strings.TrimSpace(sc.Name),
		"conditions": sc.Conditions,
		"is_shared":  sc.IsShared,
		"is_default": sc.IsDefault,
		"sort":       sc.Sort,
	}
	if sc.IsDefault {
		if err := s.clearDefault(ctx, userID, sc.EntityType, id); err != nil {
			return err
		}
	}
	if err := models.DB.Model(&models.CrmScene{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND user_id = ?",  id,  userID).
		Updates(m).Error; err != nil {
		log.Sugar.Errorw("crm scene update failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除场景（仅归属用户可删）。
func (s CrmSceneService) Delete(ctx context.Context, userID string, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ? AND user_id = ?",  ids,  userID).
		Delete(&models.CrmScene{})
	return res.RowsAffected, res.Error
}

// SetDefault 设为默认场景（同 entity_type 其他取消默认）。
func (s CrmSceneService) SetDefault(ctx context.Context, userID, id string) error {
	tenantID := identity.TenantID(ctx)


	var sc models.CrmScene
	if err := models.MustFor(ctx).Where("id = ?",  id).First(&sc).Error; err != nil {
		return err
	}
	if sc.UserID != userID {
		return errors.New("无权限设置他人场景为默认")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		// 取消同 entity_type 当前用户的其他默认。
		if err := tx.Model(&models.CrmScene{}).Scopes(models.TenantScope(tenantID)).Where("entity_type = ? AND user_id = ? AND id <> ?",  sc.EntityType,  userID,  id).
			Update("is_default", false).Error; err != nil {
			return err
		}
		// 设当前为默认。
		return tx.Model(&models.CrmScene{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
			Update("is_default", true).Error
	})
}

// DragSort 按 ids 顺序更新 sort（ids[0]->0, ids[1]->1 ...）。
func (s CrmSceneService) DragSort(ctx context.Context, userID string, ids []string) error {
	tenantID := identity.TenantID(ctx)


	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		for i, id := range ids {
			if err := tx.Model(&models.CrmScene{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND user_id = ?",  id,  userID).
				Update("sort", i).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// clearDefault 取消同 entity_type 当前用户的默认场景。
// excludeID 为空时取消全部；非空时排除该 id（用于更新场景时先清其他再写自身）。
func (s CrmSceneService) clearDefault(ctx context.Context, userID, entityType string, excludeIDs ...string) error {
	tenantID := identity.TenantID(ctx)


	q := models.DB.Model(&models.CrmScene{}).Scopes(models.TenantScope(tenantID)).Where("entity_type = ? AND user_id = ? AND is_default = ?",  entityType,  userID,  true)
	if len(excludeIDs) > 0 && excludeIDs[0] != "" {
		q = q.Where("id <> ?", excludeIDs[0])
	}
	return q.Update("is_default", false).Error
}
