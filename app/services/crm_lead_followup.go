package services

import (
	"context"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

type LeadScheduleRequest struct {
	ID             string     `json:"id"`
	NextFollowupAt *time.Time `json:"next_followup_at"`
	NextAction     string     `json:"next_action"`
}

func validateLeadAppointment(at *time.Time, action string) error {
	if at != nil && at.IsZero() {
		return errors.New("请选择有效预约时间")
	}
	if utf8.RuneCountInString(action) > 512 {
		return errors.New("下一步行动最多512字")
	}
	return nil
}

func (s CrmLeadService) SetSchedule(ctx context.Context, req LeadScheduleRequest) error {
	req.NextAction = strings.TrimSpace(req.NextAction)
	if err := validateLeadAppointment(req.NextFollowupAt, req.NextAction); err != nil {
		return err
	}
	if req.NextFollowupAt == nil {
		req.NextAction = ""
	}
	_, err := mutateLeads(ctx, []string{req.ID}, func(tx *gorm.DB, lead *models.CrmLead) error {
		if err := mutableLead(*lead); err != nil {
			return err
		}
		if lead.PoolStatus == 1 {
			return errors.New("请先领取线索再预约")
		}
		same := lead.NextFollowupAt == nil && req.NextFollowupAt == nil
		if lead.NextFollowupAt != nil && req.NextFollowupAt != nil {
			same = lead.NextFollowupAt.Equal(req.NextFollowupAt.Truncate(time.Microsecond))
		}
		if same && lead.NextAction == req.NextAction {
			return nil
		}
		if err := tx.Model(lead).Updates(map[string]any{"next_followup_at": req.NextFollowupAt, "next_action": req.NextAction}).Error; err != nil {
			return err
		}
		content := "调整线索跟进预约"
		if req.NextFollowupAt == nil {
			content = "取消线索跟进预约"
		}
		row := models.CrmFollowup{Model: models.Model{ID: id_generator.StringID()}, TenantID: lead.TenantID, LeadID: lead.ID, UserID: identity.UserID(ctx), Type: "other", Content: content, NextFollowupAt: req.NextFollowupAt, NextAction: req.NextAction}
		return createCRM(tx, &row)
	})
	return err
}

type LeadHistoryRequest struct {
	LeadID   string `json:"lead_id"`
	Cursor   string `json:"cursor"`
	PageSize int    `json:"page_size"`
}
type LeadHistoryPage struct {
	Items      []models.CrmFollowup `json:"items"`
	HasMore    bool                 `json:"has_more"`
	NextCursor string               `json:"next_cursor"`
}
type leadHistoryCursor struct{ Tenant, Lead, ID string }

func (s CrmLeadService) History(ctx context.Context, req LeadHistoryRequest) (LeadHistoryPage, error) {
	page := LeadHistoryPage{Items: []models.CrmFollowup{}}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := s.Get(ctx, req.LeadID); err != nil {
		return page, err
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	tenant := identity.TenantID(ctx)
	q := models.DB.WithContext(ctx).Where("tenant_id = ? AND lead_id = ?", tenant, req.LeadID)
	if req.Cursor != "" {
		var cursor leadHistoryCursor
		if len(req.Cursor) > 1024 {
			return page, errors.New("线索历史游标无效")
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || cursor.Tenant != tenant || cursor.Lead != req.LeadID {
			return page, errors.New("线索历史游标范围不匹配")
		}
		if n, err := strconv.ParseInt(cursor.ID, 10, 64); err != nil || n <= 0 {
			return page, errors.New("线索历史游标无效")
		}
		q = q.Where("id < ?", cursor.ID)
	}
	if err := q.Order("id DESC").Limit(req.PageSize + 1).Find(&page.Items).Error; err != nil {
		return page, err
	}
	if len(page.Items) > req.PageSize {
		page.HasMore = true
		page.Items = page.Items[:req.PageSize]
		raw, err := json.Marshal(leadHistoryCursor{tenant, req.LeadID, page.Items[len(page.Items)-1].ID})
		if err != nil {
			return page, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}
