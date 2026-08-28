package services

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func opportunityStage(ctx context.Context, tx *gorm.DB, id string) (models.CrmPipelineStage, error) {
	var stage models.CrmPipelineStage
	if id == "" {
		return stage, errors.New("请选择销售阶段")
	}
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ? AND id = ?", identity.TenantID(ctx), id).First(&stage).Error; err != nil {
		return stage, errors.New("销售阶段不存在")
	}
	if stage.IsWon && stage.IsLost || stage.Probability < 0 || stage.Probability > 100 {
		return stage, errors.New("销售阶段配置无效")
	}
	return stage, nil
}
func validateOpportunity(ctx context.Context, tx *gorm.DB, o *models.CrmOpportunity) error {
	o.Title = strings.TrimSpace(o.Title)
	if o.Title == "" {
		return errors.New("请填写商机名称")
	}
	if _, err := activeSalesCustomer(ctx, tx, o.CustomerID); err != nil {
		return err
	}
	if err := validateCustomerOwner(ctx, tx, o.OwnerUserID); err != nil {
		return err
	}
	if _, err := moneyCents(o.Amount); err != nil {
		return err
	}
	if o.LeadID != "" {
		db, err := salesAccessDB(ctx, tx, false)
		if err != nil {
			return err
		}
		var lead models.CrmLead
		if err = db.Where("id = ? AND (customer_id = ? OR convert_customer_id = ?)", o.LeadID, o.CustomerID, o.CustomerID).First(&lead).Error; err != nil {
			return errors.New("来源线索未转化为当前客户或无权操作")
		}
	}
	return nil
}
func applyOpportunityStage(o *models.CrmOpportunity, stage models.CrmPipelineStage) {
	o.StageID, o.Probability, o.Status = stage.ID, stage.Probability, "open"
	o.WonAt, o.LostAt, o.WonReason, o.LostReason, o.ActualAmount = nil, nil, "", "", 0
	now := time.Now()
	if stage.IsWon {
		o.Status, o.Probability, o.WonAt, o.ActualAmount = "won", 100, &now, o.Amount
	}
	if stage.IsLost {
		o.Status, o.Probability, o.LostAt = "lost", 0, &now
	}
	o.ExpectedRevenue = math.Round(o.Amount*float64(o.Probability)) / 100
}
func saveOpportunityState(tx *gorm.DB, o *models.CrmOpportunity) error {
	return tx.Model(o).Updates(map[string]any{"stage_id": nullableSalesID(o.StageID), "status": o.Status, "probability": o.Probability, "expected_revenue": o.ExpectedRevenue, "won_at": o.WonAt, "lost_at": o.LostAt, "won_reason": o.WonReason, "lost_reason": o.LostReason, "actual_amount": o.ActualAmount}).Error
}
func moveOpportunity(ctx context.Context, tx *gorm.DB, o *models.CrmOpportunity, stage models.CrmPipelineStage) error {
	if stage.IsWon || stage.IsLost {
		return errors.New("请使用赢单或输单操作并填写原因")
	}
	if o.StageID == stage.ID {
		return nil
	}
	if o.Status == "won" || o.Status == "lost" {
		return errors.New("商机已关闭，请先恢复跟进再调整阶段")
	}
	if err := checkMoveStageGuardDB(ctx, tx, o.ID); err != nil {
		return err
	}
	applyOpportunityStage(o, stage)
	return saveOpportunityState(tx, o)
}
func opportunityStageEvent(ctx context.Context, o models.CrmOpportunity) {
	NewCrmSop().Trigger(ctx, "stage_change", map[string]string{"customer_id": o.CustomerID, "opportunity_id": o.ID, "stage_id": o.StageID})
	if o.CustomerID != "" {
		TriggerScoreUpdate(o.TenantID, o.CustomerID)
	}
}

func transitionOpportunities(ctx context.Context, ids []string, stageID string) ([]models.CrmOpportunity, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return nil, err
	}
	var changed []models.CrmOpportunity
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		stage, err := opportunityStage(ctx, tx, stageID)
		if err != nil {
			return err
		}
		for _, id := range ids {
			o, err := lockSalesRecord[models.CrmOpportunity](ctx, tx, id, false)
			if err != nil {
				return err
			}
			if o.StageID == stage.ID {
				continue
			}
			if err = moveOpportunity(ctx, tx, &o, stage); err != nil {
				return err
			}
			changed = append(changed, o)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, o := range changed {
		opportunityStageEvent(ctx, o)
	}
	return changed, nil
}

func closeOpportunity(ctx context.Context, id, status, reason string, amount float64) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("请填写赢单或输单原因")
	}
	if _, err := moneyCents(amount); err != nil {
		return err
	}
	var o models.CrmOpportunity
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		o, err = lockSalesRecord[models.CrmOpportunity](ctx, tx, id, false)
		if err != nil {
			return err
		}
		if o.Status != "open" {
			return errors.New("仅进行中的商机可以关闭")
		}
		var stage models.CrmPipelineStage
		if err = tx.Where("tenant_id = ? AND is_won = ? AND is_lost = ?", o.TenantID, status == "won", status == "lost").Order("sort, id").First(&stage).Error; err != nil {
			return errors.New("请先配置对应的赢单或输单阶段")
		}
		if err = checkMoveStageGuardDB(ctx, tx, id); err != nil {
			return err
		}
		applyOpportunityStage(&o, stage)
		if status == "won" {
			o.WonReason, o.ActualAmount = reason, amount
		} else {
			o.LostReason = reason
		}
		if err = saveOpportunityState(tx, &o); err != nil {
			return err
		}
		f := models.CrmFollowup{Model: models.Model{ID: id_generator.StringID()}, TenantID: o.TenantID, CustomerID: o.CustomerID, OpportunityID: o.ID, UserID: identity.UserID(ctx), Type: "other", Content: "商机关闭（" + status + "）：" + reason}
		return createCRM(tx, &f)
	})
	if err == nil {
		opportunityStageEvent(ctx, o)
	}
	return err
}

func validatePipelineStage(stage *models.CrmPipelineStage) error {
	stage.Name = strings.TrimSpace(stage.Name)
	if stage.Name == "" || stage.Sort < 0 || stage.Probability < 0 || stage.Probability > 100 || stage.IsWon && stage.IsLost {
		return errors.New("请填写阶段名称、有效排序及 0 到 100 的概率，赢单与输单不能同时选择")
	}
	if stage.IsWon {
		stage.Probability = 100
	}
	if stage.IsLost {
		stage.Probability = 0
	}
	return nil
}
