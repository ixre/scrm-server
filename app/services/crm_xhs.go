package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 小红书 → CRM 同步（webhook 驱动） ----------

// XhsLeadEvent 小红书留资事件（webhook 入参）。
type XhsLeadEvent struct {
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
	AppID      string `json:"app_id"`
	ExtUserID  string `json:"ext_user_id"`
	Nickname   string `json:"nickname"`
	ActionType string `json:"action_type"` // comment/dm
	Content    string `json:"content"`
	ExtRef     string `json:"ext_ref"` // 笔记ID/会话ID
	HappenedAt int64  `json:"happened_at"`
}

// SyncFromXhs 小红书留资事件落 CRM：
// 1. 按 (tenant, xiaohongshu, ext_user_id) 幂等 upsert CrmCustomer(source=xiaohongshu)
// 2. 写 ChannelLink(customer, xiaohongshu, ext_user_id)
// 3. 写 CrmChannelInteraction(action_type=lead)
// 4. 自动创建引流任务（小红书→企微）
func (s CrmCustomerService) SyncFromXhs(ctx context.Context, appID string, ev XhsLeadEvent) error {
	tenantID := identity.TenantID(ctx)


	if ev.ExtUserID == "" {
		return errors.New("ext_user_id 不能为空")
	}
	// 解析租户的小红书渠道配置
	cfg, err := s.getXhsConfig(ctx, appID)
	if err != nil {
		return err
	}

	// 1. upsert CrmCustomer
	var c models.CrmCustomer
	err = models.MustFor(ctx).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?",  constants.ChannelXiaohongshu,  appID,  ev.ExtUserID).First(&c).Error
	switch {
	case err == nil:
		// 已存在：更新昵称/手机号
		updates := map[string]any{}
		if ev.Nickname != "" && c.Name == "" {
			updates["name"] = ev.Nickname
		}
		if ev.Mobile != "" && c.Mobile == "" {
			updates["mobile"] = ev.Mobile
		}
		if len(updates) > 0 {
			models.DB.Model(&models.CrmCustomer{}).Where("id = ?", c.ID).Updates(updates)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		c = models.CrmCustomer{
			Model:         models.Model{ID: id_generator.StringID()},
			TenantID:      tenantID,
			Name:          ev.Nickname,
			Mobile:        ev.Mobile,
			Source:        constants.ChannelXiaohongshu,
			ExtCorpID:     appID,
			ExtCustomerID: ev.ExtUserID,
			AccountType:   "小红书",
			PoolStatus:    1, // 小红书留资无归属销售，进公海池待认领
		}
		now := time.Now()
		c.PoolAt = &now
		if cerr := createCRM(models.DB, &c, "OwnerUserID"); cerr != nil {
			return cerr
		}
	default:
		return err
	}

	// 2. ChannelLink
	if lerr := upsertCustomerChannelLink(ctx, c); lerr != nil {
		log.Sugar.Errorw("xhs customer channel link failed", "err", lerr)
	}

	// 3. CrmChannelInteraction(lead)
	s.recordXhsInteraction(ctx, c.ID, "lead", ev.Content, ev.FormID, ev.HappenedAt)

	// 4. 自动创建引流任务（小红书→企微）
	if cfg.Enabled == 1 {
		NewCrmChannelTask().CreateLeadToWecomTask(ctx, c.ID)
	}

	log.Sugar.Infow("xhs lead synced", "tenant", tenantID, "ext_user", ev.ExtUserID, "customer", c.ID)
	return nil
}

// SyncInteractionFromXhs 小红书互动事件（评论/私信）落 CrmChannelInteraction。
// 若客户不存在则自动创建（source=xiaohongshu, 进公海）。
func (s CrmCustomerService) SyncInteractionFromXhs(ctx context.Context, appID string, ev XhsInteractionEvent) error {
	tenantID := identity.TenantID(ctx)


	if ev.ExtUserID == "" {
		return errors.New("ext_user_id 不能为空")
	}

	// 确保客户存在
	var c models.CrmCustomer
	err := models.MustFor(ctx).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?",  constants.ChannelXiaohongshu,  appID,  ev.ExtUserID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c = models.CrmCustomer{
			Model:         models.Model{ID: id_generator.StringID()},
			TenantID:      tenantID,
			Name:          ev.Nickname,
			Source:        constants.ChannelXiaohongshu,
			ExtCorpID:     appID,
			ExtCustomerID: ev.ExtUserID,
			AccountType:   "小红书",
			PoolStatus:    1,
		}
		now := time.Now()
		c.PoolAt = &now
		if cerr := createCRM(models.DB, &c, "OwnerUserID"); cerr != nil {
			return cerr
		}
		upsertCustomerChannelLink(ctx, c)
	} else if err != nil {
		return err
	}

	// 写互动记录
	s.recordXhsInteraction(ctx, c.ID, ev.ActionType, ev.Content, ev.ExtRef, ev.HappenedAt)
	return nil
}

// recordXhsInteraction 写一条小红书渠道互动记录。
func (s CrmCustomerService) recordXhsInteraction(ctx context.Context, customerID, actionType, content, extRef string, happenedAt int64) {
	tenantID := identity.TenantID(ctx)


	var t *time.Time
	if happenedAt > 0 {
		v := time.Unix(happenedAt, 0)
		t = &v
	}
	rec := models.CrmChannelInteraction{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		CustomerID: customerID,
		Channel:    constants.ChannelXiaohongshu,
		ActionType: actionType,
		Content:    content,
		HappenedAt: t,
		ExtRef:     extRef,
	}
	if err := createCRM(models.DB, &rec, "CustomerID"); err != nil {
		log.Sugar.Errorw("xhs interaction record failed", "err", err)
	}
	// Phase 7：敏感词检测（评论/私信内容）
	if content != "" && (actionType == "comment" || actionType == "dm") {
		go func() {
			defer func() { recover() }()
			var cust models.CrmCustomer
			ownerID := ""
			if models.DB.Where("id = ?", customerID).First(&cust).Error == nil {
				ownerID = cust.OwnerUserID
			}
			NewCrmSensitiveWord().Check(ctx, ownerID, customerID, content)
		}()
	}
}

// getXhsConfig 查租户的小红书渠道配置。
func (s CrmCustomerService) getXhsConfig(ctx context.Context, appID string) (models.CrmChannelConfig, error) {


	var cfg models.CrmChannelConfig
	err := models.MustFor(ctx).Where("channel = ? AND corp_id = ?",  constants.ChannelXiaohongshu,  appID).First(&cfg).Error
	return cfg, err
}
