package publiclimit

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestDistributedBudgets(t *testing.T) {
	addr := os.Getenv("SCRM_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("set SCRM_TEST_REDIS_ADDR to an isolated test Redis")
	}
	ctx := context.Background()
	a, b := redis.NewClient(&redis.Options{Addr: addr}), redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { a.Close(); b.Close() })
	if err := a.Ping(ctx).Err(); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("scrm:test:public-rate:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		iter := a.Scan(ctx, 0, prefix+":*", 100).Iterator()
		for iter.Next(ctx) {
			a.Del(ctx, iter.Val())
		}
	})
	limiters := []Limiter{{Client: a, Prefix: prefix}, {Client: b, Prefix: prefix}}
	var allowed atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			wait, err := limiters[i%2].Allow(ctx, "lead", "192.0.2.1", Budget{20, time.Minute}, Budget{200, time.Hour})
			if err != nil {
				t.Error(err)
				return
			}
			if wait == 0 {
				allowed.Add(1)
			} else if wait <= 0 || wait > time.Minute {
				t.Error("invalid retry duration", wait)
			}
		}(i)
	}
	wg.Wait()
	if allowed.Load() != 20 {
		t.Fatal("replicas did not share budget", allowed.Load())
	}
	if wait, err := limiters[0].Allow(ctx, "lead", "192.0.2.2", Budget{20, time.Minute}, Budget{200, time.Hour}); err != nil || wait != 0 {
		t.Fatal("independent IP blocked", wait, err)
	}
	for i := 0; i < 2; i++ {
		wait, err := limiters[0].Allow(ctx, "expiry", "192.0.2.1", Budget{1, 100 * time.Millisecond})
		if err != nil || (i == 0 && wait != 0) || (i == 1 && wait <= 0) {
			t.Fatal("expiry budget", wait, err)
		}
	}
	time.Sleep(150 * time.Millisecond)
	if wait, err := limiters[1].Allow(ctx, "expiry", "192.0.2.1", Budget{1, 100 * time.Millisecond}); err != nil || wait != 0 {
		t.Fatal("budget did not expire", wait, err)
	}
	if _, err := limiters[0].Allow(ctx, "lead", "spoofed-ip", Budget{1, time.Minute}); err == nil {
		t.Fatal("invalid client IP accepted")
	}
	if _, err := (Limiter{}).Allow(ctx, "lead", "192.0.2.1", Budget{1, time.Minute}); err == nil {
		t.Fatal("missing Redis allowed traffic")
	}
	// A failed Redis connection must produce an error rather than permit unlimited writes.
	b.Close()
	if _, err := limiters[1].Allow(ctx, "lead", "192.0.2.1", Budget{1, time.Minute}); err == nil {
		t.Fatal("closed Redis allowed traffic")
	}
}

func TestRetrySeconds(t *testing.T) {
	if RetrySeconds(time.Millisecond) != 1 || RetrySeconds(1500*time.Millisecond) != 2 {
		t.Fatal("retry duration truncated")
	}
}
