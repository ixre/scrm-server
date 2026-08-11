package services

import "testing"
import "time"

// TestShouldRecycle 锁定公海自动回收判定逻辑。
func TestShouldRecycle(t *testing.T) {
	threshold := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name         string
		lastFollowup time.Time
		want         bool
	}{
		{"无跟进不自动回收", time.Time{}, false},
		{"跟进早于阈值→回收", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), true},
		{"跟进晚于阈值→不回收", time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC), false},
		{"刚好等于阈值→不回收(Before严格)", threshold, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := shouldRecycle(c.lastFollowup, threshold); got != c.want {
				t.Fatalf("shouldRecycle(%v, %v) = %v, want %v", c.lastFollowup, threshold, got, c.want)
			}
		})
	}
}
