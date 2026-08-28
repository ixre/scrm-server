package callback

import (
	"core/app/services"
	"core/conf"
	workwx "core/pkg/easywework"
	"encoding/xml"
	"github.com/gin-gonic/gin"
	"net/http"
)

func providerCallbackXML(c *gin.Context) ([]byte, bool) {
	p := conf.Settings.WecomProvider
	if p.SuiteID == "" || p.CallbackToken == "" || p.CallbackAESKey == "" {
		c.Status(http.StatusNotFound)
		return nil, false
	}
	receiver := p.ReceiveID
	if receiver == "" {
		receiver = p.SuiteID
	}
	cb, err := workwx.NewCBHandlerForReceiver(p.CallbackToken, p.CallbackAESKey, receiver)
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return nil, false
	}
	if c.Request.Method == http.MethodGet {
		cb.EchoTestHandler(c.Writer, c.Request)
		return nil, false
	}
	if c.Request.Method != http.MethodPost {
		c.Status(http.StatusMethodNotAllowed)
		return nil, false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 2<<20)
	data, err := cb.GetCallBackXML(c.Request)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return nil, false
	}
	return data, true
}

func wecomProviderCallback(c *gin.Context) {
	data, ok := providerCallbackXML(c)
	if !ok {
		return
	}
	var event services.ProviderLifecycleEvent
	if xml.Unmarshal(data, &event) != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	if err := services.HandleProviderLifecycle(event); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.String(http.StatusOK, "success")
}

func wecomProviderData(c *gin.Context) {
	data, ok := providerCallbackXML(c)
	if !ok {
		return
	}
	msg, err := workwx.ParseCallbackMessage(data)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	cred, err := services.NewCrmChannelConfig().GetByCorpID(msg.ToUserID)
	if err != nil || cred.AuthMode != "provider" {
		c.Status(http.StatusForbidden)
		return
	}
	if err := handleVerifiedWecomEvent(cred.TenantID, msg, data); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.String(http.StatusOK, "success")
}
