package services

import "testing"

// TestCalcAchievement 锁定达成率计算。
func TestCalcAchievement(t *testing.T) {
	cases := []struct {
		name         string
		target, actual float64
		want         float64
	}{
		{"目标0→0(避免除零)", 0, 100, 0},
		{"目标负→0", -10, 100, 0},
		{"刚好达成→100", 100, 100, 100},
		{"超额达成→150", 100, 150, 150},
		{"未达成→50", 100, 50, 50},
		{"实际0→0", 100, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := calcAchievement(c.target, c.actual); got != c.want {
				t.Fatalf("calcAchievement(%v, %v) = %v, want %v", c.target, c.actual, got, c.want)
			}
		})
	}
}
