package services

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"core/app/models"
	"core/common/identity"
	"gorm.io/gorm"
)

type CustomerHistoryRequest struct {
	CustomerID string `json:"customer_id"`
	Kind       string `json:"kind"`
	Cursor     string `json:"cursor"`
	PageSize   int    `json:"page_size"`
}
type CustomerHistoryPage struct {
	Items      any    `json:"items"`
	HasMore    bool   `json:"has_more"`
	NextCursor string `json:"next_cursor"`
}
type historyCursor struct {
	Tenant   string    `json:"tenant"`
	Customer string    `json:"customer"`
	Kind     string    `json:"kind"`
	At       time.Time `json:"at"`
	ID       string    `json:"id"`
}
type HistoryPosition struct {
	CursorTime time.Time `gorm:"column:cursor_time" json:"-"`
}
type followupHistoryRow struct {
	models.CrmFollowup
	HistoryPosition
}
type activityHistoryRow struct {
	CustomerActivityRow
	HistoryPosition
}
type interactionHistoryRow struct {
	models.CrmChannelInteraction
	HistoryPosition
}
type transferHistoryRow struct {
	models.CrmTransferLog
	HistoryPosition
}

func historyPage[T any](db *gorm.DB, req CustomerHistoryRequest, tid string, key func(T) (time.Time, string)) (CustomerHistoryPage, error) {
	var rows []T
	if err := db.Limit(req.PageSize + 1).Scan(&rows).Error; err != nil {
		return CustomerHistoryPage{}, err
	}
	page := CustomerHistoryPage{HasMore: len(rows) > req.PageSize}
	if page.HasMore {
		rows = rows[:req.PageSize]
		at, id := key(rows[len(rows)-1])
		raw, err := json.Marshal(historyCursor{Tenant: tid, Customer: req.CustomerID, Kind: req.Kind, At: at, ID: id})
		if err != nil {
			return page, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	if rows == nil {
		rows = make([]T, 0)
	}
	page.Items = rows
	return page, nil
}

// History uses a timestamp+ID cursor. No whole-history count or deep OFFSET is required.
func (s CrmCustomerService) History(ctx context.Context, req CustomerHistoryRequest) (CustomerHistoryPage, error) {
	if _, err := s.Get(ctx, req.CustomerID); err != nil {
		return CustomerHistoryPage{}, err
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	tid := identity.TenantID(ctx)
	var table, when string
	switch req.Kind {
	case "followups":
		table = "crm_followup"
		when = "COALESCE(t.created_at, 'epoch'::timestamptz)"
	case "activity_records":
		table = "crm_activity_record"
		when = "COALESCE(t.happened_at, t.created_at, 'epoch'::timestamptz)"
	case "channel_interactions":
		table = "crm_channel_interaction"
		when = "COALESCE(t.happened_at, t.created_at, 'epoch'::timestamptz)"
	case "transfer_logs":
		table = "crm_transfer_log"
		when = "COALESCE(t.created_at, 'epoch'::timestamptz)"
	default:
		return CustomerHistoryPage{}, errors.New("历史记录类型无效")
	}
	db := models.DB.WithContext(ctx).Table(table+" t").Where("t.tenant_id = ? AND t.customer_id = ? AND t.deleted_at IS NULL", tid, req.CustomerID)
	if req.Kind == "channel_interactions" {
		db = db.Where("t.channel <> ?", "wecom")
	}
	if req.Cursor != "" {
		var cursor historyCursor
		if len(req.Cursor) > 1024 {
			return CustomerHistoryPage{}, errors.New("历史游标无效")
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Tenant != tid || cursor.Customer != req.CustomerID || cursor.Kind != req.Kind || cursor.At.IsZero() {
			return CustomerHistoryPage{}, errors.New("历史游标无效，请重新加载")
		}
		id, err := strconv.ParseInt(cursor.ID, 10, 64)
		if err != nil || id <= 0 {
			return CustomerHistoryPage{}, errors.New("历史游标无效")
		}
		db = db.Where("("+when+", t.id) < (?, ?)", cursor.At, cursor.ID)
	}
	selectFields := "t.*, " + when + " AS cursor_time"
	if req.Kind == "activity_records" {
		db = db.Joins("LEFT JOIN crm_activity a ON a.id = t.activity_id AND a.tenant_id = t.tenant_id AND a.deleted_at IS NULL")
		selectFields += ", a.name AS activity_name, a.activity_type AS activity_type, a.status AS activity_status"
	}
	db = db.Select(selectFields).Order(when + " DESC, t.id DESC")
	switch req.Kind {
	case "followups":
		return historyPage(db, req, tid, func(r followupHistoryRow) (time.Time, string) { return r.CursorTime, r.ID })
	case "activity_records":
		return historyPage(db, req, tid, func(r activityHistoryRow) (time.Time, string) { return r.CursorTime, r.ID })
	case "channel_interactions":
		return historyPage(db, req, tid, func(r interactionHistoryRow) (time.Time, string) { return r.CursorTime, r.ID })
	default:
		return historyPage(db, req, tid, func(r transferHistoryRow) (time.Time, string) { return r.CursorTime, r.ID })
	}
}
