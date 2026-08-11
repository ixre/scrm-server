package services

import "testing"

// TestFunnelCounts 锁定活动漏斗累计语义：触达 ⊇ 参与 ⊇ 转化。
func TestFunnelCounts(t *testing.T) {
	cases := []struct {
		name                                     string
		converted, participated, reached          int
		wantReach, wantParticipate, wantConversion int
	}{
		{"空活动全零", 0, 0, 0, 0, 0, 0},
		// 只有触达：参与/转化为 0
		{"仅触达", 0, 0, 5, 5, 0, 0},
		// 参与客户纳入参与与触达，但不计入转化
		{"有参与无转化", 0, 3, 2, 5, 3, 0},
		// 转化客户计入三档（累计）
		{"有转化累计", 2, 3, 1, 6, 5, 2},
		{"全转化", 4, 0, 0, 4, 4, 4},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reach, participate, conversion := funnelCounts(c.converted, c.participated, c.reached)
			if reach != c.wantReach || participate != c.wantParticipate || conversion != c.wantConversion {
				t.Fatalf("funnelCounts(%d,%d,%d) = reach=%d participate=%d conversion=%d, want %d/%d/%d",
					c.converted, c.participated, c.reached,
					reach, participate, conversion,
					c.wantReach, c.wantParticipate, c.wantConversion)
			}
		})
	}
}

// TestNormalizeRecordType 锁定参与档位归一化：非法值默认 reached。
func TestNormalizeRecordType(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"reached", "reached"},
		{"participated", "participated"},
		{"converted", "converted"},
		{"", "reached"},
		{"bogus", "reached"},
	}
	for _, c := range cases {
		if got := normalizeRecordType(c.in); got != c.want {
			t.Fatalf("normalizeRecordType(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRecordRank 锁定档位序：reached < participated < converted。
func TestRecordRank(t *testing.T) {
	if !(recordRank("reached") < recordRank("participated") && recordRank("participated") < recordRank("converted")) {
		t.Fatal("recordRank 未满足 reached < participated < converted")
	}
}
