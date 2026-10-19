package services

import (
	"core/common/identity"

	"context"

	"errors"
	"reflect"

	"core/app/models"
	"gorm.io/gorm/clause"

	"gorm.io/gorm"
)

// ---------- 客户去重 ----------

type CrmDedupService struct{}

func NewCrmDedup() *CrmDedupService { return &CrmDedupService{} }

// FindDuplicates 按 mobile 查同租户重复客户（mobile 非空）。
func (s CrmDedupService) FindDuplicates(ctx context.Context, mobile string) ([]models.CrmCustomer, error) {

	if mobile == "" {
		return nil, errors.New("mobile 不能为空")
	}
	var cs []models.CrmCustomer
	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, err
	}
	err = db.Where("mobile = ?", mobile).
		Order("created_at").Find(&cs).Error
	return cs, err
}

// Merge 合并客户：把登记的客户关联数据转移到 primaryID，然后软删除重复客户。
// 全部在事务内完成。primaryID 自身不被删。
func (s CrmDedupService) Merge(ctx context.Context, primaryID string, duplicateIDs []string) error {
	tenantID := identity.TenantID(ctx)

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
	unique := make(map[string]bool)
	for _, id := range duplicateIDs {
		unique[id] = true
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 锁定并验证全部客户，拒绝跨租户/无权访问/不存在的 ID。
		db, err := customerAccessDB(ctx, tx, true)
		if err != nil {
			return err
		}
		ids := append([]string{primaryID}, duplicateIDs...)
		var customers []models.CrmCustomer
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&customers).Error; err != nil {
			return err
		}
		if len(customers) != len(unique)+1 {
			return ErrCustomerAccess
		}

		for _, rel := range customerMergeRelations {
			if err := tx.Unscoped().Model(reflect.New(reflect.TypeOf(rel.Model).Elem()).Interface()).Where("tenant_id = ? AND "+rel.Column+" IN ?", tenantID, duplicateIDs).Update(rel.Column, primaryID).Error; err != nil {
				return err
			}
		}
		if err := mergeActivityRecords(tx, tenantID, primaryID, duplicateIDs); err != nil {
			return err
		}
		// 同一客户可拥有同渠道的多个外部身份，全部保留，避免丢失历史互动映射。
		if err := tx.Unscoped().Model(&models.ChannelLink{}).Where("tenant_id = ? AND entity_type = ? AND entity_id IN ?", tenantID, "customer", duplicateIDs).Update("entity_id", primaryID).Error; err != nil {
			return err
		}
		return tx.Where("tenant_id = ? AND id IN ?", tenantID, duplicateIDs).Delete(&models.CrmCustomer{}).Error
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
		if c.OwnerUserID != "" && !c.InPool() {
			return c.ID
		}
	}
	// 第二优先：最新创建的（cs 已按 created_at 升序，取最后一个）
	return cs[len(cs)-1].ID
}

// customerMergeRelations 包含普通外键；有联合唯一键的活动记录单独合并。
var customerMergeRelations = []struct {
	Model  any
	Column string
}{
	{&models.CrmLead{}, "customer_id"},
	{&models.CrmLead{}, "convert_customer_id"},
	{&models.CrmOpportunity{}, "customer_id"},
	{&models.CrmFollowup{}, "customer_id"},
	{&models.CrmOrder{}, "customer_id"},
	{&models.CrmChannelInteraction{}, "customer_id"},
	{&models.CrmPoolLog{}, "customer_id"},
	{&models.CrmTransferLog{}, "customer_id"},
	{&models.CrmChannelTask{}, "customer_id"},
	{&models.CrmCouponRecord{}, "customer_id"},
	{&models.CrmFissionInvite{}, "inviter_customer_id"},
	{&models.CrmFissionInvite{}, "invitee_customer_id"},
	{&models.CrmSopTask{}, "customer_id"},
	{&models.CrmSeckillRecord{}, "customer_id"},
	{&models.CrmLotteryRecord{}, "customer_id"},
	{&models.CrmSensitiveAlert{}, "customer_id"},
	{&models.CrmCustomerLoss{}, "customer_id"},
	{&models.CrmReplyTimeout{}, "customer_id"},
	{&models.CrmBargainRecord{}, "customer_id"},
	{&models.CrmRedPacketRecord{}, "customer_id"},
	{&models.CrmSignInRecord{}, "customer_id"},
	{&models.CrmContact{}, "customer_id"},
	{&models.CrmReceivablesPlan{}, "customer_id"},
	{&models.CrmReceivables{}, "customer_id"},
	{&models.CrmQuote{}, "customer_id"},
	{&models.CrmSignin{}, "customer_id"},
	{&models.CrmConsume{}, "customer_id"},
	{&models.CrmProject{}, "customer_id"},
	{&models.CrmFinanceRecord{}, "customer_id"},
	{&models.CrmInvoice{}, "customer_id"},
	{&models.CrmWorkorder{}, "customer_id"},
	{&models.CrmCallRecord{}, "customer_id"},
	{&models.CrmAiAnalysis{}, "customer_id"},
	{&models.CrmAftersales{}, "customer_id"},
	{&models.CrmOtherIncome{}, "customer_id"},
	{&models.CrmOtherExpense{}, "customer_id"},
	{&models.CrmCallList{}, "customer_id"},
}

func mergeActivityRecords(tx *gorm.DB, tenant, primary string, duplicates []string) error {
	var records []models.CrmActivityRecord
	ids := append([]string{primary}, duplicates...)
	if err := tx.Unscoped().Where("tenant_id = ? AND customer_id IN ?", tenant, ids).Order("id").Find(&records).Error; err != nil {
		return err
	}
	groups := make(map[string][]models.CrmActivityRecord)
	for _, rec := range records {
		groups[rec.ActivityID] = append(groups[rec.ActivityID], rec)
	}
	for activity, group := range groups {
		winner := group[0]
		for _, rec := range group {
			if rec.CustomerID == primary {
				winner = rec
				break
			}
		}
		rank := recordRank(winner.RecordType)
		alive := !winner.DeletedAt.Valid
		for _, rec := range group {
			if recordRank(rec.RecordType) > rank {
				winner.RecordType = rec.RecordType
				rank = recordRank(rec.RecordType)
			}
			alive = alive || !rec.DeletedAt.Valid
			if rec.ID != winner.ID {
				if err := tx.Unscoped().Delete(&rec).Error; err != nil {
					return err
				}
			}
		}
		updates := map[string]any{"customer_id": primary, "record_type": winner.RecordType}
		if alive {
			updates["deleted_at"] = nil
		}
		if err := tx.Unscoped().Model(&models.CrmActivityRecord{}).Where("tenant_id = ? AND id = ?", tenant, winner.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := recomputeActivityCounters(tx, tenant, activity); err != nil {
			return err
		}
	}
	return nil
}
