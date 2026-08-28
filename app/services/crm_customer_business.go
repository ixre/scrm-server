package services

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"core/app/models"
	"gorm.io/gorm"
)

// Money is returned as decimal text; aggregate totals must not lose cents in JSON numbers.
type CustomerBusinessSummary struct {
	SignedCount          int64     `json:"signed_count"`
	DueAmount            string    `json:"due_amount"`
	PaidAmount           string    `json:"paid_amount"`
	OutstandingAmount    string    `json:"outstanding_amount"`
	OverduePlanCount     int64     `json:"overdue_plan_count"`
	OverduePlanUnsettled string    `json:"overdue_plan_unsettled"`
	AsOf                 time.Time `json:"as_of"`
}
type CustomerBusinessRequest struct {
	CustomerID string `json:"customer_id"`
	Kind       string `json:"kind"`
	Cursor     string `json:"cursor"`
	PageSize   int    `json:"page_size"`
}
type CustomerBusinessRow struct {
	ID              string     `json:"id"`
	Title           string     `json:"title"`
	Number          string     `json:"number,omitempty"`
	OrderID         string     `json:"order_id,omitempty"`
	OrderTitle      string     `json:"order_title,omitempty"`
	Status          string     `json:"status"`
	ApprovalStatus  string     `json:"approval_status,omitempty"`
	Amount          string     `json:"amount"`
	DueAmount       string     `json:"due_amount,omitempty"`
	PaidAmount      string     `json:"paid_amount,omitempty"`
	RemainingAmount string     `json:"remaining_amount,omitempty"`
	EventAt         *time.Time `json:"event_at"`
	CreatedAt       *time.Time `json:"created_at"`
	PeriodNo        int        `json:"period_no,omitempty"`
}
type CustomerBusinessPage struct {
	Items      []CustomerBusinessRow `json:"items"`
	HasMore    bool                  `json:"has_more"`
	NextCursor string                `json:"next_cursor"`
}
type businessCursor struct {
	Tenant   string `json:"t"`
	Customer string `json:"c"`
	Kind     string `json:"k"`
	ID       string `json:"id"`
}

func moneySQL(expression string) string { return "ROUND(COALESCE(" + expression + ",0),2)::text" }

const businessOrderDue = "GREATEST(COALESCE(b.amount,0)-COALESCE(b.coupon_amount,0),0)"
const effectiveBusinessReceipt = "r.status = 'confirmed' AND (r.approve_status IS NULL OR r.approve_status IN ('','none','approved'))"

// Scope is checked inside the same read snapshot as business data. Each page rechecks it.
func customerBusinessActor(ctx context.Context, tx *gorm.DB, id string) (models.User, bool, error) {
	if n, err := strconv.ParseInt(id, 10, 64); err != nil || n <= 0 {
		return models.User{}, false, ErrCustomerAccess
	}
	actor, manager, err := customerActor(ctx, tx)
	if err != nil {
		return actor, manager, err
	}
	var customer models.CrmCustomer
	err = customerAccessForActor(ctx, tx, actor, manager, false).Select("id").Where("id = ?", id).First(&customer).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrCustomerAccess
	}
	return actor, manager, err
}
func businessOrders(tx *gorm.DB, actor models.User, manager bool, customer string, active bool) *gorm.DB {
	q := tx.Model(&models.CrmOrder{}).Select("id").Where("tenant_id = ? AND customer_id = ?", actor.TenantID, customer)
	if !manager {
		q = q.Where("user_id = ?", actor.ID)
	}
	if active {
		q = q.Where("status IN ('signed','completed')")
	}
	return q
}
func businessPlans(tx *gorm.DB, actor models.User, manager bool, customer string, active bool) *gorm.DB {
	// Payment sums are tenant/order/plan scoped; a wrong customer snapshot cannot leak another order.
	q := tx.Table("crm_receivables_plan b").Where("b.tenant_id = ? AND b.deleted_at IS NULL AND b.order_id IN (?)", actor.TenantID, businessOrders(tx, actor, manager, customer, active))
	return q.Joins(`LEFT JOIN LATERAL (SELECT COALESCE(SUM(r.amount),0) AS paid FROM crm_receivables r
 WHERE r.tenant_id = b.tenant_id AND r.order_id = b.order_id AND r.plan_id = b.id AND r.deleted_at IS NULL AND ` + effectiveBusinessReceipt + `) allocated ON true`)
}

func (s CrmCustomerService) BusinessSummary(ctx context.Context, id string) (CustomerBusinessSummary, error) {
	result := CustomerBusinessSummary{AsOf: time.Now()}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerBusinessActor(ctx, tx, id)
		if err != nil {
			return err
		}
		orders := tx.Table("crm_order b").Where("b.tenant_id = ? AND b.deleted_at IS NULL AND b.id IN (?)", actor.TenantID, businessOrders(tx, actor, manager, id, true))
		fields := "COUNT(*) AS signed_count, " + moneySQL("SUM("+businessOrderDue+")") + " AS due_amount, " + moneySQL("SUM(b.paid_amount)") + " AS paid_amount, " + moneySQL("SUM(GREATEST("+businessOrderDue+"-COALESCE(b.paid_amount,0),0))") + " AS outstanding_amount"
		if err := orders.Select(fields).Scan(&result).Error; err != nil {
			return err
		}
		var plans struct {
			OverduePlanCount     int64
			OverduePlanUnsettled string
		}
		if err := businessPlans(tx, actor, manager, id, true).Where("b.plan_date < ? AND b.amount > allocated.paid", dayStart(result.AsOf)).Select("COUNT(*) AS overdue_plan_count, " + moneySQL("SUM(GREATEST(b.amount-allocated.paid,0))") + " AS overdue_plan_unsettled").Scan(&plans).Error; err != nil {
			return err
		}
		result.OverduePlanCount, result.OverduePlanUnsettled = plans.OverduePlanCount, plans.OverduePlanUnsettled
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func (s CrmCustomerService) BusinessRecords(ctx context.Context, req CustomerBusinessRequest) (CustomerBusinessPage, error) {
	page := CustomerBusinessPage{Items: []CustomerBusinessRow{}}
	if req.Kind != "opportunities" && req.Kind != "quotes" && req.Kind != "orders" && req.Kind != "plans" && req.Kind != "receipts" {
		return page, errors.New("业务记录类型无效")
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	var before string
	var cursor businessCursor
	if req.Cursor != "" {
		if len(req.Cursor) > 1024 {
			return page, errors.New("业务记录游标无效")
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Customer != req.CustomerID || cursor.Kind != req.Kind {
			return page, errors.New("业务记录游标无效，请刷新")
		}
		if n, err := strconv.ParseInt(cursor.ID, 10, 64); err != nil || n <= 0 {
			return page, errors.New("业务记录游标无效")
		}
		before = cursor.ID
		// Tenant comparison is performed against the authenticated actor inside the transaction.
		if cursor.Tenant == "" {
			return page, errors.New("业务记录游标无效")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerBusinessActor(ctx, tx, req.CustomerID)
		if err != nil {
			return err
		}
		if before != "" && cursor.Tenant != actor.TenantID {
			return errors.New("业务记录游标无效")
		}
		var q *gorm.DB
		fields := "b.id, b.created_at"
		switch req.Kind {
		case "orders":
			q = tx.Table("crm_order b").Where("b.tenant_id = ? AND b.deleted_at IS NULL AND b.id IN (?)", actor.TenantID, businessOrders(tx, actor, manager, req.CustomerID, false))
			fields += ", b.title, b.order_no AS number, b.status, b.signed_at AS event_at, " + moneySQL("b.amount") + " AS amount, " + moneySQL(businessOrderDue) + " AS due_amount, " + moneySQL("b.paid_amount") + " AS paid_amount, " + moneySQL("GREATEST("+businessOrderDue+"-COALESCE(b.paid_amount,0),0)") + " AS remaining_amount"
		case "quotes", "opportunities":
			table := "crm_opportunity"
			if req.Kind == "quotes" {
				table = "crm_quote"
			}
			q = tx.Table(table+" b").Where("b.tenant_id = ? AND b.customer_id = ? AND b.deleted_at IS NULL", actor.TenantID, req.CustomerID)
			if !manager {
				q = q.Where("b.owner_user_id = ?", actor.ID)
			}
			if req.Kind == "quotes" {
				fields += ", b.subject AS title, b.quote_no AS number, b.status, b.valid_until AS event_at, " + moneySQL("b.final_amount") + " AS amount"
			} else {
				fields += ", b.title, b.status, b.expected_close_date AS event_at, " + moneySQL("b.amount") + " AS amount"
			}
		case "receipts":
			q = tx.Table("crm_receivables b").Where("b.tenant_id = ? AND b.deleted_at IS NULL AND b.order_id IN (?)", actor.TenantID, businessOrders(tx, actor, manager, req.CustomerID, false))
			q = q.Joins("JOIN crm_order parent ON parent.tenant_id = b.tenant_id AND parent.id = b.order_id AND parent.deleted_at IS NULL")
			fields += ", b.order_id, parent.title AS order_title, b.receivables_no AS number, b.status, b.approve_status AS approval_status, b.received_at AS event_at, " + moneySQL("b.amount") + " AS amount"
		case "plans":
			q = businessPlans(tx, actor, manager, req.CustomerID, false).Joins("JOIN crm_order parent ON parent.tenant_id = b.tenant_id AND parent.id = b.order_id AND parent.deleted_at IS NULL")
			fields += ", b.order_id, parent.title AS order_title, b.period_no, b.plan_date AS event_at, " + moneySQL("b.amount") + " AS amount, " + moneySQL("allocated.paid") + " AS paid_amount, " + moneySQL("GREATEST(b.amount-allocated.paid,0)") + " AS remaining_amount, CASE WHEN parent.status = 'cancelled' THEN 'cancelled' WHEN parent.status = 'draft' THEN 'draft' WHEN b.amount <= allocated.paid THEN 'received' WHEN b.plan_date < ? THEN 'overdue' ELSE 'pending' END AS status"
		}
		q = q.Select(fields)
		if req.Kind == "plans" {
			q = q.Select(fields, dayStart(time.Now()))
		}
		if before != "" {
			q = q.Where("b.id < ?", before)
		}
		if err := q.Order("b.id DESC").Limit(req.PageSize + 1).Scan(&page.Items).Error; err != nil {
			return err
		}
		if len(page.Items) > req.PageSize {
			page.HasMore = true
			page.Items = page.Items[:req.PageSize]
			raw, err := json.Marshal(businessCursor{Tenant: actor.TenantID, Customer: req.CustomerID, Kind: req.Kind, ID: page.Items[req.PageSize-1].ID})
			if err != nil {
				return err
			}
			page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return page, err
}
