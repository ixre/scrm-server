package services

import (
	"context"
	"core/app/models"
	"core/common/util"
	"core/conf"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestProviderResponseValidation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("suite_access_token") != "private-token" {
			t.Error("suite token missing")
		}
		switch r.URL.Path {
		case "/cgi-bin/service/good":
			w.Write([]byte(`{"errcode":0,"access_token":"corp-a-token"}`))
		case "/cgi-bin/service/bad":
			w.Write([]byte(`{"errcode":40014,"errmsg":"private-token"}`))
		default:
			w.WriteHeader(503)
		}
	}))
	defer server.Close()
	old := providerAPIBase
	providerAPIBase = server.URL
	t.Cleanup(func() { providerAPIBase = old })
	var result struct {
		Token string `json:"access_token"`
	}
	if err := providerRequest(context.Background(), "POST", "good", "private-token", map[string]string{"auth_corpid": "corp-a"}, &result); err != nil || result.Token != "corp-a-token" {
		t.Fatal("valid provider response rejected", err)
	}
	if err := providerRequest(context.Background(), "POST", "bad", "private-token", nil, &result); err == nil || strings.Contains(err.Error(), "private-token") {
		t.Fatal("provider error accepted or exposed secret", err)
	}
}

func TestWecomProviderLifecycleIsolation(t *testing.T) {
	db := regressionDB(t)
	requireNoError(t, db.AutoMigrate(&models.CrmChannelConfig{}, &models.WecomSuiteTicket{}))
	old := conf.Settings.WecomProvider
	conf.Settings.WecomProvider.SuiteID = "suite-a"
	t.Cleanup(func() { conf.Settings.WecomProvider = old })
	for i, corp := range []string{"corp-a", "corp-b"} {
		mustCreate(t, db, &models.CrmChannelConfig{Model: models.Model{ID: []string{"800", "801"}[i]}, TenantID: []string{"1", "2"}[i], Channel: "wecom", CorpID: corp, AuthMode: "provider", SuiteID: "suite-a", AuthStatus: "authorized", PermanentCode: "encrypted", AuthorizedAt: 10, Enabled: 1})
	}
	requireNoError(t, HandleProviderLifecycle(ProviderLifecycleEvent{SuiteID: "suite-a", InfoType: "suite_ticket", Ticket: "new-ticket", Timestamp: 20}))
	requireNoError(t, HandleProviderLifecycle(ProviderLifecycleEvent{SuiteID: "suite-a", InfoType: "suite_ticket", Ticket: "stale-ticket", Timestamp: 19}))
	var ticket models.WecomSuiteTicket
	requireNoError(t, db.Take(&ticket).Error)
	decoded, err := util.AESDecode(ticket.Ticket)
	requireNoError(t, err)
	if decoded != "new-ticket" {
		t.Fatal("stale suite ticket replaced current ticket")
	}
	if HandleProviderLifecycle(ProviderLifecycleEvent{SuiteID: "other-suite", InfoType: "cancel_auth", CorpID: "corp-b", Timestamp: 20}) == nil {
		t.Fatal("foreign suite accepted")
	}
	requireNoError(t, HandleProviderLifecycle(ProviderLifecycleEvent{SuiteID: "suite-a", InfoType: "cancel_auth", CorpID: "corp-a", Timestamp: 20}))
	requireNoError(t, HandleProviderLifecycle(ProviderLifecycleEvent{SuiteID: "suite-a", InfoType: "change_auth", CorpID: "corp-a", Timestamp: 19}))
	var a, b models.CrmChannelConfig
	requireNoError(t, db.Where("corp_id='corp-a'").Take(&a).Error)
	requireNoError(t, db.Where("corp_id='corp-b'").Take(&b).Error)
	if a.Enabled != 2 || a.AuthStatus != "revoked" || a.PermanentCode != "" || b.Enabled != 1 || b.AuthStatus != "authorized" {
		t.Fatal("authorization revocation crossed enterprises or was undone by stale event")
	}
}

func TestWecomProviderAuthorizationBindsTenantState(t *testing.T) {
	db := regressionDB(t)
	requireNoError(t, db.AutoMigrate(&models.CrmChannelConfig{}, &models.WecomSuiteTicket{}, &models.WecomAuthState{}, &models.ContactWayGroup{}, &models.CustomerInfoDisplayRule{}))
	old := conf.Settings.WecomProvider
	conf.Settings.WecomProvider = conf.WecomProviderConfig{SuiteID: "suite-a", SuiteSecret: "private-suite-secret", RedirectURL: "https://example.com/console/crm/channel-config"}
	t.Cleanup(func() { conf.Settings.WecomProvider = old })
	requireNoError(t, HandleProviderLifecycle(ProviderLifecycleEvent{SuiteID: "suite-a", InfoType: "suite_ticket", Ticket: "ticket", Timestamp: 20}))
	exchanges := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cgi-bin/service/get_suite_token":
			w.Write([]byte(`{"suite_access_token":"suite-token","expires_in":7200}`))
		case "/cgi-bin/service/get_pre_auth_code":
			w.Write([]byte(`{"pre_auth_code":"pre-auth","expires_in":600}`))
		case "/cgi-bin/service/get_permanent_code":
			exchanges++
			w.Write([]byte(`{"permanent_code":"permanent-a","auth_corp_info":{"corpid":"corp-a","corp_name":"A"},"auth_info":{"agent":[{"agentid":55}]}}`))
		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	base := providerAPIBase
	providerAPIBase = server.URL
	t.Cleanup(func() { providerAPIBase = base })
	ctx := testIdentity("1", "12")
	link, err := WecomAuthorizeURL(ctx)
	requireNoError(t, err)
	u, err := url.Parse(link["url"])
	requireNoError(t, err)
	state := u.Query().Get("state")
	if len(state) != 64 {
		t.Fatal("authorization state missing")
	}
	cfg, err := CompleteWecomAuthorization(ctx, state, "auth-code")
	requireNoError(t, err)
	if cfg.CorpID != "corp-a" || cfg.TenantID != "1" || cfg.AuthMode != "provider" || cfg.MainAgentID != 55 {
		t.Fatal("authorization bound wrong enterprise or tenant")
	}
	_, err = CompleteWecomAuthorization(ctx, state, "auth-code")
	requireNoError(t, err)
	if exchanges != 1 {
		t.Fatal("consumed authorization code exchanged again")
	}
	var saved models.CrmChannelConfig
	requireNoError(t, db.Where("corp_id='corp-a'").Take(&saved).Error)
	permanent, err := util.AESDecode(saved.PermanentCode)
	requireNoError(t, err)
	if permanent != "permanent-a" || saved.PermanentCode == permanent {
		t.Fatal("permanent authorization code not encrypted")
	}
	data, err := json.Marshal(cfg)
	requireNoError(t, err)
	if strings.Contains(string(data), "permanent-a") || strings.Contains(string(data), saved.PermanentCode) {
		t.Fatal("authorization secret leaked in config response")
	}
}
