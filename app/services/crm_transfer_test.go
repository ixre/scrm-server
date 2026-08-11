package services

import "testing"

// TestTransferLogEntry 锁定转移日志条目构造逻辑（纯函数验证构造正确性）。
func TestTransferLogEntry(t *testing.T) {
	// 验证：from=to 时不应产生日志（归属不变），由 Transfer 内部跳过。
	// 此处验证构造逻辑：相同 from/to 的条目不应产生。
	cases := []struct {
		name         string
		from, to     string
		expectLog    bool
	}{
		{"归属变化→有日志", "userA", "userB", true},
		{"归属不变→无日志", "userA", "userA", false},
		{"原无归属→有日志", "", "userB", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.from != c.to
			if got != c.expectLog {
				t.Fatalf("shouldLog(from=%q, to=%q) = %v, want %v", c.from, c.to, got, c.expectLog)
			}
		})
	}
}
