package services

import (
	"fmt"
	"net/url"

	"core/app/constants"
	"core/app/entities"
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

func (o *Login) ConsoleLogin(extCorpID string, state string, sourceURL string) (item entities.ConsoleLoginResp, err error) {

	if extCorpID == "" {
		extCorpID = conf.Settings.WeWork.ExtCorpID
	}

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

	item = entities.ConsoleLoginResp{
		AppID:   extCorpID,
		AgentID: conf.Settings.WeWork.MainAgentID,
		RedirectURI: fmt.Sprintf(
			"%s://%s/api/v1/console/action/login-callback",
			sourceURLInfo.Scheme,
			sourceURLInfo.Host,
		),
		State: state,
	}

	item.LocationURL = fmt.Sprintf(
		"https://open.work.weixin.qq.com/wwopen/sso/qrConnect?appid=%s&agentid=%d&redirect_uri=%s&state=%s",
		item.AppID, item.AgentID, gurl.Encode(item.RedirectURI), item.State,
	)

	return
}

func (o *Login) ConsoleLoginCallback(extCorpID string, code string) (item *models.Staff, err error) {
	client, err := we_work.Clients.Get(conf.Settings.WeWork.ExtCorpID)
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
		err = errors.WithStack(ecode.ForbiddenError)
		return
	}

	item, err = (&models.Staff{}).Get(userInfo.UserID, "", false)
	if err != nil {
		err = errors.Wrap(err, "GetStaffByUserID failed")
		return
	}

	if item.ExtCorpID != extCorpID {
		err = errors.WithStack(ecode.ForbiddenError)
		return
	}

	return
}

// ForceConsoleLogin 调试接口-指定员工强制登录。仅开发/测试环境可用。
// 逻辑与 app/controller/login.go 的 ConsoleForceLogin 一致：查到员工直接返回；
// 目标是超级管理员且员工不存在时自动创建。
func (o *Login) ForceConsoleLogin(extCorpID string, extStaffID string) (item *models.Staff, err error) {
	if conf.Settings == nil || (conf.Settings.App.Env != "DEV" && conf.Settings.App.Env != "TEST") {
		return nil, errors.New("force login is only available in DEV/TEST")
	}
	if extCorpID == "" {
		extCorpID = conf.Settings.WeWork.ExtCorpID
	}
	if extStaffID == "" && len(conf.Settings.App.SuperAdmin) > 0 {
		extStaffID = conf.Settings.App.SuperAdmin[0]
	}

	item, err = (&models.Staff{}).Get(extStaffID, "", false)
	if err != nil {
		err = errors.Wrap(err, "GetStaffByUserID failed")
		if len(conf.Settings.App.SuperAdmin) > 0 && extStaffID == conf.Settings.App.SuperAdmin[0] {
			log.Errorf("GetStaffByUserID failed, err: %v, try to login as super admin staff", err)
			// 超级管理员缺失时自动创建
			item = &models.Staff{
				Model:     models.Model{ID: "1"},
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
	return item, nil
}

func (o *Login) StaffLogin(extCorpID string, state string, sourceURL string) (item entities.StaffLoginResp, err error) {
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

	item = entities.StaffLoginResp{
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
	client, err := we_work.Clients.Get(conf.Settings.WeWork.ExtCorpID)
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

	item, err = (&models.Staff{}).Get(userInfo.UserID, "", false)
	if err != nil {
		err = errors.Wrap(err, "GetStaffByUserID failed")
		return
	}

	if item.ExtCorpID != extCorpID {
		err = ecode.ForbiddenError
		return
	}

	return
}

//func (o *Login) CustomerLogin(extCorpID string, state string, sourceURL string) (item entities.CustomerLoginResp, err error) {
//
//	sourceURL, err = gurl.Decode(sourceURL)
//	if err != nil {
//		err = errors.Wrap(err, "gurl.Decode failed")
//		return
//	}
//	sourceURL = gurl.Encode(sourceURL)
//
//	item = entities.CustomerLoginResp{
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
	client, err := we_work.Clients.Get(conf.Settings.WeWork.ExtCorpID)
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

	item, err = (models.Customer{}).Get(userInfo.ExternalUserID, conf.Settings.WeWork.ExtCorpID, false)
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
