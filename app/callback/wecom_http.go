package callback

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"core/app/services"
	"core/common/identity"
	"core/common/log"
	"core/conf"
	"core/pkg/easywework"
)

// StartWeComCallbackServer 启动企微事件回调 webhook（独立 gin 端口，收企微主动 POST 的加密事件）。
// URL: http://<host>:<CallbackPort>/wecom/callback/:corp_id
// 每个 corp_id 用其自有 CallbackToken/AesKey 构造 CallBackHandler 验签解密，再按 corp 归属租户分发。
// 与 legacy 单 corp 的 we_work.Callback 全局解耦；不做签名外校验（验签即鉴权）。
func StartWeComCallbackServer() {
	port := conf.Settings.Server.CallbackPort
	if port == 0 {
		port = 9003
	}
	r := gin.Default()
	r.Any("/wecom/callback/:corp_id", wecomCallback)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}
	go func() {
		log.Sugar.Infow("wecom callback server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Sugar.Errorw("wecom callback server exit", "err", err)
		}
	}()
}

func wecomCallback(c *gin.Context) {
	corpID := c.Param("corp_id")
	cred, err := services.NewCrmChannelConfig().GetByCorpID(corpID)
	if err != nil {
		c.String(http.StatusNotFound, "corp_id 未配置或已停用")
		return
	}
	cb, err := workwx.NewCBHandler(cred.CallbackToken, cred.CallbackAesKey)
	if err != nil {
		log.Sugar.Errorw("new cb handler failed", "corp_id", corpID, "err", err)
		c.String(http.StatusBadRequest, "回调凭据无效")
		return
	}
	if c.Request.Method == http.MethodGet {
		// URL 校验：验签 + 解密 echostr 回显（企微配置回调时触发）
		cb.EchoTestHandler(c.Writer, c.Request)
		return
	}
	msg, err := cb.GetCallBackMsg(c.Request)
	if err != nil {
		log.Sugar.Errorw("callback verify/decrypt failed", "corp_id", corpID, "err", err)
		c.String(http.StatusBadRequest, "验签或解密失败")
		return
	}
	dispatchWeComEvent(cred.TenantID, msg)
	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// dispatchWeComEvent 把企微事件分发到租户感知的 CRM handler（仅关注外部联系人/员工映射）。
// 其余事件（群聊/标签/部门等）本轮不接——保留在 legacy 层，避免双轨冲突。
// 每条事件记录 CrmWecomEventLog（handled=是否命中并处理，error=失败原因）。
func dispatchWeComEvent(tenantID string, msg *workwx.RxMessage) {
	logger := services.NewCrmWecomEventLog()
	corpID := msg.ToUserID
	if msg.MsgType != workwx.MessageTypeEvent {
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
		logger.Record(ctx, corpID, string(msg.MsgType), string(msg.ChangeType), "", "", false, "非事件消息，忽略")
		return
	}
	handled := false
	var extUser, extCustomer, errMsg string
	customer := services.NewCrmCustomer()
	user := services.NewUser()
	switch {
	case msg.Event == workwx.EventTypeChangeExternalContact && msg.ChangeType == workwx.ChangeTypeAddExternalContact:
		if e, ok := msg.EventAddExternalContact(); ok {
			extUser, extCustomer = e.GetUserID(), e.GetExternalUserID()
			handled = true
			ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
			if err := customer.SyncFromWeCom(ctx, corpID, e.GetUserID(), e.GetExternalUserID(), ""); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeExternalContact && msg.ChangeType == workwx.ChangeTypeEditExternalContact:
		if e, ok := msg.EventEditExternalContact(); ok {
			extUser, extCustomer = e.GetUserID(), e.GetExternalUserID()
			handled = true
			ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
			if err := customer.SyncFromWeCom(ctx, corpID, e.GetUserID(), e.GetExternalUserID(), ""); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeExternalContact && msg.ChangeType == workwx.ChangeTypeDelExternalContact:
		if e, ok := msg.EventDelExternalContact(); ok {
			extUser, extCustomer = e.GetUserID(), e.GetExternalUserID()
			handled = true
			ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
			if err := customer.MarkWecomCustomerLost(ctx, corpID, e.GetExternalUserID()); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeExternalContact && msg.ChangeType == workwx.ChangeTypeDelFollowUser:
		if e, ok := msg.EventDelFollowUser(); ok {
			extUser, extCustomer = e.GetUserID(), e.GetExternalUserID()
			handled = true
			ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
			if err := customer.MarkWecomCustomerLost(ctx, corpID, e.GetExternalUserID()); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeContact && (msg.ChangeType == workwx.ChangeTypeCreateUser || msg.ChangeType == workwx.ChangeTypeUpdateUser):
		if e, ok := msg.EventCreateUser(); ok {
			extUser = e.GetUserID()
			handled = true
			ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
			if err := user.EnsureWecomLink(ctx, corpID, e.GetUserID()); err != nil {
				errMsg = err.Error()
			}
		} else if e, ok := msg.EventUpdateUser(); ok {
			extUser = e.GetUserID()
			handled = true
			ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
			if err := user.EnsureWecomLink(ctx, corpID, e.GetUserID()); err != nil {
				errMsg = err.Error()
			}
		}
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	logger.Record(ctx, corpID, string(msg.Event), string(msg.ChangeType), extUser, extCustomer, handled, errMsg)
}
