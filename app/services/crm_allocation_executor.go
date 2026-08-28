package services

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
)

func validateAllocationExecution(r models.CrmAutoAllocation) error {
	if r.EntityType != "customer" {
		return errors.New("当前自动执行仅支持新获客客户，线索规则可保存为配置")
	}
	if r.Strategy != "round_robin" && r.Strategy != "random" {
		return errors.New("自动执行仅支持轮询或随机策略")
	}
	if len(r.Conditions) > 0 {
		return errors.New("旧条件尚不支持自动执行，请新建来源规则，避免改变旧条件含义")
	}
	if len(r.TargetIDs) == 0 || len(r.TargetIDs) > 100 {
		return errors.New("执行规则须配置1至100名销售")
	}
	if len(r.Sources) > 2 {
		return errors.New("仅支持小红书和落地页来源")
	}
	for _, source := range r.Sources {
		if source != constants.ChannelXiaohongshu && source != constants.SourceWebForm {
			return errors.New("仅支持小红书和落地页来源")
		}
	}
	return nil
}

// allocateNewCustomer is called ONLY inside the transaction that inserts a new
// unowned acquisition customer. Existing customers are never reassigned here.
// Lock order: rule, sales sorted by ID, then newly inserted customer (not visible
// to other transactions). Existing-customer assignment must keep its own lock order.
func allocateNewCustomer(ctx context.Context, tx *gorm.DB, c *models.CrmCustomer) error {
	if c.OwnerUserID != "" || c.PoolStatus != 1 || c.IsLocked || c.IsBlacklist || c.ReassignStatus != 0 {
		return nil
	}
	source, _ := json.Marshal([]string{c.Source})
	var rule models.CrmAutoAllocation
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND entity_type = 'customer' AND is_enabled = true AND execution_enabled = true", c.TenantID).
		Where("sources IS NULL OR sources IN ('[]'::jsonb, 'null'::jsonb) OR sources @> ?::jsonb", string(source)).Order("priority DESC, id DESC").First(&rule).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	entry := models.CrmAllocationLog{Model: models.Model{ID: id_generator.StringID()}, TenantID: c.TenantID, CustomerID: c.ID, CustomerName: c.Name, RuleID: rule.ID, RuleName: rule.Name, Source: c.Source, Outcome: "pending", Reason: "无有效销售或销售客户持有量已达上限，保留在公海"}
	if err := validateAllocationExecution(rule); err != nil {
		entry.Reason = err.Error()
		return createCRM(tx, &entry)
	}
	// Serialize quota decisions with manual claims and other rules, avoiding global tenant locks.
	var users []models.User
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Select("id").Where("tenant_id = ? AND id IN ? AND status = 1", c.TenantID, []string(rule.TargetIDs)).Order("id").Find(&users).Error; err != nil {
		return err
	}
	active := map[string]bool{}
	for _, u := range users {
		active[u.ID] = true
	}
	var poolRule models.CrmPoolRule
	if err := tx.Where("tenant_id = ?", c.TenantID).First(&poolRule).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	targets := []string(rule.TargetIDs)
	start := 0
	if rule.Strategy == "round_robin" {
		for i, id := range targets {
			if id == rule.LastTargetID {
				start = (i + 1) % len(targets)
				break
			}
		}
	}
	// Shuffle for random selection so unavailable recipients do not bias the choice.
	if rule.Strategy == "random" {
		targets = append([]string(nil), targets...)
		for i := len(targets) - 1; i > 0; i-- {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
			if err != nil {
				return err
			}
			j := int(n.Int64())
			targets[i], targets[j] = targets[j], targets[i]
		}
	}
	for offset := 0; offset < len(targets); offset++ {
		target := targets[(start+offset)%len(targets)]
		if !active[target] {
			continue
		}
		if poolRule.ClaimLimit > 0 {
			var count int64
			held := tx.Model(&models.CrmCustomer{}).Select("id").Where("tenant_id = ? AND owner_user_id = ? AND pool_status = 0", c.TenantID, target).Limit(poolRule.ClaimLimit)
			if err := tx.Table("(?) held", held).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(poolRule.ClaimLimit) {
				continue
			}
		}
		now := time.Now()
		if err := tx.Model(c).Updates(map[string]any{"owner_user_id": target, "pool_status": 0, "pool_at": nil, "claimed_at": now}).Error; err != nil {
			return err
		}
		if err := tx.Model(&rule).Update("last_target_id", target).Error; err != nil {
			return err
		}
		if err := poolLog(tx, c.TenantID, *c, "assign", models.User{Name: "系统自动分配"}, target, "新获客规则："+rule.Name); err != nil {
			return err
		}
		c.OwnerUserID, c.PoolStatus, c.PoolAt, c.ClaimedAt = target, 0, nil, &now
		entry.TargetUserID, entry.Outcome, entry.Reason = target, "assigned", "已按规则分配"
		break
	}
	return createCRM(tx, &entry)
}

type AllocationLogPage struct {
	Items      []models.CrmAllocationLog `json:"items"`
	HasMore    bool                      `json:"has_more"`
	NextCursor string                    `json:"next_cursor"`
}

func (s CrmAutoAllocationService) Logs(ctx context.Context, ruleID, beforeID string, size int) (AllocationLogPage, error) {
	page := AllocationLogPage{Items: []models.CrmAllocationLog{}}
	if err := requireCRMManager(ctx, models.DB); err != nil {
		return page, err
	}
	if err := requireEntityOperation(ctx, models.DB, "customer", "read", "read"); err != nil {
		return page, err
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	q := models.DB.WithContext(ctx).Model(&models.CrmAllocationLog{}).Where("tenant_id = ?", identity.TenantID(ctx))
	if ruleID != "" {
		q = q.Where("rule_id = ?", ruleID)
	}
	if beforeID != "" {
		if id, err := strconv.ParseInt(beforeID, 10, 64); err != nil || id <= 0 {
			return page, errors.New("执行记录游标无效")
		}
		q = q.Where("id < ?", beforeID)
	}
	if err := q.Order("id DESC").Limit(size + 1).Find(&page.Items).Error; err != nil {
		return page, err
	}
	if len(page.Items) > size {
		page.HasMore = true
		page.Items = page.Items[:size]
		page.NextCursor = page.Items[size-1].ID
	}
	return page, nil
}
