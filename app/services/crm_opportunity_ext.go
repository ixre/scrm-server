package services

import (
	"bytes"
	"strconv"
	"strings"

	"context"
	"errors"
	"time"

	"core/app/models"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// CrmOpportunityExtService 商机扩展服务（阶段流转 / 赢单输单）。
type CrmOpportunityExtService struct{}

func NewCrmOpportunityExt() *CrmOpportunityExtService { return &CrmOpportunityExtService{} }

// ChangeStage 商机阶段流转：更新 StageID，并同步阶段的成交概率。
func (s CrmOpportunityExtService) ChangeStage(ctx context.Context, id, stageID string) error {
	_, err := transitionOpportunities(ctx, []string{id}, stageID)
	return err
}

// Win 赢单：Status=won, WonAt=now, WonReason, ActualAmount。自动写入跟进记录。
func (s CrmOpportunityExtService) Win(ctx context.Context, id, wonReason string, actualAmount float64) error {
	return closeOpportunity(ctx, id, "won", wonReason, actualAmount)
}

// Lose 输单：Status=lost, LostAt=now, LostReason。自动写入跟进记录。
func (s CrmOpportunityExtService) Lose(ctx context.Context, id, lostReason string) error {
	return closeOpportunity(ctx, id, "lost", lostReason, 0)
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
	_, err := transitionOpportunities(ctx, ids, stageID)
	return err
}

// Reopen 恢复商机为进行中：从 won/lost 恢复为 open，清空赢单/输单相关字段。
func (s CrmOpportunityExtService) Reopen(ctx context.Context, id string) error {
	var o models.CrmOpportunity
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		o, err = lockSalesRecord[models.CrmOpportunity](ctx, tx, id, false)
		if err != nil {
			return err
		}
		if o.Status != "won" && o.Status != "lost" {
			return errors.New("仅已关闭商机可恢复")
		}
		var stage models.CrmPipelineStage
		err = tx.Where("tenant_id = ? AND is_won = false AND is_lost = false", o.TenantID).Order("sort, id").First(&stage).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		applyOpportunityStage(&o, stage)
		return saveOpportunityState(tx, &o)
	})
	if err == nil {
		opportunityStageEvent(ctx, o)
	}
	return err
}

// Export 导出商机为 Excel。
func (s CrmOpportunityExtService) Export(ctx context.Context, status, ownerUserID string) (*bytes.Buffer, string, error) {
	db, err := salesAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, "", err
	}
	db = db.Limit(10000).Order("id desc")
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
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	if toUserID == "" {
		return 0, errors.New("请选择接收销售")
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateCustomerOwner(ctx, tx, toUserID); err != nil {
			return err
		}
		for _, id := range ids {
			o, err := lockSalesRecord[models.CrmOpportunity](ctx, tx, id, false)
			if err != nil {
				return err
			}
			if err = tx.Model(&o).Update("owner_user_id", toUserID).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// BatchImport 批量导入商机。
func (s CrmOpportunityExtService) BatchImport(ctx context.Context, userID string, rows []map[string]string) (int, int, error) {
	if _, _, err := customerActor(ctx, models.DB); err != nil {
		return 0, 0, err
	}
	success, fail := 0, 0
	for _, row := range rows {
		amount, err := strconv.ParseFloat(strings.TrimSpace(row["预计金额"]), 64)
		if row["预计金额"] == "" {
			amount, err = 0, nil
		}
		if err != nil {
			fail++
			continue
		}
		_, err = NewCrmOpportunity().Create(ctx, userID, models.CrmOpportunity{Title: row["商机名称"], CustomerID: strings.TrimSpace(row["客户ID"]), Product: row["产品"], Amount: amount, Remark: row["备注"]})
		if err != nil {
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
