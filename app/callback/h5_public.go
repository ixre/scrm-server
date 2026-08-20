package callback

import (
	"context"
	"net/http"

	"core/app/models"
	"core/app/services"
	"core/common/identity"

	"github.com/gin-gonic/gin"
)

// resolveTenantIDByAppID 通过渠道配置的 app_id/corp_id 推导租户ID（安全：不信任客户端传入的 tenant_id）。
func resolveTenantIDByAppID(appID string) string {
	if appID == "" {
		return ""
	}
	var cfg models.CrmChannelConfig
	if err := models.DB.Where("corp_id = ? AND enabled = ?", appID, 1).First(&cfg).Error; err != nil {
		return ""
	}
	return cfg.TenantID
}

// smartCardView 公开访问智能名片（非登录态，支持ID或分享码访问）。
func smartCardView(c *gin.Context) {
	idOrCode := c.Param("id")
	if idOrCode == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "缺少名片标识"})
		return
	}
	var card models.CrmSmartCard
	var err error
	// 优先按分享码查（8位短码），再按ID查
	if len(idOrCode) <= 8 {
		card, err = services.NewCrmSmartCard().GetByShareCode(idOrCode)
	} else {
		card, err = services.NewCrmSmartCard().GetByID(idOrCode)
	}
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 1, "msg": "名片不存在"})
		return
	}
	// 记录浏览
	go func() {
		defer func() { recover() }()
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: card.TenantID})
		services.NewCrmSmartCard().RecordView(ctx, card.ID, "", c.ClientIP(), c.GetHeader("User-Agent"), "share", 0)
	}()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": card})
}

// h5GroupBuyOpen H5 开团。
func h5GroupBuyOpen(c *gin.Context) {
	var req struct {
		AppID      string `json:"app_id"`
		GroupBuyID string `json:"group_buy_id"`
		LeaderID   string `json:"leader_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}
	tenantID := resolveTenantIDByAppID(req.AppID)
	if tenantID == "" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "invalid app_id"})
		return
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	rec, err := services.NewCrmGroupBuy().OpenGroup(ctx, req.GroupBuyID, req.LeaderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rec})
}

// h5GroupBuyJoin H5 参团。
func h5GroupBuyJoin(c *gin.Context) {
	var req struct {
		AppID    string `json:"app_id"`
		RecordID string `json:"record_id"`
		MemberID string `json:"member_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}
	tenantID := resolveTenantIDByAppID(req.AppID)
	if tenantID == "" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "invalid app_id"})
		return
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	rec, err := services.NewCrmGroupBuy().JoinGroup(ctx, req.RecordID, req.MemberID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rec})
}

// h5BargainStart H5 发起砍价。
func h5BargainStart(c *gin.Context) {
	var req struct {
		AppID      string `json:"app_id"`
		BargainID  string `json:"bargain_id"`
		CustomerID string `json:"customer_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}
	tenantID := resolveTenantIDByAppID(req.AppID)
	if tenantID == "" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "invalid app_id"})
		return
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	rec, err := services.NewCrmBargain().Start(ctx, req.BargainID, req.CustomerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rec})
}

// h5BargainHelp H5 帮砍。
func h5BargainHelp(c *gin.Context) {
	var req struct {
		AppID    string `json:"app_id"`
		RecordID string `json:"record_id"`
		HelperID string `json:"helper_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}
	tenantID := resolveTenantIDByAppID(req.AppID)
	if tenantID == "" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "invalid app_id"})
		return
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	rec, err := services.NewCrmBargain().Help(ctx, req.RecordID, req.HelperID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rec})
}

// h5RedPacketClaim H5 领红包。
func h5RedPacketClaim(c *gin.Context) {
	var req struct {
		AppID       string `json:"app_id"`
		RedPacketID string `json:"red_packet_id"`
		CustomerID  string `json:"customer_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}
	tenantID := resolveTenantIDByAppID(req.AppID)
	if tenantID == "" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "invalid app_id"})
		return
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	rec, err := services.NewCrmRedPacket().Claim(ctx, req.RedPacketID, req.CustomerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rec})
}

// h5SignInCheck H5 签到。
func h5SignInCheck(c *gin.Context) {
	var req struct {
		AppID      string `json:"app_id"`
		SignInID   string `json:"sign_in_id"`
		CustomerID string `json:"customer_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 1, "msg": "invalid body"})
		return
	}
	tenantID := resolveTenantIDByAppID(req.AppID)
	if tenantID == "" {
		c.JSON(http.StatusForbidden, gin.H{"code": 1, "msg": "invalid app_id"})
		return
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenantID})
	rec, err := services.NewCrmSignIn().CheckIn(ctx, req.SignInID, req.CustomerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 1, "msg": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": rec})
}
