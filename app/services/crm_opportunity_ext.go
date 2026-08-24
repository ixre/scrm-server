package services

import (
	"bytes"
	"core/common/id_generator"
	"core/common/identity"
	"strconv"

	"context"
	"errors"
	"time"

	"core/app/models"
	"core/common/log"

	"github.com/xuri/excelize/v2"
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

// Export 导出商机为 Excel。
func (s CrmOpportunityExtService) Export(ctx context.Context, status, ownerUserID string) (*bytes.Buffer, string, error) {
	db := models.MustFor(ctx).Limit(10000).Order("id desc")
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	var opps []models.CrmOpportunity
	if err := db.Find(&opps).Error; err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := "商机列表"
	f.SetSheetName(f.GetSheetName(0), sheet)
	headers := []string{"ID", "商机名称", "客户ID", "产品", "预计金额", "实际金额", "阶段ID", "状态", "归属销售ID", "预计成交", "竞争对手", "备注", "创建时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for i, o := range opps {
		row := i + 2
		f.SetCellValue(sheet, "A"+strconv.Itoa(row), o.ID)
		f.SetCellValue(sheet, "B"+strconv.Itoa(row), o.Title)
		f.SetCellValue(sheet, "C"+strconv.Itoa(row), o.CustomerID)
		f.SetCellValue(sheet, "D"+strconv.Itoa(row), o.Product)
		f.SetCellValue(sheet, "E"+strconv.Itoa(row), o.Amount)
		f.SetCellValue(sheet, "F"+strconv.Itoa(row), o.ActualAmount)
		f.SetCellValue(sheet, "G"+strconv.Itoa(row), o.StageID)
		f.SetCellValue(sheet, "H"+strconv.Itoa(row), o.Status)
		f.SetCellValue(sheet, "I"+strconv.Itoa(row), o.OwnerUserID)
		f.SetCellValue(sheet, "J"+strconv.Itoa(row), o.ExpectedCloseDate)
		f.SetCellValue(sheet, "K"+strconv.Itoa(row), o.Competitor)
		f.SetCellValue(sheet, "L"+strconv.Itoa(row), o.Remark)
		f.SetCellValue(sheet, "M"+strconv.Itoa(row), o.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return buf, "opportunities_" + time.Now().Format("20060102_150405") + ".xlsx", nil
}

// BatchTransfer 批量转移商机归属人。
func (s CrmOpportunityExtService) BatchTransfer(ctx context.Context, ids []string, toUserID string) (int64, error) {
	tenantID := identity.TenantID(ctx)
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	if toUserID == "" {
		return 0, errors.New("to_user_id 不能为空")
	}
	res := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?", ids).
		Update("owner_user_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("crm opportunity batch transfer failed", "err", res.Error)
	}
	return res.RowsAffected, res.Error
}

// BatchImport 批量导入商机。
func (s CrmOpportunityExtService) BatchImport(ctx context.Context, userID string, rows []map[string]string) (int, int, error) {
	tenantID := identity.TenantID(ctx)
	success, fail := 0, 0
	for _, r := range rows {
		o := models.CrmOpportunity{
			Model:       models.Model{ID: id_generator.StringID()},
			TenantID:    tenantID,
			Title:       r["商机名称"],
			Product:     r["产品"],
			Amount:      parseFloat(r["预计金额"]),
			Status:      "open",
			OwnerUserID: userID,
			Remark:      r["备注"],
		}
		if err := models.DB.Create(&o).Error; err != nil {
			fail++
		} else {
			success++
		}
	}
	return success, fail, nil
}

func parseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}
