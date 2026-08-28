// Package publiclimit provides shared Redis budgets for anonymous acquisition traffic.
package publiclimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"
)

type Budget struct {
	Limit  int64
	Window time.Duration
}
type Limiter struct {
	Client redis.UniversalClient
	Prefix string
}

// Counters and expirations are updated atomically across every core replica.
// All keys share a Redis Cluster hash tag. No raw client IP is stored.
var budgetScript = redis.NewScript(`
local retry = 0
for i, key in ipairs(KEYS) do
 local count = redis.call('INCR', key)
 local ttl = redis.call('PTTL', key)
 if count == 1 or ttl < 0 then
  redis.call('PEXPIRE', key, ARGV[2*i])
  ttl = tonumber(ARGV[2*i])
 end
 if count > tonumber(ARGV[2*i-1]) and ttl > retry then retry = ttl end
end
return retry
`)

func (l Limiter) Allow(ctx context.Context, scope, clientIP string, budgets ...Budget) (time.Duration, error) {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		return 0, errors.New("missing trusted client IP")
	}
	return l.AllowKey(ctx, scope, ip.String(), budgets...)
}

// AllowKey also supports per-account budgets shared across source IPs.
func (l Limiter) AllowKey(ctx context.Context, scope, key string, budgets ...Budget) (time.Duration, error) {
	if key == "" {
		return 0, errors.New("missing limiter key")
	}
	if l.Client == nil {
		return 0, errors.New("rate limiter unavailable")
	}
	if len(budgets) == 0 {
		return 0, errors.New("rate budgets missing")
	}
	sum := sha256.Sum256([]byte(scope + ":" + key))
	tag := hex.EncodeToString(sum[:])
	prefix := l.Prefix
	if prefix == "" {
		prefix = "scrm:public-rate"
	}
	keys := make([]string, 0, len(budgets))
	args := make([]any, 0, len(budgets)*2)
	for i, b := range budgets {
		if b.Limit <= 0 || b.Window < time.Millisecond {
			return 0, errors.New("invalid rate budget")
		}
		keys = append(keys, fmt.Sprintf("%s:{%s}:%d", prefix, tag, i))
		args = append(args, b.Limit, b.Window.Milliseconds())
	}
	deadline, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
	defer cancel()
	result, err := budgetScript.Run(deadline, l.Client, keys, args...).Int64()
	if err != nil {
		return 0, err
	}
	return time.Duration(result) * time.Millisecond, nil
}

func Budgets(action string) []Budget {
	switch action {
	case "lead":
		return []Budget{{20, time.Minute}, {200, time.Hour}}
	case "like":
		return []Budget{{20, time.Minute}, {100, time.Hour}}
	default:
		return []Budget{{120, time.Minute}}
	}
}
func RetrySeconds(wait time.Duration) int {
	n := int((wait + time.Second - 1) / time.Second)
	if n < 1 {
		return 1
	}
	return n
}
