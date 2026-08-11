package services

import "testing"
import "time"

// TestCalcOverdueDays 锁定逾期天数计算。
func TestCalcOverdueDays(t *testing.T) {
	now := time.Date(2026, 8, 13, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		t    *time.Time
		want int
	}{
		{"nil→0", nil, 0},
		{"未来→0", ptrTime(time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)), 0},
		{"刚过→0天(不足24h)", ptrTime(time.Date(2026, 8, 13, 9, 0, 0, 0, time.UTC)), 0},
		{"1天前", ptrTime(time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)), 1},
		{"3天前", ptrTime(time.Date(2026, 8, 10, 10, 0, 0, 0, time.UTC)), 3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := calcOverdueDays(c.t, now); got != c.want {
				t.Fatalf("calcOverdueDays = %d, want %d", got, c.want)
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
