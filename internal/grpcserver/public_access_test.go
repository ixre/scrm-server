package grpcserver

import (
	"context"
	"core/common/publiclimit"
	cacheredis "core/common/redis"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/metadata"
	"os"
	"testing"
)

func TestPublicTrafficFailsClosed(t *testing.T) {
	old := cacheredis.RedisClient
	cacheredis.RedisClient = nil
	t.Cleanup(func() { cacheredis.RedisClient = old })
	if got := checkPublicTraffic(context.Background(), "PublicSubmitLead", nil); got == nil || got.Code != 503 {
		t.Fatal("missing Redis accepted", got)
	}
	if got := checkPublicTraffic(context.Background(), "PublicSubmitLead", make([]byte, (16<<10)+1)); got == nil || got.Code != 413 {
		t.Fatal("oversize body accepted", got)
	}
}

func TestPublicTrafficSharesBudgetAcrossEntryPoints(t *testing.T) {
	addr := os.Getenv("SCRM_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("requires isolated Redis")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	defer client.Close()
	old := cacheredis.RedisClient
	cacheredis.RedisClient = client
	defer func() { cacheredis.RedisClient = old }()
	if got := checkPublicTraffic(context.Background(), "PublicSubmitLead", nil); got == nil || got.Code != 503 {
		t.Fatal("missing trusted IP accepted", got)
	}
	// The direct landing-page path and the gRPC smart-card path use the same IP/action budget.
	ip := "2001:db8:abcd:9876::15"
	sum := sha256.Sum256([]byte("lead:" + ip))
	keys := []string{fmt.Sprintf("scrm:public-rate:{%x}:0", sum), fmt.Sprintf("scrm:public-rate:{%x}:1", sum)}
	if err := client.Del(context.Background(), keys...).Err(); err != nil {
		t.Fatal(err)
	}
	defer client.Del(context.Background(), keys...)
	limiter := publiclimit.Limiter{Client: client}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-scrm-client-ip", ip))
	for i := 0; i < 20; i++ {
		if _, err := limiter.Allow(ctx, "lead", ip, publiclimit.Budgets("lead")...); err != nil {
			t.Fatal(err)
		}
	}
	got := checkPublicTraffic(ctx, "PublicSubmitLead", []byte(`{}`))
	if got == nil || got.Code != 429 {
		t.Fatal("separate entry point bypassed shared limit", got)
	}
	var data struct {
		RetryAfter int `json:"retry_after"`
	}
	if err := json.Unmarshal(got.Data, &data); err != nil || data.RetryAfter <= 0 {
		t.Fatal("retry time missing", data, err)
	}
}
