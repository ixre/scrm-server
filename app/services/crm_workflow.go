package services

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func workflowIDs(ids []string) ([]string, error) {
	unique := map[string]bool{}
	for _, id := range ids {
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("记录 ID 不能为空")
		}
		unique[id] = true
	}
	if len(unique) == 0 {
		return nil, errors.New("请先选择记录")
	}
	out := make([]string, 0, len(unique))
	for id := range unique {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func lockTenantUser(ctx context.Context, tx *gorm.DB, id string) (models.User, error) {
	var user models.User
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND status = ?", identity.TenantID(ctx), id, 1).First(&user).Error
	if err != nil {
		return user, errors.New("销售不存在、已停用或不属于本企业")
	}
	return user, nil
}

func poolLog(tx *gorm.DB, tenant string, c models.CrmCustomer, action string, actor models.User, target, remark string) error {
	entry := models.CrmPoolLog{Model: models.Model{ID: id_generator.StringID()}, TenantID: tenant, CustomerID: c.ID, CustomerName: c.Name, Action: action, OperatorID: actor.ID, OperatorName: actor.Name, TargetUserID: target, Remark: remark}
	return createCRM(tx, &entry)
}

func recycledCustomerChanges(c models.CrmCustomer, now time.Time) map[string]any {
	return map[string]any{"pool_status": 1, "pool_at": now, "owner_user_id": nil, "prev_owner_user_id": c.OwnerUserID, "claimed_at": nil, "reassign_status": 0, "reassign_from": nil, "shared_staff_ids": constants.StringArrayField{}}
}

func (s CrmPoolService) takePool(ctx context.Context, target string, ids []string, assign bool) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	tenant := identity.TenantID(ctx)
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		if (assign && !manager) || (!assign && target != actor.ID) {
			return ErrCustomerAccess
		}
		// Serialize grants to the same sales account, including concurrent claims of different customers.
		if _, err = lockTenantUser(ctx, tx, target); err != nil {
			return err
		}
		var customers []models.CrmCustomer
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN ?", tenant, ids).Order("id").Find(&customers).Error; err != nil {
			return err
		}
		if len(customers) != len(ids) {
			return ErrCustomerAccess
		}
		var rule models.CrmPoolRule
		if err = tx.Where("tenant_id = ?", tenant).First(&rule).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if rule.ClaimLimit > 0 {
			var count int64
			if err = tx.Model(&models.CrmCustomer{}).Where("tenant_id = ? AND owner_user_id = ? AND pool_status = 0", tenant, target).Count(&count).Error; err != nil {
				return err
			}
			if count+int64(len(ids)) > int64(rule.ClaimLimit) {
				return errors.New("超过客户持有上限，请减少领取数量或联系管理员调整规则")
			}
		}
		now := time.Now()
		for _, c := range customers {
			if c.PoolStatus != 1 && !(c.PoolStatus == 0 && c.OwnerUserID == "" && c.ReassignStatus == 0) {
				return errors.New("所选客户已被领取，请刷新后重试")
			}
			if c.IsBlacklist {
				return errors.New("黑名单客户不可领取，请先解除黑名单")
			}
			if !assign && rule.ClaimCooldownDays > 0 && c.PrevOwnerUserID == target && c.PoolAt != nil && c.PoolAt.AddDate(0, 0, rule.ClaimCooldownDays).After(now) {
				return errors.New("客户仍处于前负责人领取冷却期")
			}
			action := "claim"
			if assign {
				action = "assign"
			}
			if err = poolLog(tx, tenant, c, action, actor, target, ""); err != nil {
				return err
			}
		}
		return tx.Model(&models.CrmCustomer{}).Where("tenant_id = ? AND id IN ?", tenant, ids).Updates(map[string]any{"pool_status": 0, "pool_at": nil, "owner_user_id": target, "claimed_at": now, "reassign_status": 0, "reassign_from": nil}).Error
	})
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		NewCrmSop().Trigger(ctx, "pool_claim", map[string]string{"customer_id": id, "pool_type": "customer"})
	}
	return int64(len(ids)), nil
}

const availablePoolCustomers = "(pool_status = 1 OR (pool_status = 0 AND owner_user_id IS NULL AND reassign_status = 0))"
