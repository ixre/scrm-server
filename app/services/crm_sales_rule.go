package services

import (
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
func (s CrmSalesRuleService) GetRule(tenantID string) (models.CrmSalesRule, error) {
	var r models.CrmSalesRule
	err := models.DB.Scopes(models.TenantScope(tenantID)).First(&r).Error
	if err == nil {
		return r, nil
	}
	if err == gorm.ErrRecordNotFound {
		return models.CrmSalesRule{FollowupDays: 3}, nil
	}
	return models.CrmSalesRule{}, err
}

// SetRule 设置租户销售规则（upsert）。
func (s CrmSalesRuleService) SetRule(tenantID string, requireFollowup bool, followupDays int) (models.CrmSalesRule, error) {
	if followupDays < 1 {
		followupDays = 3
	}
	var r models.CrmSalesRule
	err := models.DB.Scopes(models.TenantScope(tenantID)).First(&r).Error
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
		Model:          models.Model{ID: id_generator.StringID()},
		TenantID:       tenantID,
		RequireFollowup: requireFollowup,
		FollowupDays:   followupDays,
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
func checkMoveStageGuard(tenantID, opportunityID string) error {
	rule, err := NewCrmSalesRule().GetRule(tenantID)
	if err != nil || !rule.RequireFollowup {
		return nil
	}
	var fs []models.CrmFollowup
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("opportunity_id = ?",  opportunityID).
		Order("created_at desc").Find(&fs).Error; err != nil {
		return err
	}
	times := make([]time.Time, 0, len(fs))
	for _, f := range fs {
		times = append(times, f.CreatedAt)
	}
	if !hasRecentFollowup(times, rule.FollowupDays, time.Now()) {
		return errors.New("近" + itoa(rule.FollowupDays) + "天无跟进记录，无法移动阶段")
	}
	return nil
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
