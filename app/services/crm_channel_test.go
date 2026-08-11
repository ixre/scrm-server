package services

import "testing"

// TestCalcChannelConversionRate 锁定渠道转化率计算。
func TestCalcChannelConversionRate(t *testing.T) {
	cases := []struct {
		name             string
		opportunity, won int64
		want             float64
	}{
		{"商机0→0", 0, 5, 0},
		{"全赢单→100", 10, 10, 100},
		{"半赢单→50", 10, 5, 50},
		{"无赢单→0", 10, 0, 0},
		{"超额(不可能但防除零)→200", 10, 20, 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := calcChannelConversionRate(c.opportunity, c.won); got != c.want {
				t.Fatalf("calcChannelConversionRate(%d, %d) = %v, want %v", c.opportunity, c.won, got, c.want)
			}
		})
	}
}
