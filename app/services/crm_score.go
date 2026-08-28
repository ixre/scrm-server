package services

import (
	"core/common/identity"

	"context"

	"time"

	"core/app/models"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 客户评分 ----------

type CrmScoreService struct{ db *gorm.DB }

func NewCrmScore() *CrmScoreService { return &CrmScoreService{db: models.DB} }

// 评分维度权重（总分100）：
// - 互动频次（CrmChannelInteraction + CrmFollowup）：30分，每条1分上限30
// - 商机金额（CrmOpportunity）：30分，每万元1分上限30
// - 订单金额（CrmOrder）：25分，每万元1分上限25
// - 客户等级（CrmCustomer.Level）：15分，Level 5=15, 4=12, 3=9, 2=6, 1=3, 0=0

// RecalculateAll 重算所有客户评分（定时调用）。
func (s CrmScoreService) RecalculateAll(ctx context.Context) error {
	tenantID := identity.TenantID(ctx)

	var customers []models.CrmCustomer
	if err := s.database().WithContext(ctx).Scopes(models.TenantScope(identity.TenantID(ctx))).Find(&customers).Error; err != nil {
		return err
	}
	for _, c := range customers {
		score := s.CalculateScore(c)
		now := time.Now()
		s.database().Model(&models.CrmCustomer{}).Where("id = ?", c.ID).
			Updates(map[string]any{"score": score, "score_updated_at": &now})
	}
	log.Sugar.Infow("customer score recalculated", "tenant", tenantID, "count", len(customers))
	return nil
}

// CalculateScore 计算单个客户评分（纯函数，可单测）。
func (s CrmScoreService) CalculateScore(c models.CrmCustomer) int {
	score := 0
	// 客户等级分
	levelScore := c.Level * 3
	if levelScore > 15 {
		levelScore = 15
	}
	score += levelScore

	// 互动频次分
	var interactionCnt int64
	s.database().Model(&models.CrmChannelInteraction{}).Scopes(models.TenantScope(c.TenantID)).Where("customer_id = ?", c.ID).Count(&interactionCnt)
	var followupCnt int64
	s.database().Model(&models.CrmFollowup{}).Scopes(models.TenantScope(c.TenantID)).Where("customer_id = ?", c.ID).Count(&followupCnt)
	interactionScore := int(interactionCnt) + int(followupCnt)
	if interactionScore > 30 {
		interactionScore = 30
	}
	score += interactionScore

	// 商机金额分
	var oppAmt struct {
		Total float64 `gorm:"column:total"`
	}
	s.database().Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(c.TenantID)).Where("customer_id = ? AND status = ?", c.ID, "open").
		Select("COALESCE(SUM(amount),0) AS total").Scan(&oppAmt)
	oppScore := int(oppAmt.Total / 10000) // 每万元1分
	if oppScore > 30 {
		oppScore = 30
	}
	score += oppScore

	// 订单金额分
	var orderAmt struct {
		Total float64 `gorm:"column:total"`
	}
	s.database().Model(&models.CrmOrder{}).Scopes(models.TenantScope(c.TenantID)).Where("customer_id = ? AND status IN ?", c.ID, []string{"signed", "completed"}).
		Select("COALESCE(SUM(amount),0) AS total").Scan(&orderAmt)
	orderScore := int(orderAmt.Total / 10000)
	if orderScore > 25 {
		orderScore = 25
	}
	score += orderScore

	return score
}

// TopByScore 按评分排序返回 Top N 客户。
func (s CrmScoreService) TopByScore(ctx context.Context, limit int) ([]models.CrmCustomer, error) {

	if limit <= 0 {
		limit = 10
	}
	var items []models.CrmCustomer
	err := s.database().WithContext(ctx).Scopes(models.TenantScope(identity.TenantID(ctx))).Order("score desc").Limit(limit).Find(&items).Error
	return items, err
}

// RecalculateOne 重算单个客户评分（业务事件触发时调用）。
func (s CrmScoreService) RecalculateOne(tenantID, customerID string) {
	var c models.CrmCustomer
	if err := s.database().Scopes(models.TenantScope(tenantID)).Where("id = ?", customerID).First(&c).Error; err != nil {
		return
	}
	score := s.CalculateScore(c)
	now := time.Now()
	s.database().Model(&models.CrmCustomer{}).Where("id = ?", c.ID).
		Updates(map[string]any{"score": score, "score_updated_at": &now})
}

// TriggerScoreUpdate 在业务事件后触发客户评分重算（异步安全调用）。
func TriggerScoreUpdate(tenantID, customerID string) {
	if tenantID == "" || customerID == "" {
		return
	}
	service := NewCrmScore()
	go service.RecalculateOne(tenantID, customerID)
}

var _ = gorm.ErrRecordNotFound

func (s CrmScoreService) database() *gorm.DB {
	if s.db != nil {
		return s.db
	}
	return models.DB
}
