package services

import (
	"core/common/identity"

	"context"

	"errors"
	"time"

	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// ---------- 销售规则（阶段流转校验） ----------

type CrmSalesRuleService struct{}

func NewCrmSalesRule() *CrmSalesRuleService { return &CrmSalesRuleService{} }

// GetRule 读取租户销售规则，不存在则返回默认（不强制校验）。
func (s CrmSalesRuleService) GetRule(ctx context.Context) (models.CrmSalesRule, error) {

	var r models.CrmSalesRule
	err := models.MustFor(ctx).First(&r).Error
	if err == nil {
		return r, nil
	}
	if err == gorm.ErrRecordNotFound {
		return models.CrmSalesRule{FollowupDays: 3}, nil
	}
	return models.CrmSalesRule{}, err
}

// SetRule 设置租户销售规则（upsert）。
func (s CrmSalesRuleService) SetRule(ctx context.Context, requireFollowup bool, followupDays int) (models.CrmSalesRule, error) {
	tenantID := identity.TenantID(ctx)

	if followupDays < 1 {
		followupDays = 3
	}
	var r models.CrmSalesRule
	err := models.MustFor(ctx).First(&r).Error
	if err == nil {
		r.RequireFollowup = requireFollowup
		r.FollowupDays = followupDays
		err = models.DB.Save(&r).Error
		return r, err
	}
	if err != gorm.ErrRecordNotFound {
		return r, err
	}
	r = models.CrmSalesRule{
		Model:           models.Model{ID: id_generator.StringID()},
		TenantID:        tenantID,
		RequireFollowup: requireFollowup,
		FollowupDays:    followupDays,
	}
	err = models.DB.Create(&r).Error
	return r, err
}

// hasRecentFollowup 判断跟进时间列表中是否有在近 N 天内的。纯函数，供单测。
func hasRecentFollowup(followupTimes []time.Time, days int, now time.Time) bool {
	if days <= 0 {
		return true
	}
	threshold := now.AddDate(0, 0, -days)
	for _, t := range followupTimes {
		if !t.Before(threshold) {
			return true
		}
	}
	return false
}

// checkMoveStageGuard 校验商机移动阶段的前置条件（跟进强制）。
// 返回 nil 表示通过，返回 error 表示拒绝。
func checkMoveStageGuard(ctx context.Context, opportunityID string) error {
	return checkMoveStageGuardDB(ctx, models.DB, opportunityID)
}

// itoa 简易 int→string，避免引入 strconv 仅为单处用。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Use the caller's transaction and fail closed if the rule cannot be read.
func checkMoveStageGuardDB(ctx context.Context, tx *gorm.DB, opportunityID string) error {
	var rule models.CrmSalesRule
	err := tx.Where("tenant_id = ?", identity.TenantID(ctx)).First(&rule).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !rule.RequireFollowup {
		return nil
	}
	days := rule.FollowupDays
	if days < 1 {
		days = 3
	}
	var count int64
	now := time.Now()
	if err = tx.Model(&models.CrmFollowup{}).Where("tenant_id = ? AND opportunity_id = ? AND created_at >= ? AND created_at <= ?", identity.TenantID(ctx), opportunityID, now.AddDate(0, 0, -days), now).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("近" + itoa(days) + "天无跟进记录，无法移动阶段")
	}
	return nil
}
