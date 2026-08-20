package callback

import (
	"context"
	"encoding/json"
	"net/http"

	"core/app/services"
	"core/common/identity"
	"core/common/log"

	"github.com/gin-gonic/gin"
)

// gzhCallback 处理公众号 webhook 事件。
// 事件类型：subscribe（关注留资）/message（消息）/menu_click（菜单点击）。
// 签名校验：若请求带 X-Signature header，则用渠道 CallbackToken 做 HMAC-SHA256 校验。
func gzhCallback(c *gin.Context) {
	appID := c.Param("app_id")

	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "read body failed"})
		return
	}

	tenantID, err := services.NewCrmChannelConfig().GetTenantByChannelApp(appID)
	if err != nil {
		log.Sugar.Warnw("gzh callback: app_id not configured", "app_id", appID, "err", err)
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "app_id 未配置"})
		return
	}

	// 签名校验（复用小红书签名校验逻辑，CallbackToken 共用）
	if sig := c.GetHeader("X-Signature"); sig != "" {
		if !services.NewCrmChannelConfig().VerifyXhsSignature(appID, rawBody, sig) {
			log.Sugar.Warnw("gzh callback: signature mismatch", "app_id", appID)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 1, "msg": "签名校验失败"})
			return
		}
	}

	var body struct {
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		log.Sugar.Errorw("gzh callback: unmarshal failed", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}

	customer := services.NewCrmCustomer()
	switch body.EventType {
	case "subscribe", "lead":
		var ev services.GzhLeadEvent
		if err := json.Unmarshal(body.Data, &ev); err != nil {
			log.Sugar.Errorw("gzh callback: lead unmarshal failed", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid lead data"})
			return
		}
		ev.AppID = appID
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
		if err := customer.SyncFromGzh(ctx, appID, ev); err != nil {
			log.Sugar.Errorw("gzh callback: lead sync failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
			return
		}
	case "message", "menu_click":
		var ev services.GzhInteractionEvent
		if err := json.Unmarshal(body.Data, &ev); err != nil {
			log.Sugar.Errorw("gzh callback: interaction unmarshal failed", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid interaction data"})
			return
		}
		ev.AppID = appID
		ev.ActionType = body.EventType
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
		if err := customer.SyncInteractionFromGzh(ctx, appID, ev); err != nil {
			log.Sugar.Errorw("gzh callback: interaction sync failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
			return
		}
	default:
		log.Sugar.Warnw("gzh callback: unknown event_type", "event_type", body.EventType)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// douyinCallback 处理抖音 webhook 事件。
// 事件类型：lead（主页留资）/dm（私信）/comment（评论）。
func douyinCallback(c *gin.Context) {
	appID := c.Param("app_id")

	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "read body failed"})
		return
	}

	tenantID, err := services.NewCrmChannelConfig().GetTenantByChannelApp(appID)
	if err != nil {
		log.Sugar.Warnw("douyin callback: app_id not configured", "app_id", appID, "err", err)
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "app_id 未配置"})
		return
	}

	// 签名校验
	if sig := c.GetHeader("X-Signature"); sig != "" {
		if !services.NewCrmChannelConfig().VerifyXhsSignature(appID, rawBody, sig) {
			log.Sugar.Warnw("douyin callback: signature mismatch", "app_id", appID)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 1, "msg": "签名校验失败"})
			return
		}
	}

	var body struct {
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		log.Sugar.Errorw("douyin callback: unmarshal failed", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}

	customer := services.NewCrmCustomer()
	switch body.EventType {
	case "lead":
		var ev services.DouyinLeadEvent
		if err := json.Unmarshal(body.Data, &ev); err != nil {
			log.Sugar.Errorw("douyin callback: lead unmarshal failed", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid lead data"})
			return
		}
		ev.AppID = appID
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
		if err := customer.SyncFromDouyin(ctx, appID, ev); err != nil {
			log.Sugar.Errorw("douyin callback: lead sync failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
			return
		}
	case "dm", "comment":
		var ev services.DouyinInteractionEvent
		if err := json.Unmarshal(body.Data, &ev); err != nil {
			log.Sugar.Errorw("douyin callback: interaction unmarshal failed", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid interaction data"})
			return
		}
		ev.AppID = appID
		ev.ActionType = body.EventType
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
		if err := customer.SyncInteractionFromDouyin(ctx, appID, ev); err != nil {
			log.Sugar.Errorw("douyin callback: interaction sync failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
			return
		}
	default:
		log.Sugar.Warnw("douyin callback: unknown event_type", "event_type", body.EventType)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}
