package callback

import (
	cacheredis "core/common/redis"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
)

func TestPublicTrafficUnavailableDoesNotSubmit(t *testing.T) {
	old := cacheredis.RedisClient
	cacheredis.RedisClient = nil
	defer func() { cacheredis.RedisClient = old }()
	t.Setenv("TRUSTED_PROXY_CIDRS", "")
	r := gin.New()
	if err := configurePublicProxies(r); err != nil {
		t.Fatal(err)
	}
	called := false
	r.POST("/lead", publicTraffic("lead"), func(c *gin.Context) { called = true; c.Status(200) })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/lead", nil))
	if w.Code != 503 || called {
		t.Fatal("unlimited submission while Redis unavailable", w.Code, called)
	}
	t.Setenv("TRUSTED_PROXY_CIDRS", "bad-proxy")
	if err := configurePublicProxies(gin.New()); err == nil {
		t.Fatal("invalid trusted proxy accepted")
	}
}
