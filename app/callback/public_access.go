package callback

import (
	"os"
	"strconv"
	"strings"

	"core/common/publiclimit"
	cacheredis "core/common/redis"
	"github.com/gin-gonic/gin"
)

func publicTraffic(kind string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cacheredis.RedisClient == nil {
			c.AbortWithStatusJSON(503, gin.H{"code": 503, "msg": "公开入口暂不可用，请稍后重试"})
			return
		}
		wait, err := (publiclimit.Limiter{Client: cacheredis.RedisClient}).Allow(c.Request.Context(), kind, c.ClientIP(), publiclimit.Budgets(kind)...)
		if err != nil {
			c.AbortWithStatusJSON(503, gin.H{"code": 503, "msg": "公开入口暂不可用，请稍后重试"})
			return
		}
		if wait > 0 {
			seconds := publiclimit.RetrySeconds(wait)
			c.Header("Retry-After", strconv.Itoa(seconds))
			c.AbortWithStatusJSON(429, gin.H{"code": 429, "msg": "操作频繁，请稍后重试", "data": gin.H{"retry_after": seconds}})
			return
		}
		c.Next()
	}
}

func configurePublicProxies(r *gin.Engine) error {
	var proxies []string
	if raw := strings.TrimSpace(os.Getenv("TRUSTED_PROXY_CIDRS")); raw != "" {
		for _, part := range strings.Split(raw, ",") {
			proxies = append(proxies, strings.TrimSpace(part))
		}
	}
	return r.SetTrustedProxies(proxies)
}
