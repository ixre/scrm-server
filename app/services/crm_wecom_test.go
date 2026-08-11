package services

import (
	"strings"
	"testing"

	"core/app/constants"
	"core/app/models"
	"core/common/util"
	"core/conf"
)

func TestMain(m *testing.M) {
	if err := conf.SetupTestSetting(); err != nil {
		panic("SetupTestSetting failed: " + err.Error())
	}
	m.Run()
}

// TestMaskSecret 锁定掩码语义：非空 → "******"，空串 → 空（编辑回显留空=不改）。
func TestMaskSecret(t *testing.T) {
	if got := MaskSecret("secret"); got != "******" {
		t.Fatalf("MaskSecret(nonempty) = %q, want ******", got)
	}
	if got := MaskSecret(""); got != "" {
		t.Fatalf("MaskSecret(empty) = %q, want empty", got)
	}
}

// TestValidateChannelConfig 锁定渠道配置基础校验。
func TestValidateChannelConfig(t *testing.T) {
	valid := func() *models.CrmChannelConfig {
		return &models.CrmChannelConfig{Channel: constants.ChannelWecom, CorpID: "ww123456789012345"}
	}
	cases := []struct {
		name string
		cfgc func() *models.CrmChannelConfig
		want string // 期望错误子串；空=应通过
	}{
		{"合法", valid, ""},
		{"corp_id为空", func() *models.CrmChannelConfig { c := valid(); c.CorpID = ""; return c }, "corp_id 不能为空"},
		{"corp_id超长", func() *models.CrmChannelConfig { c := valid(); c.CorpID = strings.Repeat("w", 19); return c }, "corp_id 长度不能超过 18"},
		{"小红书合法", func() *models.CrmChannelConfig {
			c := valid()
			c.Channel = "xiaohongshu"
			c.CorpID = "xhs_app_123"
			return c
		}, ""},
		{"公众号合法", func() *models.CrmChannelConfig {
			c := valid()
			c.Channel = "gzh"
			c.CorpID = "wx_gzh_app_123"
			return c
		}, ""},
		{"抖音合法", func() *models.CrmChannelConfig {
			c := valid()
			c.Channel = "douyin"
			c.CorpID = "dy_app_123"
			return c
		}, ""},
		{"不支持渠道", func() *models.CrmChannelConfig { c := valid(); c.Channel = "unknown"; return c }, "不支持的渠道"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateChannelConfig(c.cfgc())
			if c.want == "" {
				if err != nil {
					t.Fatalf("validateChannelConfig 应通过，got err=%v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("validateChannelConfig err=%v, want contain %q", err, c.want)
			}
		})
	}
}

// TestIsTodo 锁定 dev 占位判定：空串或含 "todo"。
func TestIsTodo(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"", true},
		{"todo", true},
		{"wxxx/todo/yyy", true},
		{"ww2d3e2957190c6e4c", false},
		{"token-abc", false},
	}
	for _, c := range cases {
		if got := isTodo(c.in); got != c.want {
			t.Fatalf("isTodo(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestChannelSecretsRoundTrip 锁定 secrets 加密落库→解密往返：encryptChannelSecrets 加密后
// 每项 AESDecode 能还原原文（验证 util.AESEncode 用的 app.key 在单测配置下可用）。
func TestChannelSecretsRoundTrip(t *testing.T) {
	req := ChannelConfigReq{
		ContactSecret:   "contact-secret-1",
		CustomerSecret:  "customer-secret-2",
		MainAgentSecret: "agent-secret-3",
		CallbackToken:   "cb-token-4",
		CallbackAesKey:  "cb-aes-key-5",
	}
	cfg := &models.CrmChannelConfig{}
	if err := encryptChannelSecrets(cfg, req); err != nil {
		t.Fatalf("encryptChannelSecrets: %v", err)
	}
	checks := map[string]struct{ enc, want string }{
		"contact_secret":    {cfg.ContactSecret, req.ContactSecret},
		"customer_secret":   {cfg.CustomerSecret, req.CustomerSecret},
		"main_agent_secret": {cfg.MainAgentSecret, req.MainAgentSecret},
		"callback_token":    {cfg.CallbackToken, req.CallbackToken},
		"callback_aes_key":  {cfg.CallbackAesKey, req.CallbackAesKey},
	}
	for name, c := range checks {
		if c.enc == "" {
			t.Fatalf("%s 未加密", name)
		}
		if c.enc == c.want {
			t.Fatalf("%s 未真正加密（与明文相同）", name)
		}
		got, err := util.AESDecode(c.enc)
		if err != nil {
			t.Fatalf("%s AESDecode: %v", name, err)
		}
		if got != c.want {
			t.Fatalf("%s 往返不一致: got %q want %q", name, got, c.want)
		}
	}
}

// TestBuildWecomEnrichUpdates 锁定补全字段的幂等/非空过滤逻辑：空字段不覆盖、无效值不采纳。
func TestBuildWecomEnrichUpdates(t *testing.T) {
	cases := []struct {
		name, want string // 期望字段（用逗号分隔断言）；空=期望空 map
	}{
		{"全空", ""},
		{"仅名", "name"},
		{"名+职位", "name,position"},
		{"仅职位", "position"},
		{"仅公司", "company"},
		{"全量", "name,gender,company,position"},
	}
	// 无效值：gender=0 不应进 map
	if m := buildWecomEnrichUpdates("", 0, "", ""); len(m) != 0 {
		t.Fatalf("全空应得空 map，got %v", m)
	}
	if _, ok := buildWecomEnrichUpdates("王总", 0, "", "")["gender"]; ok {
		t.Fatal("gender=0 不应被采纳")
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var m map[string]any
			switch c.name {
			case "全空":
				m = buildWecomEnrichUpdates("", 0, "", "")
			case "仅名":
				m = buildWecomEnrichUpdates("王总", 0, "", "")
			case "名+职位":
				m = buildWecomEnrichUpdates("王总", 0, "", "销售总监")
			case "仅职位":
				m = buildWecomEnrichUpdates("", 0, "", "销售总监")
			case "仅公司":
				m = buildWecomEnrichUpdates("", 0, "云帆软件", "")
			case "全量":
				m = buildWecomEnrichUpdates("王总", 1, "云帆软件", "销售总监")
			}
			want := map[string]string{}
			if c.want != "" {
				for _, k := range []string{"name", "gender", "company", "position"} {
					if strings.Contains(c.want, k) {
						want[k] = k
					}
				}
			}
			if len(m) != len(want) {
				t.Fatalf("字段数=%d, want %d (got %v)", len(m), len(want), m)
			}
			for k := range want {
				if _, ok := m[k]; !ok {
					t.Fatalf("缺字段 %s, got %v", k, m)
				}
			}
		})
	}
}
