package services

import (
	"context"

	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/models"

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
	if len(periods) == 0 {
		return errors.New("请至少添加一期回款计划")
	}
	plans := make([]models.CrmReceivablesPlan, 0, len(periods))
	for i, p := range periods {
		date, err := parsePlanDate(p.PlanDate)
		if err != nil {
			return fmt.Errorf("第 %d 期计划日期格式错误: %w", i+1, err)
		}
		plans = append(plans, models.CrmReceivablesPlan{OrderID: orderID, PeriodNo: i + 1, Amount: p.Amount, PlanDate: date})
	}
	return NewCrmReceivablesPlan().BatchCreate(ctx, plans)
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
	db, err := receiptScope(ctx, models.DB)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where("status = ? AND plan_date >= ? AND plan_date <= ?", "pending", start, end)
	var total int64
	if err := db.Model(&models.CrmReceivablesPlan{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmReceivablesPlan
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("plan_date asc, id desc").Find(&items).Error
	return items, total, err
}

// ListOverdue 已逾期计划：PlanDate < today 且 Status 未完成（pending/overdue）。
func (s CrmReceivablesExtService) ListOverdue(ctx context.Context, req PageReq) ([]models.CrmReceivablesPlan, int64, error) {

	p := req.pager()
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	db, err := receiptScope(ctx, models.DB)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where("status IN ? AND plan_date < ?", []string{"pending", "overdue"}, today)
	var total int64
	if err := db.Model(&models.CrmReceivablesPlan{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmReceivablesPlan
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("plan_date asc, id desc").Find(&items).Error
	return items, total, err
}

// Approve 回款审批通过：ApproveStatus=approved, ApproverID, ApproveAt=now, Status=confirmed。
func (s CrmReceivablesExtService) Approve(ctx context.Context, id, approverID string) error {
	return s.review(ctx, id, approverID, true, "")
}

// Reject 回款审批驳回：ApproveStatus=rejected, ApproverID, ApproveAt=now, ApproveRemark。
func (s CrmReceivablesExtService) Reject(ctx context.Context, id, approverID, remark string) error {
	if strings.TrimSpace(remark) == "" {
		return errors.New("请填写驳回原因")
	}
	return s.review(ctx, id, approverID, false, remark)
}

func (s CrmReceivablesExtService) review(ctx context.Context, id, approverID string, approve bool, remark string) error {
	actor, manager, err := customerActor(ctx, models.DB)
	if err != nil {
		return err
	}
	if !manager || actor.ID != approverID {
		return errors.New("仅管理员或经理可审批回款")
	}
	return NewCrmReceivables().changeReceipt(ctx, id, func(tx *gorm.DB, r *models.CrmReceivables, order models.CrmOrder) error {
		if r.Status == "void" || r.ApproveStatus != "pending" {
			return errors.New("回款记录非待审批状态")
		}
		status := "pending"
		result := "rejected"
		if approve {
			r.Status = "confirmed"
			if err := validateReceipt(tx, order, r); err != nil {
				return err
			}
			status = "confirmed"
			result = "approved"
		}
		now := time.Now()
		r.Status = status
		r.ApproveStatus = result
		return tx.Model(r).Updates(map[string]any{"status": status, "approve_status": result, "approver_id": actor.ID, "approve_at": now, "approve_remark": remark}).Error
	})
}
