package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"time"

	"core/app/models"
	"core/common/log"

	"gorm.io/gorm"
)

// CrmOpportunityExtService 商机扩展服务（阶段流转 / 赢单输单）。
type CrmOpportunityExtService struct{}

func NewCrmOpportunityExt() *CrmOpportunityExtService { return &CrmOpportunityExtService{} }

// ChangeStage 商机阶段流转：更新 StageID，并同步阶段的成交概率。
func (s CrmOpportunityExtService) ChangeStage(ctx context.Context, id, stageID string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	if stageID == "" {
		return errors.New("stage_id 不能为空")
	}
	// 校验阶段存在且属于本租户。
	var stage models.CrmPipelineStage
	if err := models.MustFor(ctx).Where("id = ?", stageID).First(&stage).Error; err != nil {
		return errors.New("阶段不存在")
	}
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Updates(map[string]any{"stage_id": stageID, "probability": stage.Probability})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity change stage failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Win 赢单：Status=won, WonAt=now。
func (s CrmOpportunityExtService) Win(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status = ?", id,  "open").
		Updates(map[string]any{"status": "won", "won_at": now, "lost_at": nil, "lost_reason": ""})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity win failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("商机不存在或非进行中状态")
	}
	return nil
}

// Lose 输单：Status=lost, LostAt=now, LostReason。
func (s CrmOpportunityExtService) Lose(ctx context.Context, id, lostReason string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status = ?", id,  "open").
		Updates(map[string]any{"status": "lost", "lost_at": now, "lost_reason": lostReason, "won_at": nil})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity lose failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("商机不存在或非进行中状态")
	}
	return nil
}

// BatchChangeStage 批量改阶段。
func (s CrmOpportunityExtService) BatchChangeStage(ctx context.Context, ids []string, stageID string) error {
	tenantID := identity.TenantID(ctx)


	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	if stageID == "" {
		return errors.New("stage_id 不能为空")
	}
	var stage models.CrmPipelineStage
	if err := models.MustFor(ctx).Where("id = ?", stageID).First(&stage).Error; err != nil {
		return errors.New("阶段不存在")
	}
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Updates(map[string]any{"stage_id": stageID, "probability": stage.Probability})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity batch change stage failed", "err", res.Error)
	}
	return res.Error
}
