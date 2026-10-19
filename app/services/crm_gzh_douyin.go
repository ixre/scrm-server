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

// ---------- 公众号 → CRM 同步（OAuth + 消息回调） ----------

// GzhLeadEvent 公众号留资/关注事件（webhook 入参）。
type GzhLeadEvent struct {
	AppID      string `json:"app_id"`      // 公众号 AppID（对应 CrmChannelConfig.CorpID）
	OpenID     string `json:"openid"`      // 公众号用户 OpenID
	Nickname   string `json:"nickname"`    // 微信昵称
	HeadImg    string `json:"head_img"`    // 头像URL
	Mobile     string `json:"mobile"`      // 留资手机号（OAuth 授权获取）
	FormID     string `json:"form_id"`     // 表单 ID
	Content    string `json:"content"`     // 留资内容
	HappenedAt int64  `json:"happened_at"` // 事件时间戳（秒）
}

// GzhInteractionEvent 公众号互动事件（消息/菜单点击）。
type GzhInteractionEvent struct {
	AppID      string `json:"app_id"`
	OpenID     string `json:"openid"`
	Nickname   string `json:"nickname"`
	ActionType string `json:"action_type"` // message/menu_click/subscribe
	Content    string `json:"content"`
	ExtRef     string `json:"ext_ref"` // MsgID/会话ID
	HappenedAt int64  `json:"happened_at"`
}

// SyncFromGzh 公众号留资/关注事件落 CRM：
// 1. 按 (tenant, gzh, openid) 幂等 upsert CrmCustomer(source=gzh)
// 2. 写 ChannelLink
// 3. 写 CrmChannelInteraction
func (s CrmCustomerService) SyncFromGzh(ctx context.Context, appID string, ev GzhLeadEvent) error {
	tenantID := identity.TenantID(ctx)

	if ev.OpenID == "" {
		return errors.New("openid 不能为空")
	}

	// 1. upsert CrmCustomer
	var c models.CrmCustomer
	err := models.MustFor(ctx).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?", constants.ChannelGzh, appID, ev.OpenID).First(&c).Error
	switch {
	case err == nil:
		updates := map[string]any{}
		if ev.Nickname != "" && c.Name == "" {
			updates["name"] = ev.Nickname
		}
		if ev.Mobile != "" && c.Mobile == "" {
			updates["mobile"] = ev.Mobile
		}
		if len(updates) > 0 {
			models.MustFor(ctx).Model(&models.CrmCustomer{}).Where("id = ?", c.ID).Updates(updates)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		c = models.CrmCustomer{
			Model:         models.Model{ID: id_generator.StringID()},
			TenantID:      tenantID,
			Name:          ev.Nickname,
			Mobile:        ev.Mobile,
			Source:        constants.ChannelGzh,
			ExtCorpID:     appID,
			ExtCustomerID: ev.OpenID,
			AccountType:   "微信公众号",
			PoolStatus:    models.CustomerPoolIn,
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
		log.Sugar.Errorw("gzh customer channel link failed", "err", lerr)
	}

	// 3. CrmChannelInteraction
	s.recordGzhInteraction(ctx, c.ID, "lead", ev.Content, ev.FormID, ev.HappenedAt)

	log.Sugar.Infow("gzh lead synced", "tenant", tenantID, "openid", ev.OpenID, "customer", c.ID)
	return nil
}

// SyncInteractionFromGzh 公众号互动事件（消息/菜单点击）落 CrmChannelInteraction。
func (s CrmCustomerService) SyncInteractionFromGzh(ctx context.Context, appID string, ev GzhInteractionEvent) error {
	tenantID := identity.TenantID(ctx)

	if ev.OpenID == "" {
		return errors.New("openid 不能为空")
	}

	var c models.CrmCustomer
	err := models.MustFor(ctx).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?", constants.ChannelGzh, appID, ev.OpenID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c = models.CrmCustomer{
			Model:         models.Model{ID: id_generator.StringID()},
			TenantID:      tenantID,
			Name:          ev.Nickname,
			Source:        constants.ChannelGzh,
			ExtCorpID:     appID,
			ExtCustomerID: ev.OpenID,
			AccountType:   "微信公众号",
			PoolStatus:    models.CustomerPoolIn,
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

	s.recordGzhInteraction(ctx, c.ID, ev.ActionType, ev.Content, ev.ExtRef, ev.HappenedAt)
	return nil
}

// recordGzhInteraction 写一条公众号渠道互动记录。
func (s CrmCustomerService) recordGzhInteraction(ctx context.Context, customerID, actionType, content, extRef string, happenedAt int64) {
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
		Channel:    constants.ChannelGzh,
		ActionType: actionType,
		Content:    content,
		HappenedAt: t,
		ExtRef:     extRef,
	}
	if err := createCRM(models.DB, &rec, "CustomerID"); err != nil {
		log.Sugar.Errorw("gzh interaction record failed", "err", err)
	}
	// Phase 7：敏感词检测（消息内容）
	if content != "" && (actionType == "message" || actionType == "comment") {
		go func() {
			defer func() { recover() }()
			var cust models.CrmCustomer
			ownerID := ""
			if models.MustFor(ctx).Where("id = ?", customerID).First(&cust).Error == nil {
				ownerID = cust.OwnerUserID
			}
			NewCrmSensitiveWord().Check(ctx, ownerID, customerID, content)
		}()
	}
}

// ---------- 抖音 → CRM 同步（主页留资 + 私信） ----------

// DouyinLeadEvent 抖音留资事件（webhook 入参）。
type DouyinLeadEvent struct {
	AppID      string `json:"app_id"`      // 抖音应用 AppID（对应 CrmChannelConfig.CorpID）
	ExtUserID  string `json:"ext_user_id"` // 抖音用户外部 ID
	Nickname   string `json:"nickname"`    // 抖音昵称
	Mobile     string `json:"mobile"`      // 留资手机号
	FormID     string `json:"form_id"`     // 留资表单 ID
	AwemeID    string `json:"aweme_id"`    // 视频 ID（可选）
	Content    string `json:"content"`     // 留资内容
	HappenedAt int64  `json:"happened_at"`
}

// DouyinInteractionEvent 抖音互动事件（私信/评论）。
type DouyinInteractionEvent struct {
	AppID      string `json:"app_id"`
	ExtUserID  string `json:"ext_user_id"`
	Nickname   string `json:"nickname"`
	ActionType string `json:"action_type"` // dm/comment
	Content    string `json:"content"`
	ExtRef     string `json:"ext_ref"` // AwemeID/会话ID
	HappenedAt int64  `json:"happened_at"`
}

// SyncFromDouyin 抖音留资事件落 CRM。
func (s CrmCustomerService) SyncFromDouyin(ctx context.Context, appID string, ev DouyinLeadEvent) error {
	tenantID := identity.TenantID(ctx)

	if ev.ExtUserID == "" {
		return errors.New("ext_user_id 不能为空")
	}

	var c models.CrmCustomer
	err := models.MustFor(ctx).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?", constants.ChannelDouyin, appID, ev.ExtUserID).First(&c).Error
	switch {
	case err == nil:
		updates := map[string]any{}
		if ev.Nickname != "" && c.Name == "" {
			updates["name"] = ev.Nickname
		}
		if ev.Mobile != "" && c.Mobile == "" {
			updates["mobile"] = ev.Mobile
		}
		if len(updates) > 0 {
			models.MustFor(ctx).Model(&models.CrmCustomer{}).Where("id = ?", c.ID).Updates(updates)
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		c = models.CrmCustomer{
			Model:         models.Model{ID: id_generator.StringID()},
			TenantID:      tenantID,
			Name:          ev.Nickname,
			Mobile:        ev.Mobile,
			Source:        constants.ChannelDouyin,
			ExtCorpID:     appID,
			ExtCustomerID: ev.ExtUserID,
			AccountType:   "抖音",
			PoolStatus:    models.CustomerPoolIn,
		}
		now := time.Now()
		c.PoolAt = &now
		if cerr := createCRM(models.DB, &c, "OwnerUserID"); cerr != nil {
			return cerr
		}
	default:
		return err
	}

	if lerr := upsertCustomerChannelLink(ctx, c); lerr != nil {
		log.Sugar.Errorw("douyin customer channel link failed", "err", lerr)
	}

	s.recordDouyinInteraction(ctx, c.ID, "lead", ev.Content, ev.FormID, ev.HappenedAt)

	log.Sugar.Infow("douyin lead synced", "tenant", tenantID, "ext_user", ev.ExtUserID, "customer", c.ID)
	return nil
}

// SyncInteractionFromDouyin 抖音互动事件（私信/评论）落 CrmChannelInteraction。
func (s CrmCustomerService) SyncInteractionFromDouyin(ctx context.Context, appID string, ev DouyinInteractionEvent) error {
	tenantID := identity.TenantID(ctx)

	if ev.ExtUserID == "" {
		return errors.New("ext_user_id 不能为空")
	}

	var c models.CrmCustomer
	err := models.MustFor(ctx).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?", constants.ChannelDouyin, appID, ev.ExtUserID).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c = models.CrmCustomer{
			Model:         models.Model{ID: id_generator.StringID()},
			TenantID:      tenantID,
			Name:          ev.Nickname,
			Source:        constants.ChannelDouyin,
			ExtCorpID:     appID,
			ExtCustomerID: ev.ExtUserID,
			AccountType:   "抖音",
			PoolStatus:    models.CustomerPoolIn,
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

	s.recordDouyinInteraction(ctx, c.ID, ev.ActionType, ev.Content, ev.ExtRef, ev.HappenedAt)
	return nil
}

// recordDouyinInteraction 写一条抖音渠道互动记录。
func (s CrmCustomerService) recordDouyinInteraction(ctx context.Context, customerID, actionType, content, extRef string, happenedAt int64) {
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
		Channel:    constants.ChannelDouyin,
		ActionType: actionType,
		Content:    content,
		HappenedAt: t,
		ExtRef:     extRef,
	}
	if err := createCRM(models.DB, &rec, "CustomerID"); err != nil {
		log.Sugar.Errorw("douyin interaction record failed", "err", err)
	}
	// Phase 7：敏感词检测（私信内容）
	if content != "" && actionType == "dm" {
		go func() {
			defer func() { recover() }()
			var cust models.CrmCustomer
			ownerID := ""
			if models.MustFor(ctx).Where("id = ?", customerID).First(&cust).Error == nil {
				ownerID = cust.OwnerUserID
			}
			NewCrmSensitiveWord().Check(ctx, ownerID, customerID, content)
		}()
	}
}
