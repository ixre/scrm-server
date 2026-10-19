package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strconv"
	"strings"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"
)

// ---------- 自动打标签 ----------

type CrmAutoTagService struct{}

func NewCrmAutoTag() *CrmAutoTagService { return &CrmAutoTagService{} }

// AutoTagRuleCreateReq 自动打标签规则创建请求。
type AutoTagRuleCreateReq struct {
	Rule models.CrmAutoTagRule `json:"rule"`
}

// CreateRule 创建自动打标签规则。
func (s CrmAutoTagService) CreateRule(ctx context.Context, req models.CrmAutoTagRule) (models.CrmAutoTagRule, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.TriggerType == "" || req.TagName == "" {
		return req, errors.New("trigger_type/tag_name 不能为空")
	}
	if err := createCRM(models.DB, &req); err != nil {
		log.Sugar.Errorw("auto tag rule create failed", "err", err)
		return req, err
	}
	return req, nil
}

// QueryRules 列出自动打标签规则。
func (s CrmAutoTagService) QueryRules(ctx context.Context, status string, pager *app.Pager) ([]models.CrmAutoTagRule, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmAutoTagRule{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmAutoTagRule
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// DeleteRules 删除规则。
func (s CrmAutoTagService) DeleteRules(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmAutoTagRule{})
	return res.RowsAffected, res.Error
}

// ApplyToCustomer 对单个客户应用所有匹配的自动打标签规则。
func (s CrmAutoTagService) ApplyToCustomer(ctx context.Context, customerID string) error {

	var rules []models.CrmAutoTagRule
	models.MustFor(ctx).Where("status = ?", "active").Find(&rules)
	var cust models.CrmCustomer
	if err := models.MustFor(ctx).Where("id = ?", customerID).First(&cust).Error; err != nil {
		return err
	}
	tagsToAdd := []string{}
	for _, rule := range rules {
		if s.matchRule(rule, cust) {
			// 避免重复打标
			already := false
			for _, t := range cust.ChannelTags {
				if t == rule.TagName {
					already = true
					break
				}
			}
			if !already {
				tagsToAdd = append(tagsToAdd, rule.TagName)
			}
		}
	}
	if len(tagsToAdd) > 0 {
		newTags := append(cust.ChannelTags, tagsToAdd...)
		models.MustFor(ctx).Model(&models.CrmCustomer{}).Where("id = ?", customerID).
			Update("channel_tags", newTags)
		log.Sugar.Infow("auto tag applied", "customer", customerID, "tags", tagsToAdd)
	}
	return nil
}

// matchRule 检查规则是否匹配客户。
func (s CrmAutoTagService) matchRule(rule models.CrmAutoTagRule, cust models.CrmCustomer) bool {
	// 渠道过滤
	if rule.Channel != "" && cust.Source != rule.Channel {
		return false
	}
	switch rule.TriggerType {
	case "interaction_count":
		// 触发条件格式：>=N
		threshold := parseThreshold(rule.TriggerCond)
		if threshold <= 0 {
			return false
		}
		var cnt int64
		models.DB.Model(&models.CrmChannelInteraction{}).Scopes(models.TenantScope(cust.TenantID)).Where("customer_id = ?", cust.ID).Count(&cnt)
		return cnt >= int64(threshold)
	case "order_amount":
		// 触发条件格式：>=N
		threshold := parseThreshold(rule.TriggerCond)
		if threshold <= 0 {
			return false
		}
		var amt struct {
			Total float64 `gorm:"column:total"`
		}
		models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(cust.TenantID)).Where("customer_id = ? AND status IN ?", cust.ID, []string{"signed", "completed"}).
			Select("COALESCE(SUM(amount),0) AS total").Scan(&amt)
		return amt.Total >= float64(threshold)
	case "has_followup":
		var cnt int64
		models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(cust.TenantID)).Where("customer_id = ?", cust.ID).Count(&cnt)
		return cnt > 0
	default:
		return false
	}
}

// parseThreshold 解析触发条件中的阈值（格式 ">=3" → 3）。
func parseThreshold(cond string) int {
	cond = strings.TrimSpace(cond)
	cond = strings.TrimPrefix(cond, ">=")
	cond = strings.TrimPrefix(cond, ">")
	cond = strings.TrimPrefix(cond, "=")
	n, _ := strconv.Atoi(strings.TrimSpace(cond))
	return n
}

// ApplyAll 对所有客户应用自动打标签（定时调用）。
func (s CrmAutoTagService) ApplyAll(ctx context.Context) {

	var customers []models.CrmCustomer
	models.MustFor(ctx).Find(&customers)
	for _, c := range customers {
		s.ApplyToCustomer(ctx, c.ID)
	}
}
