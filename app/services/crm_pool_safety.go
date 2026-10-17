package services

import (
	"context"
	"core/app/models"
	"core/common/id_generator"
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validatePoolRule(r models.CrmPoolRule) error {
	for _, days := range []int{r.RecycleDays, r.NoDealDays, r.ClaimCooldownDays, r.ProtectDays} {
		if days < 0 || days > 36500 {
			return errors.New("规则天数须为0至36500的整数")
		}
	}
	if r.ClaimLimit < 0 || r.ClaimLimit > 1000000 {
		return errors.New("客户持有上限须为0至1000000")
	}
	if r.Enabled && r.RecycleDays == 0 && r.NoDealDays == 0 {
		return errors.New("启用自动回收时至少设置一项回收天数")
	}
	return nil
}

// SetRule is an atomic upsert. Authorization also applies to internal callers.
func (s CrmPoolService) SetRule(ctx context.Context, input models.CrmPoolRule) (models.CrmPoolRule, error) {
	var saved models.CrmPoolRule
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user, _, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		var role models.CrmRole
		if err := tx.Where("tenant_id = ? AND id = ? AND code = 'admin'", user.TenantID, user.RoleID).First(&role).Error; err != nil {
			return errors.New("公海规则仅管理员可修改")
		}
		if err := validatePoolRule(input); err != nil {
			return err
		}
		saved = models.CrmPoolRule{Model: models.Model{ID: id_generator.StringID()}, TenantID: user.TenantID, RecycleDays: input.RecycleDays, NoDealDays: input.NoDealDays, Enabled: input.Enabled, ClaimLimit: input.ClaimLimit, ClaimCooldownDays: input.ClaimCooldownDays, ProtectDays: input.ProtectDays}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "tenant_id"}}, DoUpdates: clause.AssignmentColumns([]string{"recycle_days", "no_deal_days", "enabled", "claim_limit", "claim_cooldown_days", "protect_days", "updated_at"})}, clause.Returning{}).Create(&saved).Error
	})
	return saved, err
}

// Private customer histories follow current read access; pool histories are shared.
func poolLogScope(ctx context.Context) (*gorm.DB, error) {
	actor, manager, err := customerActor(ctx, models.DB)
	if err != nil {
		return nil, err
	}
	q := models.DB.WithContext(ctx).Model(&models.CrmPoolLog{}).Where("tenant_id = ?", actor.TenantID)
	if !manager {
		shared, _ := json.Marshal([]string{actor.ID})
		q = q.Where(`EXISTS (SELECT 1 FROM crm_customer c WHERE c.id = crm_pool_log.customer_id
            AND c.tenant_id = crm_pool_log.tenant_id AND c.deleted_at IS NULL
            AND (`+availablePoolCustomers+` OR owner_user_id = ? OR shared_staff_ids @> ?::jsonb))`, actor.ID, string(shared))
	}
	return q, nil
}

// Get exposes only a currently available pool customer, without broadening private customer access.
func (s CrmPoolService) Get(ctx context.Context, id string) (models.CrmCustomer, error) {
	var customer models.CrmCustomer
	actor, _, err := customerActor(ctx, models.DB)
	if err != nil {
		return customer, err
	}
	err = models.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", actor.TenantID, id).Where(availablePoolCustomers).First(&customer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return customer, errors.New("客户已被领取、不在公海或无权查看，请刷新列表")
	}
	return customer, err
}
