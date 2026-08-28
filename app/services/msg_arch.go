package services

import (
	"context"
	"core/app/constants"
	"core/app/models"
	"core/app/requests"
	"core/common/util"
	"core/conf"
	"encoding/json"
	"fmt"
	"github.com/go-resty/resty/v2"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"time"
)

type MsgArch struct{ httpClient *resty.Client }

func (a *MsgArch) archiveCall(method, endpoint, corpID string, input interface{}) (interface{}, error) {
	cfg, err := ResolveWecomConfig(context.Background(), corpID)
	if err != nil {
		return nil, err
	}
	if err := models.RequirePlanModules(models.DB, cfg.TenantID, "msg_archive"); err != nil {
		return nil, err
	}
	if conf.Settings.App.InnerSrvAppCode == "" {
		return nil, fmt.Errorf("archive internal authentication is not configured")
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	var values map[string]interface{}
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, err
	}
	values["ext_corp_id"], values["tenant_id"] = cfg.CorpID, cfg.TenantID
	request := a.httpClient.R().SetHeader("Accept", "application/json")
	endpoint = path.Join("/api/v1", endpoint)
	var body []byte
	query := url.Values{}
	if method == http.MethodGet {
		for key, value := range values {
			if value != nil {
				query.Set(key, fmt.Sprint(value))
			}
		}
		request.SetQueryString(query.Encode())
	} else {
		body, err = json.Marshal(values)
		if err != nil {
			return nil, err
		}
		request.SetHeader("Content-Type", "application/json").SetBody(body)
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	request.SetHeader("X-Scrm-Timestamp", ts).SetHeader("X-Scrm-Signature", util.ArchiveRequestSignature(conf.Settings.App.InnerSrvAppCode, ts, method, endpoint, query.Encode(), body))
	var result constants.JsonResult
	response, err := request.SetResult(&result).Execute(method, endpoint)
	if err != nil {
		return nil, err
	}
	if response.StatusCode() != http.StatusOK || result.Code != 0 {
		return nil, fmt.Errorf("archive request failed: HTTP %d, code %d", response.StatusCode(), result.Code)
	}
	return result.Data, nil
}

func (a *MsgArch) QuerySessions(req requests.QuerySessionReq, corpID string) (interface{}, error) {
	return a.archiveCall(http.MethodGet, constants.MsgArchSrvPathSessions, corpID, req)
}
func (a *MsgArch) QueryMsgs(req requests.QueryChatMsgReq, corpID string) (interface{}, error) {
	return a.archiveCall(http.MethodGet, constants.MsgArchSrvPathMsgs, corpID, req)
}
func (a *MsgArch) SearchMsgs(req requests.SearchMsgReq, corpID string) (interface{}, error) {
	return a.archiveCall(http.MethodGet, constants.MsgArchSrvSearchMsgs, corpID, req)
}
func (a *MsgArch) Sync(corpID string) error {
	_, err := a.archiveCall(http.MethodPost, constants.MsgArchSrvPathSync, corpID, map[string]string{})
	return err
}
func NewMsgArch() *MsgArch {
	host := fmt.Sprintf("http://%s:%d", conf.Settings.Server.MsgArchSrvHost, conf.Settings.Server.MsgArchHttpPort)
	return &MsgArch{httpClient: resty.New().SetHostURL(host).SetTimeout(30 * time.Second)}
}
