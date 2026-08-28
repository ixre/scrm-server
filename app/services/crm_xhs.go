package services

import (
	"core/common/identity"

	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"

	"errors"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------- 小红书 → CRM 同步（webhook 驱动） ----------

// XhsLeadEvent 小红书留资事件（webhook 入参）。
type XhsLeadEvent struct {
	EventID    string `json:"event_id"`
	AppID      string `json:"app_id"`      // 小红书应用 ID（对应 CrmChannelConfig.CorpID）
	ExtUserID  string `json:"ext_user_id"` // 小红书用户外部 ID
	Nickname   string `json:"nickname"`    // 小红书昵称
	Mobile     string `json:"mobile"`      // 留资手机号
	FormID     string `json:"form_id"`     // 留资表单 ID
	NoteID     string `json:"note_id"`     // 笔记 ID（可选）
	Content    string `json:"content"`     // 留资内容/备注
	HappenedAt int64  `json:"happened_at"` // 事件时间戳（秒）
}

// XhsInteractionEvent 小红书互动事件（评论/私信）。
type XhsInteractionEvent struct {
	EventID    string `json:"event_id"`
	AppID      string `json:"app_id"`
	ExtUserID  string `json:"ext_user_id"`
	Nickname   string `json:"nickname"`
	ActionType string `json:"action_type"` // comment/dm
	Content    string `json:"content"`
	ExtRef     string `json:"ext_ref"` // 笔记ID/会话ID
	HappenedAt int64  `json:"happened_at"`
}

// SyncFromXhs commits customer, identity, interaction and follow-through task together.
func (s CrmCustomerService) SyncFromXhs(ctx context.Context, appID string, ev XhsLeadEvent) error {
	return s.syncXhsEvent(ctx, appID, ev.ExtUserID, ev.Nickname, ev.Mobile, "lead", ev.Content, ev.FormID, ev.EventID, ev.HappenedAt)
}
func (s CrmCustomerService) SyncInteractionFromXhs(ctx context.Context, appID string, ev XhsInteractionEvent) error {
	if ev.ActionType != "comment" && ev.ActionType != "dm" {
		return errors.New("互动类型无效")
	}
	return s.syncXhsEvent(ctx, appID, ev.ExtUserID, ev.Nickname, "", ev.ActionType, ev.Content, ev.ExtRef, ev.EventID, ev.HappenedAt)
}
func (s CrmCustomerService) syncXhsEvent(ctx context.Context, appID, extID, nickname, mobile, action, content, ref, eventID string, happenedAt int64) error {
	tid := identity.TenantID(ctx)
	appID, extID = strings.TrimSpace(appID), strings.TrimSpace(extID)
	if tid == "" || appID == "" || extID == "" {
		return errors.New("租户、渠道账号和外部用户ID不能为空")
	}
	identityKey, _ := json.Marshal([]string{appID, extID})
	fingerprint := []any{appID, extID, action, eventID}
	if eventID == "" {
		fingerprint = append(fingerprint, ref, happenedAt, content, mobile)
	}
	raw, _ := json.Marshal(fingerprint)
	eventKey := fmt.Sprintf("%x", sha256.Sum256(raw))
	var customerID string
	createdEvent := false
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cfg models.CrmChannelConfig
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ? AND channel = ? AND corp_id = ? AND enabled = 1", tid, constants.ChannelXiaohongshu, appID).First(&cfg).Error; err != nil {
			return errors.New("小红书渠道未启用或不存在")
		}
		if err := lockSalesKey(ctx, tx, "xhs-customer", string(identityKey)); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.CrmChannelInteraction{}).Where("tenant_id = ? AND event_key = ? AND event_key <> ''", tid, eventKey).Unscoped().Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		var links []models.ChannelLink
		if err := tx.Where("tenant_id = ? AND channel = ? AND entity_type = 'customer' AND ext_corp_id = ? AND ext_id = ?", tid, constants.ChannelXiaohongshu, appID, extID).Limit(2).Find(&links).Error; err != nil {
			return err
		}
		if len(links) > 1 && links[0].EntityID != links[1].EntityID {
			return errors.New("外部身份映射冲突，请先合并客户")
		}
		var c models.CrmCustomer
		if len(links) > 0 {
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tid, links[0].EntityID).First(&c).Error; err != nil {
				return errors.New("映射客户已删除，请先恢复或修复映射")
			}
		} else {
			var matches []models.CrmCustomer
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND source = ? AND ext_corp_id = ? AND ext_customer_id = ?", tid, constants.ChannelXiaohongshu, appID, extID).Limit(2).Find(&matches).Error; err != nil {
				return err
			}
			if len(matches) > 1 {
				return errors.New("渠道身份存在重复客户，请先合并")
			}
			if len(matches) == 1 {
				c = matches[0]
			} else {
				now := time.Now()
				c = models.CrmCustomer{Model: models.Model{ID: id_generator.StringID()}, TenantID: tid, Name: nickname, Mobile: mobile, Source: constants.ChannelXiaohongshu, ExtCorpID: appID, ExtCustomerID: extID, AccountType: "小红书", PoolStatus: 1, PoolAt: &now}
				if err := createCRM(tx, &c); err != nil {
					return err
				}
				if err := allocateNewCustomer(ctx, tx, &c); err != nil {
					return err
				}
			}
			link := models.ChannelLink{Model: models.Model{ID: id_generator.StringID()}, TenantID: tid, Channel: constants.ChannelXiaohongshu, EntityType: "customer", EntityID: c.ID, ExtID: extID, ExtCorpID: appID}
			if err := createCRM(tx, &link); err != nil {
				return err
			}
		}
		updates := map[string]any{}
		if c.Name == "" && nickname != "" {
			updates["name"] = nickname
		}
		if c.Mobile == "" && mobile != "" {
			updates["mobile"] = mobile
		}
		if len(updates) > 0 {
			if err := tx.Model(&c).Updates(updates).Error; err != nil {
				return err
			}
		}
		var when *time.Time
		if happenedAt > 0 {
			t := time.Unix(happenedAt, 0)
			when = &t
		}
		rec := models.CrmChannelInteraction{Model: models.Model{ID: id_generator.StringID()}, TenantID: tid, CustomerID: c.ID, Channel: constants.ChannelXiaohongshu, ActionType: action, Content: content, ExtRef: ref, HappenedAt: when, EventKey: eventKey}
		if err := createCRM(tx, &rec); err != nil {
			return err
		}
		if action == "lead" {
			if err := createLeadToWecomTask(ctx, tx, c.ID); err != nil {
				return err
			}
		}
		customerID = c.ID
		createdEvent = true
		return nil
	})
	if err == nil && createdEvent && content != "" && (action == "comment" || action == "dm") {
		// Sensitive-word detection remains after commit, as in the previous webhook path.
		go func() {
			defer func() { recover() }()
			var c models.CrmCustomer
			if models.DB.Where("tenant_id = ? AND id = ?", tid, customerID).First(&c).Error == nil {
				NewCrmSensitiveWord().Check(ctx, c.OwnerUserID, customerID, content)
			}
		}()
	}
	return err
}
