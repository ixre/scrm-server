package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// PlanPeriod 回款计划期次入参。
type PlanPeriod struct {
	Amount   float64 `json:"amount"`
	PlanDate string  `json:"plan_date"` // yyyy-MM-dd
}

// CrmReceivablesExtService 回款扩展服务（计划生成 / 到期逾期视图 / 审批）。
type CrmReceivablesExtService struct{}

func NewCrmReceivablesExt() *CrmReceivablesExtService { return &CrmReceivablesExtService{} }

// GeneratePlan 按合同生成回款计划：批量创建 CrmReceivablesPlan，CustomerID 取自合同。
func (s CrmReceivablesExtService) GeneratePlan(ctx context.Context, orderID string, periods []PlanPeriod) error {
	tenantID := identity.TenantID(ctx)


	if orderID == "" {
		return errors.New("order_id 不能为空")
	}
	if len(periods) == 0 {
		return errors.New("periods 不能为空")
	}
	var order models.CrmOrder
	if err := models.MustFor(ctx).Where("id = ?", orderID).First(&order).Error; err != nil {
		return errors.New("合同不存在")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		for i, p := range periods {
			planDate, err := parsePlanDate(p.PlanDate)
			if err != nil {
				return fmt.Errorf("第%d期计划日期格式错误: %w", i+1, err)
			}
			plan := models.CrmReceivablesPlan{
				Model:      models.Model{ID: id_generator.StringID()},
				TenantID:   tenantID,
				OrderID:    orderID,
				CustomerID: order.CustomerID,
				PeriodNo:   i + 1,
				Amount:     p.Amount,
				PlanDate:   planDate,
				Status:     "pending",
			}
			if err := createCRM(tx, &plan, "OrderID", "CustomerID"); err != nil {
				log.Sugar.Errorw("receivables plan generate failed", "err", err)
				return err
			}
		}
		return nil
	})
}

// parsePlanDate 解析 yyyy-MM-dd 为 *time.Time（零时本地）。
func parsePlanDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, errors.New("plan_date 不能为空")
	}
	t, err := time.ParseInLocation("2006-01-02", s, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListExpiring 即将到期的回款计划：PlanDate 在未来 days 天内且 Status 未完成（pending）。
func (s CrmReceivablesExtService) ListExpiring(ctx context.Context, days int, req PageReq) ([]models.CrmReceivablesPlan, int64, error) {


	if days <= 0 {
		days = 7
	}
	p := req.pager()
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 0, days)
	db := models.MustFor(ctx).Where("status = ? AND plan_date >= ? AND plan_date <= ?",  "pending",  start,  end)
	var total int64
	if err := db.Model(&models.CrmReceivablesPlan{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmReceivablesPlan
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("plan_date asc, id desc").Find(&items).Error
	return items, total, err
}

// ListOverdue 已逾期计划：PlanDate < today 且 Status 未完成（pending/overdue）。
func (s CrmReceivablesExtService) ListOverdue(ctx context.Context, req PageReq) ([]models.CrmReceivablesPlan, int64, error) {


	p := req.pager()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	db := models.MustFor(ctx).Where("status IN ? AND plan_date < ?",  []string{"pending", "overdue"},  today)
	var total int64
	if err := db.Model(&models.CrmReceivablesPlan{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmReceivablesPlan
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("plan_date asc, id desc").Find(&items).Error
	return items, total, err
}

// Approve 回款审批通过：ApproveStatus=approved, ApproverID, ApproveAt=now, Status=confirmed。
func (s CrmReceivablesExtService) Approve(ctx context.Context, id, approverID string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmReceivables{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND approve_status = ?", id,  "pending").
		Updates(map[string]any{"approve_status": "approved", "approver_id": approverID, "approve_at": now, "status": "confirmed"})
	if res.Error != nil {
		log.Sugar.Errorw("receivables approve failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("回款记录不存在或非待审批状态")
	}
	return nil
}

// Reject 回款审批驳回：ApproveStatus=rejected, ApproverID, ApproveAt=now, ApproveRemark。
func (s CrmReceivablesExtService) Reject(ctx context.Context, id, approverID, remark string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmReceivables{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND approve_status = ?", id,  "pending").
		Updates(map[string]any{"approve_status": "rejected", "approver_id": approverID, "approve_at": now, "approve_remark": remark})
	if res.Error != nil {
		log.Sugar.Errorw("receivables reject failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("回款记录不存在或非待审批状态")
	}
	return nil
}
