package services

import (
	"context"
	"core/app/constants"
	"core/app/models"
	"database/sql"
	"errors"
	"time"

	"gorm.io/gorm"
)

type DuePlanQuery struct {
	PageReq
	OrderID string
}

type DuePlanItem struct {
	models.CrmReceivablesPlan
	RemainingAmount        string `json:"remaining_amount"`
	OrderNo                string `json:"order_no"`
	OrderTitle             string `json:"order_title"`
	OrderOutstandingAmount string `json:"order_outstanding_amount"`
	DaysUntilDue           int    `json:"days_until_due"`
}

// ListExpiring includes today and the full next N calendar days in China.
func (s CrmReceivablesExtService) ListExpiring(ctx context.Context, days int, req DuePlanQuery) ([]DuePlanItem, int64, error) {
	if days == 0 {
		days = 7
	}
	if days < 1 || days > 365 {
		return nil, 0, errors.New("到期范围须为1至365天")
	}
	return s.listDuePlans(ctx, req, days, false)
}

func (s CrmReceivablesExtService) ListOverdue(ctx context.Context, req DuePlanQuery) ([]DuePlanItem, int64, error) {
	return s.listDuePlans(ctx, req, 0, true)
}

func calendarDaysUntil(now, due time.Time) int {
	date := func(t time.Time) time.Time {
		t = t.In(constants.PRCLocation)
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	return int(date(due).Unix()/86400 - date(now).Unix()/86400)
}

func (s CrmReceivablesExtService) listDuePlans(ctx context.Context, req DuePlanQuery, days int, overdue bool) ([]DuePlanItem, int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	items := []DuePlanItem{}
	var total int64
	pager := req.pager()
	now := time.Now()
	today := dayStart(now)
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		q := tx.Table("crm_receivables_plan p").
			Joins("JOIN crm_order o ON o.id = p.order_id AND o.tenant_id = p.tenant_id AND o.deleted_at IS NULL AND o.status IN ('signed','completed') AND o.amount - COALESCE(o.coupon_amount,0) > COALESCE(o.paid_amount,0)").
			Joins(`LEFT JOIN LATERAL (SELECT COALESCE(SUM(r.amount),0) AS paid FROM crm_receivables r WHERE r.tenant_id = p.tenant_id AND r.order_id = p.order_id AND r.plan_id = p.id AND r.deleted_at IS NULL AND `+effectiveBusinessReceipt+`) allocated ON true`).
			Where("p.tenant_id = ? AND p.deleted_at IS NULL AND p.plan_date IS NOT NULL AND p.amount > allocated.paid", actor.TenantID)
		if !manager {
			q = q.Where("o.user_id = ?", actor.ID)
		}
		if req.OrderID != "" {
			q = q.Where("p.order_id = ?", req.OrderID)
		}
		if overdue {
			q = q.Where("p.plan_date < ?", today)
		} else {
			q = q.Where("p.plan_date >= ? AND p.plan_date < ?", today, today.AddDate(0, 0, days+1))
		}
		if err := q.Count(&total).Error; err != nil {
			return err
		}
		return q.Select("p.*,o.order_no,o.title AS order_title," + moneySQL("GREATEST(p.amount-allocated.paid,0)") + " AS remaining_amount," + moneySQL("GREATEST(o.amount-COALESCE(o.coupon_amount,0)-COALESCE(o.paid_amount,0),0)") + " AS order_outstanding_amount").
			Order("p.plan_date ASC,p.id DESC").Offset(pager.GetOffset()).Limit(pager.GetLimit()).Scan(&items).Error
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].DaysUntilDue = calendarDaysUntil(now, *items[i].PlanDate)
		items[i].Status = "pending"
		if overdue {
			items[i].Status = "overdue"
		}
	}
	return items, total, nil
}
