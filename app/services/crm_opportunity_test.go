package services

import "testing"
import "time"

// TestHasRecentFollowup 锁定阶段流转跟进校验逻辑。
func TestHasRecentFollowup(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name     string
		times    []time.Time
		days     int
		want     bool
	}{
		{"无跟进→false", nil, 3, false},
		{"days<=0→true(不校验)", nil, 0, true},
		{"3天内跟进→true", []time.Time{time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)}, 3, true},
		{"刚好3天前→true(不严格)", []time.Time{time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)}, 3, true},
		{"4天前→false", []time.Time{time.Date(2026, 8, 9, 10, 0, 0, 0, time.UTC)}, 3, false},
		{"多条含一条近期→true", []time.Time{
			time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		}, 3, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasRecentFollowup(c.times, c.days, now); got != c.want {
				t.Fatalf("hasRecentFollowup = %v, want %v", got, c.want)
			}
		})
	}
}
