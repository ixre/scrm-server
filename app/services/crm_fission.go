package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 裂变活动 ----------

type CrmFissionService struct{}

func NewCrmFission() *CrmFissionService { return &CrmFissionService{} }

// FissionCreateReq 裂变活动创建请求。
type FissionCreateReq struct {
	Fission models.CrmFission `json:"fission"`
}

// Create 创建裂变活动。
func (s CrmFissionService) Create(ctx context.Context, req models.CrmFission) (models.CrmFission, error) {
	tenantID := identity.TenantID(ctx)


	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.RewardThreshold <= 0 {
		req.RewardThreshold = 1
	}
	if req.MaxRewardPerUser <= 0 {
		req.MaxRewardPerUser = 1
	}
	if err := createCRM(models.DB, &req, "ActivityID"); err != nil {
		log.Sugar.Errorw("fission create failed", "err", err)
		return req, err
	}
	return req, nil
}

// Query 列出裂变活动。
func (s CrmFissionService) Query(ctx context.Context, status string, pager *app.Pager) ([]models.CrmFission, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmFission{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmFission
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// Get 查单个裂变活动。
func (s CrmFissionService) Get(ctx context.Context, id string) (models.CrmFission, error) {


	var f models.CrmFission
	err := models.MustFor(ctx).Where("id = ?", id).First(&f).Error
	return f, err
}

// Delete 删除裂变活动。
func (s CrmFissionService) Delete(ctx context.Context, ids []string) (int64, error) {


	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmFission{})
	return res.RowsAffected, res.Error
}

// RecordInvite 记录邀请关系：邀请人邀请新客户。
// 幂等：同 (fission, inviter, invitee) 不重复。
// 自动检测是否达到奖励阈值，达到则标记 reward_triggered。
func (s CrmFissionService) RecordInvite(ctx context.Context, fissionID, inviterCustomerID, inviteeCustomerID string) (models.CrmFissionInvite, error) {
	tenantID := identity.TenantID(ctx)


	var rec models.CrmFissionInvite
	if inviterCustomerID == "" || inviteeCustomerID == "" {
		return rec, errors.New("inviter/invitee 不能为空")
	}
	if inviterCustomerID == inviteeCustomerID {
		return rec, errors.New("不能邀请自己")
	}
	var f models.CrmFission
	if err := models.MustFor(ctx).Where("id = ?", fissionID).First(&f).Error; err != nil {
		return rec, errors.New("裂变活动不存在")
	}
	if f.Status != "active" {
		return rec, errors.New("裂变活动已停用")
	}
	// 幂等检查
	var cnt int64
	models.MustFor(ctx).Model(&models.CrmFissionInvite{}).
		Where("fission_id = ? AND inviter_customer_id = ? AND invitee_customer_id = ?", fissionID, inviterCustomerID, inviteeCustomerID).Count(&cnt)
	if cnt > 0 {
		return rec, errors.New("邀请关系已存在")
	}

	rec = models.CrmFissionInvite{
		Model:             models.Model{ID: id_generator.StringID()},
		TenantID:          tenantID,
		FissionID:         fissionID,
		InviterCustomerID: inviterCustomerID,
		InviteeCustomerID: inviteeCustomerID,
		RewardTriggered:   false,
	}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		// 检查邀请人是否达到奖励阈值
		var inviteCnt int64
		tx.Model(&models.CrmFissionInvite{}).
			Where("tenant_id = ? AND fission_id = ? AND inviter_customer_id = ?", tenantID, fissionID, inviterCustomerID).Count(&inviteCnt)
		// 检查已触发奖励次数
		var rewardedCnt int64
		tx.Model(&models.CrmFissionInvite{}).
			Where("tenant_id = ? AND fission_id = ? AND inviter_customer_id = ? AND reward_triggered = true", tenantID, fissionID, inviterCustomerID).Count(&rewardedCnt)
		// 达到阈值且未超过最大奖励次数
		if int(inviteCnt) >= f.RewardThreshold && int(rewardedCnt) < f.MaxRewardPerUser {
			// 标记本次邀请触发奖励
			tx.Model(&models.CrmFissionInvite{}).Where("tenant_id = ? AND id = ?", tenantID, rec.ID).Update("reward_triggered", true)
			rec.RewardTriggered = true
		}
		return nil
	})
	if err != nil {
		log.Sugar.Errorw("fission record invite failed", "err", err)
		return rec, err
	}
	return rec, nil
}

// ListInvites 列出邀请关系。
func (s CrmFissionService) ListInvites(ctx context.Context, fissionID, inviterCustomerID string, pager *app.Pager) ([]models.CrmFissionInvite, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if fissionID != "" {
		db = db.Where("fission_id = ?", fissionID)
	}
	if inviterCustomerID != "" {
		db = db.Where("inviter_customer_id = ?", inviterCustomerID)
	}
	var total int64
	if err := db.Model(&models.CrmFissionInvite{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmFissionInvite
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// InviteStats 邀请统计（某邀请人的邀请数/已奖励数）。
type FissionInviteStats struct {
	InviterCustomerID string `json:"inviter_customer_id"`
	InviteCount       int64  `json:"invite_count"`
	RewardCount       int64  `json:"reward_count"`
}

// StatsByInviter 某邀请人的邀请统计。
func (s CrmFissionService) StatsByInviter(ctx context.Context, fissionID, inviterCustomerID string) (FissionInviteStats, error) {
	tenantID := identity.TenantID(ctx)


	var stats FissionInviteStats
	stats.InviterCustomerID = inviterCustomerID
	models.DB.Model(&models.CrmFissionInvite{}).Scopes(models.TenantScope(tenantID)).Where("fission_id = ? AND inviter_customer_id = ?",  fissionID,  inviterCustomerID).
		Count(&stats.InviteCount)
	models.DB.Model(&models.CrmFissionInvite{}).Scopes(models.TenantScope(tenantID)).Where("fission_id = ? AND inviter_customer_id = ? AND reward_triggered = true",  fissionID,  inviterCustomerID).
		Count(&stats.RewardCount)
	return stats, nil
}
