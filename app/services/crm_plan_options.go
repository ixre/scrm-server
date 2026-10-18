package services

import (
	"context"
	"core/app/models"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"strconv"
	"strings"
	"time"
)

type PlanOptionsRequest struct {
	OrderID  string `json:"order_id"`
	Keyword  string `json:"keyword"`
	Cursor   string `json:"cursor"`
	PageSize int    `json:"page_size"`
}
type PlanOption struct {
	ID              string     `json:"id"`
	PeriodNo        int        `json:"period_no"`
	Amount          string     `json:"amount"`
	RemainingAmount string     `json:"remaining_amount"`
	PlanDate        *time.Time `json:"plan_date"`
	Label           string     `json:"label"`
}
type PlanOptionsPage struct {
	Items      []PlanOption `json:"items"`
	HasMore    bool         `json:"has_more"`
	NextCursor string       `json:"next_cursor"`
}
type planOptionCursor struct{ Tenant, Order, Keyword, ID string }

// Options is scoped to the contract rather than customer sharing, matching receipt writers.
func (s CrmReceivablesPlanService) Options(ctx context.Context, req PlanOptionsRequest) (PlanOptionsPage, error) {
	page := PlanOptionsPage{Items: []PlanOption{}}
	if n, err := strconv.ParseInt(req.OrderID, 10, 64); err != nil || n <= 0 {
		return page, errors.New("请选择有效合同")
	}
	req.Keyword = strings.TrimSpace(req.Keyword)
	if req.Keyword != "" {
		n, err := strconv.ParseInt(req.Keyword, 10, 32)
		if err != nil || n <= 0 {
			return page, errors.New("请输入需要查找的正整数期次")
		}
		req.Keyword = strconv.FormatInt(n, 10)
	}
	var cursor planOptionCursor
	if req.Cursor != "" {
		if len(req.Cursor) > 1024 {
			return page, errors.New("期次游标无效")
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Order != req.OrderID || cursor.Keyword != req.Keyword {
			return page, errors.New("期次游标范围不匹配，请重新查询")
		}
		if n, err := strconv.ParseInt(cursor.ID, 10, 64); err != nil || n <= 0 {
			return page, errors.New("期次游标无效")
		}
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		if req.Cursor != "" && cursor.Tenant != actor.TenantID {
			return errors.New("期次游标无效")
		}
		q := tx.Where("tenant_id = ? AND id = ?", actor.TenantID, req.OrderID)
		if !manager {
			q = q.Where("user_id = ?", actor.ID)
		}
		var order models.CrmOrder
		if err := q.First(&order).Error; err != nil {
			return errors.New("合同不存在或无权查看")
		}
		if order.Status == "cancelled" {
			return nil
		}
		q = tx.Table("crm_receivables_plan p").Where("p.tenant_id = ? AND p.order_id = ? AND p.deleted_at IS NULL", actor.TenantID, order.ID).
			Joins(`LEFT JOIN LATERAL (SELECT COALESCE(SUM(r.amount),0) AS paid FROM crm_receivables r WHERE r.tenant_id = p.tenant_id AND r.order_id = p.order_id AND r.plan_id = p.id AND r.deleted_at IS NULL AND ` + effectiveBusinessReceipt + `) allocated ON true`).Where("p.amount > allocated.paid")
		if req.Keyword != "" {
			q = q.Where("p.period_no = ?", req.Keyword)
		}
		if req.Cursor != "" {
			q = q.Where("p.id < ?", cursor.ID)
		}
		if err := q.Select("p.id,p.period_no,p.plan_date," + moneySQL("p.amount") + " AS amount," + moneySQL("GREATEST(p.amount-allocated.paid,0)") + " AS remaining_amount").Order("p.id DESC").Limit(req.PageSize + 1).Scan(&page.Items).Error; err != nil {
			return err
		}
		if len(page.Items) > req.PageSize {
			page.HasMore = true
			page.Items = page.Items[:req.PageSize]
			raw, err := json.Marshal(planOptionCursor{actor.TenantID, order.ID, req.Keyword, page.Items[req.PageSize-1].ID})
			if err != nil {
				return err
			}
			page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
		}
		for i := range page.Items {
			p := &page.Items[i]
			p.Label = fmt.Sprintf("第 %d 期 · 未收 ¥%s", p.PeriodNo, p.RemainingAmount)
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return page, err
}
