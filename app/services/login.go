package services

import (
	"context"
	"core/common/id_generator"
	"fmt"
	"net/url"
	"os"

	"core/app/constants"
	"core/app/dto"
	"core/app/models"
	"core/common/ecode"
	"core/common/log"
	"core/common/we_work"
	"core/conf"

	"github.com/gogf/gf/encoding/gurl"
	"github.com/pkg/errors"
)

type Login struct {
	staffModel models.Staff
}

func NewDefaultLogin() *Login {
	return &Login{
		staffModel: models.Staff{},
	}
}

func NewLogin(staff models.Staff) *Login {
	return &Login{
		staffModel: staff,
	}
}

func (o *Login) ConsoleLogin(extCorpID string, state string, sourceURL string) (item dto.ConsoleLoginResp, err error) {

	cfg, err := ResolveWecomConfig(context.Background(), extCorpID)
	if err != nil {
		return item, err
	}
	extCorpID = cfg.CorpID

	sourceURL, err = gurl.Decode(sourceURL)
	if err != nil {
		err = errors.Wrap(err, "gurl.Decode failed")
		return
	}

	sourceURLInfo, err := url.Parse(sourceURL)
	if err != nil {
		err = errors.Wrap(err, "url.Parse failed")
		return
	}

	item = dto.ConsoleLoginResp{
		AppID:   extCorpID,
		AgentID: cfg.MainAgentID,
		RedirectURI: fmt.Sprintf(
			"%s://%s/api/v1/console/action/login-callback?appid=%s",
			sourceURLInfo.Scheme,
			sourceURLInfo.Host,
			extCorpID,
		),
		State: state,
	}

	item.LocationURL = fmt.Sprintf(
		"https://open.work.weixin.qq.com/wwopen/sso/qrConnect?appid=%s&agentid=%d&redirect_uri=%s&state=%s",
		item.AppID, item.AgentID, gurl.Encode(item.RedirectURI), item.State,
	)

	if cfg.AuthMode == "provider" {
		item.LocationURL = "https://open.work.weixin.qq.com/wwopen/sso/3rd_qrConnect?" + url.Values{"appid": {cfg.SuiteID}, "redirect_uri": {item.RedirectURI}, "state": {state}}.Encode()
	}

	return
}

func (o *Login) ConsoleLoginCallback(extCorpID string, code string) (item *models.Staff, err error) {
	extStaffID, err := loginWecomStaffByCode(extCorpID, code)
	if err != nil {
		return nil, err
	}

	item, err = (&models.Staff{}).Get(extStaffID, extCorpID, false)
	if err != nil {
		err = errors.Wrap(err, "GetStaffByUserID failed")
		return
	}

	if item.Status == constants.UserStatusDeactivated || item.ExtCorpID != extCorpID {
		err = errors.WithStack(ecode.ForbiddenError)
		return
	}

	return
}

// ForceConsoleLogin 调试接口-指定员工强制登录。仅开发/测试环境可用。
// 逻辑与 app/controller/login.go 的 ConsoleForceLogin 一致：查到员工直接返回；
// 目标是超级管理员且员工不存在时自动创建。
func (o *Login) ForceConsoleLogin(extCorpID string, extStaffID string) (item *models.Staff, err error) {
	if conf.Settings == nil || (conf.Settings.App.Env != "DEV" && conf.Settings.App.Env != "TEST") || os.Getenv("CRM_ENABLE_FORCE_LOGIN") != "true" {
		return nil, errors.New("force login requires DEV/TEST and CRM_ENABLE_FORCE_LOGIN=true")
	}
	if extCorpID == "" {
		return nil, errors.New("请指定企微企业")
	}
	if extStaffID == "" && len(conf.Settings.App.SuperAdmin) > 0 {
		extStaffID = conf.Settings.App.SuperAdmin[0]
	}

	item, err = (&models.Staff{}).Get(extStaffID, extCorpID, false)
	if err != nil {
		err = errors.Wrap(err, "GetStaffByUserID failed")
		if len(conf.Settings.App.SuperAdmin) > 0 && extStaffID == conf.Settings.App.SuperAdmin[0] {
			log.Errorf("GetStaffByUserID failed, err: %v, try to login as super admin staff", err)
			// 超级管理员缺失时自动创建
			item = &models.Staff{
				Model:     models.Model{ID: id_generator.StringID()},
				ExtID:     extStaffID,
				ExtCorpID: extCorpID,
				Name:      extStaffID,
				RoleType:  string(constants.RoleTypeSuperAdmin),
				RoleID:    string(constants.DefaultCorpSuperAdminRoleID),
				Status:    constants.UserStatusActivated,
			}
			if err = models.DB.Create(item).Error; err != nil {
				err = errors.Wrap(err, "create super admin staff failed")
				return nil, err
			}
			return item, nil
		}
		return nil, err
	}
	if item.Status == constants.UserStatusDeactivated {
		return nil, ecode.ForbiddenError
	}
	return item, nil
}

func (o *Login) StaffLogin(extCorpID string, state string, sourceURL string) (item dto.StaffLoginResp, err error) {
	cfg, err := ResolveWecomConfig(context.Background(), extCorpID)
	if err != nil {
		return item, err
	}
	extCorpID = cfg.CorpID
	sourceURL, err = gurl.Decode(sourceURL)
	if err != nil {
		err = errors.Wrap(err, "gurl.Decode failed")
		return
	}

	sourceURLInfo, err := url.Parse(sourceURL)
	if err != nil {
		err = errors.Wrap(err, "url.Parse failed")
		return
	}

	item = dto.StaffLoginResp{
		AppID: extCorpID,
		RedirectURI: fmt.Sprintf(
			"%s://%s/api/v1/staff-frontend/action/login-callback?appid=%s&source_url=%s",
			sourceURLInfo.Scheme,
			sourceURLInfo.Host,
			extCorpID,
			gurl.Encode(sourceURL),
		),
		SourceURL: sourceURL,
		State:     state,
	}

	item.LocationURL = fmt.Sprintf(
		"https://open.weixin.qq.com/connect/oauth2/authorize?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_base&state=%s#wechat_redirect",
		item.AppID, gurl.Encode(item.RedirectURI), item.State,
	)

	return
}

func (o *Login) StaffLoginCallback(extCorpID string, code string) (item *models.Staff, err error) {
	extStaffID, err := loginWecomStaffByCode(extCorpID, code)
	if err != nil {
		return nil, err
	}

	item, err = (&models.Staff{}).Get(extStaffID, extCorpID, false)
	if err != nil {
		err = errors.Wrap(err, "GetStaffByUserID failed")
		return
	}

	if item.Status == constants.UserStatusDeactivated || item.ExtCorpID != extCorpID {
		err = ecode.ForbiddenError
		return
	}

	return
}

//func (o *Login) CustomerLogin(extCorpID string, state string, sourceURL string) (item dto.CustomerLoginResp, err error) {
//
//	sourceURL, err = gurl.Decode(sourceURL)
//	if err != nil {
//		err = errors.Wrap(err, "gurl.Decode failed")
//		return
//	}
//	sourceURL = gurl.Encode(sourceURL)
//
//	item = dto.CustomerLoginResp{
//		AppID:       extCorpID,
//		RedirectURI: fmt.Sprintf("%s/api/v1/customer-frontend/action/login-callback?appid=%s&source_url=%s", conf.Settings.App.URL, extCorpID, sourceURL),
//		SourceURL:   sourceURL,
//		State:       state,
//	}
//
//	item.LocationURL = fmt.Sprintf(
//		"https://open.weixin.qq.com/connect/oauth2/authorize?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_base&state=%s#wechat_redirect",
//		item.AppID, gurl.Encode(item.RedirectURI), item.State,
//	)
//
//	return
//}

func (o *Login) CustomerLoginCallback(extCorpID string, code string) (item *models.Customer, err error) {
	client, err := we_work.Clients.Get(extCorpID)
	if err != nil {
		err = errors.Wrap(err, "Get Client failed")
		return
	}

	userInfo, err := client.MainApp.GetUserInfoByCode(code)
	if err != nil {
		err = errors.Wrap(err, "GetUserInfoByCode failed")
		return
	}

	if userInfo.UserID == "" {
		err = ecode.ForbiddenError
		return
	}

	item, err = (models.Customer{}).Get(userInfo.ExternalUserID, extCorpID, false)
	if err != nil {
		err = errors.Wrap(err, "Get Customer failed")
		return
	}

	if item.ExtCorpID != extCorpID {
		err = ecode.ForbiddenError
		return
	}

	return
}
