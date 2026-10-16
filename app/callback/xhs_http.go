package callback

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"

	"core/app/services"
	"core/common/identity"
	"core/common/log"
	"core/conf"

	"github.com/gin-gonic/gin"
)

// StartXhsCallbackServer 启动小红书事件回调 webhook（独立 gin 端口，收小红书主动 POST 的事件）。
// URL: http://<host>:<XhsCallbackPort>/xhs/callback/:app_id
// 小红书 webhook 为明文 JSON（必须携带 X-Signature 并验证签名）。
// 同时承载 H5 落地页表单回收：POST /landing/:slug/submit
func StartXhsCallbackServer() {
	port := conf.Settings.Server.XhsCallbackPort
	if port == 0 {
		port = 9004
	}
	r := gin.Default()
	if err := configurePublicProxies(r); err != nil {
		panic("invalid TRUSTED_PROXY_CIDRS: " + err.Error())
	}
	r.Any("/xhs/callback/:app_id", xhsCallback)
	// H5 落地页表单回收（公开接口，不需登录）
	r.POST("/landing/:slug/submit", publicTraffic("lead"), landingPageSubmit)
	r.GET("/landing/:slug", publicTraffic("view"), landingPageMeta)
	// 公众号回调
	r.Any("/gzh/callback/:app_id", gzhCallback)
	// 抖音回调
	r.Any("/douyin/callback/:app_id", douyinCallback)
	// Phase 7 公开端点：营销活动 H5 交互 + 智能名片
	r.GET("/card/:id", publicTraffic("view"), smartCardView)
	r.POST("/h5/group_buy/open", h5GroupBuyOpen)
	r.POST("/h5/group_buy/join", h5GroupBuyJoin)
	r.POST("/h5/bargain/start", h5BargainStart)
	r.POST("/h5/bargain/help", h5BargainHelp)
	r.POST("/h5/red_packet/claim", h5RedPacketClaim)
	r.POST("/h5/sign_in/check", h5SignInCheck)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: r,
	}
	go func() {
		log.Sugar.Infow("xhs callback server listening", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Sugar.Errorw("xhs callback server exit", "err", err)
		}
	}()
}

// landingPageMeta 返回落地页元信息（供 H5 前端渲染表单）。
func landingPageMeta(c *gin.Context) {
	slug := c.Param("slug")
	page, err := services.NewCrmLandingPage().GetBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "落地页不存在"})
		return
	}
	// 返回 H5 表单 HTML 页面（移动端适配）
	html := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
<title>` + html.EscapeString(page.Title) + `</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; background: #f5f6f8; min-height: 100vh; }
.container { max-width: 480px; margin: 0 auto; padding: 20px; }
.hero { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: #fff; padding: 32px 20px; border-radius: 12px; text-align: center; margin-bottom: 20px; }
.hero h1 { font-size: 22px; margin-bottom: 8px; }
.hero p { font-size: 14px; opacity: 0.9; }
.form-card { background: #fff; border-radius: 12px; padding: 24px 20px; box-shadow: 0 2px 8px rgba(0,0,0,0.06); }
.form-group { margin-bottom: 16px; }
.form-group label { display: block; font-size: 14px; color: #333; margin-bottom: 6px; font-weight: 500; }
.form-group input { width: 100%; height: 44px; border: 1px solid #e5e6eb; border-radius: 8px; padding: 0 12px; font-size: 15px; outline: none; }
.form-group input:focus { border-color: #165dff; }
.btn { width: 100%; height: 48px; background: #165dff; color: #fff; border: none; border-radius: 8px; font-size: 16px; font-weight: 500; cursor: pointer; margin-top: 8px; }
.btn:active { background: #0e42d2; }
.btn:disabled { background: #c9cdd4; }
.success { display: none; text-align: center; padding: 40px 20px; }
.success .icon { font-size: 48px; color: #00b42a; margin-bottom: 12px; }
.success h2 { font-size: 18px; color: #333; margin-bottom: 8px; }
.success p { font-size: 14px; color: #888; }
.error-msg { color: #f53f3f; font-size: 13px; margin-top: 8px; display: none; }
</style>
</head>
<body>
<div class="container">
  <div class="hero">
    <h1>` + html.EscapeString(page.Title) + `</h1>
    <p>` + html.EscapeString(page.Template) + ` 模板</p>
  </div>
  <div class="form-card" id="formCard">
    <div class="form-group">
      <label>姓名</label>
      <input type="text" id="name" placeholder="请输入您的姓名" />
    </div>
    <div class="form-group">
      <label>手机号</label>
      <input type="tel" id="mobile" placeholder="请输入手机号" maxlength="11" />
    </div>
    <div class="form-group">
      <label>公司（选填）</label>
      <input type="text" id="company" placeholder="请输入公司名称" />
    </div>
    <div class="error-msg" id="errMsg"></div>
    <button class="btn" id="submitBtn" onclick="submitForm()">立即提交</button>
  </div>
  <div class="success" id="successPage">
    <div class="icon">✓</div>
    <h2>提交成功</h2>
    <p>我们将尽快与您联系，请保持手机畅通</p>
  </div>
</div>
<script>
function submitForm() {
  var name = document.getElementById('name').value.trim();
  var mobile = document.getElementById('mobile').value.trim();
  var company = document.getElementById('company').value.trim();
  var errMsg = document.getElementById('errMsg');
  var btn = document.getElementById('submitBtn');
  if (btn.disabled) return;
  if (!name) { errMsg.textContent = '请输入姓名'; errMsg.style.display = 'block'; return; }
  if (!mobile || mobile.length < 11) { errMsg.textContent = '请输入正确的手机号'; errMsg.style.display = 'block'; return; }
  errMsg.style.display = 'none';
  btn.disabled = true; btn.textContent = '提交中...';
  fetch('/landing/` + url.PathEscape(slug) + `/submit', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name: name, mobile: mobile, company: company })
  }).then(function(r) { return r.json(); }).then(function(res) {
    if (res.code === 0) {
      document.getElementById('formCard').style.display = 'none';
      document.getElementById('successPage').style.display = 'block';
    } else {
      errMsg.textContent = res.msg || '提交失败，请重试'; errMsg.style.display = 'block';
      if (res.code === 429) {
        var remaining = Math.min(3600, Math.max(1, Number(res.data && res.data.retry_after) || 60));
        btn.textContent = remaining + ' 秒后重试';
        var retryTimer = setInterval(function() {
          remaining--;
          if (remaining <= 0) { clearInterval(retryTimer); btn.disabled = false; btn.textContent = '立即提交'; }
          else btn.textContent = remaining + ' 秒后重试';
        }, 1000);
      } else { btn.disabled = false; btn.textContent = '立即提交'; }
    }
  }).catch(function() {
    errMsg.textContent = '网络错误，请重试'; errMsg.style.display = 'block';
    btn.disabled = false; btn.textContent = '立即提交';
  });
}
</script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// landingPageSubmit 接收 H5 落地页表单提交。
func landingPageSubmit(c *gin.Context) {
	slug := c.Param("slug")
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	var formData map[string]string
	if err := c.ShouldBindJSON(&formData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}
	_, err := services.NewCrmLandingPage().SubmitForm(slug, formData)
	if err != nil {
		log.Sugar.Errorw("landing page submit failed", "slug", slug, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"ok": true}})
}

// xhsCallback 处理小红书 webhook 事件。
// 事件类型由 body.event_type 区分：lead（留资）/comment（评论）/dm（私信）。
// 签名校验：请求必须带 X-Signature header，用渠道 CallbackToken 做 HMAC-SHA256 校验。
func xhsCallback(c *gin.Context) {
	if c.GetHeader("X-Signature") == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "msg": "缺少回调签名"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	appID := c.Param("app_id")

	// 读 raw body（签名校验需要原始字节）
	rawBody, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "read body failed"})
		return
	}

	// 查租户的小红书渠道配置（按 app_id）
	tenantID, err := services.NewCrmChannelConfig().GetTenantByChannelApp(appID)
	if err != nil {
		log.Sugar.Warnw("xhs callback: app_id not configured", "app_id", appID, "err", err)
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "app_id 未配置"})
		return
	}

	// 签名校验（X-Signature 必填，未通过校验拒绝写入）
	if sig := c.GetHeader("X-Signature"); sig != "" {
		if !services.NewCrmChannelConfig().VerifyXhsSignature(appID, rawBody, sig) {
			log.Sugar.Warnw("xhs callback: signature mismatch", "app_id", appID)
			c.JSON(http.StatusUnauthorized, gin.H{"code": 1, "msg": "签名校验失败"})
			return
		}
	}

	var body struct {
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(rawBody, &body); err != nil {
		log.Sugar.Errorw("xhs callback: unmarshal failed", "err", err)
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}

	customer := services.NewCrmCustomer()
	switch body.EventType {
	case "lead":
		var ev services.XhsLeadEvent
		if err := json.Unmarshal(body.Data, &ev); err != nil {
			log.Sugar.Errorw("xhs callback: lead unmarshal failed", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid lead data"})
			return
		}
		ev.AppID = appID
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
		if err := customer.SyncFromXhs(ctx, appID, ev); err != nil {
			log.Sugar.Errorw("xhs callback: lead sync failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
			return
		}
	case "comment", "dm":
		var ev services.XhsInteractionEvent
		if err := json.Unmarshal(body.Data, &ev); err != nil {
			log.Sugar.Errorw("xhs callback: interaction unmarshal failed", "err", err)
			c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid interaction data"})
			return
		}
		ev.AppID = appID
		ev.ActionType = body.EventType
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
		if err := customer.SyncInteractionFromXhs(ctx, appID, ev); err != nil {
			log.Sugar.Errorw("xhs callback: interaction sync failed", "err", err)
			c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
			return
		}
	default:
		log.Sugar.Warnw("xhs callback: unknown event_type", "event_type", body.EventType)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0})
}
