package services

import (
	"core/common/id_generator"
	"core/common/identity"
	"strconv"

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
	// 根据目标阶段推导 status，与 MoveStage 保持一致。
	newStatus := "open"
	switch {
	case stage.IsWon:
		newStatus = "won"
	case stage.IsLost:
		newStatus = "lost"
	}
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Updates(map[string]any{"stage_id": stageID, "probability": stage.Probability, "status": newStatus})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity change stage failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Win 赢单：Status=won, WonAt=now, WonReason, ActualAmount。自动写入跟进记录。
func (s CrmOpportunityExtService) Win(ctx context.Context, id, wonReason string, actualAmount float64) error {
	tenantID := identity.TenantID(ctx)
	userID := identity.UserID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	// 先查出商机（用于写跟进记录的 customer_id）
	var opp models.CrmOpportunity
	if err := models.MustFor(ctx).Where("id = ?", id).First(&opp).Error; err != nil {
		return err
	}
	now := time.Now()
	updates := map[string]any{
		"status":        "won",
		"won_at":        now,
		"won_reason":    wonReason,
		"actual_amount": actualAmount,
		"lost_at":       nil,
		"lost_reason":   "",
	}
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status = ?", id, "open").
		Updates(updates)
	if res.Error != nil {
		log.Sugar.Errorw("opportunity win failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("商机不存在或非进行中状态")
	}
	// 自动写入跟进记录
	s.recordFollowup(ctx, userID, tenantID, opp, "other", "赢单", "赢单原因："+wonReason+"；实际成交金额："+formatFloat(actualAmount))
	return nil
}

// Lose 输单：Status=lost, LostAt=now, LostReason。自动写入跟进记录。
func (s CrmOpportunityExtService) Lose(ctx context.Context, id, lostReason string) error {
	tenantID := identity.TenantID(ctx)
	userID := identity.UserID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	var opp models.CrmOpportunity
	if err := models.MustFor(ctx).Where("id = ?", id).First(&opp).Error; err != nil {
		return err
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status = ?", id, "open").
		Updates(map[string]any{"status": "lost", "lost_at": now, "lost_reason": lostReason, "won_at": nil})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity lose failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("商机不存在或非进行中状态")
	}
	// 自动写入跟进记录
	s.recordFollowup(ctx, userID, tenantID, opp, "other", "输单", "输单原因："+lostReason)
	return nil
}

// recordFollowup 写入一条自动跟进记录（赢单/输单等状态变更）。
func (s CrmOpportunityExtService) recordFollowup(ctx context.Context, userID, tenantID string, opp models.CrmOpportunity, fType, result, content string) {
	f := models.CrmFollowup{
		Model:         models.Model{ID: id_generator.StringID()},
		TenantID:      tenantID,
		CustomerID:    opp.CustomerID,
		OpportunityID: opp.ID,
		UserID:        userID,
		Type:          fType,
		Content:       content,
		Result:        result,
	}
	if err := createCRM(models.DB, &f, "CustomerID", "OpportunityID", "UserID", "ContactID"); err != nil {
		log.Sugar.Errorw("opportunity status change followup create failed", "err", err)
	}
}

// formatFloat 格式化金额，去掉无意义小数。
func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// BatchChangeStage 批量改阶段。根据目标阶段自动推导 status，与 MoveStage/ChangeStage 保持一致。
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
	// 根据目标阶段推导 status。
	newStatus := "open"
	switch {
	case stage.IsWon:
		newStatus = "won"
	case stage.IsLost:
		newStatus = "lost"
	}
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?", ids).
		Updates(map[string]any{"stage_id": stageID, "probability": stage.Probability, "status": newStatus})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity batch change stage failed", "err", res.Error)
	}
	return res.Error
}

// Reopen 恢复商机为进行中：从 won/lost 恢复为 open，清空赢单/输单相关字段。
func (s CrmOpportunityExtService) Reopen(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).
		Where("id = ? AND status IN ?", id, []string{"won", "lost"}).
		Updates(map[string]any{
			"status":        "open",
			"won_at":        nil,
			"lost_at":       nil,
			"won_reason":    "",
			"lost_reason":   "",
			"actual_amount": 0,
		})
	if res.Error != nil {
		log.Sugar.Errorw("opportunity reopen failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("商机不存在或非赢单/输单状态")
	}
	return nil
}
