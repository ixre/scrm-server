package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"core/common/util"
	"core/common/we_work"
	"core/conf"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var providerAPIBase = "https://qyapi.weixin.qq.com"
var providerHTTP = &http.Client{Timeout: 8 * time.Second}
var suiteTokenCache struct {
	sync.Mutex
	SuiteID, Ticket, Token string
	ExpiresAt              time.Time
}

func providerRequest(ctx context.Context, method, endpoint, token string, input, output any) error {
	var body []byte
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	u, err := url.Parse(providerAPIBase + "/cgi-bin/service/" + endpoint)
	if err != nil {
		return err
	}
	if token != "" {
		q := u.Query()
		q.Set("suite_access_token", token)
		u.RawQuery = q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := providerHTTP.Do(req)
	if err != nil {
		return errors.New("服务商接口请求失败")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("服务商接口 HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	var status struct {
		ErrCode int `json:"errcode"`
	}
	if err := json.Unmarshal(data, &status); err != nil {
		return errors.New("服务商接口响应无效")
	}
	if status.ErrCode != 0 {
		return fmt.Errorf("服务商接口错误码 %d", status.ErrCode)
	}
	return json.Unmarshal(data, output)
}

func providerSuiteToken(ctx context.Context) (string, error) {
	p := conf.Settings.WecomProvider
	if p.SuiteID == "" || p.SuiteSecret == "" {
		return "", errors.New("服务商应用未配置")
	}
	var ticket models.WecomSuiteTicket
	if err := models.DB.Where("suite_id=?", p.SuiteID).Take(&ticket).Error; err != nil {
		return "", errors.New("尚未收到服务商 suite_ticket")
	}
	suiteTokenCache.Lock()
	defer suiteTokenCache.Unlock()
	if suiteTokenCache.SuiteID == p.SuiteID && suiteTokenCache.Ticket == ticket.Ticket && time.Now().Before(suiteTokenCache.ExpiresAt) {
		return suiteTokenCache.Token, nil
	}
	value, err := decodeWecomSecret(ticket.Ticket)
	if err != nil {
		return "", err
	}
	var result struct {
		Token     string `json:"suite_access_token"`
		ExpiresIn int64  `json:"expires_in"`
	}
	if err := providerRequest(ctx, http.MethodPost, "get_suite_token", "", map[string]string{"suite_id": p.SuiteID, "suite_secret": p.SuiteSecret, "suite_ticket": value}, &result); err != nil {
		return "", err
	}
	if result.Token == "" || result.ExpiresIn <= 60 {
		return "", errors.New("服务商 token 响应无效")
	}
	suiteTokenCache.SuiteID, suiteTokenCache.Ticket, suiteTokenCache.Token = p.SuiteID, ticket.Ticket, result.Token
	suiteTokenCache.ExpiresAt = time.Now().Add(time.Duration(result.ExpiresIn-60) * time.Second)
	return result.Token, nil
}

func providerCorpToken(corpID string) (string, int64, error) {
	var cfg models.CrmChannelConfig
	if err := models.CrossTenant().Where("channel='wecom' AND corp_id=? AND enabled=1 AND auth_mode='provider' AND auth_status IN ('authorized','refresh_pending')", corpID).Take(&cfg).Error; err != nil {
		return "", 0, errors.New("服务商企业授权已失效")
	}
	if cfg.SuiteID != conf.Settings.WecomProvider.SuiteID {
		return "", 0, errors.New("服务商应用不匹配")
	}
	permanent, err := decodeWecomSecret(cfg.PermanentCode)
	if err != nil || permanent == "" {
		return "", 0, errors.New("企业永久授权码无效")
	}
	token, err := providerSuiteToken(context.Background())
	if err != nil {
		return "", 0, err
	}
	var result struct {
		Token     string `json:"access_token"`
		ExpiresIn int64  `json:"expires_in"`
	}
	if err := providerRequest(context.Background(), http.MethodPost, "get_corp_token", token, map[string]string{"auth_corpid": corpID, "permanent_code": permanent}, &result); err != nil {
		return "", 0, err
	}
	if result.Token == "" || result.ExpiresIn <= 0 {
		return "", 0, errors.New("企业授权 token 响应无效")
	}
	return result.Token, result.ExpiresIn, nil
}

func authStateHash(state string) string {
	sum := sha256.Sum256([]byte(state))
	return hex.EncodeToString(sum[:])
}

func WecomAuthorizeURL(ctx context.Context) (map[string]string, error) {
	if err := requireCRMManager(ctx, models.DB); err != nil {
		return nil, err
	}
	p := conf.Settings.WecomProvider
	u, err := url.Parse(p.RedirectURL)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("服务商授权返回地址须配置为 HTTPS 地址")
	}
	token, err := providerSuiteToken(ctx)
	if err != nil {
		return nil, err
	}
	var result struct {
		Code      string `json:"pre_auth_code"`
		ExpiresIn int64  `json:"expires_in"`
	}
	if err := providerRequest(ctx, http.MethodGet, "get_pre_auth_code", token, nil, &result); err != nil {
		return nil, err
	}
	if result.Code == "" {
		return nil, errors.New("服务商预授权码无效")
	}
	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return nil, err
	}
	state := hex.EncodeToString(random)
	row := models.WecomAuthState{StateHash: authStateHash(state), TenantID: identity.TenantID(ctx), UserID: identity.UserID(ctx), SuiteID: p.SuiteID, ExpiresAt: time.Now().Add(10 * time.Minute)}
	if err := models.DB.Create(&row).Error; err != nil {
		return nil, err
	}
	query := url.Values{"suite_id": {p.SuiteID}, "pre_auth_code": {result.Code}, "redirect_uri": {p.RedirectURL}, "state": {state}}
	return map[string]string{"url": "https://open.work.weixin.qq.com/3rdapp/install?" + query.Encode()}, nil
}

// StageWecomAuthorization persists encrypted callback data before acknowledging it.
func StageWecomAuthorization(state, code string) error {
	if len(state) != 64 || code == "" {
		return errors.New("授权 state 或临时授权码缺失")
	}
	encoded, err := util.AESEncode(code)
	if err != nil {
		return err
	}
	res := models.CrossTenant().Model(&models.WecomAuthState{}).Where("state_hash=? AND suite_id=? AND expires_at>? AND completed_at IS NULL AND COALESCE(auth_code,'')=''", authStateHash(state), conf.Settings.WecomProvider.SuiteID, time.Now()).Update("auth_code", encoded)
	return res.Error
}

type providerAuthorization struct {
	PermanentCode string `json:"permanent_code"`
	Corp          struct {
		ID   string `json:"corpid"`
		Name string `json:"corp_name"`
	} `json:"auth_corp_info"`
	Auth struct {
		Agents []struct {
			ID int64 `json:"agentid"`
		} `json:"agent"`
	} `json:"auth_info"`
}

func CompleteWecomAuthorization(ctx context.Context, state, code string) (ChannelConfigVO, error) {
	if err := requireCRMManager(ctx, models.DB); err != nil {
		return ChannelConfigVO{}, err
	}
	if len(state) != 64 {
		return ChannelConfigVO{}, errors.New("授权 state 无效")
	}
	var owned models.WecomAuthState
	if err := models.MustFor(ctx).Where("state_hash=? AND user_id=?", authStateHash(state), identity.UserID(ctx)).Take(&owned).Error; err != nil {
		return ChannelConfigVO{}, errors.New("授权不属于当前账号")
	}
	if code != "" {
		if err := StageWecomAuthorization(state, code); err != nil {
			return ChannelConfigVO{}, err
		}
	}
	return completeWecomState(ctx, owned.StateHash)
}

func completeWecomState(ctx context.Context, hash string) (ChannelConfigVO, error) {
	var cfg models.CrmChannelConfig
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		var state models.WecomAuthState
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("state_hash=? AND tenant_id=? AND user_id=?", hash, identity.TenantID(ctx), identity.UserID(ctx)).Take(&state).Error; err != nil {
			return err
		}
		if state.CompletedAt != nil {
			return tx.Where("tenant_id=? AND corp_id=? AND auth_mode='provider'", state.TenantID, state.CorpID).Take(&cfg).Error
		}
		if state.ExpiresAt.Before(time.Now()) || state.SuiteID != conf.Settings.WecomProvider.SuiteID || state.Attempts >= 5 {
			return errors.New("授权已过期或失败，请重新发起授权")
		}
		code, err := decodeWecomSecret(state.AuthCode)
		if err != nil || code == "" {
			return errors.New("尚未收到企业授权码")
		}
		token, err := providerSuiteToken(ctx)
		if err != nil {
			return err
		}
		var auth providerAuthorization
		if err := providerRequest(ctx, http.MethodPost, "get_permanent_code", token, map[string]string{"auth_code": code}, &auth); err != nil {
			return err
		}
		if auth.Corp.ID == "" || auth.PermanentCode == "" || len(auth.Auth.Agents) != 1 {
			return errors.New("企业授权信息不完整或包含多个应用")
		}
		err = models.CrossTenantDB(tx).Where("channel='wecom' AND corp_id=?", auth.Corp.ID).Take(&cfg).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err == nil && (cfg.TenantID != state.TenantID || cfg.AuthMode != "provider" || cfg.SuiteID != state.SuiteID) {
			return errors.New("该企业已绑定其他租户或接入方式")
		}
		if cfg.ID == "" {
			cfg = models.CrmChannelConfig{Model: models.Model{ID: id_generator.StringID()}, TenantID: state.TenantID, Channel: "wecom", CorpID: auth.Corp.ID}
		}
		cfg.Name, cfg.AuthMode, cfg.AuthStatus, cfg.SuiteID = auth.Corp.Name, "provider", "authorized", state.SuiteID
		cfg.Enabled, cfg.MainAgentID, cfg.AuthorizedAt = 1, auth.Auth.Agents[0].ID, time.Now().Unix()
		if cfg.PermanentCode, err = util.AESEncode(auth.PermanentCode); err != nil {
			return err
		}
		if err := validateChannelConfig(&cfg); err != nil {
			return err
		}
		if err := models.EnsureWecomTenantLink(tx, state.TenantID, cfg.CorpID); err != nil {
			return err
		}
		if err := models.EnsureWecomDefaults(tx, cfg.CorpID); err != nil {
			return err
		}
		if err := tx.Save(&cfg).Error; err != nil {
			return err
		}
		now := time.Now()
		return tx.Model(&models.WecomAuthState{}).Where("state_hash=? AND tenant_id=?", hash, state.TenantID).Updates(map[string]any{"completed_at": now, "corp_id": cfg.CorpID, "auth_code": "", "last_error": ""}).Error
	})
	if err != nil {
		return ChannelConfigVO{}, err
	}
	if err := registerWecomClient(cfg); err != nil {
		return ChannelConfigVO{}, err
	}
	return toChannelConfigVO(cfg), nil
}

func ProcessWecomAuthorizations() {
	if conf.Settings.WecomProvider.SuiteID == "" {
		return
	}
	refreshProviderAuthorizations()
	var states []models.WecomAuthState
	if models.CrossTenant().Where("suite_id=? AND expires_at>? AND completed_at IS NULL AND auth_code<>'' AND attempts<5", conf.Settings.WecomProvider.SuiteID, time.Now()).Limit(20).Find(&states).Error != nil {
		return
	}
	for _, state := range states {
		ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: state.TenantID, UserID: state.UserID})
		if err := requireCRMManager(ctx, models.DB); err != nil {
			continue
		}
		if _, err := completeWecomState(ctx, state.StateHash); err != nil {
			models.CrossTenant().Model(&models.WecomAuthState{}).Where("state_hash=? AND completed_at IS NULL", state.StateHash).Updates(map[string]any{"attempts": gorm.Expr("attempts+1"), "last_error": err.Error()})
		}
	}
}

// ProviderLifecycleEvent is decoded only after callback signature/recipient checks.
type ProviderLifecycleEvent struct {
	SuiteID   string `xml:"SuiteId"`
	InfoType  string `xml:"InfoType"`
	Timestamp int64  `xml:"TimeStamp"`
	Ticket    string `xml:"SuiteTicket"`
	AuthCode  string `xml:"AuthCode"`
	CorpID    string `xml:"AuthCorpId"`
	State     string `xml:"State"`
}

func HandleProviderLifecycle(event ProviderLifecycleEvent) error {
	if event.SuiteID == "" || event.SuiteID != conf.Settings.WecomProvider.SuiteID {
		return errors.New("服务商 SuiteId 不匹配")
	}
	switch event.InfoType {
	case "suite_ticket":
		if event.Ticket == "" {
			return errors.New("suite_ticket 为空")
		}
		encoded, err := util.AESEncode(event.Ticket)
		if err != nil {
			return err
		}
		row := models.WecomSuiteTicket{SuiteID: event.SuiteID, Ticket: encoded, Timestamp: event.Timestamp}
		return models.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "suite_id"}}, DoUpdates: clause.AssignmentColumns([]string{"ticket", "timestamp"}), Where: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "wecom_suite_ticket.timestamp < excluded.timestamp"}}}}).Create(&row).Error
	case "create_auth":
		// Marketplace installations without a tenant-bound state remain unbound.
		if event.State == "" {
			return nil
		}
		return StageWecomAuthorization(event.State, event.AuthCode)
	case "cancel_auth", "change_auth":
		updates := map[string]any{"updated_at": time.Now(), "authorized_at": event.Timestamp}
		if event.InfoType == "cancel_auth" {
			updates["enabled"], updates["auth_status"], updates["permanent_code"], updates["msg_arch_enabled"] = 2, "revoked", "", 2
		}
		if event.InfoType == "change_auth" {
			updates["auth_status"] = "refresh_pending"
		}
		res := models.CrossTenant().Model(&models.CrmChannelConfig{}).Where("channel='wecom' AND corp_id=? AND suite_id=? AND auth_mode='provider' AND authorized_at<=?", event.CorpID, event.SuiteID, event.Timestamp).Updates(updates)
		if res.Error == nil && res.RowsAffected > 0 && we_work.Clients != nil {
			we_work.Clients.Remove(event.CorpID)
		}
		return res.Error
	default:
		return nil
	}
}

func refreshProviderAuthorizations() {
	var configs []models.CrmChannelConfig
	if models.CrossTenant().Where("suite_id=? AND auth_mode='provider' AND enabled=1 AND auth_status='refresh_pending'", conf.Settings.WecomProvider.SuiteID).Limit(20).Find(&configs).Error != nil {
		return
	}
	for _, cfg := range configs {
		permanent, err := decodeWecomSecret(cfg.PermanentCode)
		if err != nil {
			continue
		}
		token, err := providerSuiteToken(context.Background())
		if err != nil {
			continue
		}
		var auth providerAuthorization
		if providerRequest(context.Background(), http.MethodPost, "get_auth_info", token, map[string]string{"auth_corpid": cfg.CorpID, "permanent_code": permanent}, &auth) != nil || len(auth.Auth.Agents) != 1 {
			continue
		}
		res := models.CrossTenant().Model(&models.CrmChannelConfig{}).Where("id=? AND authorized_at=? AND enabled=1 AND auth_status='refresh_pending'", cfg.ID, cfg.AuthorizedAt).Updates(map[string]any{"main_agent_id": auth.Auth.Agents[0].ID, "auth_status": "authorized"})
		if res.Error == nil && res.RowsAffected > 0 && we_work.Clients != nil {
			we_work.Clients.Remove(cfg.CorpID)
		}
	}
}

func loginWecomStaffByCode(corpID, code string) (string, error) {
	cfg, err := ResolveWecomConfig(context.Background(), corpID)
	if err != nil {
		return "", err
	}
	if cfg.AuthMode == "provider" {
		token, err := providerSuiteToken(context.Background())
		if err != nil {
			return "", err
		}
		var info struct {
			CorpID string `json:"CorpId"`
			UserID string `json:"UserId"`
		}
		if err := providerRequest(context.Background(), http.MethodGet, "getuserinfo3rd?code="+url.QueryEscape(code), token, nil, &info); err != nil {
			return "", err
		}
		if info.CorpID != corpID || info.UserID == "" {
			return "", errors.New("登录用户不属于指定企业")
		}
		return info.UserID, nil
	}
	client, err := we_work.Clients.Get(corpID)
	if err != nil {
		return "", err
	}
	info, err := client.MainApp.GetUserInfoByCode(code)
	if err != nil {
		return "", err
	}
	if info.UserID == "" {
		return "", errors.New("企微登录未返回员工身份")
	}
	return info.UserID, nil
}
