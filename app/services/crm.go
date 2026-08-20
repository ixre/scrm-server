package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/identity"
	"core/common/log"
	"core/common/util"
	"core/common/we_work"
	"core/conf"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// createCRM 创建记录，自动省略值为空的 FK 列。
// 本地模型的可选外键是 bigint，空字符串会被 PG 拒绝，需以 NULL 表示"未设"。
func createCRM(db *gorm.DB, v any, fkFields ...string) error {
	if len(fkFields) == 0 {
		return db.Create(v).Error
	}
	rv := reflect.ValueOf(v).Elem()
	omits := []string{}
	for _, f := range fkFields {
		fv := rv.FieldByName(f)
		if fv.IsValid() && fv.Kind() == reflect.String && fv.String() == "" {
			omits = append(omits, f)
		}
	}
	if len(omits) == 0 {
		return db.Create(v).Error
	}
	return db.Omit(omits...).Create(v).Error
}

// updatesCRM 从更新 map 中剔除值为空字符串的 FK 列（避免空串写入 bigint）。
func updatesCRM(m map[string]any, fkFields ...string) map[string]any {
	for _, f := range fkFields {
		if v, ok := m[f]; ok {
			if s, isStr := v.(string); isStr && s == "" {
				delete(m, f)
			}
		}
	}
	return m
}

// CRM 原生服务 —— 渠道无关，全部按 tenant_id 分区、归 user_id，不依赖企微字段。
// 企微 external id 的桥接在 ChannelLink，不在此处。

// ---------- Tenant / User ----------

type TenantService struct{}

func NewTenant() *TenantService { return &TenantService{} }

func (s TenantService) Create(name, code string) (models.Tenant, error) {
	t := models.Tenant{
		Model:  models.Model{ID: id_generator.StringID()},
		Name:   name,
		Code:   code,
		Status: 1,
	}
	err := models.DB.Create(&t).Error
	if err != nil {
		log.Sugar.Errorw("crm tenant create failed", "err", err)
		return t, err
	}
	seedTenantDefaults(t.ID)
	return t, nil
}

func (s TenantService) List() ([]models.Tenant, error) {
	var ts []models.Tenant
	err := models.DB.Order("id").Find(&ts).Error
	return ts, err
}

// ---------- 默认角色 / 默认销售阶段种子 ----------

var defaultRoles = []struct {
	Code, Name string
	Perms      constants.StringArrayField
	IsDefault  bool
}{
	{"admin", "租户管理员", constants.StringArrayField{
		"crm:user:set_password", "crm:user:create",
		"crm:risk:full", "crm:marketing_ext:full", "crm:group_sop:full", "crm:smart_card:full",
	}, false},
	{"manager", "销售经理", constants.StringArrayField{
		"crm:user:create",
		"crm:risk:full", "crm:marketing_ext:full", "crm:group_sop:full", "crm:smart_card:full",
	}, false},
	{"sales", "销售", constants.StringArrayField{
		"crm:smart_card:full",
	}, true},
}

var defaultStages = []struct {
	Name          string
	Sort          int
	Probability   int
	IsWon, IsLost bool
}{
	{"初步接洽", 1, 10, false, false},
	{"需求确认", 2, 30, false, false},
	{"方案报价", 3, 50, false, false},
	{"商务谈判", 4, 70, false, false},
	{"赢单", 5, 100, true, false},
	{"输单", 6, 0, false, true},
}

// defaultChannelTags 各渠道标签目录预设（参考 SCRM 标签中心）。
var defaultChannelTags = []struct{ Channel, Name, Category string }{
	{constants.ChannelWecom, "重点客户", "客户价值"},
	{constants.ChannelWecom, "VIP", "客户价值"},
	{constants.ChannelWecom, "高意向", "意向"},
	{constants.ChannelWecom, "已成交", "成交"},
	{constants.ChannelXiaohongshu, "种草", "互动"},
	{constants.ChannelXiaohongshu, "高互动", "互动"},
	{constants.ChannelXiaohongshu, "高潜", "意向"},
	{constants.ChannelXiaohongshu, "已承接", "承接"},
	{constants.ChannelGzh, "已关注", "关系"},
	{constants.ChannelGzh, "已取关", "关系"},
	{constants.ChannelGzh, "已激活", "互动"},
	{constants.ChannelGzh, "高潜", "意向"},
	{constants.ChannelDouyin, "粉丝", "关系"},
	{constants.ChannelDouyin, "高互动", "互动"},
	{constants.ChannelDouyin, "直播新增", "来源"},
	{constants.ChannelDouyin, "已承接", "承接"},
}

// seedTenantDefaults 幂等地为租户种子默认角色与销售阶段。
func seedTenantDefaults(tenantID string) {
	var cnt int64
	if models.DB.Model(&models.CrmRole{}).Scopes(models.TenantScope(tenantID)).Count(&cnt).Error == nil && cnt == 0 {
		for _, r := range defaultRoles {
			models.DB.Create(&models.CrmRole{
				Model:     models.Model{ID: id_generator.StringID()},
				TenantID:  tenantID,
				Code:      r.Code,
				Name:      r.Name,
				Perms:     r.Perms,
				IsDefault: r.IsDefault,
			})
		}
	}
	if models.DB.Model(&models.CrmPipelineStage{}).Scopes(models.TenantScope(tenantID)).Count(&cnt).Error == nil && cnt == 0 {
		for _, st := range defaultStages {
			models.DB.Create(&models.CrmPipelineStage{
				Model:       models.Model{ID: id_generator.StringID()},
				TenantID:    tenantID,
				Name:        st.Name,
				Sort:        st.Sort,
				Probability: st.Probability,
				IsWon:       st.IsWon,
				IsLost:      st.IsLost,
			})
		}
	}
	// 渠道标签目录按 (tenant_id, channel) 维度幂等补齐：存量租户只补缺失渠道，已存在的渠道不动。
	seedChannelTags(tenantID)
}

// seedChannelTags 为租户补齐默认渠道标签：遍历 defaultChannelTags，某渠道若尚无标签则整批插入该渠道预设。
func seedChannelTags(tenantID string) {
	seeded := map[string]bool{}
	for _, t := range defaultChannelTags {
		if seeded[t.Channel] {
			continue
		}
		seeded[t.Channel] = true
		var cnt int64
		if err := models.DB.Model(&models.CrmChannelTag{}).Scopes(models.TenantScope(tenantID)).Where("channel = ?",  t.Channel).Count(&cnt).Error; err != nil || cnt > 0 {
			continue
		}
		sort := 0
		for _, tag := range defaultChannelTags {
			if tag.Channel != t.Channel {
				continue
			}
			models.DB.Create(&models.CrmChannelTag{
				Model:    models.Model{ID: id_generator.StringID()},
				TenantID: tenantID,
				Channel:  tag.Channel,
				Name:     tag.Name,
				Category: tag.Category,
				Sort:     sort,
			})
			sort++
		}
	}
}

// SeedCRMDefaults 为所有已有租户补齐默认角色/阶段（幂等），启动时调用。
func SeedCRMDefaults() {
	var ts []models.Tenant
	if err := models.DB.Find(&ts).Error; err != nil {
		return
	}
	for _, t := range ts {
		seedTenantDefaults(t.ID)
		seedChannelConfig(t.ID)
	}
}

// demoTenantCode 演示租户编码；demoAdminPassword 演示管理员初始密码。
const (
	demoTenantCode    = "demo"
	demoAdminUsername = "admin"
	demoAdminPassword = "123456"
)

// SeedDemoAccount 幂等地创建演示租户与登录账号（demo/admin/123456），仅非生产环境执行。
func SeedDemoAccount() {
	if conf.Settings.App.Env == "PROD" {
		return
	}
	// 幂等：租户不存在则创建，并补齐默认角色/阶段。
	var t models.Tenant
	if err := models.DB.Where("code = ?", demoTenantCode).First(&t).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return
		}
		t = models.Tenant{
			Model:  models.Model{ID: id_generator.StringID()},
			Name:   "演示企业",
			Code:   demoTenantCode,
			Status: 1,
		}
		if err := models.DB.Create(&t).Error; err != nil {
			return
		}
		seedTenantDefaults(t.ID)
	}
	// 幂等：演示管理员账号已存在则跳过。
	var cnt int64
	if err := models.DB.Model(&models.User{}).Scopes(models.TenantScope(t.ID)).Where("username = ?", demoAdminUsername).Count(&cnt).Error; err != nil || cnt > 0 {
		return
	}
	h, err := hashPassword(demoAdminPassword)
	if err != nil {
		return
	}
	var roleID string
	var r models.CrmRole
	if models.DB.Scopes(models.TenantScope(t.ID)).Where("code = ?", "admin").First(&r).Error == nil {
		roleID = r.ID
	}
	u := models.User{
		Model:    models.Model{ID: id_generator.StringID()},
		TenantID: t.ID,
		Username: demoAdminUsername,
		Password: h,
		Name:     "管理员",
		RoleID:   roleID,
		Status:   1,
	}
	// 用 createCRM 省略为空的 bigint 外键（department_id），避免 PG 拒绝空串。
	if err := createCRM(models.DB, &u, "RoleID", "DepartmentID"); err != nil {
		return
	}
}

// seedChannelConfig 若配置了真实企微 corp（conf.Settings.WeWork），为租户幂等引导一条渠道接入配置 + 租户级 ChannelLink。
// dev 占位（"todo"/空）或租户已有该 corp 配置则跳过。仅部署真实企微凭据时生效。
func seedChannelConfig(tenantID string) {
	ww := conf.Settings.WeWork
	if isTodo(ww.ExtCorpID) || isTodo(ww.CallbackToken) || isTodo(ww.CallbackAesKey) {
		return
	}
	var cnt int64
	if err := models.DB.Model(&models.CrmChannelConfig{}).Scopes(models.TenantScope(tenantID)).Where("channel = ? AND corp_id = ?",  constants.ChannelWecom,  ww.ExtCorpID).
		Count(&cnt).Error; err != nil || cnt > 0 {
		return
	}
	cfg := models.CrmChannelConfig{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		Channel:     constants.ChannelWecom,
		CorpID:      ww.ExtCorpID,
		Name:        "企业微信",
		MainAgentID: ww.MainAgentID,
		Enabled:     1,
	}
	var cerr error
	if cfg.ContactSecret, cerr = util.AESEncode(ww.ContactSecret); cerr != nil {
		return
	}
	if cfg.CustomerSecret, cerr = util.AESEncode(ww.CustomerSecret); cerr != nil {
		return
	}
	if cfg.MainAgentSecret, cerr = util.AESEncode(ww.MainAgentSecret); cerr != nil {
		return
	}
	if cfg.CallbackToken, cerr = util.AESEncode(ww.CallbackToken); cerr != nil {
		return
	}
	if cfg.CallbackAesKey, cerr = util.AESEncode(ww.CallbackAesKey); cerr != nil {
		return
	}
	if err := models.DB.Create(&cfg).Error; err != nil {
		log.Sugar.Errorw("seed channel config failed", "err", err)
		return
	}
	registerWecomClient(cfg)
	// 租户级 wecom 链路（供 ChannelSummary 读 corp_id 统计企微资源）
	var link models.ChannelLink
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("channel = ? AND entity_type = ?", constants.ChannelWecom,  "tenant").
		First(&link).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			models.DB.Create(&models.ChannelLink{
				Model:      models.Model{ID: id_generator.StringID()},
				TenantID:   tenantID,
				Channel:    constants.ChannelWecom,
				EntityType: "tenant",
				EntityID:   tenantID,
				ExtID:      ww.ExtCorpID,
				ExtCorpID:  ww.ExtCorpID,
			})
		}
	}
}

// isTodo 判断配置值是否为 dev 占位（空或含 "todo"）。
func isTodo(s string) bool {
	return s == "" || strings.Contains(s, "todo")
}

type UserService struct{}

func NewUser() *UserService { return &UserService{} }

// UserCreateReq 创建本地用户的入参。Password 为空时生成随机初始密码（仅返回一次）。
type UserCreateReq struct {
	Username     string `json:"username"`
	Name         string `json:"name"`
	Mobile       string `json:"mobile"`
	RoleID       string `json:"role_id"`
	Position     string `json:"position"`
	DepartmentID string `json:"department_id"`
	Password     string `json:"password"`
}

func (s UserService) Create(ctx context.Context, req UserCreateReq) (models.User, string, error) {
	tenantID := identity.TenantID(ctx)

	if req.RoleID == "" {
		var r models.CrmRole
		if err := models.MustFor(ctx).Where("code = ?", "sales").First(&r).Error; err == nil {
			req.RoleID = r.ID
		}
	}
	u := models.User{
		Model:        models.Model{ID: id_generator.StringID()},
		TenantID:     tenantID,
		Username:     req.Username,
		Name:         req.Name,
		Mobile:       req.Mobile,
		RoleID:       req.RoleID,
		Position:     req.Position,
		DepartmentID: req.DepartmentID,
		Status:       1,
	}
	initial := ""
	if req.Password != "" {
		if err := validatePassword(req.Password); err != nil {
			return u, "", err
		}
		h, err := hashPassword(req.Password)
		if err != nil {
			return u, "", err
		}
		u.Password = h
	} else {
		pw, err := randPassword(12)
		if err != nil {
			return u, "", err
		}
		initial = pw
		h, err := hashPassword(pw)
		if err != nil {
			return u, "", err
		}
		u.Password = h
	}
	err := createCRM(models.MustFor(ctx), &u, "RoleID", "DepartmentID")
	if err != nil {
		log.Sugar.Errorw("crm user create failed", "err", err)
	}
	return u, initial, err
}

func (s UserService) Query(ctx context.Context, pager *app.Pager) ([]map[string]any, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx).Model(&models.User{})
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var us []models.User
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id").Find(&us).Error
	if err != nil {
		return nil, 0, err
	}
	// 批量查角色 code
	roleIDs := make(map[string]bool)
	for _, u := range us {
		if u.RoleID != "" {
			roleIDs[u.RoleID] = true
		}
	}
	roleCodeMap := map[string]string{}
	if len(roleIDs) > 0 {
		ids := make([]string, 0, len(roleIDs))
		for id := range roleIDs {
			ids = append(ids, id)
		}
		var roles []models.CrmRole
		models.MustFor(ctx).Where("id IN ?", ids).Find(&roles)
		for _, r := range roles {
			roleCodeMap[r.ID] = r.Code
		}
	}
	// 批量查企微绑定
	userIDs := make([]string, 0, len(us))
	for _, u := range us {
		userIDs = append(userIDs, u.ID)
	}
	wecomMap := map[string]string{}
	if len(userIDs) > 0 {
		var links []models.ChannelLink
		models.MustFor(ctx).Where("channel = ? AND entity_type = ? AND entity_id IN ?", constants.ChannelWecom, "user", userIDs).Find(&links)
		for _, l := range links {
			wecomMap[l.EntityID] = l.ExtID
		}
	}
	result := make([]map[string]any, 0, len(us))
	for _, u := range us {
		result = append(result, map[string]any{
			"id":            u.ID,
			"username":      u.Username,
			"name":          u.Name,
			"mobile":        u.Mobile,
			"email":         u.Email,
			"role_id":       u.RoleID,
			"role_code":     roleCodeMap[u.RoleID],
			"position":      u.Position,
			"department_id": u.DepartmentID,
			"status":        u.Status,
			"wecom_ext_id":  wecomMap[u.ID],
		})
	}
	return result, total, nil
}

// UserUpdateReq 更新本地用户的入参。
type UserUpdateReq struct {
	Name         string `json:"name"`
	Mobile       string `json:"mobile"`
	RoleID       string `json:"role_id"`
	Position     string `json:"position"`
	DepartmentID string `json:"department_id"`
	Status       *int   `json:"status"`
}

func (s UserService) Update(ctx context.Context, userID string, req UserUpdateReq) error {

	m := map[string]any{
		"name":          req.Name,
		"mobile":        req.Mobile,
		"position":      req.Position,
		"department_id": req.DepartmentID,
	}
	if req.RoleID != "" {
		m["role_id"] = req.RoleID
	}
	if req.Status != nil {
		m["status"] = *req.Status
	}
	m = updatesCRM(m, "RoleID", "DepartmentID")
	return models.MustFor(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(m).Error
}

// BindWecom 将本地用户与企微员工绑定（创建或更新 ChannelLink）。
// staffExtID 为企微员工 ext_id，corpID 为企微 corp_id。
func (s UserService) BindWecom(ctx context.Context, userID, corpID, staffExtID string) error {
	tenantID := identity.TenantID(ctx)

	if userID == "" || staffExtID == "" {
		return errors.New("user_id 和 staff_ext_id 不能为空")
	}
	// 确认用户存在
	var u models.User
	if err := models.MustFor(ctx).Where("id = ?", userID).First(&u).Error; err != nil {
		return errors.New("用户不存在")
	}
	// 查找已有 link
	var link models.ChannelLink
	err := models.MustFor(ctx).Where("channel = ? AND entity_type = ? AND entity_id = ?", constants.ChannelWecom, "user", userID).First(&link).Error
	if err == nil {
		return models.MustFor(ctx).Model(&models.ChannelLink{}).Where("id = ?", link.ID).
			Updates(map[string]any{"ext_id": staffExtID, "ext_corp_id": corpID}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return models.MustFor(ctx).Create(&models.ChannelLink{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		Channel:    constants.ChannelWecom,
		EntityType: "user",
		EntityID:   userID,
		ExtID:      staffExtID,
		ExtCorpID:  corpID,
	}).Error
}

// UnbindWecom 解除本地用户与企微员工的绑定。
func (s UserService) UnbindWecom(ctx context.Context, userID string) error {

	return models.MustFor(ctx).Where("channel = ? AND entity_type = ? AND entity_id = ?", constants.ChannelWecom, "user", userID).
		Delete(&models.ChannelLink{}).Error
}

// ---------- 密码工具 ----------

// hashPassword bcrypt 哈希。
func hashPassword(p string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	return string(h), err
}

func validatePassword(p string) error {
	if n := len(p); n < 6 || n > 64 {
		return errors.New("密码长度需 6-64 位")
	}
	return nil
}

// dummyPasswordHash 登录失败时做一次 bcrypt 等时比较，避免通过响应耗时枚举账号/租户。
var dummyPasswordHash, _ = bcrypt.GenerateFromPassword([]byte("scrm-timing-equalizer"), bcrypt.DefaultCost)

// randPassword 生成 n 位随机密码（crypto/rand，去易混淆字符）。
func randPassword(n int) (string, error) {
	const charset = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b), nil
}

// ---------- CrmAuth（本地账号登录，SaaS 多租户）----------

type CrmAuthService struct{}

func NewCrmAuth() *CrmAuthService { return &CrmAuthService{} }

// LoginResult 登录成功后返回的本地身份，网关据此签发 JWT。
type LoginResult struct {
	TenantID     string   `json:"tenant_id"`
	TenantName   string   `json:"tenant_name"`
	UserID       string   `json:"user_id"`
	Username     string   `json:"username"`
	Name         string   `json:"name"`
	Mobile       string   `json:"mobile"`
	Position     string   `json:"position"`
	DepartmentID string   `json:"department_id"`
	RoleID       string   `json:"role_id"`
	RoleCode     string   `json:"role_code"`
	Perms        []string `json:"perms"`
	Authorities  []string `json:"authorities"` // 前端菜单 authority 用（Biz 格式 + role_code）
	Avatar       string   `json:"avatar"`
	CorpID       string   `json:"corp_id"`
	StaffID      string   `json:"staff_id"`
}

// crmPermsToAuthorities 将 CRM 角色编码 + perms 映射为前端菜单 authority 格式（Biz_*）。
// CRM 原生权限点（crm:xxx:full）与企微菜单 authority（BizXxx_Full）格式不同，
// 这里做桥接，使密码登录用户也能被菜单级权限过滤正确处理。
func crmPermsToAuthorities(roleCode string, perms []string) []string {
	auth := []string{roleCode} // role_code 本身也作为 authority（支持菜单按角色控制）
	has := func(p string) bool {
		for _, x := range perms {
			if x == p {
				return true
			}
		}
		return false
	}
	// admin/manager 拥有全部 Phase 7 权限
	if roleCode == "admin" || roleCode == "manager" || has("crm:risk:full") {
		auth = append(auth, "BizSensitiveWord_Full", "BizSensitiveWord_Read")
		auth = append(auth, "BizRiskCenter_Full", "BizRiskCenter_Read")
	}
	if roleCode == "admin" || roleCode == "manager" || has("crm:group_sop:full") {
		auth = append(auth, "BizGroupSop_Full", "BizGroupSop_Read")
	}
	if roleCode == "admin" || roleCode == "manager" || has("crm:marketing_ext:full") {
		auth = append(auth, "BizMarketingExt_Full", "BizMarketingExt_Read")
	}
	if has("crm:smart_card:full") || roleCode == "admin" || roleCode == "manager" || roleCode == "sales" {
		auth = append(auth, "BizSmartCard_Full", "BizSmartCard_Read")
	}
	// admin/manager 补齐基础 CRM 菜单权限（客户/线索/商机等菜单目前无 authority 限制，这里补上以备将来）
	if roleCode == "admin" || roleCode == "manager" {
		auth = append(auth, "BizCustomerInfo_Full", "BizCustomerInfo_Read")
		auth = append(auth, "BizCustomerTag_Full", "BizCustomerTag_Read")
		auth = append(auth, "BizCustomerLoss_Full", "BizCustomerLoss_Read")
		auth = append(auth, "BizContactWay_Full", "BizContactWay_Read")
		auth = append(auth, "BizMassMsg_Full", "BizMassMsg_Read")
		auth = append(auth, "BizWelcomeMsg_Full", "BizWelcomeMsg_Read")
		auth = append(auth, "BizCustomerGroupChat_Full", "BizCustomerGroupChat_Read")
		auth = append(auth, "BizDeleteCustomer_Full", "BizDeleteCustomer_Read")
		auth = append(auth, "BizMsgArch_Full", "BizMsgArch_Read")
	}
	if roleCode == "admin" {
		auth = append(auth, "BizRole_Full", "BizRole_Read")
		auth = append(auth, "BizStaffInfo_Full", "BizStaffInfo_Read")
		auth = append(auth, "BizDepartment_Full", "BizDepartment_Read")
	}
	return auth
}

// Login 按 tenant_code + username + password 校验，返回本地身份。
// SaaS 多租户：租户编码全局唯一，用户归属在租户内解析，与企微渠道无关。
func (s CrmAuthService) Login(tenantCode, username, password string) (LoginResult, error) {
	var t models.Tenant
	if err := models.DB.Where("code = ?", tenantCode).First(&t).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return LoginResult{}, errors.New("租户不存在或已停用")
		}
		return LoginResult{}, err
	}
	if t.Status != 1 {
		bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
		return LoginResult{}, errors.New("租户不存在或已停用")
	}
	var u models.User
	if err := models.DB.Scopes(models.TenantScope(t.ID)).Where("username = ?", username).First(&u).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return LoginResult{}, errors.New("用户名或密码错误")
		}
		return LoginResult{}, err
	}
	if u.Status != 1 {
		return LoginResult{}, errors.New("账号已停用")
	}
	if u.Password == "" || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password)) != nil {
		return LoginResult{}, errors.New("用户名或密码错误")
	}
	models.DB.Model(&models.User{}).Where("id = ?", u.ID).Update("last_login_at", time.Now())
	// 解析该本地用户企微渠道身份（ChannelLink user→wecom staff），供网关签发统一会话 JWT，
	// 使密码登录同样能访问企微渠道功能。无映射（非企微员工）则两字段留空，退回纯本地会话。
	var corpID, staffID string
	var link models.ChannelLink
	if err := models.DB.Where("channel = ? AND entity_type = ? AND entity_id = ?", "wecom", "user", u.ID).
		First(&link).Error; err == nil {
		corpID = link.ExtCorpID
		staffID = link.ExtID
	}
	// 查询角色编码与权限点，供前端做菜单级权限过滤。
	var roleCode string
	var perms []string
	if u.RoleID != "" {
		var r models.CrmRole
		if err := models.DB.Scopes(models.TenantScope(t.ID)).Where("id = ?", u.RoleID).First(&r).Error; err == nil {
			roleCode = r.Code
			perms = r.Perms
		}
	}
	return LoginResult{
		TenantID:     t.ID,
		TenantName:   t.Name,
		UserID:       u.ID,
		Username:     u.Username,
		Name:         u.Name,
		Mobile:       u.Mobile,
		Position:     u.Position,
		DepartmentID: u.DepartmentID,
		RoleID:       u.RoleID,
		RoleCode:     roleCode,
		Perms:        perms,
		Authorities:  crmPermsToAuthorities(roleCode, perms),
		Avatar:       u.Avatar,
		CorpID:       corpID,
		StaffID:      staffID,
	}, nil
}

// SetPassword 管理员重置用户密码（须有本地身份）。
func (s CrmAuthService) SetPassword(ctx context.Context, id, password string) error {
	tenantID := identity.TenantID(ctx)

	if err := validatePassword(password); err != nil {
		return err
	}
	h, err := hashPassword(password)
	if err != nil {
		return err
	}
	res := models.DB.Model(&models.User{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Update("password", h)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("用户不存在")
	}
	return nil
}

// ChangePassword 用户自助改密（须有本地身份）。
func (s CrmAuthService) ChangePassword(ctx context.Context, userID, oldPassword, newPassword string) error {
	tenantID := identity.TenantID(ctx)

	var u models.User
	if err := models.MustFor(ctx).Where("id = ?", userID).First(&u).Error; err != nil {
		return errors.New("账号不存在")
	}
	if u.Password == "" || bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(oldPassword)) != nil {
		return errors.New("原密码错误")
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	h, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return models.DB.Model(&models.User{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", userID).Update("password", h).Error
}

// ---------- CrmCustomer ----------

type CrmCustomerService struct{}

func NewCrmCustomer() *CrmCustomerService { return &CrmCustomerService{} }

func (s CrmCustomerService) Create(ctx context.Context, userID string, req models.CrmCustomer) (models.CrmCustomer, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.OwnerUserID == "" {
		req.OwnerUserID = userID
	}
	// 无归属销售（导入/表单等无 userID 场景）→ 进公海池待认领。
	if req.OwnerUserID == "" {
		req.PoolStatus = 1
		now := time.Now()
		req.PoolAt = &now
	}
	// mobile 去重校验：同租户同手机号已存在则拒绝（空 mobile 不校验）。
	if req.Mobile != "" {
		var cnt int64
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("mobile = ? AND deleted_at IS NULL",  req.Mobile).Count(&cnt)
		if cnt > 0 {
			return req, errors.New("该手机号已存在客户记录，请使用查重合并功能")
		}
	}
	err := createCRM(models.DB, &req, "OwnerUserID")
	if err != nil {
		log.Sugar.Errorw("crm customer create failed", "err", err)
		return req, err
	}
	if err := upsertCustomerChannelLink(ctx, req); err != nil {
		log.Sugar.Errorw("crm customer channel link upsert failed", "err", err)
	}
	return req, nil
}

func (s CrmCustomerService) Update(ctx context.Context, id string, req models.CrmCustomer) (models.CrmCustomer, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id
	req.TenantID = tenantID
	upd := updatesCRM(map[string]any{
		"name": req.Name, "mobile": req.Mobile, "email": req.Email, "company": req.Company,
		"industry": req.Industry, "gender": req.Gender, "birthday": req.Birthday,
		"level": req.Level, "address": req.Address, "owner_user_id": req.OwnerUserID,
		"next_followup_at": req.NextFollowupAt, "remark": req.Remark, "tags": req.Tags,
	}, "owner_user_id")
	selectCols := []string{"name", "mobile", "email", "company", "industry", "gender", "birthday", "level", "address", "owner_user_id", "next_followup_at", "remark", "tags"}
	if req.Source != "" {
		upd["source"] = req.Source
		selectCols = append(selectCols, "source")
	}
	// 渠道快照字段：只写非零值，避免旧调用/编辑表单把渠道快照清空。
	// channel_tags 例外：用户可手工打标/清空，nil=未提交跳过，空数组=清空。
	channelFields := map[string]any{
		"ext_customer_id": req.ExtCustomerID,
		"ext_corp_id":     req.ExtCorpID,
		"add_way":         req.AddWay,
		"account_type":    req.AccountType,
		"out_flow_status": req.OutFlowStatus,
	}
	for col, v := range channelFields {
		if !zeroChannelValue(v) {
			upd[col] = v
			selectCols = append(selectCols, col)
		}
	}
	if req.ChannelTags != nil {
		upd["channel_tags"] = req.ChannelTags
		selectCols = append(selectCols, "channel_tags")
	}
	// owner 变化时同步公海状态：设了 owner → 出公海；owner 清空 → 进公海。
	if req.OwnerUserID != "" {
		upd["pool_status"] = 0
		upd["pool_at"] = nil
		selectCols = append(selectCols, "pool_status", "pool_at")
	}
	err := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Select(selectCols).Updates(upd).Error
	if err != nil {
		log.Sugar.Errorw("crm customer update failed", "err", err)
		return req, err
	}
	if err := upsertCustomerChannelLink(ctx, req); err != nil {
		log.Sugar.Errorw("crm customer channel link upsert failed", "err", err)
	}
	return req, nil
}

// zeroChannelValue 判断渠道快照字段是否为零值（空串/0/空数组），避免覆盖已存快照。
func zeroChannelValue(v any) bool {
	switch t := v.(type) {
	case string:
		return t == ""
	case int:
		return t == 0
	case []string:
		return len(t) == 0
	case constants.StringArrayField:
		return len(t) == 0
	}
	return false
}

// upsertCustomerChannelLink 渠道客户(wecom/xiaohongshu)在创建/更新时同步 ChannelLink 映射。
// 非渠道客户(manual/import/web_form)或无外部身份时不建映射。
func upsertCustomerChannelLink(ctx context.Context, c models.CrmCustomer) error {
	if c.Source != constants.ChannelWecom && c.Source != constants.ChannelXiaohongshu {
		return nil
	}
	if c.ExtCustomerID == "" {
		return nil
	}
	tenantID := identity.TenantID(ctx)
	var link models.ChannelLink
	err := models.DB.Where("channel = ? AND entity_type = ? AND entity_id = ?", c.Source, "customer", c.ID).First(&link).Error
	if err == nil {
		return models.DB.Model(&models.ChannelLink{}).Where("id = ?", link.ID).
			Updates(map[string]any{"ext_id": c.ExtCustomerID, "ext_corp_id": c.ExtCorpID}).Error
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}
	return models.DB.Create(&models.ChannelLink{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		Channel:    c.Source,
		EntityType: "customer",
		EntityID:   c.ID,
		ExtID:      c.ExtCustomerID,
		ExtCorpID:  c.ExtCorpID,
	}).Error
}

func (s CrmCustomerService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmCustomer{})
	return res.RowsAffected, res.Error
}

func (s CrmCustomerService) Get(ctx context.Context, id string) (models.CrmCustomer, error) {

	var c models.CrmCustomer
	err := models.MustFor(ctx).Where("id = ?", id).First(&c).Error
	return c, err
}

// GetDetail 客户 360 详情：客户主档(含企微渠道快照) + 跟进记录 timeline。
func (s CrmCustomerService) GetDetail(ctx context.Context, id string) (map[string]any, error) {

	c, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var follows []models.CrmFollowup
	if err := models.MustFor(ctx).Where("customer_id = ?",  id).
		Order("created_at desc").Find(&follows).Error; err != nil {
		return nil, err
	}
	// 企微互动（消息存档单聊）：经 CrmCustomer 上的渠道快照 ext_customer_id 关联 chat_msg。
	var msgs []models.ChatMsg
	if c.ExtCustomerID != "" && c.ExtCorpID != "" {
		target, _ := json.Marshal([]string{c.ExtCustomerID})
		if err := models.DB.Where(`ext_corp_id = ? AND room_id = '' AND ("from" = ? OR to_list @> ?::jsonb)`,
			c.ExtCorpID, c.ExtCustomerID, string(target)).
			Order("msg_time desc").Limit(20).Find(&msgs).Error; err != nil {
			return nil, err
		}
	}
	// 非企微渠道互动（小红书留资/评论/私信等），企微互动仍走上方 chat_msg。
	var interactions []models.CrmChannelInteraction
	if err := models.MustFor(ctx).Where("customer_id = ? AND channel <> ?",  id,  constants.ChannelWecom).
		Order("happened_at desc").Limit(20).Find(&interactions).Error; err != nil {
		return nil, err
	}
	// 活动参与史（营销活动触达/参与/转化明细）。
	activityRecs, err := NewCrmActivity().ListByCustomer(ctx, id)
	if err != nil {
		return nil, err
	}
	// 转移历史。
	transferLogs, _, _ := NewCrmTransfer().ListTransferLog(ctx, id, &app.Pager{Page: 1, PageSize: 20})
	return map[string]any{
		"customer": c, "followups": follows, "wecom_messages": msgs,
		"channel_interactions": interactions, "activity_records": activityRecs,
		"transfer_logs": transferLogs,
	}, nil
}

func (s CrmCustomerService) Query(ctx context.Context, keyword string, ownerUserID string, source string, outFlowStatus int, poolStatus int, pager *app.Pager) ([]models.CrmCustomer, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	if source != "" {
		db = db.Where("source = ?", source)
	}
	if outFlowStatus != 0 {
		db = db.Where("out_flow_status = ?", outFlowStatus)
	}
	if poolStatus >= 0 {
		db = db.Where("pool_status = ?", poolStatus)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var cs []models.CrmCustomer
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("score desc, id desc").Find(&cs).Error
	return cs, total, err
}

// Export 导出客户列表为 Excel（最多 10000 行）。
func (s CrmCustomerService) Export(ctx context.Context) (*bytes.Buffer, string, error) {

	var cs []models.CrmCustomer
	if err := models.MustFor(ctx).Limit(10000).Order("id desc").Find(&cs).Error; err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := "客户列表"
	f.SetSheetName(f.GetSheetName(0), sheet)
	headers := []string{"ID", "姓名", "手机", "公司", "来源", "归属销售ID", "评分", "公海", "创建时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for i, c := range cs {
		row := i + 2
		f.SetCellValue(sheet, "A"+strconv.Itoa(row), c.ID)
		f.SetCellValue(sheet, "B"+strconv.Itoa(row), c.Name)
		f.SetCellValue(sheet, "C"+strconv.Itoa(row), c.Mobile)
		f.SetCellValue(sheet, "D"+strconv.Itoa(row), c.Company)
		f.SetCellValue(sheet, "E"+strconv.Itoa(row), c.Source)
		f.SetCellValue(sheet, "F"+strconv.Itoa(row), c.OwnerUserID)
		f.SetCellValue(sheet, "G"+strconv.Itoa(row), c.Score)
		f.SetCellValue(sheet, "H"+strconv.Itoa(row), c.PoolStatus)
		f.SetCellValue(sheet, "I"+strconv.Itoa(row), c.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return buf, "customers_" + time.Now().Format("20060102_150405") + ".xlsx", nil
}

// BatchImport 批量导入客户（Excel 行数据）。返回成功数和失败数。
func (s CrmCustomerService) BatchImport(ctx context.Context, userID string, rows []map[string]string) (int, int, error) {
	tenantID := identity.TenantID(ctx)

	success, fail := 0, 0
	for _, r := range rows {
		c := models.CrmCustomer{
			Model:    models.Model{ID: id_generator.StringID()},
			TenantID: tenantID,
			Name:     r["姓名"],
			Mobile:   r["手机"],
			Company:  r["公司"],
			Source:   constants.SourceImport,
		}
		if c.Name == "" && c.Mobile == "" {
			fail++
			continue
		}
		// 去重：mobile 非空时查同租户是否已存在
		if c.Mobile != "" {
			var cnt int64
			models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("mobile = ?",  c.Mobile).Count(&cnt)
			if cnt > 0 {
				fail++
				continue
			}
		}
		if err := createCRM(models.DB, &c, "OwnerUserID"); err != nil {
			log.Sugar.Errorw("batch import customer failed", "err", err, "name", c.Name)
			fail++
			continue
		}
		success++
	}
	return success, fail, nil
}

// ---------- CrmLead ----------

type CrmLeadService struct{}

func NewCrmLead() *CrmLeadService { return &CrmLeadService{} }

func (s CrmLeadService) Create(ctx context.Context, userID string, req models.CrmLead) (models.CrmLead, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.OwnerUserID == "" {
		req.OwnerUserID = userID
	}
	err := createCRM(models.DB, &req, "CustomerID", "OwnerUserID")
	if err != nil {
		log.Sugar.Errorw("crm lead create failed", "err", err)
	}
	return req, err
}

func (s CrmLeadService) Update(ctx context.Context, id string, req models.CrmLead) (models.CrmLead, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id
	req.TenantID = tenantID
	err := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Select("customer_id", "owner_user_id", "title", "company", "name", "mobile", "source", "status", "remark").
		Updates(updatesCRM(map[string]any{
			"customer_id": req.CustomerID, "owner_user_id": req.OwnerUserID, "title": req.Title,
			"company": req.Company, "name": req.Name, "mobile": req.Mobile, "source": req.Source,
			"status": req.Status, "remark": req.Remark,
		}, "customer_id", "owner_user_id")).Error
	if err != nil {
		log.Sugar.Errorw("crm lead update failed", "err", err)
	}
	return req, err
}

func (s CrmLeadService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmLead{})
	return res.RowsAffected, res.Error
}

func (s CrmLeadService) Get(ctx context.Context, id string) (models.CrmLead, error) {

	var l models.CrmLead
	err := models.MustFor(ctx).Where("id = ?", id).First(&l).Error
	return l, err
}

func (s CrmLeadService) Query(ctx context.Context, status string, ownerUserID string, pager *app.Pager) ([]models.CrmLead, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	var total int64
	if err := db.Model(&models.CrmLead{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ls []models.CrmLead
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&ls).Error
	return ls, total, err
}

// Convert 线索→客户转化：据线索创建 CrmCustomer 并回写线索的 customer_id、标记为 qualified。
// 幂等：线索已关联客户则直接返回，不重复创建。
// 若线索带手机号且已有同手机号客户，则直接关联已有客户（去重）。
func (s CrmLeadService) Convert(ctx context.Context, userID, leadID string) (models.CrmCustomer, error) {
	tenantID := identity.TenantID(ctx)

	l, err := s.Get(ctx, leadID)
	if err != nil {
		return models.CrmCustomer{}, err
	}
	if l.CustomerID != "" {
		c, err := NewCrmCustomer().Get(ctx, l.CustomerID)
		return c, err
	}
	// 去重：同手机号已有客户则直接关联
	if l.Mobile != "" {
		var exist models.CrmCustomer
		if err := models.MustFor(ctx).Where("mobile = ?",  l.Mobile).First(&exist).Error; err == nil {
			models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", leadID).
				Updates(map[string]any{"customer_id": exist.ID, "status": "qualified"})
			return exist, nil
		}
	}
	cust := models.CrmCustomer{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		Name:        l.Name,
		Company:     l.Company,
		Mobile:      l.Mobile,
		Source:      l.Source,
		OwnerUserID: userID,
		Level:       1,
	}
	if err := createCRM(models.DB, &cust, "OwnerUserID", "ReassignFrom"); err != nil {
		log.Sugar.Errorw("crm lead convert: create customer failed", "err", err)
		return models.CrmCustomer{}, err
	}
	if err := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", leadID).
		Updates(map[string]any{"customer_id": cust.ID, "status": "qualified"}).Error; err != nil {
		return cust, err
	}
	return cust, nil
}

// ---------- CrmPipelineStage ----------

type CrmPipelineStageService struct{}

func NewCrmPipelineStage() *CrmPipelineStageService { return &CrmPipelineStageService{} }

func (s CrmPipelineStageService) Create(ctx context.Context, req models.CrmPipelineStage) (models.CrmPipelineStage, error) {

	req.ID = id_generator.StringID()
	req.TenantID = identity.TenantID(ctx)
	err := models.MustFor(ctx).Create(&req).Error
	if err != nil {
		log.Sugar.Errorw("crm stage create failed", "err", err)
	}
	return req, err
}

func (s CrmPipelineStageService) Update(ctx context.Context, id string, req models.CrmPipelineStage) (models.CrmPipelineStage, error) {

	req.ID = id
	req.TenantID = identity.TenantID(ctx)
	err := models.MustFor(ctx).Model(&models.CrmPipelineStage{}).Where("id = ?", id).
		Select("name", "sort", "probability", "is_won", "is_lost").
		Updates(map[string]any{
			"name": req.Name, "sort": req.Sort, "probability": req.Probability,
			"is_won": req.IsWon, "is_lost": req.IsLost,
		}).Error
	if err != nil {
		log.Sugar.Errorw("crm stage update failed", "err", err)
	}
	return req, err
}

func (s CrmPipelineStageService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmPipelineStage{})
	return res.RowsAffected, res.Error
}

func (s CrmPipelineStageService) List(ctx context.Context) ([]models.CrmPipelineStage, error) {

	var ss []models.CrmPipelineStage
	err := models.MustFor(ctx).Order("sort").Find(&ss).Error
	return ss, err
}

// ---------- CrmOpportunity ----------

type CrmOpportunityService struct{}

func NewCrmOpportunity() *CrmOpportunityService { return &CrmOpportunityService{} }

func (s CrmOpportunityService) Create(ctx context.Context, userID string, req models.CrmOpportunity) (models.CrmOpportunity, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.OwnerUserID == "" {
		req.OwnerUserID = userID
	}
	// stage 是状态的唯一真相：创建时若指定阶段，按目标阶段终态推导 status。
	if req.StageID != "" {
		var stage models.CrmPipelineStage
		if err := models.MustFor(ctx).Where("id = ?", req.StageID).First(&stage).Error; err == nil {
			switch {
			case stage.IsWon:
				req.Status = "won"
			case stage.IsLost:
				req.Status = "lost"
			default:
				req.Status = "open"
			}
		}
	}
	if req.Status == "" {
		req.Status = "open"
	}
	err := createCRM(models.DB, &req, "CustomerID", "LeadID", "OwnerUserID", "StageID")
	if err != nil {
		log.Sugar.Errorw("crm opportunity create failed", "err", err)
	}
	return req, err
}

func (s CrmOpportunityService) Update(ctx context.Context, id string, req models.CrmOpportunity) (models.CrmOpportunity, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id
	req.TenantID = tenantID
	// stage 是状态的唯一真相：改 stage 时按目标阶段的终态推导 status，避免与 MoveStage 不一致。
	if req.StageID != "" {
		var stage models.CrmPipelineStage
		if err := models.MustFor(ctx).Where("id = ?", req.StageID).First(&stage).Error; err == nil {
			switch {
			case stage.IsWon:
				req.Status = "won"
			case stage.IsLost:
				req.Status = "lost"
			default:
				req.Status = "open"
			}
		}
	}
	err := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Select("customer_id", "lead_id", "owner_user_id", "stage_id", "title", "product", "amount", "expected_close_date", "status", "remark").
		Updates(updatesCRM(map[string]any{
			"customer_id": req.CustomerID, "lead_id": req.LeadID, "owner_user_id": req.OwnerUserID,
			"stage_id": req.StageID, "title": req.Title, "product": req.Product, "amount": req.Amount,
			"expected_close_date": req.ExpectedCloseDate, "status": req.Status, "remark": req.Remark,
		}, "customer_id", "lead_id", "owner_user_id", "stage_id")).Error
	if err != nil {
		log.Sugar.Errorw("crm opportunity update failed", "err", err)
	}
	return req, err
}

func (s CrmOpportunityService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmOpportunity{})
	return res.RowsAffected, res.Error
}

func (s CrmOpportunityService) Get(ctx context.Context, id string) (models.CrmOpportunity, error) {

	var o models.CrmOpportunity
	err := models.MustFor(ctx).Where("id = ?", id).First(&o).Error
	return o, err
}

func (s CrmOpportunityService) Query(ctx context.Context, status string, stageID string, ownerUserID string, pager *app.Pager) ([]models.CrmOpportunity, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if stageID != "" {
		db = db.Where("stage_id = ?", stageID)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	var total int64
	if err := db.Model(&models.CrmOpportunity{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var os []models.CrmOpportunity
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&os).Error
	return os, total, err
}

// MoveStage 移动商机阶段；到达赢单/输单终态时同步 status。
// 若租户开启了跟进强制校验，需近 N 天有跟进记录方可移动。
func (s CrmOpportunityService) MoveStage(ctx context.Context, id, stageID string) (models.CrmOpportunity, error) {
	tenantID := identity.TenantID(ctx)

	var o models.CrmOpportunity
	if err := models.MustFor(ctx).Where("id = ?", id).First(&o).Error; err != nil {
		return o, err
	}
	var stage models.CrmPipelineStage
	if err := models.MustFor(ctx).Where("id = ?", stageID).First(&stage).Error; err != nil {
		return o, err
	}
	// 阶段流转校验：移动到非终态阶段时校验跟进（赢单/输单终态不校验，允许直接关闭）。
	if !stage.IsWon && !stage.IsLost {
		if err := checkMoveStageGuard(ctx, id); err != nil {
			return o, err
		}
	}
	updates := map[string]any{"stage_id": stageID}
	switch {
	case stage.IsWon:
		updates["status"] = "won"
	case stage.IsLost:
		updates["status"] = "lost"
	default:
		updates["status"] = "open"
	}
	err := models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(updates).Error
	o.StageID = stageID
	o.Status, _ = updates["status"].(string)
	// Phase 4：阶段变化触发 SOP 自动化
	if err == nil {
		NewCrmSop().Trigger(ctx, "stage_change", map[string]string{
			"customer_id":    o.CustomerID,
			"opportunity_id": o.ID,
			"stage_id":       stageID,
		})
	}
	return o, err
}

// ---------- CrmFollowup ----------

type CrmFollowupService struct{}

func NewCrmFollowup() *CrmFollowupService { return &CrmFollowupService{} }

func (s CrmFollowupService) Create(ctx context.Context, userID string, req models.CrmFollowup) (models.CrmFollowup, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.UserID == "" {
		req.UserID = userID
	}
	err := createCRM(models.DB, &req, "CustomerID", "OpportunityID", "UserID")
	if err != nil {
		log.Sugar.Errorw("crm followup create failed", "err", err)
	}
	return req, err
}

func (s CrmFollowupService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmFollowup{})
	return res.RowsAffected, res.Error
}

func (s CrmFollowupService) ListByCustomer(ctx context.Context, customerID string) ([]models.CrmFollowup, error) {

	var fs []models.CrmFollowup
	err := models.MustFor(ctx).Where("customer_id = ?",  customerID).
		Order("created_at desc").Find(&fs).Error
	return fs, err
}

func (s CrmFollowupService) ListByOpportunity(ctx context.Context, opportunityID string) ([]models.CrmFollowup, error) {

	var fs []models.CrmFollowup
	err := models.MustFor(ctx).Where("opportunity_id = ?",  opportunityID).
		Order("created_at desc").Find(&fs).Error
	return fs, err
}

// FollowupReminder 跟进提醒项：客户 + 到期时间 + 逾期天数。
type FollowupReminder struct {
	CustomerID     string     `json:"customer_id"`
	CustomerName   string     `json:"customer_name"`
	NextFollowupAt *time.Time `json:"next_followup_at"`
	OverdueDays    int        `json:"overdue_days"`
}

// ListReminders 今日待跟进：owner=userID 且 next_followup_at <= 今天结束 且非公海。
func (s CrmFollowupService) ListReminders(ctx context.Context, userID string) ([]FollowupReminder, error) {

	now := time.Now()
	endOfToday := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
	return s.queryReminders(ctx, userID, "next_followup_at <= ?", endOfToday)
}

// ListOverdue 已逾期跟进：owner=userID 且 next_followup_at < 今天开始 且非公海。
func (s CrmFollowupService) ListOverdue(ctx context.Context, userID string) ([]FollowupReminder, error) {

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return s.queryReminders(ctx, userID, "next_followup_at < ?", startOfToday)
}

func (s CrmFollowupService) queryReminders(ctx context.Context, userID, cond string, bound time.Time) ([]FollowupReminder, error) {

	var cs []models.CrmCustomer
	if err := models.MustFor(ctx).Where("owner_user_id = ? AND pool_status = ? AND next_followup_at IS NOT NULL AND "+cond,
		userID, 0, bound).
		Order("next_followup_at asc").Limit(50).Find(&cs).Error; err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]FollowupReminder, 0, len(cs))
	for _, c := range cs {
		out = append(out, FollowupReminder{
			CustomerID:     c.ID,
			CustomerName:   c.Name,
			NextFollowupAt: c.NextFollowupAt,
			OverdueDays:    calcOverdueDays(c.NextFollowupAt, now),
		})
	}
	return out, nil
}

// calcOverdueDays 计算逾期天数（next_followup_at 早于 now 的天数，未逾期返回 0）。纯函数，供单测。
func calcOverdueDays(nextFollowupAt *time.Time, now time.Time) int {
	if nextFollowupAt == nil {
		return 0
	}
	if nextFollowupAt.After(now) {
		return 0
	}
	return int(now.Sub(*nextFollowupAt).Hours() / 24)
}

// ---------- CrmOrder ----------

type CrmOrderService struct{}

func NewCrmOrder() *CrmOrderService { return &CrmOrderService{} }

func (s CrmOrderService) Create(ctx context.Context, userID string, req models.CrmOrder) (models.CrmOrder, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.UserID == "" {
		req.UserID = userID
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	if req.PayStatus == "" {
		req.PayStatus = "unpaid"
	}
	if req.OrderNo == "" {
		req.OrderNo = "SO" + time.Now().Format("20060102150405") + req.ID[len(req.ID)-6:]
	}
	// 优惠券联动：传入 coupon_code 则核销券 + 计算折扣金额
	if req.CouponCode != "" {
		rec, err := NewCrmCoupon().Verify(ctx, req.CouponCode, "")
		if err != nil {
			return req, fmt.Errorf("优惠券核销失败: %v", err)
		}
		req.CouponID = rec.CouponID
		// 查券模板计算折扣
		var coupon models.CrmCoupon
		if err := models.DB.Where("id = ?", rec.CouponID).First(&coupon).Error; err == nil {
			switch coupon.CouponType {
			case "discount":
				req.CouponAmount = req.Amount * (1 - coupon.FaceValue)
			case "cash":
				if req.Amount >= coupon.Threshold {
					req.CouponAmount = coupon.FaceValue
				}
			case "gift":
				req.CouponAmount = 0 // 兑换券不抵扣金额
			}
		}
	}
	err := createCRM(models.DB, &req, "CustomerID", "OpportunityID", "UserID")
	if err != nil {
		log.Sugar.Errorw("crm order create failed", "err", err)
	}
	return req, err
}

func (s CrmOrderService) Update(ctx context.Context, id string, req models.CrmOrder) (models.CrmOrder, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id
	req.TenantID = tenantID
	err := models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Select("customer_id", "opportunity_id", "user_id", "title", "amount", "paid_amount", "status", "pay_status", "signed_at", "remark").
		Updates(updatesCRM(map[string]any{
			"customer_id": req.CustomerID, "opportunity_id": req.OpportunityID, "user_id": req.UserID,
			"title": req.Title, "amount": req.Amount, "paid_amount": req.PaidAmount,
			"status": req.Status, "pay_status": req.PayStatus,
			"signed_at": req.SignedAt, "remark": req.Remark,
		}, "customer_id", "opportunity_id", "user_id")).Error
	if err != nil {
		log.Sugar.Errorw("crm order update failed", "err", err)
	}
	return req, err
}

func (s CrmOrderService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmOrder{})
	return res.RowsAffected, res.Error
}

func (s CrmOrderService) Query(ctx context.Context, status string, pager *app.Pager) ([]models.CrmOrder, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmOrder{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var os []models.CrmOrder
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&os).Error
	return os, total, err
}

// ---------- CrmReport ----------

type CrmReportService struct{}

func NewCrmReport() *CrmReportService { return &CrmReportService{} }

// SalesSummary 销售汇总：客户数、线索数、open 商机数、签约订单金额。
type SalesSummary struct {
	CustomerCount   int64   `json:"customer_count"`
	LeadCount       int64   `json:"lead_count"`
	OpenOpportunity int64   `json:"open_opportunity"`
	WonOpportunity  int64   `json:"won_opportunity"`
	OrderAmount     float64 `json:"order_amount"`
	OrderCount      int64   `json:"order_count"`
}

func (s CrmReportService) SalesSummary(ctx context.Context) (SalesSummary, error) {
	tenantID := identity.TenantID(ctx)

	var sum SalesSummary
	count := func(m any) int64 {
		var n int64
		models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Count(&n)
		return n
	}
	sum.CustomerCount = count(&models.CrmCustomer{})
	sum.LeadCount = count(&models.CrmLead{})
	sum.OpenOpportunity = func() int64 {
		var n int64
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("status = ?",  "open").Count(&n)
		return n
	}()
	sum.WonOpportunity = func() int64 {
		var n int64
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("status = ?",  "won").Count(&n)
		return n
	}()
	sum.OrderCount = count(&models.CrmOrder{})
	var amt struct {
		Total float64 `gorm:"column:total"`
	}
	err := models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("status IN ?",  []string{"signed", "completed"}).
		Select("COALESCE(SUM(amount),0) AS total").Scan(&amt).Error
	if err != nil {
		return sum, err
	}
	sum.OrderAmount = amt.Total
	return sum, nil
}

// PipelineFunnel 销售漏斗：open 商机按阶段聚合金额/数量。
func (s CrmReportService) PipelineFunnel(ctx context.Context) ([]map[string]any, error) {

	stages, err := NewCrmPipelineStage().List(ctx)
	if err != nil {
		return nil, err
	}
	type row struct {
		StageID string  `gorm:"column:stage_id"`
		Cnt     int64   `gorm:"column:cnt"`
		Amount  float64 `gorm:"column:amount"`
	}
	var rows []row
	if err := models.MustFor(ctx).Model(&models.CrmOpportunity{}).Where("status = ?", "open").
		Select("stage_id, COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS amount").
		Group("stage_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	byStage := map[string]row{}
	for _, r := range rows {
		byStage[r.StageID] = r
	}
	out := make([]map[string]any, 0, len(stages))
	for _, st := range stages {
		r := byStage[st.ID]
		out = append(out, map[string]any{
			"stage_id": st.ID, "stage_name": st.Name, "sort": st.Sort,
			"count": r.Cnt, "amount": r.Amount,
		})
	}
	return out, nil
}

// ChannelCustomerCount 渠道客户构成（按 source 分组）。
type ChannelCustomerCount struct {
	Channel string `json:"channel"`
	Count   int64  `json:"count"`
}

// ChannelSummary 渠道概览：供统一工作台展示渠道资源规模与客户构成。
// 企微运营资源按 ext_corp_id 分区，先经 ChannelLink(tenant,wecom) 解析本租户对应的 corp_id；
// 客户构成按 source 分组（wecom/xiaohongshu/manual/...），不再硬编码单渠道。
func (s CrmReportService) ChannelSummary(ctx context.Context) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)

	var corpID string
	var link models.ChannelLink
	if err := models.MustFor(ctx).Where("channel = ? AND entity_type = ?", constants.ChannelWecom,  "tenant").
		First(&link).Error; err == nil {
		corpID = link.ExtID
	}
	countByCorp := func(m any) int64 {
		var n int64
		if corpID != "" {
			models.DB.Model(m).Where("ext_corp_id = ?", corpID).Count(&n)
		}
		return n
	}
	out := map[string]any{
		"wecom_customer_count":    0,
		"contact_way_count":       0,
		"mass_msg_count":          0,
		"group_chat_count":        0,
		"welcome_msg_count":       0,
		"material_count":          0,
		"channel_customer_counts": []ChannelCustomerCount{},
	}
	type row struct {
		Source string `gorm:"column:source"`
		Cnt    int64  `gorm:"column:cnt"`
	}
	var rows []row
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source <> ''", ).
		Select("source, COUNT(*) AS cnt").Group("source").Scan(&rows)
	counts := make([]ChannelCustomerCount, 0, len(rows))
	for _, r := range rows {
		counts = append(counts, ChannelCustomerCount{Channel: r.Source, Count: r.Cnt})
		if r.Source == constants.ChannelWecom {
			out["wecom_customer_count"] = r.Cnt
		}
	}
	out["channel_customer_counts"] = counts
	out["contact_way_count"] = countByCorp(&models.ContactWay{})
	out["mass_msg_count"] = countByCorp(&models.MassMsg{})
	out["group_chat_count"] = countByCorp(&models.GroupChat{})
	out["welcome_msg_count"] = countByCorp(&models.WelcomeMsg{})
	out["material_count"] = countByCorp(&models.Material{})
	return out, nil
}

// ChannelCompareRow 渠道对比看板行。
type ChannelCompareRow struct {
	Channel          string  `json:"channel"`
	CustomerCount    int64   `json:"customer_count"`
	LeadCount        int64   `json:"lead_count"`
	OpportunityCount int64   `json:"opportunity_count"`
	WonCount         int64   `json:"won_count"`
	OrderAmount      float64 `json:"order_amount"`
	InteractionCount int64   `json:"interaction_count"`
	ConversionRate   float64 `json:"conversion_rate"`  // 转化率(%) = won/opportunity
	TotalCost        float64 `json:"total_cost"`       // 渠道活动总成本
	AcquisitionCost  float64 `json:"acquisition_cost"` // 获客成本 = 总成本/新客户数
}

// ChannelCompare 渠道对比看板：各渠道客户数/线索/商机/赢单/订单金额/互动数/转化率/获客成本。
func (s CrmReportService) ChannelCompare(ctx context.Context) ([]ChannelCompareRow, error) {
	tenantID := identity.TenantID(ctx)

	channels := []string{constants.ChannelWecom, constants.ChannelXiaohongshu, constants.ChannelGzh, constants.ChannelDouyin, "manual", "import", "web_form"}
	rows := make([]ChannelCompareRow, 0, len(channels))
	for _, ch := range channels {
		row := ChannelCompareRow{Channel: ch}
		// 客户数
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source = ?",  ch).Count(&row.CustomerCount)
		// 线索数（CrmLead.Source）
		models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("source = ?",  ch).Count(&row.LeadCount)
		// 商机数：通过客户 source 关联
		models.DB.Model(&models.CrmOpportunity{}).
			Joins("JOIN crm_customer ON crm_customer.id = crm_opportunity.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_customer.source = ?",  ch).
			Count(&row.OpportunityCount)
		// 赢单数
		models.DB.Model(&models.CrmOpportunity{}).
			Joins("JOIN crm_customer ON crm_customer.id = crm_opportunity.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_customer.source = ? AND crm_opportunity.status = ?",  ch,  "won").
			Count(&row.WonCount)
		// 订单金额
		var amt struct {
			Total float64 `gorm:"column:total"`
		}
		models.DB.Model(&models.CrmOrder{}).
			Joins("JOIN crm_customer ON crm_customer.id = crm_order.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_customer.source = ? AND crm_order.status IN ?",  ch,  []string{"signed", "completed"}).
			Select("COALESCE(SUM(crm_order.amount),0) AS total").Scan(&amt)
		row.OrderAmount = amt.Total
		// 互动数
		models.DB.Model(&models.CrmChannelInteraction{}).Scopes(models.TenantScope(tenantID)).Where("channel = ?",  ch).Count(&row.InteractionCount)
		// 转化率
		row.ConversionRate = calcChannelConversionRate(row.OpportunityCount, row.WonCount)
		// 获客成本：渠道活动总成本 / 新客户数
		var costAmt struct {
			Total float64 `gorm:"column:total"`
		}
		models.DB.Model(&models.CrmActivity{}).Scopes(models.TenantScope(tenantID)).Where("(channel = ? OR channels @> ?)",  ch,  `["`+ch+`"]`).
			Select("COALESCE(SUM(cost),0) AS total").Scan(&costAmt)
		row.TotalCost = costAmt.Total
		if row.CustomerCount > 0 && row.TotalCost > 0 {
			row.AcquisitionCost = row.TotalCost / float64(row.CustomerCount)
		}
		if row.CustomerCount > 0 || row.LeadCount > 0 || row.OpportunityCount > 0 || row.InteractionCount > 0 {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

// calcChannelConversionRate 渠道转化率(%) = won/opportunity*100。纯函数，供单测。
func calcChannelConversionRate(opportunityCount, wonCount int64) float64 {
	if opportunityCount <= 0 {
		return 0
	}
	return float64(wonCount) / float64(opportunityCount) * 100
}

// ActivityFunnelRow 活动效果漏斗行。
type ActivityFunnelRow struct {
	ActivityID        string  `json:"activity_id"`
	ActivityName      string  `json:"activity_name"`
	ActivityType      string  `json:"activity_type"`
	Channel           string  `json:"channel"`
	ReachCount        int     `json:"reach_count"`
	ParticipateCount  int     `json:"participate_count"`
	ConversionCount   int     `json:"conversion_count"`
	Cost              float64 `json:"cost"`
	ReachRate         float64 `json:"reach_rate"`          // 触达→参与转化率(%)
	ConversionRate    float64 `json:"conversion_rate"`     // 参与→转化转化率(%)
	CostPerConversion float64 `json:"cost_per_conversion"` // 单次转化成本
}

// ActivityFunnel 活动效果漏斗报表：按活动维度统计触达/参与/转化 + ROI。
func (s CrmReportService) ActivityFunnel(ctx context.Context) ([]ActivityFunnelRow, error) {

	var acts []models.CrmActivity
	if err := models.MustFor(ctx).Find(&acts).Error; err != nil {
		return nil, err
	}
	rows := make([]ActivityFunnelRow, 0, len(acts))
	for _, a := range acts {
		row := ActivityFunnelRow{
			ActivityID:       a.ID,
			ActivityName:     a.Name,
			ActivityType:     a.ActivityType,
			Channel:          a.Channel,
			ReachCount:       a.ReachCount,
			ParticipateCount: a.ParticipateCount,
			ConversionCount:  a.ConversionCount,
			Cost:             a.Cost,
		}
		// 触达→参与转化率
		if a.ReachCount > 0 {
			row.ReachRate = float64(a.ParticipateCount) / float64(a.ReachCount) * 100
		}
		// 参与→转化转化率
		if a.ParticipateCount > 0 {
			row.ConversionRate = float64(a.ConversionCount) / float64(a.ParticipateCount) * 100
		}
		// 单次转化成本
		if a.ConversionCount > 0 && a.Cost > 0 {
			row.CostPerConversion = a.Cost / float64(a.ConversionCount)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ---- Phase 6 报表扩展 ----

// SalesRankingRow 销售排行行。
type SalesRankingRow struct {
	UserID          string  `json:"user_id"`
	UserName        string  `json:"user_name"`
	CustomerCount   int     `json:"customer_count"`
	OpportunityCnt  int     `json:"opportunity_count"`
	WonCount        int     `json:"won_count"`
	OrderAmount     float64 `json:"order_amount"`
	QuotaAmount     float64 `json:"quota_amount"`
	AchievementRate float64 `json:"achievement_rate"` // 达成率(%)
}

// SalesRanking 销售业绩排行：按销售统计客户数/商机数/赢单数/订单金额/达成率。
func (s CrmReportService) SalesRanking(ctx context.Context, yearMonth string) ([]SalesRankingRow, error) {
	tenantID := identity.TenantID(ctx)

	// 查所有 CRM 用户（销售）
	var users []models.User
	if err := models.MustFor(ctx).Find(&users).Error; err != nil {
		return nil, err
	}
	rows := make([]SalesRankingRow, 0, len(users))
	for _, u := range users {
		row := SalesRankingRow{UserID: u.ID, UserName: u.Name}
		// 客户数
		var custCnt int64
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ?",  u.ID).Count(&custCnt)
		row.CustomerCount = int(custCnt)
		// 商机数
		var oppCnt int64
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ?",  u.ID).Count(&oppCnt)
		row.OpportunityCnt = int(oppCnt)
		// 赢单数
		var wonCnt int64
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ? AND status = ?",  u.ID,  "won").Count(&wonCnt)
		row.WonCount = int(wonCnt)
		// 订单金额（按签约销售）
		models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND status <> ?",  u.ID,  "cancelled").
			Select("COALESCE(SUM(amount), 0)").Scan(&row.OrderAmount)
		// 业绩目标
		if yearMonth != "" {
			var q models.CrmQuota
			if err := models.MustFor(ctx).Where("user_id = ? AND year_month = ?",  u.ID,  yearMonth).First(&q).Error; err == nil {
				row.QuotaAmount = q.TargetAmount
				if q.TargetAmount > 0 {
					row.AchievementRate = row.OrderAmount / q.TargetAmount * 100
				}
			}
		}
		rows = append(rows, row)
	}
	// 按订单金额降序
	sort.Slice(rows, func(i, j int) bool { return rows[i].OrderAmount > rows[j].OrderAmount })
	return rows, nil
}

// LifecycleRow 客户生命周期分布行。
type LifecycleRow struct {
	Stage string  `json:"stage"` // new/active/sleeping/churned
	Label string  `json:"label"`
	Count int     `json:"count"`
	Ratio float64 `json:"ratio"` // 占比(%)
}

// CustomerLifecycle 客户生命周期分析：按最后互动时间分类。
func (s CrmReportService) CustomerLifecycle(ctx context.Context) ([]LifecycleRow, error) {
	tenantID := identity.TenantID(ctx)

	var total int64
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL", ).Count(&total)
	now := time.Now()
	d30 := now.AddDate(0, 0, -30)
	d90 := now.AddDate(0, 0, -90)
	d180 := now.AddDate(0, 0, -180)
	// 用 updated_at 近似最后互动时间
	var newCnt, activeCnt, sleepingCnt, churnedCnt int64
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at >= ?",  d30).Count(&newCnt)
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at >= ? AND updated_at < ?",  d90,  d30).Count(&activeCnt)
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at >= ? AND updated_at < ?",  d180,  d90).Count(&sleepingCnt)
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at < ?",  d180).Count(&churnedCnt)
	rows := []LifecycleRow{
		{Stage: "new", Label: "新客(30天内)", Count: int(newCnt)},
		{Stage: "active", Label: "活跃(30-90天)", Count: int(activeCnt)},
		{Stage: "sleeping", Label: "沉睡(90-180天)", Count: int(sleepingCnt)},
		{Stage: "churned", Label: "流失(180天+)", Count: int(churnedCnt)},
	}
	if total > 0 {
		for i := range rows {
			rows[i].Ratio = float64(rows[i].Count) / float64(total) * 100
		}
	}
	return rows, nil
}

// ChannelCACRow 渠道获客成本行。
type ChannelCACRow struct {
	Channel        string  `json:"channel"`
	ChannelLabel   string  `json:"channel_label"`
	TotalCost      float64 `json:"total_cost"`
	NewCustomerCnt int     `json:"new_customer_count"`
	CAC            float64 `json:"cac"` // 获客成本 = 总成本/新客户数
}

// ChannelCAC 渠道获客成本：按渠道汇总活动成本/新客户数，计算 CAC。
func (s CrmReportService) ChannelCAC(ctx context.Context) ([]ChannelCACRow, error) {
	tenantID := identity.TenantID(ctx)

	channels := []string{constants.ChannelWecom, constants.ChannelXiaohongshu, constants.ChannelGzh, constants.ChannelDouyin, "general"}
	rows := make([]ChannelCACRow, 0, len(channels))
	for _, ch := range channels {
		row := ChannelCACRow{Channel: ch, ChannelLabel: constants.ChannelLabel(ch)}
		// 该渠道活动总成本
		models.DB.Model(&models.CrmActivity{}).Scopes(models.TenantScope(tenantID)).Where("(channel = ? OR channels @> ?)",  ch,  `["`+ch+`"]`).
			Select("COALESCE(SUM(cost), 0)").Scan(&row.TotalCost)
		// 该渠道新客户数
		var newCnt int64
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source = ?",  ch).Count(&newCnt)
		row.NewCustomerCnt = int(newCnt)
		if row.NewCustomerCnt > 0 {
			row.CAC = row.TotalCost / float64(row.NewCustomerCnt)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ---------- CrmActivity ----------

type CrmActivityService struct{}

func NewCrmActivity() *CrmActivityService { return &CrmActivityService{} }

func (s CrmActivityService) Create(ctx context.Context, userID string, req models.CrmActivity) (models.CrmActivity, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.OwnerUserID == "" {
		req.OwnerUserID = userID
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	normalizeActivityChannels(&req)
	err := createCRM(models.DB, &req, "OwnerUserID")
	if err != nil {
		log.Sugar.Errorw("crm activity create failed", "err", err)
	}
	return req, err
}

// normalizeActivityChannels 归一化投放渠道：Channel 为主渠道，Channels 为多选，互相补齐。
func normalizeActivityChannels(req *models.CrmActivity) {
	if len(req.Channels) == 0 && req.Channel != "" {
		req.Channels = constants.StringArrayField{req.Channel}
	}
	if len(req.Channels) > 0 && req.Channel == "" {
		req.Channel = req.Channels[0]
	}
}

func (s CrmActivityService) Update(ctx context.Context, id string, req models.CrmActivity) (models.CrmActivity, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id
	req.TenantID = tenantID
	normalizeActivityChannels(&req)
	// status 由 SetStatus 专管（上下架有向流转），Update 不改状态。
	upd := map[string]any{
		"name": req.Name, "activity_type": req.ActivityType,
		"channel": req.Channel, "channels": req.Channels, "start_at": req.StartAt, "end_at": req.EndAt,
		"cost": req.Cost, "reward_desc": req.RewardDesc, "remark": req.Remark,
	}
	// 计数：有参与明细时由明细派生（忽略手工值，避免手填失真）；无明细才允许手工维护（兼容存量）。
	var recCnt int64
	models.DB.Model(&models.CrmActivityRecord{}).Scopes(models.TenantScope(tenantID)).Where("activity_id = ?",  id).Count(&recCnt)
	if recCnt == 0 {
		upd["reach_count"] = req.ReachCount
		upd["participate_count"] = req.ParticipateCount
		upd["conversion_count"] = req.ConversionCount
	}
	err := models.DB.Model(&models.CrmActivity{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(upd).Error
	if err != nil {
		log.Sugar.Errorw("crm activity update failed", "err", err)
	}
	return req, err
}

// SetStatus 切换活动状态（有向流转）：
//
//	draft → active/ended；active → off/ended；off → active；ended 为终态。
func (s CrmActivityService) SetStatus(ctx context.Context, id, status string) error {

	var a models.CrmActivity
	if err := models.MustFor(ctx).Where("id = ?", id).First(&a).Error; err != nil {
		return err
	}
	transitions := map[string]map[string]bool{
		"draft":  {"active": true, "ended": true},
		"active": {"off": true, "ended": true},
		"off":    {"active": true},
		"ended":  {},
	}
	if !transitions[a.Status][status] {
		return errors.New("非法的状态流转：" + a.Status + " → " + status)
	}
	return models.DB.Model(&models.CrmActivity{}).Where("id = ?", id).Update("status", status).Error
}

func (s CrmActivityService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmActivity{})
	return res.RowsAffected, res.Error
}

func (s CrmActivityService) Get(ctx context.Context, id string) (models.CrmActivity, error) {

	var a models.CrmActivity
	err := models.MustFor(ctx).Where("id = ?", id).First(&a).Error
	return a, err
}

func (s CrmActivityService) Query(ctx context.Context, keyword string, activityType string, status string, ownerUserID string, channel string, pager *app.Pager) ([]models.CrmActivity, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ?", like)
	}
	if activityType != "" {
		db = db.Where("activity_type = ?", activityType)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	// 渠道筛选：匹配多选 channels 数组，或旧数据仅设了主渠道 channel 的单值。
	if channel != "" {
		db = db.Where(`(channels @> ?::jsonb OR channel = ?)`, `["`+channel+`"]`, channel)
	}
	var total int64
	if err := db.Model(&models.CrmActivity{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmActivity
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- CrmChannelTag ----------

type CrmChannelTagService struct{}

func NewCrmChannelTag() *CrmChannelTagService { return &CrmChannelTagService{} }

// List 按渠道返回标签目录（channel 为空则返回全部渠道）。
func (s CrmChannelTagService) List(ctx context.Context, channel string) ([]models.CrmChannelTag, error) {

	db := models.MustFor(ctx)
	if channel != "" {
		db = db.Where("channel = ?", channel)
	}
	var tags []models.CrmChannelTag
	err := db.Order("channel asc, sort asc, id asc").Find(&tags).Error
	return tags, err
}

// Create 创建渠道标签。
func (s CrmChannelTagService) Create(ctx context.Context, req models.CrmChannelTag) (models.CrmChannelTag, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Channel == "" {
		return req, errors.New("channel 不能为空")
	}
	if req.Name == "" {
		return req, errors.New("name 不能为空")
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

// Delete 删除渠道标签。
func (s CrmChannelTagService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmChannelTag{})
	return res.RowsAffected, res.Error
}

// ---------- CrmActivityRecord（活动↔客户参与明细） ----------

// recordRank 参与档位：reached(1) < participated(2) < converted(3)。参与/转化客户必被触达。
func recordRank(t string) int {
	switch t {
	case "converted":
		return 3
	case "participated":
		return 2
	default:
		return 1
	}
}

// normalizeRecordType 归一化参与档位，非法值默认 reached。
func normalizeRecordType(t string) string {
	switch t {
	case "reached", "participated", "converted":
		return t
	default:
		return "reached"
	}
}

// ActivityRecordRow 活动参与明细 + 客户名（列表展示）。
type ActivityRecordRow struct {
	models.CrmActivityRecord
	CustomerName string `json:"customer_name"`
}

// CustomerActivityRow 客户活动史：参与明细 + 活动名/类型/状态（客户360展示）。
type CustomerActivityRow struct {
	models.CrmActivityRecord
	ActivityName   string `json:"activity_name"`
	ActivityType   string `json:"activity_type"`
	ActivityStatus string `json:"activity_status"`
}

// Record 录入活动参与明细（触达/参与/转化）。
// 每 (activity, customer) 仅保留一行，UPSERT 原子提档（不降级）：已存在且新档位更高则升级，
// 档位更低或持平则仅刷新通道/时间/备注。写完后同事务重算活动三计数。
func (s CrmActivityService) Record(ctx context.Context, userID string, req models.CrmActivityRecord) (models.CrmActivityRecord, error) {
	tenantID := identity.TenantID(ctx)

	if req.ActivityID == "" || req.CustomerID == "" {
		return req, errors.New("activity_id/customer_id 不能为空")
	}
	var act models.CrmActivity
	if err := models.MustFor(ctx).Where("id = ?", req.ActivityID).First(&act).Error; err != nil {
		return req, errors.New("活动不存在")
	}
	var c models.CrmCustomer
	if err := models.MustFor(ctx).Where("id = ?", req.CustomerID).First(&c).Error; err != nil {
		return req, errors.New("客户不存在")
	}
	req.TenantID = tenantID
	req.RecordType = normalizeRecordType(req.RecordType)
	req.OperatorUserID = userID
	if req.Channel == "" {
		req.Channel = act.Channel
	}
	if req.HappenedAt == nil {
		now := time.Now()
		req.HappenedAt = &now
	}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if req.ID == "" {
			req.ID = id_generator.StringID()
		}
		// UPSERT：存在则仅当新档位更高时升级 record_type，通道/时间/备注非空即刷新。
		return tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "tenant_id"}, {Name: "activity_id"}, {Name: "customer_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"record_type": gorm.Expr(
					"CASE WHEN crm_activity_record.record_type IN ('converted','participated') OR ? IN ('converted','participated') THEN "+
						"CASE WHEN crm_activity_record.record_type = 'converted' OR ? = 'converted' THEN 'converted' ELSE 'participated' END "+
						"ELSE crm_activity_record.record_type END",
					req.RecordType, req.RecordType),
				"channel":          gorm.Expr("CASE WHEN ? <> '' THEN ? ELSE crm_activity_record.channel END", req.Channel, req.Channel),
				"happened_at":      gorm.Expr("CASE WHEN ? IS NOT NULL THEN ? ELSE crm_activity_record.happened_at END", req.HappenedAt, req.HappenedAt),
				"remark":           gorm.Expr("CASE WHEN ? <> '' THEN ? ELSE crm_activity_record.remark END", req.Remark, req.Remark),
				"operator_user_id": req.OperatorUserID,
			}),
		}).Create(&req).Error
	})
	if err != nil {
		log.Sugar.Errorw("crm activity record failed", "err", err)
		return req, err
	}
	if err := s.recomputeActivityCounters(ctx, req.ActivityID); err != nil {
		return req, err
	}
	// 活动→线索自动转化：参与即建线索（若该客户在该活动下尚无线索）
	s.autoCreateLeadFromActivity(ctx, req.ActivityID, req.CustomerID, req.RecordType)
	return req, nil
}

// autoCreateLeadFromActivity 活动参与自动建线索（幂等：同活动+客户不重复建）。
func (s CrmActivityService) autoCreateLeadFromActivity(ctx context.Context, activityID, customerID, recordType string) {
	tenantID := identity.TenantID(ctx)

	// 幂等：同活动+客户已有线索则跳过
	var cnt int64
	models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("customer_id = ? AND remark LIKE ?",  customerID,  "%"+activityID+"%").Count(&cnt)
	if cnt > 0 {
		return
	}
	var act models.CrmActivity
	if err := models.MustFor(ctx).Where("id = ?", activityID).First(&act).Error; err != nil {
		return
	}
	var c models.CrmCustomer
	if err := models.MustFor(ctx).Where("id = ?", customerID).First(&c).Error; err != nil {
		return
	}
	lead := models.CrmLead{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		CustomerID: customerID,
		Title:      fmt.Sprintf("活动[%s]参与", act.Name),
		Name:       c.Name,
		Mobile:     c.Mobile,
		Company:    c.Company,
		Source:     "activity_" + act.ActivityType,
		Status:     "new",
		Remark:     fmt.Sprintf("活动ID:%s,参与类型:%s", activityID, recordType),
	}
	if err := createCRM(models.DB, &lead, "OwnerUserID", "CustomerID"); err != nil {
		log.Sugar.Errorw("auto create lead from activity failed", "err", err)
	}
}

// DeleteRecord 删除参与明细，并重算受影响活动的三计数。
func (s CrmActivityService) DeleteRecord(ctx context.Context, ids []string) (int64, error) {

	var recs []models.CrmActivityRecord
	if err := models.MustFor(ctx).Where("id IN ?",  ids).Find(&recs).Error; err != nil {
		return 0, err
	}
	if len(recs) == 0 {
		return 0, nil
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmActivityRecord{})
	if res.Error != nil {
		return 0, res.Error
	}
	activities := map[string]bool{}
	for _, r := range recs {
		activities[r.ActivityID] = true
	}
	for aid := range activities {
		if err := s.recomputeActivityCounters(ctx, aid); err != nil {
			return res.RowsAffected, err
		}
	}
	return res.RowsAffected, nil
}

// funnelCounts 累计漏斗：触达 ⊇ 参与 ⊇ 转化。converted/participated/reached 为各档位客户数。
func funnelCounts(converted, participated, reached int) (reach, participate, conversion int) {
	conversion = converted
	participate = converted + participated
	reach = converted + participated + reached
	return
}

// recomputeActivityCounters 从参与明细重算活动触达/参与/转化计数（累计漏斗）。
func (s CrmActivityService) recomputeActivityCounters(ctx context.Context, activityID string) error {
	tenantID := identity.TenantID(ctx)

	var perType []struct {
		RecordType string
		N          int
	}
	if err := models.DB.Model(&models.CrmActivityRecord{}).
		Select("record_type, COUNT(*) AS n").Scopes(models.TenantScope(tenantID)).Where("activity_id = ?",  activityID).
		Group("record_type").Scan(&perType).Error; err != nil {
		return err
	}
	converted, participated, reached := 0, 0, 0
	for _, r := range perType {
		switch r.RecordType {
		case "converted":
			converted = r.N
		case "participated":
			participated = r.N
		case "reached":
			reached = r.N
		}
	}
	reach, participate, conversion := funnelCounts(converted, participated, reached)
	return models.DB.Model(&models.CrmActivity{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", activityID).
		Updates(map[string]any{
			"reach_count":       reach,
			"participate_count": participate,
			"conversion_count":  conversion,
		}).Error
}

// ListRecords 按活动分页返回参与明细（带客户名），可按档位过滤。
func (s CrmActivityService) ListRecords(ctx context.Context, activityID, recordType string, pager *app.Pager) ([]ActivityRecordRow, int64, error) {
	tenantID := identity.TenantID(ctx)

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	var total int64
	cnt := models.DB.Model(&models.CrmActivityRecord{}).Scopes(models.TenantScope(tenantID)).Where("activity_id = ?",  activityID)
	if recordType != "" {
		cnt = cnt.Where("record_type = ?", recordType)
	}
	if err := cnt.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	db := models.DB.Table("crm_activity_record").
		Select("crm_activity_record.*, crm_customer.name AS customer_name").
		Joins("LEFT JOIN crm_customer ON crm_customer.id = crm_activity_record.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_activity_record.activity_id = ?",  activityID)
	if recordType != "" {
		db = db.Where("crm_activity_record.record_type = ?", recordType)
	}
	var rows []ActivityRecordRow
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).
		Order("crm_activity_record.happened_at desc, crm_activity_record.id desc").Scan(&rows).Error
	return rows, total, err
}

// ListByCustomer 返回某客户的活动参与史（带活动名/类型/状态），客户360使用。
func (s CrmActivityService) ListByCustomer(ctx context.Context, customerID string) ([]CustomerActivityRow, error) {
	tenantID := identity.TenantID(ctx)

	var rows []CustomerActivityRow
	err := models.DB.Table("crm_activity_record").
		Select("crm_activity_record.*, crm_activity.name AS activity_name, crm_activity.activity_type AS activity_type, crm_activity.status AS activity_status").
		Joins("LEFT JOIN crm_activity ON crm_activity.id = crm_activity_record.activity_id").Scopes(models.TenantScope(tenantID)).Where("crm_activity_record.customer_id = ?",  customerID).
		Order("crm_activity_record.happened_at desc, crm_activity_record.id desc").Scan(&rows).Error
	return rows, err
}

// ---------- CrmChannelInteraction（渠道互动） ----------

type CrmChannelInteractionService struct{}

func NewCrmChannelInteraction() *CrmChannelInteractionService { return &CrmChannelInteractionService{} }

// Create 录入渠道互动（非企微渠道的留资/评论/私信等，企微互动仍走消息存档）。
func (s CrmChannelInteractionService) Create(ctx context.Context, userID string, req models.CrmChannelInteraction) (models.CrmChannelInteraction, error) {
	tenantID := identity.TenantID(ctx)

	if req.CustomerID == "" {
		return req, errors.New("customer_id 不能为空")
	}
	var c models.CrmCustomer
	if err := models.MustFor(ctx).Where("id = ?", req.CustomerID).First(&c).Error; err != nil {
		return req, errors.New("客户不存在")
	}
	if req.Channel == "" {
		req.Channel = constants.ChannelGeneral
	}
	if req.ActionType == "" {
		req.ActionType = "other"
	}
	if req.HappenedAt == nil {
		now := time.Now()
		req.HappenedAt = &now
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	err := createCRM(models.DB, &req)
	if err != nil {
		log.Sugar.Errorw("crm channel interaction create failed", "err", err)
	}
	return req, err
}

func (s CrmChannelInteractionService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmChannelInteraction{})
	return res.RowsAffected, res.Error
}

// Query 分页查询渠道互动，可按客户/渠道过滤。
func (s CrmChannelInteractionService) Query(ctx context.Context, customerID, channel string, pager *app.Pager) ([]models.CrmChannelInteraction, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if customerID != "" {
		db = db.Where("customer_id = ?", customerID)
	}
	if channel != "" {
		db = db.Where("channel = ?", channel)
	}
	var total int64
	if err := db.Model(&models.CrmChannelInteraction{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmChannelInteraction
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("happened_at desc, id desc").Find(&items).Error
	return items, total, err
}

// ---------- CrmChannelConfig（渠道接入配置，一租户一企微）----------

// ChannelConfigReq 渠道配置入参。secrets 空串语义：创建=未填；更新=不修改。
type ChannelConfigReq struct {
	Channel         string `json:"channel"`
	CorpID          string `json:"corp_id"`
	Name            string `json:"name"`
	ContactSecret   string `json:"contact_secret"`
	CustomerSecret  string `json:"customer_secret"`
	MainAgentID     int64  `json:"main_agent_id"`
	MainAgentSecret string `json:"main_agent_secret"`
	CallbackToken   string `json:"callback_token"`
	CallbackAesKey  string `json:"callback_aes_key"`
	CallbackURL     string `json:"callback_url"`
	Enabled         int    `json:"enabled"`
	Remark          string `json:"remark"`
	XhsAppSecret    string `json:"xhs_app_secret"`
	GzhAppSecret    string `json:"gzh_app_secret"`
	DouyinAppSecret string `json:"douyin_app_secret"`
}

type CrmChannelConfigService struct{}

func NewCrmChannelConfig() *CrmChannelConfigService { return &CrmChannelConfigService{} }

// MaskSecret 掩码展示（纯函数，可单测）：非空回 "******"，空串原样回空。
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	return "******"
}

// Create 新增渠道接入配置。corp_id 全局唯一（一个企微号只被一个租户接入）。
func (s CrmChannelConfigService) Create(ctx context.Context, req ChannelConfigReq) (ChannelConfigVO, error) {
	tenantID := identity.TenantID(ctx)

	cfg := models.CrmChannelConfig{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		Channel:     req.Channel,
		CorpID:      req.CorpID,
		Name:        req.Name,
		MainAgentID: req.MainAgentID,
		CallbackURL: req.CallbackURL,
		Enabled:     req.Enabled,
		Remark:      req.Remark,
	}
	if cfg.Channel == "" {
		cfg.Channel = constants.ChannelWecom
	}
	if cfg.Enabled == 0 {
		cfg.Enabled = 1
	}
	// 渠道专用 AppID 冗余（便于识别）
	switch cfg.Channel {
	case constants.ChannelGzh:
		cfg.GzhAppID = cfg.CorpID
	case constants.ChannelDouyin:
		cfg.DouyinAppID = cfg.CorpID
	}
	if err := validateChannelConfig(&cfg); err != nil {
		return toChannelConfigVO(cfg), err
	}
	// corp_id 全局唯一
	var cnt int64
	if err := models.DB.Model(&models.CrmChannelConfig{}).Where("corp_id = ?", cfg.CorpID).Count(&cnt).Error; err != nil {
		return toChannelConfigVO(cfg), err
	}
	if cnt > 0 {
		return toChannelConfigVO(cfg), errors.New("该 corp_id 已被其他租户配置")
	}
	if err := encryptChannelSecrets(&cfg, req); err != nil {
		return toChannelConfigVO(cfg), err
	}
	if err := createCRM(models.DB, &cfg); err != nil {
		log.Sugar.Errorw("crm channel config create failed", "err", err)
		return toChannelConfigVO(cfg), err
	}
	if cfg.Channel == constants.ChannelWecom {
		registerWecomClient(cfg)
	}
	return toChannelConfigVO(cfg), nil
}

// toChannelConfigVO 把 CrmChannelConfig 转为展示 VO（secrets 转掩码）。
func toChannelConfigVO(cfg models.CrmChannelConfig) ChannelConfigVO {
	return ChannelConfigVO{
		CrmChannelConfig: cfg,
		ContactSecret:    MaskSecret(cfg.ContactSecret),
		CustomerSecret:   MaskSecret(cfg.CustomerSecret),
		MainAgentSecret:  MaskSecret(cfg.MainAgentSecret),
		CallbackToken:    MaskSecret(cfg.CallbackToken),
		CallbackAesKey:   MaskSecret(cfg.CallbackAesKey),
		XhsAppSecret:     MaskSecret(cfg.XhsAppSecret),
		GzhAppSecret:     MaskSecret(cfg.GzhAppSecret),
		DouyinAppSecret:  MaskSecret(cfg.DouyinAppSecret),
	}
}

// Update 更新渠道配置。secrets 空串=保留原值；corp_id 变更时校验全局唯一。
func (s CrmChannelConfigService) Update(ctx context.Context, id string, req ChannelConfigReq) (ChannelConfigVO, error) {

	var cfg models.CrmChannelConfig
	if err := models.MustFor(ctx).Where("id = ?", id).First(&cfg).Error; err != nil {
		return toChannelConfigVO(cfg), errors.New("渠道配置不存在")
	}
	if req.CorpID != "" && req.CorpID != cfg.CorpID {
		var cnt int64
		if err := models.DB.Model(&models.CrmChannelConfig{}).
			Where("corp_id = ? AND id <> ?", req.CorpID, id).Count(&cnt).Error; err != nil {
			return toChannelConfigVO(cfg), err
		}
		if cnt > 0 {
			return toChannelConfigVO(cfg), errors.New("该 corp_id 已被其他租户配置")
		}
		cfg.CorpID = req.CorpID
	}
	if req.Channel != "" {
		cfg.Channel = req.Channel
	}
	if req.Name != "" {
		cfg.Name = req.Name
	}
	if req.CallbackURL != "" {
		cfg.CallbackURL = req.CallbackURL
	}
	if req.Remark != "" {
		cfg.Remark = req.Remark
	}
	if req.Enabled != 0 {
		cfg.Enabled = req.Enabled
	}
	if req.MainAgentID != 0 {
		cfg.MainAgentID = req.MainAgentID
	}
	// secrets：空串不改；否则重新加密覆盖
	var err error
	if req.ContactSecret != "" {
		if cfg.ContactSecret, err = util.AESEncode(req.ContactSecret); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	if req.CustomerSecret != "" {
		if cfg.CustomerSecret, err = util.AESEncode(req.CustomerSecret); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	if req.MainAgentSecret != "" {
		if cfg.MainAgentSecret, err = util.AESEncode(req.MainAgentSecret); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	if req.CallbackToken != "" {
		if cfg.CallbackToken, err = util.AESEncode(req.CallbackToken); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	if req.CallbackAesKey != "" {
		if cfg.CallbackAesKey, err = util.AESEncode(req.CallbackAesKey); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	if req.XhsAppSecret != "" {
		if cfg.XhsAppSecret, err = util.AESEncode(req.XhsAppSecret); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	if req.GzhAppSecret != "" {
		if cfg.GzhAppSecret, err = util.AESEncode(req.GzhAppSecret); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	if req.DouyinAppSecret != "" {
		if cfg.DouyinAppSecret, err = util.AESEncode(req.DouyinAppSecret); err != nil {
			return toChannelConfigVO(cfg), err
		}
	}
	// 渠道专用 AppID 冗余
	switch cfg.Channel {
	case constants.ChannelGzh:
		cfg.GzhAppID = cfg.CorpID
	case constants.ChannelDouyin:
		cfg.DouyinAppID = cfg.CorpID
	}
	if err := models.DB.Model(&models.CrmChannelConfig{}).Where("id = ?", id).Updates(map[string]any{
		"channel": cfg.Channel, "corp_id": cfg.CorpID, "name": cfg.Name,
		"contact_secret": cfg.ContactSecret, "customer_secret": cfg.CustomerSecret,
		"main_agent_id": cfg.MainAgentID, "main_agent_secret": cfg.MainAgentSecret,
		"callback_token": cfg.CallbackToken, "callback_aes_key": cfg.CallbackAesKey,
		"callback_url": cfg.CallbackURL, "enabled": cfg.Enabled, "remark": cfg.Remark,
		"xhs_app_secret": cfg.XhsAppSecret,
		"gzh_app_id":     cfg.GzhAppID, "gzh_app_secret": cfg.GzhAppSecret,
		"douyin_app_id": cfg.DouyinAppID, "douyin_app_secret": cfg.DouyinAppSecret,
	}).Error; err != nil {
		return toChannelConfigVO(cfg), err
	}
	if cfg.Channel == constants.ChannelWecom {
		registerWecomClient(cfg)
	}
	return toChannelConfigVO(cfg), nil
}

// ChannelConfigVO 渠道配置展示对象（secrets 转掩码）。
type ChannelConfigVO struct {
	models.CrmChannelConfig
	ContactSecret   string `json:"contact_secret"`
	CustomerSecret  string `json:"customer_secret"`
	MainAgentSecret string `json:"main_agent_secret"`
	CallbackToken   string `json:"callback_token"`
	CallbackAesKey  string `json:"callback_aes_key"`
	XhsAppSecret    string `json:"xhs_app_secret"`
	GzhAppSecret    string `json:"gzh_app_secret"`
	DouyinAppSecret string `json:"douyin_app_secret"`
}

func (s CrmChannelConfigService) Query(ctx context.Context, pager *app.Pager) ([]ChannelConfigVO, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	var total int64
	if err := db.Model(&models.CrmChannelConfig{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmChannelConfig
	if err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error; err != nil {
		return nil, 0, err
	}
	vos := make([]ChannelConfigVO, 0, len(items))
	for _, it := range items {
		vos = append(vos, ChannelConfigVO{
			CrmChannelConfig: it,
			ContactSecret:    MaskSecret(it.ContactSecret),
			CustomerSecret:   MaskSecret(it.CustomerSecret),
			MainAgentSecret:  MaskSecret(it.MainAgentSecret),
			CallbackToken:    MaskSecret(it.CallbackToken),
			CallbackAesKey:   MaskSecret(it.CallbackAesKey),
			XhsAppSecret:     MaskSecret(it.XhsAppSecret),
		})
	}
	return vos, total, nil
}

func (s CrmChannelConfigService) Delete(ctx context.Context, ids []string) (int64, error) {

	var cfgs []models.CrmChannelConfig
	models.MustFor(ctx).Where("id IN ?",  ids).Find(&cfgs)
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmChannelConfig{})
	for _, c := range cfgs {
		if we_work.Clients != nil {
			we_work.Clients.Remove(c.CorpID)
		}
	}
	return res.RowsAffected, res.Error
}

// GetByCorpID 按 corp_id 取回调所需解密凭据 + 归属租户（供企微回调入口用）。
func (s CrmChannelConfigService) GetByCorpID(corpID string) (*WecomCallbackCred, error) {
	var cfg models.CrmChannelConfig
	if err := models.DB.Where("corp_id = ?", corpID).First(&cfg).Error; err != nil {
		return nil, errors.New("未配置该 corp_id 的渠道")
	}
	if cfg.Enabled != 1 {
		return nil, errors.New("该渠道已停用")
	}
	token, _ := util.AESDecode(cfg.CallbackToken)
	aesKey, _ := util.AESDecode(cfg.CallbackAesKey)
	return &WecomCallbackCred{
		TenantID:       cfg.TenantID,
		CorpID:         cfg.CorpID,
		CallbackToken:  token,
		CallbackAesKey: aesKey,
	}, nil
}

// GetTenantByChannelApp 按渠道 app_id（CorpID 字段）查归属租户（供小红书回调入口用）。
func (s CrmChannelConfigService) GetTenantByChannelApp(appID string) (string, error) {
	var cfg models.CrmChannelConfig
	if err := models.DB.Where("corp_id = ? AND enabled = ?", appID, 1).First(&cfg).Error; err != nil {
		return "", errors.New("未配置该 app_id 的渠道或已停用")
	}
	return cfg.TenantID, nil
}

// VerifyXhsSignature 校验小红书 webhook 签名：HMAC-SHA256(body, CallbackToken) 与 sig 比较。
// 未配置 CallbackToken 则校验失败（fail closed）。
func (s CrmChannelConfigService) VerifyXhsSignature(appID string, body []byte, sig string) bool {
	var cfg models.CrmChannelConfig
	if err := models.DB.Where("corp_id = ?", appID).First(&cfg).Error; err != nil {
		return false
	}
	token, err := util.AESDecode(cfg.CallbackToken)
	if err != nil || token == "" {
		return false
	}
	h := hmac.New(sha256.New, []byte(token))
	h.Write(body)
	expected := hex.EncodeToString(h.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}

// WecomCallbackCred 企微回调入口所需的解密凭据。
type WecomCallbackCred struct {
	TenantID       string
	CorpID         string
	CallbackToken  string
	CallbackAesKey string
}

// validateChannelConfig 基础校验（纯函数，可单测）。
func validateChannelConfig(cfg *models.CrmChannelConfig) error {
	if cfg.CorpID == "" {
		return errors.New("corp_id 不能为空")
	}
	switch cfg.Channel {
	case constants.ChannelWecom:
		if len(cfg.CorpID) > 18 {
			return errors.New("corp_id 长度不能超过 18")
		}
	case constants.ChannelXiaohongshu:
		if len(cfg.CorpID) > 64 {
			return errors.New("app_id 长度不能超过 64")
		}
	case constants.ChannelGzh:
		if len(cfg.CorpID) > 64 {
			return errors.New("公众号 AppID 长度不能超过 64")
		}
	case constants.ChannelDouyin:
		if len(cfg.CorpID) > 64 {
			return errors.New("抖音 AppID 长度不能超过 64")
		}
	default:
		return errors.New("不支持的渠道: " + cfg.Channel)
	}
	return nil
}

// encryptChannelSecrets 用 util.AESEncode 加密全部 secrets 落库。
func encryptChannelSecrets(cfg *models.CrmChannelConfig, req ChannelConfigReq) error {
	var err error
	if cfg.ContactSecret, err = util.AESEncode(req.ContactSecret); err != nil {
		return err
	}
	if cfg.CustomerSecret, err = util.AESEncode(req.CustomerSecret); err != nil {
		return err
	}
	if cfg.MainAgentSecret, err = util.AESEncode(req.MainAgentSecret); err != nil {
		return err
	}
	if cfg.CallbackToken, err = util.AESEncode(req.CallbackToken); err != nil {
		return err
	}
	if cfg.CallbackAesKey, err = util.AESEncode(req.CallbackAesKey); err != nil {
		return err
	}
	if cfg.XhsAppSecret, err = util.AESEncode(req.XhsAppSecret); err != nil {
		return err
	}
	if cfg.GzhAppSecret, err = util.AESEncode(req.GzhAppSecret); err != nil {
		return err
	}
	if cfg.DouyinAppSecret, err = util.AESEncode(req.DouyinAppSecret); err != nil {
		return err
	}
	return nil
}

// registerWecomClient 按配置启用态注册/移除企微 API 客户端（供回调补名等 API 拉取用）。
// 客户端 map 懒初始化；秘钥解密后入 per-corp 缓存，首次 API 调用才拉 token，注册本身不碰网络。
func registerWecomClient(cfg models.CrmChannelConfig) {
	we_work.EnsureClients()
	if cfg.Enabled != 1 {
		we_work.Clients.Remove(cfg.CorpID)
		return
	}
	contact, _ := util.AESDecode(cfg.ContactSecret)
	customer, _ := util.AESDecode(cfg.CustomerSecret)
	main, _ := util.AESDecode(cfg.MainAgentSecret)
	if err := we_work.Clients.Add(we_work.CorpConf{
		ExtCorpID:       cfg.CorpID,
		ContactSecret:   contact,
		CustomerSecret:  customer,
		MainAgentID:     cfg.MainAgentID,
		MainAgentSecret: main,
	}); err != nil {
		log.Sugar.Errorw("register wecom client failed", "corp_id", cfg.CorpID, "err", err)
	}
}
