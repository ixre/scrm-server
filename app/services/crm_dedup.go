package services

import (
	"errors"

	"core/app/models"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 客户去重 ----------

type CrmDedupService struct{}

func NewCrmDedup() *CrmDedupService { return &CrmDedupService{} }

// FindDuplicates 按 mobile 查同租户重复客户（mobile 非空）。
func (s CrmDedupService) FindDuplicates(tenantID, mobile string) ([]models.CrmCustomer, error) {
	if mobile == "" {
		return nil, errors.New("mobile 不能为空")
	}
	var cs []models.CrmCustomer
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("mobile = ?",  mobile).
		Order("created_at").Find(&cs).Error
	return cs, err
}

// Merge 合并客户：把 duplicateIDs 的所有关联数据转移到 primaryID，然后软删除重复客户。
// 全部在事务内完成。primaryID 自身不被删。
func (s CrmDedupService) Merge(tenantID, primaryID string, duplicateIDs []string) error {
	if primaryID == "" {
		return errors.New("primary_id 不能为空")
	}
	if len(duplicateIDs) == 0 {
		return errors.New("duplicate_ids 不能为空")
	}
	for _, id := range duplicateIDs {
		if id == primaryID {
			return errors.New("duplicate_ids 不能包含 primary_id")
		}
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		// 1) 迁移关联数据 customer_id → primaryID
		tables := []struct {
			Table string
			Col   string
		}{
			{"crm_followup", "customer_id"},
			{"crm_opportunity", "customer_id"},
			{"crm_order", "customer_id"},
			{"crm_activity_record", "customer_id"},
			{"crm_channel_interaction", "customer_id"},
			{"crm_transfer_log", "customer_id"},
			{"crm_coupon_record", "customer_id"},
			{"crm_seckill_record", "customer_id"},
			{"crm_lottery_record", "customer_id"},
		}
		for _, t := range tables {
			if err := tx.Table(t.Table).
				Scopes(models.TenantScope(tenantID)).
				Where(t.Col+" IN ?", duplicateIDs).
				Update(t.Col, primaryID).Error; err != nil {
				return err
			}
		}
		// 2) ChannelLink 迁移：重复客户的渠道映射转移到 primary（若 primary 无该渠道映射）。
		var dupLinks []models.ChannelLink
		if err := tx.Where("channel = ? AND entity_type = ? AND entity_id IN ?",
			"customer", duplicateIDs).Find(&dupLinks).Error; err != nil {
			// ChannelLink 表名无 tenant 隔离字段直接用 entity_id
		}
		for _, dl := range dupLinks {
			// 检查 primary 是否已有同渠道映射
			var existing models.ChannelLink
			err := tx.Where("channel = ? AND entity_type = ? AND entity_id = ?",
				dl.Channel, "customer", primaryID).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				// primary 无该渠道映射 → 迁移
				if err := tx.Model(&models.ChannelLink{}).Where("id = ?", dl.ID).
					Update("entity_id", primaryID).Error; err != nil {
					return err
				}
			} else if err == nil {
				// primary 已有该渠道映射 → 删除重复映射
				if err := tx.Delete(&models.ChannelLink{}, dl.ID).Error; err != nil {
					return err
				}
			}
		}
		// 3) 软删除重复客户
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  duplicateIDs).
			Delete(&models.CrmCustomer{}).Error; err != nil {
			return err
		}
		log.Sugar.Infow("crm customer merged", "tenant", tenantID, "primary", primaryID, "duplicates", duplicateIDs)
		return nil
	})
}

// pickPrimary 从重复客户中选主记录：优先选有归属销售(owner非空)且非公海的，其次选最新创建的。
// 纯函数，供单测。
func pickPrimary(cs []models.CrmCustomer) string {
	if len(cs) == 0 {
		return ""
	}
	// 第一优先：有 owner 且非公海
	for _, c := range cs {
		if c.OwnerUserID != "" && c.PoolStatus == 0 {
			return c.ID
		}
	}
	// 第二优先：最新创建的（cs 已按 created_at 升序，取最后一个）
	return cs[len(cs)-1].ID
}
