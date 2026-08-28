package callback

import (
	"context"
	"core/app/callback/department_event"
	"core/app/callback/staff_event"
	"core/app/constants"
	"core/app/models"
	"errors"
	"fmt"
	"net/http"

	"core/app/services"
	"core/common/identity"
	"core/common/log"
	"core/conf"
	"core/pkg/easywework"
	"github.com/gin-gonic/gin"
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
	r.Any("/wecom/provider/callback", wecomProviderCallback)
	r.Any("/wecom/provider/data", wecomProviderData)

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
	cb, err := workwx.NewCBHandlerForReceiver(cred.CallbackToken, cred.CallbackAesKey, cred.ReceiverID)
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
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	data, err := cb.GetCallBackXML(c.Request)
	var msg *workwx.RxMessage
	if err == nil {
		msg, err = workwx.ParseCallbackMessage(data)
	}
	if err != nil {
		log.Sugar.Errorw("callback verify/decrypt failed", "corp_id", corpID, "err", err)
		c.String(http.StatusBadRequest, "验签或解密失败")
		return
	}
	if msg.ToUserID != corpID {
		c.String(http.StatusBadRequest, "回调企业与接收企业不一致")
		return
	}
	if err := handleVerifiedWecomEvent(cred.TenantID, msg, data); err != nil {
		c.String(http.StatusInternalServerError, "事件处理失败")
		return
	}
	c.String(http.StatusOK, "success")
}

// dispatchWeComEvent updates both legacy WeCom data and CRM channel mappings.
// 每条事件记录 CrmWecomEventLog（handled=是否命中并处理，error=失败原因）。
func dispatchWeComEvent(tenantID string, msg *workwx.RxMessage) error {
	logger := services.NewCrmWecomEventLog()
	corpID := msg.ToUserID
	ctx := services.SyncTenantCtx(identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID}), corpID)
	if msg.MsgType != workwx.MessageTypeEvent {
		logger.Record(ctx, corpID, string(msg.MsgType), string(msg.ChangeType), "", "", false, "非事件消息，忽略")
		return nil
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
			if err := services.NewCustomer().SyncSingleCustomerData(ctx, extUser, extCustomer); err != nil {
				errMsg = err.Error()
				break
			}
			if err := customer.SyncFromWeCom(ctx, corpID, e.GetUserID(), e.GetExternalUserID(), ""); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeExternalContact && msg.ChangeType == workwx.ChangeTypeEditExternalContact:
		if e, ok := msg.EventEditExternalContact(); ok {
			extUser, extCustomer = e.GetUserID(), e.GetExternalUserID()
			handled = true
			if err := services.NewCustomer().SyncSingleCustomerData(ctx, extUser, extCustomer); err != nil {
				errMsg = err.Error()
				break
			}
			if err := customer.SyncFromWeCom(ctx, corpID, e.GetUserID(), e.GetExternalUserID(), ""); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeExternalContact && msg.ChangeType == workwx.ChangeTypeDelExternalContact:
		if e, ok := msg.EventDelExternalContact(); ok {
			extUser, extCustomer = e.GetUserID(), e.GetExternalUserID()
			handled = true
			if err := services.RemoveWecomCustomerRelation(ctx, corpID, extUser, extCustomer, msg.ChangeType == workwx.ChangeTypeDelFollowUser); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeExternalContact && msg.ChangeType == workwx.ChangeTypeDelFollowUser:
		if e, ok := msg.EventDelFollowUser(); ok {
			extUser, extCustomer = e.GetUserID(), e.GetExternalUserID()
			handled = true
			if err := services.RemoveWecomCustomerRelation(ctx, corpID, extUser, extCustomer, msg.ChangeType == workwx.ChangeTypeDelFollowUser); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeContact && (msg.ChangeType == workwx.ChangeTypeCreateUser || msg.ChangeType == workwx.ChangeTypeUpdateUser):
		if e, ok := msg.EventCreateUser(); ok {
			extUser = e.GetUserID()
			handled = true
			if err := staff_event.SyncStaff(corpID, extUser); err != nil {
				errMsg = err.Error()
				break
			}
			if err := user.EnsureWecomLink(ctx, corpID, e.GetUserID()); err != nil {
				errMsg = err.Error()
			}
		} else if e, ok := msg.EventUpdateUser(); ok {
			extUser = e.GetUserID()
			handled = true
			if err := staff_event.SyncStaff(corpID, extUser); err != nil {
				errMsg = err.Error()
				break
			}
			if err := user.EnsureWecomLink(ctx, corpID, e.GetUserID()); err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeContact && msg.ChangeType == workwx.ChangeTypeDelUser:
		if e, ok := msg.EventDeleteUser(); ok {
			handled, extUser = true, e.GetUserID()
			if err := models.DB.Where("ext_corp_id = ? AND ext_id = ?", corpID, extUser).Delete(&models.Staff{}).Error; err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeContact && (msg.ChangeType == workwx.ChangeTypeCreateParty || msg.ChangeType == workwx.ChangeTypeUpdateParty):
		handled = true
		if err := services.NewDepartment().Sync(ctx, corpID); err != nil {
			errMsg = err.Error()
		}
	case msg.Event == workwx.EventTypeChangeContact && msg.ChangeType == workwx.ChangeTypeDeleteParty:
		handled = true
		if err := department_event.EventDeleteDepartment(msg); err != nil {
			errMsg = err.Error()
		}
	case msg.Event == workwx.EventTypeChangeExternalChat:
		if e, ok := msg.EventChangeExternalChat(); ok {
			handled = true
			var err error
			if msg.ChangeType == workwx.ChangeTypeDismissChat {
				err = models.DB.Model(&models.GroupChat{}).Where("ext_corp_id = ? AND ext_chat_id = ?", corpID, e.GetChatID()).Update("status", constants.GroupChatStatusIsDismissed).Error
			} else {
				err = services.NewGroupChatService().Syncs(ctx, e.GetChatID(), corpID)
			}
			if err != nil {
				errMsg = err.Error()
			}
		}
	case msg.Event == workwx.EventTypeChangeExternalTag:
		handled = true
		if msg.ChangeType == workwx.ChangeTypeDeleteTag {
			if e, ok := msg.EventDeleteTag(); ok {
				err := services.DeleteWecomTag(corpID, e.GetID(), e.GetTagType())
				if err != nil {
					errMsg = err.Error()
				}
			}
		} else if err := services.NewTag().Sync(corpID); err != nil {
			errMsg = err.Error()
		}
	}
	logger.Record(ctx, corpID, string(msg.Event), string(msg.ChangeType), extUser, extCustomer, handled, errMsg)
	if errMsg != "" {
		return errors.New(errMsg)
	}
	return nil
}
