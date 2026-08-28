package grpcserver

import (
	"context"
	cacheredis "core/common/redis"
	"fmt"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/metadata"
	"os"
	"testing"
	"time"
)

func TestLoginRateLimitsAccountAcrossSourceIPs(t *testing.T) {
	addr := os.Getenv("SCRM_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires isolated Redis")
	}
	previous := cacheredis.RedisClient
	client := redis.NewClient(&redis.Options{Addr: addr})
	cacheredis.RedisClient = client
	t.Cleanup(func() { cacheredis.RedisClient = previous; client.Close() })
	input := []byte(fmt.Sprintf(`{"tenant_code":"test","username":"test-%d"}`, time.Now().UnixNano()))
	seed := time.Now().UnixNano()
	for i := 0; i < 10; i++ {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(mdClientIP, fmt.Sprintf("192.0.%d.%d", byte(seed>>8), byte(seed+int64(i)))))
		if result := checkLoginTraffic(ctx, "crm_auth", input); result != nil {
			t.Fatal("valid budget rejected", result)
		}
	}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(mdClientIP, "198.51.100.2"))
	if result := checkLoginTraffic(ctx, "crm_auth", input); result == nil || result.Code != 429 {
		t.Fatal("rotating IP bypassed account rate limit", result)
	}
}

func TestLoginLimiterFailsClosed(t *testing.T) {
	previous := cacheredis.RedisClient
	cacheredis.RedisClient = nil
	t.Cleanup(func() { cacheredis.RedisClient = previous })
	if result := checkLoginTraffic(context.Background(), "crm_auth", []byte(`{}`)); result == nil || result.Code != 503 {
		t.Fatal("missing limiter allowed login")
	}
}
