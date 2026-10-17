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
	"math"
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
	"core/common/storage"
	"core/common/util"
	"core/common/we_work"
	"core/conf"

	"github.com/xuri/excelize/v2"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncTenantCtx 从 corp_id 反查对应的 tenant_id，包成 ctx 返回。
// 用于 Syncs() 等没有 grpc ctx 的启动期同步任务，以及 controller/consumer 等
// 已有 corpID 但 ctx 无 tenant 的调用方。反查走 channel_link(wecom, tenant)；
// 找不到则返回原 ctx（保持兼容，未配 channel_link 的 corp 也能跑）。
func SyncTenantCtx(ctx context.Context, corpID string) context.Context {
	if corpID == "" {
		return ctx
	}
	if tID := identity.TenantID(ctx); tID != "" {
		// CRM 上下文已带 tenant，直接用，不被 corp 覆盖
		return ctx
	}
	var link models.ChannelLink
	if err := models.DB.Where("channel = ? AND entity_type = ? AND ext_id = ?", constants.ChannelWecom, "tenant", corpID).
		First(&link).Error; err != nil {
		return ctx
	}
	return identity.WithIdentity(ctx, identity.Identity{TenantID: link.EntityID})
}

// syncTenantCtx 等价于 SyncTenantCtx，仅供包内调用方。
func syncTenantCtx(corpID string) context.Context {
	return SyncTenantCtx(context.Background(), corpID)
}

// createCRM 创建记录，自动省略值为空的 FK 列。
// 本地模型的可选外键是 bigint，空字符串会被 PG 拒绝，需以 NULL 表示"未设"。
// 两层防护：
//  1. 自动扫描 struct 的 gorm tag，凡是 `type:bigint` 的 string 字段且值为 "" 都 omit
//     （覆盖所有未显式列出的 FK，避免漏写导致 22P02）；
//  2. 显式传入的 fkFields 仍然按原逻辑保留（兼容历史调用方）。
func createCRM(db *gorm.DB, v any, fkFields ...string) error {
	rv := reflect.ValueOf(v).Elem()
	rt := rv.Type()
	explicit := make(map[string]bool, len(fkFields))
	for _, f := range fkFields {
		explicit[f] = true
	}
	omits := []string{}
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if explicit[f.Name] {
			continue // 由外层 fkFields 循环处理（语义一致）
		}
		if !strings.Contains(f.Tag.Get("gorm"), "type:bigint") {
			continue
		}
		fv := rv.Field(i)
		if fv.IsValid() && fv.Kind() == reflect.String && fv.String() == "" {
			omits = append(omits, f.Name)
		}
	}
	// 显式列出的 FK 也按值决定 omit
	for _, fname := range fkFields {
		fv := rv.FieldByName(fname)
		if fv.IsValid() && fv.Kind() == reflect.String && fv.String() == "" {
			omits = append(omits, fname)
		}
	}
	if len(omits) == 0 {
		return db.Create(v).Error
	}
	return db.Omit(omits...).Create(v).Error
}

// updatesCRM 从更新 map 中剔除值为空字符串的 FK 列（避免空串写入 bigint）。
// 同时剔除所有 type:bigint 的空字符串字段（与 createCRM 对齐）。
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

// TenantCreateReq 创建租户入参（平台超管调用）。
// AdminUsername/AdminPassword/AdminName/AdminMobile 用于一并创建该租户的管理员账号，
// 解决"建租户后无法登录"的鸡生蛋问题。AdminPassword 为空时生成随机初始密码并返回。
type TenantCreateReq struct {
	Name          string     `json:"name"`
	Code          string     `json:"code"`
	Remark        string     `json:"remark"`
	Plan          string     `json:"plan"`
	ExpireAt      *time.Time `json:"expire_at"`
	ContactName   string     `json:"contact_name"`
	ContactPhone  string     `json:"contact_phone"`
	ContactEmail  string     `json:"contact_email"`
	Industry      string     `json:"industry"`
	Logo          string     `json:"logo"`
	MaxUsers      int        `json:"max_users"`
	AdminUsername string     `json:"admin_username"`
	AdminPassword string     `json:"admin_password"`
	AdminName     string     `json:"admin_name"`
	AdminMobile   string     `json:"admin_mobile"`
}

// TenantCreateResult 创建租户结果：返回租户 + 初始管理员密码（AdminPassword 为空时生成）。
type TenantCreateResult struct {
	Tenant          models.Tenant `json:"tenant"`
	InitialPassword string        `json:"initial_password"`
}

func (s TenantService) Create(req TenantCreateReq) (TenantCreateResult, error) {
	var result TenantCreateResult
	if req.Name == "" || req.Code == "" {
		return result, errors.New("租户名称和编码不能为空")
	}
	if req.AdminUsername == "" {
		return result, errors.New("管理员账号不能为空")
	}
	// 校验编码唯一
	var cnt int64
	models.DB.Model(&models.Tenant{}).Where("code = ?", req.Code).Count(&cnt)
	if cnt > 0 {
		return result, errors.New("租户编码已存在")
	}
	// 事务：建租户 + 种子默认角色/阶段 + 建管理员账号
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		t := models.Tenant{
			Model:        models.Model{ID: id_generator.StringID()},
			Name:         req.Name,
			Code:         req.Code,
			Status:       1,
			Remark:       req.Remark,
			Plan:         req.Plan,
			ExpireAt:     req.ExpireAt,
			ContactName:  req.ContactName,
			ContactPhone: req.ContactPhone,
			ContactEmail: req.ContactEmail,
			Industry:     req.Industry,
			Logo:         req.Logo,
			MaxUsers:     req.MaxUsers,
		}
		if t.Plan == "" {
			t.Plan = "free"
		}
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		result.Tenant = t
		// 种子默认角色/阶段（复用既有逻辑，但用 tx 而非全局 DB）
		if err := seedTenantDefaultsTx(tx, t.ID); err != nil {
			return err
		}
		// 查 admin 角色 ID
		var role models.CrmRole
		if err := tx.Scopes(models.TenantScope(t.ID)).Where("code = ?", "admin").First(&role).Error; err != nil {
			return err
		}
		// 处理密码
		initial := ""
		pw := req.AdminPassword
		if pw == "" {
			var pwErr error
			pw, pwErr = randPassword(12)
			if pwErr != nil {
				return pwErr
			}
			initial = pw
		} else {
			if err := validatePassword(pw); err != nil {
				return err
			}
		}
		h, err := hashPassword(pw)
		if err != nil {
			return err
		}
		result.InitialPassword = initial
		u := models.User{
			Model:    models.Model{ID: id_generator.StringID()},
			TenantID: t.ID,
			Username: req.AdminUsername,
			Password: h,
			Name:     req.AdminName,
			Mobile:   req.AdminMobile,
			RoleID:   role.ID,
			Status:   1,
		}
		return createCRM(tx, &u, "RoleID", "DepartmentID")
	})
	return result, err
}

func (s TenantService) List() ([]models.Tenant, error) {
	var ts []models.Tenant
	err := models.DB.Order("id").Find(&ts).Error
	return ts, err
}

// TenantListReq 搜索/分页参数。
type TenantListReq struct {
	Keyword  string `json:"keyword"` // 按名称或编码模糊搜索
	Status   int    `json:"status"`  // 0=全部 1=启用 2=停用
	Plan     string `json:"plan"`    // 空=全部
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// TenantListResult 分页结果。
type TenantListResult struct {
	Total int64           `json:"total"`
	List  []models.Tenant `json:"list"`
}

// ListPaged 分页+搜索列出租户。
func (s TenantService) ListPaged(req TenantListReq) (TenantListResult, error) {
	var result TenantListResult
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 200 {
		req.PageSize = 200
	}
	q := models.DB.Model(&models.Tenant{})
	if req.Keyword != "" {
		kw := "%" + req.Keyword + "%"
		q = q.Where("name LIKE ? OR code LIKE ?", kw, kw)
	}
	if req.Status == 1 || req.Status == 2 {
		q = q.Where("status = ?", req.Status)
	}
	if req.Plan != "" {
		q = q.Where("plan = ?", req.Plan)
	}
	if err := q.Count(&result.Total).Error; err != nil {
		return result, err
	}
	var ts []models.Tenant
	offset := (req.Page - 1) * req.PageSize
	if err := q.Order("id DESC").Offset(offset).Limit(req.PageSize).Find(&ts).Error; err != nil {
		return result, err
	}
	result.List = ts
	return result, nil
}

// TenantDetail 租户详情（含统计信息）。
type TenantDetail struct {
	models.Tenant
	UserCount       int64 `json:"user_count"`
	CustomerCount   int64 `json:"customer_count"`
	ActiveUserCount int64 `json:"active_user_count"`
}

// GetDetail 获取租户详情（含用户数、客户数等统计）。
func (s TenantService) GetDetail(id string) (TenantDetail, error) {
	var detail TenantDetail
	if err := models.DB.Where("id = ?", id).First(&detail.Tenant).Error; err != nil {
		return detail, err
	}
	models.DB.Model(&models.User{}).Where("tenant_id = ? AND deleted_at IS NULL", id).Count(&detail.UserCount)
	models.DB.Model(&models.User{}).Where("tenant_id = ? AND status = 1 AND deleted_at IS NULL", id).Count(&detail.ActiveUserCount)
	models.DB.Model(&models.CrmCustomer{}).Where("tenant_id = ? AND deleted_at IS NULL", id).Count(&detail.CustomerCount)
	return detail, nil
}

// CurrentTenantInfo 租户端当前套餐/到期信息 + 当前用户角色。
type CurrentTenantInfo struct {
	Name      string     `json:"name"`
	Code      string     `json:"code"`
	Plan      string     `json:"plan"`
	PlanName  string     `json:"plan_name"`
	ExpireAt  *time.Time `json:"expire_at"`
	MaxUsers  int        `json:"max_users"`
	UserCount int64      `json:"user_count"`
	RoleCode  string     `json:"role_code"`
	IsAdmin   bool       `json:"is_admin"`
}

// GetCurrentInfo 租户端：当前登录用户查看本租户套餐信息 + 自己的角色编码。
func (s TenantService) GetCurrentInfo(tenantID, userID string) (CurrentTenantInfo, error) {
	var info CurrentTenantInfo
	var t models.Tenant
	if err := models.DB.Where("id = ?", tenantID).First(&t).Error; err != nil {
		return info, err
	}
	info.Name = t.Name
	info.Code = t.Code
	info.Plan = t.Plan
	info.ExpireAt = t.ExpireAt
	info.MaxUsers = t.MaxUsers
	// 查询套餐名称
	if t.Plan != "" {
		var plan models.Plan
		if err := models.DB.Where("code = ?", t.Plan).First(&plan).Error; err == nil {
			info.PlanName = plan.Name
		}
	}
	models.DB.Model(&models.User{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&info.UserCount)

	if userID != "" {
		var u models.User
		if err := models.DB.Where("id = ? AND tenant_id = ?", userID, tenantID).First(&u).Error; err == nil {
			if u.RoleID != "" {
				var role models.CrmRole
				if err := models.DB.Where("id = ?", u.RoleID).First(&role).Error; err == nil {
					info.RoleCode = role.Code
				}
			}
		}
	}
	info.IsAdmin = info.RoleCode == "admin"
	return info, nil
}

// ListUsers 列出租户下的用户（供平台超管查看）。
func (s TenantService) ListUsers(tenantID string) ([]map[string]any, error) {
	var users []models.User
	if err := models.DB.Where("tenant_id = ?", tenantID).Order("created_at").Find(&users).Error; err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(users))
	for _, u := range users {
		roleName := ""
		if u.RoleID != "" {
			var role models.CrmRole
			models.DB.Where("id = ?", u.RoleID).First(&role)
			roleName = role.Name
		}
		result = append(result, map[string]any{
			"id":         u.ID,
			"username":   u.Username,
			"name":       u.Name,
			"mobile":     u.Mobile,
			"status":     u.Status,
			"role_name":  roleName,
			"created_at": u.CreatedAt,
		})
	}
	return result, nil
}

// ResetAdminPassword 重置租户管理员密码。返回新密码（明文，仅此一次）。
func (s TenantService) ResetAdminPassword(tenantID, userID string) (string, error) {
	var u models.User
	if err := models.DB.Where("id = ? AND tenant_id = ?", userID, tenantID).First(&u).Error; err != nil {
		return "", errors.New("用户不存在")
	}
	pw, err := randPassword(12)
	if err != nil {
		return "", err
	}
	h, err := hashPassword(pw)
	if err != nil {
		return "", err
	}
	if err := models.DB.Model(&models.User{}).Where("id = ?", userID).Update("password", h).Error; err != nil {
		return "", err
	}
	return pw, nil
}

// RenewReq 续期参数。
type RenewReq struct {
	TenantID string     `json:"tenant_id"`
	ExpireAt *time.Time `json:"expire_at"` // 新的到期时间（覆盖）
	AddDays  int        `json:"add_days"`  // 或在现有基础上加天数（expire_at 为空时使用）
}

// Renew 续期/延长租户有效期。
func (s TenantService) Renew(req RenewReq) (models.Tenant, error) {
	var t models.Tenant
	if err := models.DB.Where("id = ?", req.TenantID).First(&t).Error; err != nil {
		return t, errors.New("租户不存在")
	}
	if req.ExpireAt != nil {
		t.ExpireAt = req.ExpireAt
	} else if req.AddDays > 0 {
		base := time.Now()
		if t.ExpireAt != nil && t.ExpireAt.After(base) {
			base = *t.ExpireAt
		}
		newExpire := base.AddDate(0, 0, req.AddDays)
		t.ExpireAt = &newExpire
	} else {
		return t, errors.New("请指定新的到期时间或延长天数")
	}
	if err := models.DB.Model(&models.Tenant{}).Where("id = ?", req.TenantID).Update("expire_at", t.ExpireAt).Error; err != nil {
		return t, err
	}
	return t, nil
}

// TenantUpdateReq 更新租户入参（平台超管调用）。Code 不可改。
type TenantUpdateReq struct {
	Name         string     `json:"name"`
	Remark       string     `json:"remark"`
	Plan         string     `json:"plan"`
	ExpireAt     *time.Time `json:"expire_at"`
	ContactName  string     `json:"contact_name"`
	ContactPhone string     `json:"contact_phone"`
	ContactEmail string     `json:"contact_email"`
	Industry     string     `json:"industry"`
	Logo         string     `json:"logo"`
	MaxUsers     int        `json:"max_users"`
}

func (s TenantService) Update(id string, req TenantUpdateReq) error {
	updates := map[string]any{
		"name":          req.Name,
		"remark":        req.Remark,
		"plan":          req.Plan,
		"expire_at":     req.ExpireAt,
		"contact_name":  req.ContactName,
		"contact_phone": req.ContactPhone,
		"contact_email": req.ContactEmail,
		"industry":      req.Industry,
		"logo":          req.Logo,
		"max_users":     req.MaxUsers,
	}
	res := models.DB.Model(&models.Tenant{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("租户不存在")
	}
	return nil
}

func (s TenantService) SetStatus(id string, status int) error {
	res := models.DB.Model(&models.Tenant{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("租户不存在")
	}
	return nil
}

func (s TenantService) Delete(id string) error {
	res := models.DB.Where("id = ?", id).Delete(&models.Tenant{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("租户不存在")
	}
	return nil
}

// ---------- PlatformAuth（平台超管登录，独立于租户）----------

type PlatformAuthService struct{}

func NewPlatformAuth() *PlatformAuthService { return &PlatformAuthService{} }

// PlatformLoginResult 平台超管登录结果，网关据此签发 is_platform=true 的 JWT。
type PlatformLoginResult struct {
	AdminID     string   `json:"admin_id"`
	Username    string   `json:"username"`
	Name        string   `json:"name"`
	Role        string   `json:"role"`
	Mobile      string   `json:"mobile"`
	Email       string   `json:"email"`
	Authorities []string `json:"authorities"` // 前端菜单 authority（Platform* 格式 + role 本身）
}

// platformRoleAuthorities 把平台角色映射为前端菜单 authority。
// super_admin 拥有全部平台权限；其它角色按职责分配。
func platformRoleAuthorities(role string) []string {
	auth := []string{role} // role 本身也作为 authority（支持菜单按角色控制）
	switch role {
	case "super_admin":
		auth = append(auth,
			"PlatformTenant_Full", "PlatformTenant_Read",
			"PlatformBilling_Full", "PlatformBilling_Read",
			"PlatformStats_Full", "PlatformStats_Read",
			"PlatformLog_Full", "PlatformLog_Read",
			"PlatformSetting_Full",
		)
	case "operator":
		auth = append(auth,
			"PlatformTenant_Full", "PlatformTenant_Read",
			"PlatformStats_Full", "PlatformStats_Read",
		)
	case "finance":
		auth = append(auth,
			"PlatformBilling_Full", "PlatformBilling_Read",
			"PlatformTenant_Read",
			"PlatformStats_Read",
		)
	case "support":
		auth = append(auth,
			"PlatformTenant_Read",
			"PlatformLog_Read",
			"PlatformStats_Read",
		)
	}
	return auth
}

func (s PlatformAuthService) Login(username, password string) (PlatformLoginResult, error) {
	var a models.PlatformAdmin
	if err := models.DB.Where("username = ?", username).First(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(password))
			return PlatformLoginResult{}, errors.New("用户名或密码错误")
		}
		return PlatformLoginResult{}, err
	}
	if a.Status != 1 {
		return PlatformLoginResult{}, errors.New("账号已停用")
	}
	if a.Password == "" || bcrypt.CompareHashAndPassword([]byte(a.Password), []byte(password)) != nil {
		return PlatformLoginResult{}, errors.New("用户名或密码错误")
	}
	models.DB.Model(&models.PlatformAdmin{}).Where("id = ?", a.ID).Update("last_login_at", time.Now())
	return PlatformLoginResult{
		AdminID:     a.ID,
		Username:    a.Username,
		Name:        a.Name,
		Role:        a.Role,
		Mobile:      a.Mobile,
		Email:       a.Email,
		Authorities: platformRoleAuthorities(a.Role),
	}, nil
}

// SeedPlatformAdmin 幂等地创建默认平台超管账号（master/123456），仅非生产环境执行。
func SeedPlatformAdmin() {
	if conf.Settings.App.Env == "PROD" {
		return
	}
	var cnt int64
	models.DB.Model(&models.PlatformAdmin{}).Where("username = ?", "master").Count(&cnt)
	if cnt > 0 {
		return
	}
	h, err := hashPassword("123456")
	if err != nil {
		return
	}
	models.DB.Create(&models.PlatformAdmin{
		Model:    models.Model{ID: id_generator.StringID()},
		Username: "master",
		Password: h,
		Name:     "平台超管",
		Role:     "super_admin",
		Status:   1,
	})
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

// seedTenantDefaultsTx 事务版 seedTenantDefaults：用传入的 tx 而非全局 DB，
// 供 TenantService.Create 在事务内一并种子默认角色/阶段。仅种子角色与阶段（渠道标签走全局 DB）。
func seedTenantDefaultsTx(tx *gorm.DB, tenantID string) error {
	var cnt int64
	if err := tx.Model(&models.CrmRole{}).Scopes(models.TenantScope(tenantID)).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		for _, r := range defaultRoles {
			if err := tx.Create(&models.CrmRole{
				Model:     models.Model{ID: id_generator.StringID()},
				TenantID:  tenantID,
				Code:      r.Code,
				Name:      r.Name,
				Perms:     r.Perms,
				IsDefault: r.IsDefault,
			}).Error; err != nil {
				return err
			}
		}
	}
	if err := tx.Model(&models.CrmPipelineStage{}).Scopes(models.TenantScope(tenantID)).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		for _, st := range defaultStages {
			if err := tx.Create(&models.CrmPipelineStage{
				Model:       models.Model{ID: id_generator.StringID()},
				TenantID:    tenantID,
				Name:        st.Name,
				Sort:        st.Sort,
				Probability: st.Probability,
				IsWon:       st.IsWon,
				IsLost:      st.IsLost,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
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
		if err := models.DB.Model(&models.CrmChannelTag{}).Scopes(models.TenantScope(tenantID)).Where("channel = ?", t.Channel).Count(&cnt).Error; err != nil || cnt > 0 {
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
	if err := models.DB.Model(&models.CrmChannelConfig{}).Scopes(models.TenantScope(tenantID)).Where("channel = ? AND corp_id = ?", constants.ChannelWecom, ww.ExtCorpID).
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
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := models.EnsureWecomTenantLink(tx, tenantID, cfg.CorpID); err != nil {
			return err
		}
		return tx.Create(&cfg).Error
	}); err != nil {
		log.Sugar.Errorw("seed channel config failed", "err", err)
		return
	}
	registerWecomClient(cfg)
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

	// 套餐坐席数限制：MaxUsers > 0 时检查当前用户数是否已达上限。
	var tenant models.Tenant
	if err := models.DB.Where("id = ?", tenantID).First(&tenant).Error; err == nil {
		if tenant.MaxUsers > 0 {
			var count int64
			models.DB.Model(&models.User{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&count)
			if int(count) >= tenant.MaxUsers {
				return models.User{}, "", errors.New("当前套餐坐席数已达上限(" + strconv.Itoa(tenant.MaxUsers) + ")，请升级套餐或联系平台管理员")
			}
		}
	}

	if req.RoleID == "" {
		req.RoleID = "sales"
	}
	req.RoleID = resolveRoleID(ctx, req.RoleID)
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
			"avatar":        u.Avatar,
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

// UserUpdateReq 管理员更新本地用户的入参。
// 指针字段区分「未传(nil)」vs「清空(*"")」vs「赋值(*"xxx")」；
// 未传的字段保持原值，允许管理员把字段显式清空。
type UserUpdateReq struct {
	Name         *string `json:"name"`
	Mobile       *string `json:"mobile"`
	Email        *string `json:"email"`
	Avatar       *string `json:"avatar"`
	RoleID       string  `json:"role_id"`
	Position     *string `json:"position"`
	DepartmentID string  `json:"department_id"`
	Status       *int    `json:"status"`
}

// resolveRoleID 把角色编码(admin/manager/sales)解析为 CrmRole.ID。
// 入参为空时默认取 sales 角色；若按 code 查不到则视为已是角色ID原样返回。
func resolveRoleID(ctx context.Context, roleID string) string {
	code := roleID
	if code == "" {
		code = "sales"
	}
	var r models.CrmRole
	if err := models.MustFor(ctx).Scopes(models.TenantScope(identity.TenantID(ctx))).
		Where("code = ?", code).First(&r).Error; err == nil {
		return r.ID
	}
	return roleID
}

func (s UserService) Update(ctx context.Context, userID string, req UserUpdateReq) error {

	m := map[string]any{}
	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		if v == "" {
			return errors.New("姓名不能为空")
		}
		m["name"] = v
	}
	if req.Mobile != nil {
		m["mobile"] = strings.TrimSpace(*req.Mobile)
	}
	if req.Email != nil {
		v := strings.TrimSpace(*req.Email)
		if v != "" && !strings.Contains(v, "@") {
			return errors.New("邮箱格式不正确")
		}
		m["email"] = v
	}
	if req.Position != nil {
		m["position"] = strings.TrimSpace(*req.Position)
	}
	if req.Avatar != nil {
		m["avatar"] = *req.Avatar
	}
	if req.DepartmentID != "" {
		m["department_id"] = req.DepartmentID
	}
	if req.RoleID != "" {
		m["role_id"] = resolveRoleID(ctx, req.RoleID)
	}
	if req.Status != nil {
		m["status"] = *req.Status
	}
	m = updatesCRM(m, "RoleID", "DepartmentID")
	return models.MustFor(ctx).Model(&models.User{}).Where("id = ?", userID).Updates(m).Error
}

// GetCurrent 返回 ctx 中本地用户（user_id）的完整 User 行，供前端 header/个人中心使用。
func (s UserService) GetCurrent(ctx context.Context) (models.User, error) {
	uid := identity.UserID(ctx)
	if uid == "" {
		return models.User{}, errors.New("missing user identity")
	}
	var u models.User
	if err := models.MustFor(ctx).Where("id = ?", uid).First(&u).Error; err != nil {
		return models.User{}, err
	}
	return u, nil
}

// UserUpdateProfileReq 自助改资料的入参（PATCH 语义）。
// 指针字段区分「未传(nil)」vs「清空(*"")」vs「赋值(*"xxx")」。
// 必填校验：Name 非 nil 时不允许为空串。
type UserUpdateProfileReq struct {
	Name     *string `json:"name"`
	Mobile   *string `json:"mobile"`
	Email    *string `json:"email"`
	Position *string `json:"position"`
	Avatar   *string `json:"avatar"`
}

// UpdateProfile 自助更新自己的基本资料：name/mobile/email/position/avatar。
// 禁止修改 username/password/role_id/department_id/status（这些由 crm_user/Update 管理）。
func (s UserService) UpdateProfile(ctx context.Context, req UserUpdateProfileReq) (models.User, error) {
	uid := identity.UserID(ctx)
	tenantID := identity.TenantID(ctx)
	if uid == "" {
		return models.User{}, errors.New("missing user identity")
	}
	m := map[string]any{}
	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		if v == "" {
			return models.User{}, errors.New("姓名不能为��")
		}
		m["name"] = v
	}
	if req.Mobile != nil {
		m["mobile"] = strings.TrimSpace(*req.Mobile)
	}
	if req.Email != nil {
		v := strings.TrimSpace(*req.Email)
		if v != "" && !strings.Contains(v, "@") {
			return models.User{}, errors.New("邮箱格式不正确")
		}
		m["email"] = v
	}
	if req.Position != nil {
		m["position"] = strings.TrimSpace(*req.Position)
	}
	if req.Avatar != nil {
		m["avatar"] = *req.Avatar
	}
	if len(m) == 0 {
		// 幂等：什么都不改，直接回查
		return s.GetCurrent(ctx)
	}
	if err := models.MustFor(ctx).Model(&models.User{}).
		Where("id = ? AND tenant_id = ?", uid, tenantID).Updates(m).Error; err != nil {
		return models.User{}, err
	}
	return s.GetCurrent(ctx)
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

// ---------- 头像直传（OSS/COS 预签 URL） ----------

// UserStorageService 提供用户/租户侧的存储上传 URL（仅签名，不上传二进制）。
type UserStorageService struct{}

func NewUserStorage() *UserStorageService { return &UserStorageService{} }

var avatarMimeToExt = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
}

// GetAvatarUploadURL 为当前用户返回头像直传的预签 PUT URL。
// 前端流程：调本接口拿到 upload_url + object_key；PUT 文件到 upload_url；
// 再调 crm_user/UpdateProfile { avatar: object_key } 落到 User.Avatar。
func (s UserStorageService) GetAvatarUploadURL(ctx context.Context, mimeType string) (map[string]any, error) {
	uid := identity.UserID(ctx)
	tenantID := identity.TenantID(ctx)
	if uid == "" {
		return nil, errors.New("missing user identity")
	}
	ext, ok := avatarMimeToExt[mimeType]
	if !ok {
		return nil, errors.New("仅支持 png/jpeg/webp 图片")
	}
	if storage.FileStorage == nil {
		return nil, errors.New("存储未初始化")
	}
	objectKey := fmt.Sprintf("tenants/%s/users/%s/avatar-%d.%s", tenantID, uid, time.Now().Unix(), ext)
	uploadURL, err := storage.FileStorage.SignURL(objectKey, constants.HTTPPut, 3600)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"upload_url": uploadURL,
		"object_key": objectKey,
		"expires_in": 3600,
		"mime_type":  mimeType,
	}, nil
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
	if strings.TrimSpace(req.Name) == "" && strings.TrimSpace(req.Mobile) == "" {
		return req, errors.New("客户名称或手机号至少填写一项")
	}
	if req.PoolStatus == 1 {
		req.OwnerUserID = ""
	} else if req.OwnerUserID == "" {
		req.OwnerUserID = userID
	}
	if identity.UserID(ctx) != "" {
		if err := validateCustomerOwner(ctx, models.DB, req.OwnerUserID); err != nil {
			return req, err
		}
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
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("mobile = ? AND deleted_at IS NULL", req.Mobile).Count(&cnt)
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
	// SOP 触发：新客户
	NewCrmSop().Trigger(ctx, "new_customer", map[string]string{
		"customer_id": req.ID,
		"source":      req.Source,
	})
	return req, nil
}

func (s CrmCustomerService) Update(ctx context.Context, id string, req models.CrmCustomer) (models.CrmCustomer, error) {
	var result models.CrmCustomer
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error { var err error; result, err = s.updateCustomer(ctx, tx, id, req); return err })
	if err == nil {
		if linkErr := upsertCustomerChannelLink(ctx, result); linkErr != nil {
			log.Sugar.Errorw("crm customer channel link upsert failed", "err", linkErr)
		}
	}
	return result, err
}

func (s CrmCustomerService) updateCustomer(ctx context.Context, tx *gorm.DB, id string, req models.CrmCustomer) (models.CrmCustomer, error) {
	tenantID := identity.TenantID(ctx)
	db, err := customerAccessDB(ctx, tx, true)
	if err != nil {
		return req, err
	}
	if err := validateCustomerOwner(ctx, tx, req.OwnerUserID); err != nil {
		return req, err
	}

	var existing models.CrmCustomer
	if err = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", id).First(&existing).Error; err != nil {
		return req, ErrCustomerAccess
	}
	if existing.OwnerUserID != req.OwnerUserID {
		if existing.IsLocked || existing.ReassignStatus != 0 {
			return req, errors.New("客户已锁定或正在转交，不能直接修改归属")
		}
		if existing.PoolStatus == 1 && req.OwnerUserID != "" {
			return req, errors.New("公海客户请使用认领或分配操作")
		}
	}
	req.ID = id
	req.TenantID = tenantID
	upd := updatesCRM(map[string]any{
		"name": req.Name, "mobile": req.Mobile, "email": req.Email, "company": req.Company,
		"industry": req.Industry, "gender": req.Gender, "birthday": req.Birthday,
		"level": req.Level, "address": req.Address, "province": req.Province, "city": req.City, "district": req.District, "owner_user_id": req.OwnerUserID,
		"next_followup_at": req.NextFollowupAt, "remark": req.Remark, "tags": req.Tags,
	}, "owner_user_id")
	selectCols := []string{"name", "mobile", "email", "company", "industry", "gender", "birthday", "level", "address", "province", "city", "district", "owner_user_id", "next_followup_at", "remark", "tags"}
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
	} else {
		upd["pool_status"] = 1
		upd["pool_at"] = time.Now()
		upd["owner_user_id"] = nil
	}
	if existing.OwnerUserID != req.OwnerUserID && req.OwnerUserID == "" {
		for key, value := range recycledCustomerChanges(existing, time.Now()) {
			upd[key] = value
			selectCols = append(selectCols, key)
		}
		actor, _, err := customerActor(ctx, tx)
		if err != nil {
			return req, err
		}
		if err = poolLog(tx, tenantID, existing, "recycle", actor, "", "编辑客户移入公海"); err != nil {
			return req, err
		}
	} else if existing.OwnerUserID != req.OwnerUserID {
		upd["claimed_at"] = time.Now()
		selectCols = append(selectCols, "claimed_at")
		entry := models.CrmTransferLog{Model: models.Model{ID: id_generator.StringID()}, TenantID: tenantID, CustomerID: id, FromUserID: existing.OwnerUserID, ToUserID: req.OwnerUserID, OperatorUserID: identity.UserID(ctx), Reason: "编辑客户调整归属"}
		if err = createCRM(tx, &entry); err != nil {
			return req, err
		}
	}
	selectCols = append(selectCols, "pool_status", "pool_at")
	result := tx.Model(&models.CrmCustomer{}).Where("tenant_id = ? AND id = ?", tenantID, id).Select(selectCols).Updates(upd)
	err = result.Error
	if err == nil && result.RowsAffected == 0 {
		return req, ErrCustomerAccess
	}
	if err != nil {
		log.Sugar.Errorw("crm customer update failed", "err", err)
		return req, err
	}
	if err = tx.Where("tenant_id = ? AND id = ?", tenantID, id).First(&req).Error; err != nil {
		return req, err
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
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	var count int64
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		db, err := customerAccessDB(ctx, tx, true)
		if err != nil {
			return err
		}
		var rows []models.CrmCustomer
		if err := db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Find(&rows).Error; err != nil {
			return err
		}
		unique := make(map[string]bool)
		for _, id := range ids {
			unique[id] = true
		}
		if len(rows) != len(unique) {
			return ErrCustomerAccess
		}
		for _, row := range rows {
			if err := guardEntityReferences(tx, row.TenantID, "customer", row.ID, false); err != nil {
				return err
			}
			if err := recycleEntity(ctx, tx, "customer", row.ID, row.Name); err != nil {
				return err
			}
		}
		res := tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Delete(&models.CrmCustomer{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

func (s CrmCustomerService) Get(ctx context.Context, id string) (models.CrmCustomer, error) {
	var c models.CrmCustomer
	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return c, err
	}
	err = db.Where("id = ?", id).First(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = ErrCustomerAccess
	}
	return c, err
}

// GetDetail 客户 360 详情：客户主档(含企微渠道快照) + 跟进记录 timeline。
func (s CrmCustomerService) GetDetail(ctx context.Context, id string) (map[string]any, error) {

	c, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	var follows []models.CrmFollowup
	if err := models.MustFor(ctx).Where("customer_id = ?", id).
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
	if err := models.MustFor(ctx).Where("customer_id = ? AND channel <> ?", id, constants.ChannelWecom).
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

func (s CrmCustomerService) Query(ctx context.Context, keyword string, ownerUserID string, source string, outFlowStatus int, poolStatus int, category string, pager *app.Pager) ([]models.CrmCustomer, int64, error) {
	if ownerUserID == "__me__" {
		ownerUserID = identity.UserID(ctx)
	}

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
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
	// 分类筛选（参照 Salesforce List View）
	uid := identity.UserID(ctx)
	switch category {
	case "key":
		db = db.Where("is_key = ?", true)
	case "blacklist":
		db = db.Where("is_blacklist = ?", true)
	case "shared":
		db = db.Where("shared_staff_ids @> ?", constants.StringArrayField{uid})
	case "unreceive":
		db = db.Where("owner_user_id IS NULL AND pool_status = ?", 0)
	case "reassign":
		db = db.Where("reassign_status = ?", 1)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var cs []models.CrmCustomer
	err = db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("score desc, id desc").Find(&cs).Error
	return cs, total, err
}

// Export 导出客户列表为 Excel（最多 10000 行）。
func (s CrmCustomerService) Export(ctx context.Context, ownerUserID string) (*bytes.Buffer, string, error) {

	db, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, "", err
	}
	db = db.Limit(10000).Order("id desc")
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	var cs []models.CrmCustomer
	if err := db.Find(&cs).Error; err != nil {
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
			Model:      models.Model{ID: id_generator.StringID()},
			TenantID:   tenantID,
			Name:       r["姓名"],
			Mobile:     r["手机"],
			Company:    r["公司"],
			Source:     constants.SourceImport,
			PoolStatus: 1,
		}
		now := time.Now()
		c.PoolAt = &now
		if c.Name == "" && c.Mobile == "" {
			fail++
			continue
		}
		// 去重：mobile 非空时查同租户是否已存在
		if c.Mobile != "" {
			var cnt int64
			models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("mobile = ?", c.Mobile).Count(&cnt)
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
	req.ID, req.TenantID = id_generator.StringID(), identity.TenantID(ctx)
	if userID != identity.UserID(ctx) {
		return req, ErrCustomerAccess
	}
	if req.Status == "" {
		req.Status = "new"
	}
	if req.Status != "new" && req.Status != "contacted" {
		return req, errors.New("新线索必须为待跟进或已联系状态")
	}
	req.ConvertCustomerID, req.ConvertAt, req.ConvertRemark = "", nil, ""
	if req.PoolStatus == 1 {
		req.OwnerUserID = ""
		now := time.Now()
		req.PoolAt = &now
	} else {
		req.PoolStatus, req.PoolAt = 0, nil
		if req.OwnerUserID == "" {
			req.OwnerUserID = userID
		}
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateLead(ctx, tx, &req); err != nil {
			return err
		}
		return createCRM(tx, &req)
	})
	return req, err
}

func (s CrmLeadService) Update(ctx context.Context, id string, req models.CrmLead) (models.CrmLead, error) {
	var lead models.CrmLead
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		lead, err = lockSalesRecord[models.CrmLead](ctx, tx, id, false)
		if err != nil {
			return err
		}
		if err = mutableLead(lead); err != nil {
			return err
		}
		if lead.PoolStatus == 1 {
			return errors.New("请先领取线索再编辑")
		}
		if req.Status != "" && req.Status != lead.Status {
			return errors.New("请使用标记联系、流失或转客户操作变更状态")
		}
		req.ID, req.TenantID = lead.ID, lead.TenantID
		if req.OwnerUserID == "" {
			req.OwnerUserID = lead.OwnerUserID
		}
		if err = validateLead(ctx, tx, &req); err != nil {
			return err
		}
		changes := map[string]any{"title": req.Title, "company": req.Company, "name": req.Name, "mobile": req.Mobile, "source": req.Source, "remark": req.Remark, "email": req.Email, "position": req.Position, "address": req.Address, "province": req.Province, "city": req.City, "district": req.District, "owner_user_id": req.OwnerUserID, "customer_id": nullableSalesID(req.CustomerID)}
		if err = tx.Model(&lead).Updates(changes).Error; err != nil {
			return err
		}
		return tx.First(&lead, "id = ?", id).Error
	})
	return lead, err
}

func (s CrmLeadService) Delete(ctx context.Context, ids []string) (int64, error) {
	return mutateLeads(ctx, ids, func(tx *gorm.DB, lead *models.CrmLead) error {
		if lead.ConvertCustomerID != "" || lead.Status == "qualified" {
			return errors.New("已转化线索需要保留来源记录")
		}
		for _, model := range []any{&models.CrmFollowup{}, &models.CrmOpportunity{}} {
			var count int64
			if err := tx.Model(model).Where("tenant_id = ? AND lead_id = ?", lead.TenantID, lead.ID).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("线索已有跟进或商机记录，请保留或标记流失")
			}
		}
		if err := recycleEntity(ctx, tx, "lead", lead.ID, lead.Title); err != nil {
			return err
		}
		return tx.Delete(lead).Error
	})
}

// BatchImport 批量导入线索。rows 为解析后的行列映射。
// 导入的线索默认 status=new，source=import；无归属人时进入线索池。
func (s CrmLeadService) BatchImport(ctx context.Context, userID string, rows []map[string]string) (int, int, error) {
	tenantID := identity.TenantID(ctx)

	success, fail := 0, 0
	for _, r := range rows {
		lead := models.CrmLead{
			Model:    models.Model{ID: id_generator.StringID()},
			TenantID: tenantID,
			Title:    r["标题"],
			Name:     r["姓名"],
			Mobile:   r["手机"],
			Company:  r["公司"],
			Email:    r["邮箱"],
			Position: r["职位"],
			Source:   constants.SourceImport,
			Status:   "new",
		}
		// 标题为空且姓名为空时跳过
		if lead.Title == "" && lead.Name == "" {
			fail++
			continue
		}
		// 标题为空时用姓名补
		if lead.Title == "" {
			lead.Title = lead.Name
		}
		// 去重：mobile 非空时查同租户是否已存在
		if lead.Mobile != "" {
			var cnt int64
			models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("mobile = ?", lead.Mobile).Count(&cnt)
			if cnt > 0 {
				fail++
				continue
			}
		}
		// 无归属人 → 进入线索池
		if lead.OwnerUserID == "" {
			lead.PoolStatus = 1
			now := time.Now()
			lead.PoolAt = &now
		}
		if err := createCRM(models.DB, &lead, "CustomerID", "OwnerUserID", "ConvertCustomerID"); err != nil {
			log.Sugar.Errorw("batch import lead failed", "err", err, "title", lead.Title)
			fail++
			continue
		}
		success++
	}
	return success, fail, nil
}

func (s CrmLeadService) Get(ctx context.Context, id string) (models.CrmLead, error) {
	var lead models.CrmLead
	db, err := salesAccessDB(ctx, models.DB, true)
	if err != nil {
		return lead, err
	}
	err = db.Where("id = ?", id).First(&lead).Error
	return lead, err
}

func (s CrmLeadService) Query(ctx context.Context, status string, ownerUserID string, keyword string, pager *app.Pager) ([]models.CrmLead, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db, err := salesAccessDB(ctx, models.DB, true)
	if err != nil {
		return nil, 0, err
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("title LIKE ? OR name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmLead{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ls []models.CrmLead
	err = db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&ls).Error
	return ls, total, err
}

// ---------- CrmPipelineStage ----------

type CrmPipelineStageService struct{}

func NewCrmPipelineStage() *CrmPipelineStageService { return &CrmPipelineStageService{} }

func (s CrmPipelineStageService) Create(ctx context.Context, req models.CrmPipelineStage) (models.CrmPipelineStage, error) {
	req.ID, req.TenantID = id_generator.StringID(), identity.TenantID(ctx)
	if err := requireCRMManager(ctx, models.DB); err != nil {
		return req, err
	}
	if err := validatePipelineStage(&req); err != nil {
		return req, err
	}
	return req, createCRM(models.DB.WithContext(ctx), &req)
}

func (s CrmPipelineStageService) Update(ctx context.Context, id string, req models.CrmPipelineStage) (models.CrmPipelineStage, error) {
	req.ID, req.TenantID = id, identity.TenantID(ctx)
	if err := validatePipelineStage(&req); err != nil {
		return req, err
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		var old models.CrmPipelineStage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", req.TenantID, id).First(&old).Error; err != nil {
			return err
		}
		if old.IsWon != req.IsWon || old.IsLost != req.IsLost || old.Probability != req.Probability {
			var count int64
			if err := tx.Unscoped().Model(&models.CrmOpportunity{}).Where("tenant_id = ? AND stage_id = ?", req.TenantID, id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("阶段已有商机，不能改变终态或成交概率；请新增阶段后迁移")
			}
		}
		return tx.Model(&old).Updates(map[string]any{"name": req.Name, "sort": req.Sort, "probability": req.Probability, "is_won": req.IsWon, "is_lost": req.IsLost, "color": req.Color, "description": req.Description}).Error
	})
	return req, err
}

func (s CrmPipelineStageService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		var stages []models.CrmPipelineStage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Order("id").Find(&stages).Error; err != nil {
			return err
		}
		if len(stages) != len(ids) {
			return errors.New("销售阶段不存在")
		}
		var count int64
		if err := tx.Unscoped().Model(&models.CrmOpportunity{}).Where("tenant_id = ? AND stage_id IN ?", identity.TenantID(ctx), ids).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("销售阶段已有商机，不能删除")
		}
		return tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Delete(&models.CrmPipelineStage{}).Error
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
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
	req.ID, req.TenantID = id_generator.StringID(), identity.TenantID(ctx)
	if userID != identity.UserID(ctx) {
		return req, ErrCustomerAccess
	}
	if req.OwnerUserID == "" {
		req.OwnerUserID = userID
	}
	if req.Status != "" && req.Status != "open" {
		return req, errors.New("新商机应为进行中状态")
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateOpportunity(ctx, tx, &req); err != nil {
			return err
		}
		var stage models.CrmPipelineStage
		var err error
		if req.StageID != "" {
			stage, err = opportunityStage(ctx, tx, req.StageID)
		} else {
			err = tx.Where("tenant_id = ? AND is_won = false AND is_lost = false", req.TenantID).Order("sort, id").First(&stage).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = nil
			}
		}
		if err != nil {
			return err
		}
		if stage.IsWon || stage.IsLost {
			return errors.New("请先创建进行中的商机，再办理赢单或输单")
		}
		applyOpportunityStage(&req, stage)
		return createCRM(tx, &req)
	})
	return req, err
}

func (s CrmOpportunityService) Update(ctx context.Context, id string, req models.CrmOpportunity) (models.CrmOpportunity, error) {
	var o models.CrmOpportunity
	changed := false
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		o, err = lockSalesRecord[models.CrmOpportunity](ctx, tx, id, false)
		if err != nil {
			return err
		}
		if o.Status != "open" {
			return errors.New("已关闭商机请先恢复再编辑")
		}
		if req.Status != "" && req.Status != o.Status {
			return errors.New("请通过阶段或赢单/输单操作变更状态")
		}
		req.ID, req.TenantID = o.ID, o.TenantID
		if req.OwnerUserID == "" {
			req.OwnerUserID = o.OwnerUserID
		}
		if req.CustomerID != o.CustomerID {
			return errors.New("商机客户不能直接更换，请为新客户另建商机")
		}
		if err = validateOpportunity(ctx, tx, &req); err != nil {
			return err
		}
		o.Amount = req.Amount
		if req.StageID != "" && req.StageID != o.StageID {
			stage, err := opportunityStage(ctx, tx, req.StageID)
			if err != nil {
				return err
			}
			if err = moveOpportunity(ctx, tx, &o, stage); err != nil {
				return err
			}
			changed = true
		}
		// Preserve the stage-derived state; editing the form cannot bypass transition guards.
		updates := map[string]any{"lead_id": nullableSalesID(req.LeadID), "owner_user_id": req.OwnerUserID, "title": req.Title, "product": req.Product, "amount": req.Amount, "expected_close_date": req.ExpectedCloseDate, "remark": req.Remark}
		o.ExpectedRevenue = math.Round(o.Amount*float64(o.Probability)) / 100
		updates["expected_revenue"] = o.ExpectedRevenue
		if err = tx.Model(&o).Updates(updates).Error; err != nil {
			return err
		}
		return tx.First(&o, "id = ?", id).Error
	})
	if err == nil && changed {
		opportunityStageEvent(ctx, o)
	}
	return o, err
}

func (s CrmOpportunityService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			o, err := lockSalesRecord[models.CrmOpportunity](ctx, tx, id, false)
			if err != nil {
				return err
			}
			if o.Status != "open" {
				return errors.New("已关闭商机应保留业务记录")
			}
			for _, model := range []any{&models.CrmFollowup{}, &models.CrmQuote{}, &models.CrmOrder{}} {
				var count int64
				if err = tx.Model(model).Where("tenant_id = ? AND opportunity_id = ?", o.TenantID, id).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return errors.New("商机已有跟进、报价或合同，应保留业务记录")
				}
			}
			if err = recycleEntity(ctx, tx, "opportunity", o.ID, o.Title); err != nil {
				return err
			}
			if err = tx.Delete(&o).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func (s CrmOpportunityService) Get(ctx context.Context, id string) (models.CrmOpportunity, error) {
	var o models.CrmOpportunity
	db, err := salesAccessDB(ctx, models.DB, false)
	if err != nil {
		return o, err
	}
	err = db.Where("id = ?", id).First(&o).Error
	return o, err
}

func (s CrmOpportunityService) Query(ctx context.Context, status string, stageID string, ownerUserID string, keyword string, pager *app.Pager, customerIDs ...string) ([]models.CrmOpportunity, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db, err := salesAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	if len(customerIDs) > 0 && customerIDs[0] != "" {
		db = db.Where("customer_id = ?", customerIDs[0])
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if stageID != "" {
		db = db.Where("stage_id = ?", stageID)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	if kw := strings.TrimSpace(keyword); kw != "" {
		db = db.Where("title LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := db.Model(&models.CrmOpportunity{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var os []models.CrmOpportunity
	err = db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&os).Error
	return os, total, err
}

// MoveStage 移动商机阶段；到达赢单/输单终态时同步 status。
// 若租户开启了跟进强制校验，需近 N 天有跟进记录方可移动。
func (s CrmOpportunityService) MoveStage(ctx context.Context, id, stageID string) (models.CrmOpportunity, error) {
	if _, err := transitionOpportunities(ctx, []string{id}, stageID); err != nil {
		return models.CrmOpportunity{}, err
	}
	return s.Get(ctx, id)
}

// ---------- CrmFollowup ----------

type CrmFollowupService struct{}

func NewCrmFollowup() *CrmFollowupService { return &CrmFollowupService{} }

func (s CrmFollowupService) Create(ctx context.Context, userID string, req models.CrmFollowup) (models.CrmFollowup, error) {
	req.ID = id_generator.StringID()
	req.TenantID = identity.TenantID(ctx)
	req.UserID = identity.UserID(ctx)
	if req.UserID == "" || req.UserID != userID {
		return req, ErrCustomerAccess
	}
	req.Content = strings.TrimSpace(req.Content)
	if req.Content == "" {
		return req, errors.New("请填写跟进内容")
	}
	if req.Type == "" {
		req.Type = "other"
	}
	if req.Duration < 0 {
		return req, errors.New("跟进时长不能为负数")
	}
	switch req.Type {
	case "call", "phone", "visit", "message", "wechat", "email", "meeting", "other":
	default:
		return req, errors.New("跟进方式无效")
	}
	switch req.Result {
	case "", "positive", "neutral", "negative":
	default:
		return req, errors.New("跟进结果无效")
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if req.ContactID != "" {
			if err := contactWriteLock(ctx, tx); err != nil {
				return err
			}
		}
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		if req.OpportunityID != "" {
			var o models.CrmOpportunity
			if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", req.TenantID, req.OpportunityID).First(&o).Error; err != nil {
				return errors.New("关联商机不存在")
			}
			if !manager && o.OwnerUserID != actor.ID {
				return ErrCustomerAccess
			}
			if req.CustomerID != "" && req.CustomerID != o.CustomerID {
				return errors.New("跟进客户与商机客户不一致")
			}
			req.CustomerID = o.CustomerID
		}
		if req.LeadID != "" {
			var l models.CrmLead
			if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", req.TenantID, req.LeadID).First(&l).Error; err != nil {
				return errors.New("关联线索不存在")
			}
			if !manager && l.OwnerUserID != actor.ID {
				return ErrCustomerAccess
			}
			if err = mutableLead(l); err != nil {
				return err
			}
			if l.PoolStatus == 1 {
				return errors.New("请先领取线索再跟进")
			}
			if req.CustomerID != "" && req.CustomerID != l.CustomerID {
				return errors.New("跟进客户与线索客户不一致")
			}
			req.CustomerID = l.CustomerID
		}
		if req.ContactID != "" && req.CustomerID == "" {
			return errors.New("请选择联系人所属客户")
		}
		if req.CustomerID == "" && req.LeadID == "" && req.OpportunityID == "" {
			return errors.New("请选择跟进客户、线索或商机")
		}
		if req.CustomerID != "" {
			db, err := customerAccessDB(ctx, tx, true)
			if err != nil {
				return err
			}
			var customer models.CrmCustomer
			if err = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", req.CustomerID).First(&customer).Error; err != nil {
				return ErrCustomerAccess
			}
			if customer.ReassignStatus == 1 {
				return errors.New("请先接收转交客户再跟进")
			}
			if customer.PoolStatus == 1 {
				return errors.New("请先领取公海客户再跟进")
			}
			if req.ContactID != "" {
				var contact models.CrmContact
				if err = tx.Where("tenant_id = ? AND id = ? AND customer_id = ?", req.TenantID, req.ContactID, req.CustomerID).First(&contact).Error; err != nil {
					return errors.New("联系人不属于当前客户")
				}
			}
			// A new follow-up supersedes the previous reminder, including clearing a completed reminder.
			if err = tx.Model(&customer).Update("next_followup_at", req.NextFollowupAt).Error; err != nil {
				return err
			}
		}
		return createCRM(tx, &req)
	})
	if err == nil && req.CustomerID != "" {
		TriggerScoreUpdate(req.TenantID, req.CustomerID)
	}
	return req, err
}

func (s CrmFollowupService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		var rows []models.CrmFollowup
		if err = tx.Where("tenant_id = ? AND id IN ?", actor.TenantID, ids).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return errors.New("跟进记录不存在或已删除")
		}
		// Lock lead parents before customers, matching conversion's lock order.
		var leads []models.CrmLead
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN (?)", actor.TenantID, tx.Model(&models.CrmFollowup{}).Select("lead_id").Where("tenant_id = ? AND id IN ?", actor.TenantID, ids)).Order("id").Find(&leads).Error; err != nil {
			return err
		}
		if err = tx.Where("tenant_id = ? AND id IN ?", actor.TenantID, ids).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		customerIDs := []string{}
		for _, row := range rows {
			if !manager && row.UserID != actor.ID {
				return ErrCustomerAccess
			}
			if row.CustomerID != "" {
				customerIDs = append(customerIDs, row.CustomerID)
			}
		}
		var customers []models.CrmCustomer
		access, err := customerAccessDB(ctx, tx, true)
		if err != nil {
			return err
		}
		if len(customerIDs) > 0 {
			if err = access.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", customerIDs).Order("id").Find(&customers).Error; err != nil {
				return err
			}
			allowed := map[string]bool{}
			for _, c := range customers {
				allowed[c.ID] = true
			}
			for _, id := range customerIDs {
				if !allowed[id] {
					return ErrCustomerAccess
				}
			}
		}
		for _, l := range leads {
			if !manager && l.OwnerUserID != actor.ID {
				return ErrCustomerAccess
			}
		}
		res := tx.Where("tenant_id = ? AND id IN ?", actor.TenantID, ids).Delete(&models.CrmFollowup{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != int64(len(ids)) {
			return errors.New("跟进记录已变化，请刷新后重试")
		}
		for _, c := range customers {
			var latest models.CrmFollowup
			err = tx.Where("tenant_id = ? AND customer_id = ?", actor.TenantID, c.ID).Order("created_at desc, id desc").First(&latest).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			if err = tx.Model(&c).Update("next_followup_at", latest.NextFollowupAt).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func (s CrmFollowupService) ListByCustomer(ctx context.Context, customerID string) ([]models.CrmFollowup, error) {
	if _, err := NewCrmCustomer().Get(ctx, customerID); err != nil {
		return nil, err
	}
	var rows []models.CrmFollowup
	err := models.MustFor(ctx).Where("customer_id = ?", customerID).Order("created_at desc, id desc").Find(&rows).Error
	return rows, err
}

func (s CrmFollowupService) ListByOpportunity(ctx context.Context, opportunityID string) ([]models.CrmFollowup, error) {
	if _, err := NewCrmOpportunity().Get(ctx, opportunityID); err != nil {
		return nil, err
	}
	var rows []models.CrmFollowup
	err := models.MustFor(ctx).Where("opportunity_id = ?", opportunityID).Order("created_at desc, id desc").Find(&rows).Error
	return rows, err
}

func (s CrmFollowupService) ListByLead(ctx context.Context, leadID string) ([]models.CrmFollowup, error) {
	if _, err := NewCrmLead().Get(ctx, leadID); err != nil {
		return nil, err
	}
	var rows []models.CrmFollowup
	err := models.MustFor(ctx).Where("lead_id = ?", leadID).Order("created_at desc, id desc").Find(&rows).Error
	return rows, err
}

// FollowupReminder 客户跟进提醒，逾期天数按当前时间计算。
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

func (s CrmOrderService) Create(ctx context.Context, userID string, req models.CrmOrder, couponCode string) (models.CrmOrder, error) {
	req = newOrder(ctx, userID, req)
	if req.PaidAmount != 0 {
		return req, errors.New("请先保存合同，再通过回款登记记录已收款")
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateOrder(ctx, tx, &req); err != nil {
			return err
		}
		return createOrderWithCoupon(ctx, tx, &req, strings.TrimSpace(couponCode))
	})
	if err == nil {
		TriggerScoreUpdate(req.TenantID, req.CustomerID)
		NewCrmSop().Trigger(ctx, "order_created", map[string]string{"customer_id": req.CustomerID, "order_id": req.ID, "order_status": req.Status})
	}
	return req, err
}

func (s CrmOrderService) Update(ctx context.Context, id string, req models.CrmOrder) (models.CrmOrder, error) {
	return s.updateContract(ctx, id, req)
}

func (s CrmOrderService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			order, err := lockPaymentOrder(ctx, tx, id)
			if err != nil {
				return err
			}
			if order.Status != "draft" && order.Status != "cancelled" {
				return errors.New("已签约合同请先作废，不能直接删除")
			}
			if order.PaidAmount > 0 || order.CouponID != "" {
				return errors.New("已有收款或优惠券的合同应保留业务记录")
			}
			for _, model := range []any{&models.CrmReceivables{}, &models.CrmReceivablesPlan{}, &models.CrmQuote{}} {
				var count int64
				if err = tx.Model(model).Where("tenant_id = ? AND order_id = ?", order.TenantID, id).Count(&count).Error; err != nil {
					return err
				}
				if count > 0 {
					return errors.New("合同已有回款或计划，请保留合同及其关联记录")
				}
			}
			if err = guardEntityReferences(tx, order.TenantID, "order", order.ID, false); err != nil {
				return err
			}
			if err = recycleEntity(ctx, tx, "order", order.ID, order.Title); err != nil {
				return err
			}
			if err = tx.Delete(&order).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func (s CrmOrderService) Query(ctx context.Context, status string, ownerUserID string, pager *app.Pager) ([]models.CrmOrder, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	actor, manager, err := customerActor(ctx, models.DB)
	if err != nil {
		return nil, 0, err
	}
	db := models.MustFor(ctx)
	if !manager {
		db = db.Where("user_id = ?", actor.ID)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if ownerUserID != "" {
		db = db.Where("user_id = ?", ownerUserID)
	}
	var total int64
	if err := db.Model(&models.CrmOrder{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var os []models.CrmOrder
	err = db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&os).Error
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
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("status = ?", "open").Count(&n)
		return n
	}()
	sum.WonOpportunity = func() int64 {
		var n int64
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("status = ?", "won").Count(&n)
		return n
	}()
	sum.OrderCount = count(&models.CrmOrder{})
	var amt struct {
		Total float64 `gorm:"column:total"`
	}
	err := models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("status IN ?", []string{"signed", "completed"}).
		Select("COALESCE(SUM(amount),0) AS total").Scan(&amt).Error
	if err != nil {
		return sum, err
	}
	sum.OrderAmount = amt.Total
	return sum, nil
}

// PipelineFunnel 销售漏斗：open 商机按阶段聚合金额/数量。
func (s CrmReportService) PipelineFunnel(ctx context.Context, ownerUserID string) ([]map[string]any, error) {

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
	q := models.MustFor(ctx).Model(&models.CrmOpportunity{}).Where("status = ?", "open")
	if ownerUserID != "" {
		q = q.Where("owner_user_id = ?", ownerUserID)
	}
	if err := q.Select("stage_id, COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS amount").
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

// PipelineInspection 销售管线分析：对标 Salesforce Pipeline Inspection。
// 汇总各阶段的商机数量、金额、加权金额、平均停留天数，以及整体赢单/输单率。
func (s CrmReportService) PipelineInspection(ctx context.Context, ownerUserID string) (map[string]any, error) {
	stages, err := NewCrmPipelineStage().List(ctx)
	if err != nil {
		return nil, err
	}

	// 各阶段 open 商机聚合
	type stageRow struct {
		StageID string  `gorm:"column:stage_id"`
		Cnt     int64   `gorm:"column:cnt"`
		Amount  float64 `gorm:"column:amount"`
	}
	var openRows []stageRow
	openQ := models.MustFor(ctx).Model(&models.CrmOpportunity{}).Where("status = ?", "open")
	if ownerUserID != "" {
		openQ = openQ.Where("owner_user_id = ?", ownerUserID)
	}
	if err := openQ.Select("stage_id, COUNT(*) AS cnt, COALESCE(SUM(amount),0) AS amount").
		Group("stage_id").Scan(&openRows).Error; err != nil {
		return nil, err
	}
	openMap := map[string]stageRow{}
	for _, r := range openRows {
		openMap[r.StageID] = r
	}

	// 各阶段平均停留天数（open 商机从 created_at 到现在）
	type ageRow struct {
		StageID string  `gorm:"column:stage_id"`
		AvgDays float64 `gorm:"column:avg_days"`
	}
	var ageRows []ageRow
	ageQ := models.MustFor(ctx).Model(&models.CrmOpportunity{}).Where("status = ?", "open")
	if ownerUserID != "" {
		ageQ = ageQ.Where("owner_user_id = ?", ownerUserID)
	}
	if err := ageQ.Select("stage_id, AVG(EXTRACT(EPOCH FROM (NOW() - created_at)) / 86400) AS avg_days").
		Group("stage_id").Scan(&ageRows).Error; err != nil {
		return nil, err
	}
	ageMap := map[string]float64{}
	for _, r := range ageRows {
		ageMap[r.StageID] = r.AvgDays
	}

	// 赢单/输单统计
	type wonLostRow struct {
		Status string  `gorm:"column:status"`
		Cnt    int64   `gorm:"column:cnt"`
		Amount float64 `gorm:"column:amount"`
	}
	var wlRows []wonLostRow
	wlQ := models.MustFor(ctx).Model(&models.CrmOpportunity{}).Where("status IN ?", []string{"won", "lost"})
	if ownerUserID != "" {
		wlQ = wlQ.Where("owner_user_id = ?", ownerUserID)
	}
	if err := wlQ.Select("status, COUNT(*) AS cnt, COALESCE(SUM(actual_amount),0) AS amount").
		Group("status").Scan(&wlRows).Error; err != nil {
		return nil, err
	}
	var wonCnt, lostCnt int64
	var wonAmount, lostAmount float64
	for _, r := range wlRows {
		if r.Status == "won" {
			wonCnt = r.Cnt
			wonAmount = r.Amount
		} else {
			lostCnt = r.Cnt
			lostAmount = r.Amount
		}
	}
	totalClosed := wonCnt + lostCnt
	winRate := 0.0
	if totalClosed > 0 {
		winRate = float64(wonCnt) / float64(totalClosed) * 100
	}

	// 总 open 金额
	var totalOpenAmount float64
	var totalOpenCount int64
	for _, r := range openRows {
		totalOpenAmount += r.Amount
		totalOpenCount += r.Cnt
	}

	// 阶段明细
	stageDetails := make([]map[string]any, 0, len(stages))
	for _, st := range stages {
		r := openMap[st.ID]
		weighted := r.Amount * float64(st.Probability) / 100
		stageDetails = append(stageDetails, map[string]any{
			"stage_id":    st.ID,
			"stage_name":  st.Name,
			"sort":        st.Sort,
			"probability": st.Probability,
			"color":       st.Color,
			"is_won":      st.IsWon,
			"is_lost":     st.IsLost,
			"open_count":  r.Cnt,
			"open_amount": r.Amount,
			"weighted":    weighted,
			"avg_days":    ageMap[st.ID],
		})
	}

	return map[string]any{
		"stages":            stageDetails,
		"total_open_count":  totalOpenCount,
		"total_open_amount": totalOpenAmount,
		"won_count":         wonCnt,
		"won_amount":        wonAmount,
		"lost_count":        lostCnt,
		"lost_amount":       lostAmount,
		"win_rate":          winRate,
	}, nil
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
	if err := models.MustFor(ctx).Where("channel = ? AND entity_type = ?", constants.ChannelWecom, "tenant").
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
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source <> ''").
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
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source = ?", ch).Count(&row.CustomerCount)
		// 线索数（CrmLead.Source）
		models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("source = ?", ch).Count(&row.LeadCount)
		// 商机数：通过客户 source 关联
		models.DB.Model(&models.CrmOpportunity{}).
			Joins("JOIN crm_customer ON crm_customer.id = crm_opportunity.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_customer.source = ?", ch).
			Count(&row.OpportunityCount)
		// 赢单数
		models.DB.Model(&models.CrmOpportunity{}).
			Joins("JOIN crm_customer ON crm_customer.id = crm_opportunity.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_customer.source = ? AND crm_opportunity.status = ?", ch, "won").
			Count(&row.WonCount)
		// 订单金额
		var amt struct {
			Total float64 `gorm:"column:total"`
		}
		models.DB.Model(&models.CrmOrder{}).
			Joins("JOIN crm_customer ON crm_customer.id = crm_order.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_customer.source = ? AND crm_order.status IN ?", ch, []string{"signed", "completed"}).
			Select("COALESCE(SUM(crm_order.amount),0) AS total").Scan(&amt)
		row.OrderAmount = amt.Total
		// 互动数
		models.DB.Model(&models.CrmChannelInteraction{}).Scopes(models.TenantScope(tenantID)).Where("channel = ?", ch).Count(&row.InteractionCount)
		// 转化率
		row.ConversionRate = calcChannelConversionRate(row.OpportunityCount, row.WonCount)
		// 获客成本：渠道活动总成本 / 新客户数
		var costAmt struct {
			Total float64 `gorm:"column:total"`
		}
		models.DB.Model(&models.CrmActivity{}).Scopes(models.TenantScope(tenantID)).Where("(channel = ? OR channels @> ?)", ch, `["`+ch+`"]`).
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
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ?", u.ID).Count(&custCnt)
		row.CustomerCount = int(custCnt)
		// 商机数
		var oppCnt int64
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ?", u.ID).Count(&oppCnt)
		row.OpportunityCnt = int(oppCnt)
		// 赢单数
		var wonCnt int64
		models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ? AND status = ?", u.ID, "won").Count(&wonCnt)
		row.WonCount = int(wonCnt)
		// 订单金额（按签约销售）
		models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND status <> ?", u.ID, "cancelled").
			Select("COALESCE(SUM(amount), 0)").Scan(&row.OrderAmount)
		// 业绩目标
		if yearMonth != "" {
			var q models.CrmQuota
			if err := models.MustFor(ctx).Where("user_id = ? AND year_month = ?", u.ID, yearMonth).First(&q).Error; err == nil {
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
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL").Count(&total)
	now := time.Now()
	d30 := now.AddDate(0, 0, -30)
	d90 := now.AddDate(0, 0, -90)
	d180 := now.AddDate(0, 0, -180)
	// 用 updated_at 近似最后互动时间
	var newCnt, activeCnt, sleepingCnt, churnedCnt int64
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at >= ?", d30).Count(&newCnt)
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at >= ? AND updated_at < ?", d90, d30).Count(&activeCnt)
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at >= ? AND updated_at < ?", d180, d90).Count(&sleepingCnt)
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at < ?", d180).Count(&churnedCnt)
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
		models.DB.Model(&models.CrmActivity{}).Scopes(models.TenantScope(tenantID)).Where("(channel = ? OR channels @> ?)", ch, `["`+ch+`"]`).
			Select("COALESCE(SUM(cost), 0)").Scan(&row.TotalCost)
		// 该渠道新客户数
		var newCnt int64
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source = ?", ch).Count(&newCnt)
		row.NewCustomerCnt = int(newCnt)
		if row.NewCustomerCnt > 0 {
			row.CAC = row.TotalCost / float64(row.NewCustomerCnt)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// ---------- 管理视角看板：业务健康度 / 员工效能 / 风险预警 ----------

// HealthResult 业务健康度。
type HealthResult struct {
	FollowupRate      float64 `json:"followup_rate"`      // 跟进率(%) = 有跟进记录的客户数/总客户数
	ConversionRate    float64 `json:"conversion_rate"`    // 线索转化率(%) = 已转化线索数/总线索数
	ChurnRate         float64 `json:"churn_rate"`         // 客户流失率(%) = 流失客户数/总客户数
	WonRate           float64 `json:"won_rate"`           // 赢单率(%) = 赢单商机数/总商机数(已结束)
	TotalCustomers    int64   `json:"total_customers"`    // 总客户数
	FollowedCustomers int64   `json:"followed_customers"` // 有跟进记录的客户数
	TotalLeads        int64   `json:"total_leads"`        // 总线索数
	ConvertedLeads    int64   `json:"converted_leads"`    // 已转化线索数
	ChurnedCustomers  int64   `json:"churned_customers"`  // 流失客户数
	TotalClosedOpp    int64   `json:"total_closed_opp"`   // 已结束商机数(赢单+输单)
	WonOpp            int64   `json:"won_opp"`            // 赢单商机数
}

// Health 业务健康度：跟进率、线索转化率、客户流失率、赢单率。
func (s CrmReportService) Health(ctx context.Context) (HealthResult, error) {
	tenantID := identity.TenantID(ctx)
	var h HealthResult

	// 总客户数
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL").Count(&h.TotalCustomers)
	// 流失客户数
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND out_flow_status = 1").Count(&h.ChurnedCustomers)
	// 有跟进记录的客户数（COUNT(DISTINCT customer_id)）
	var followedCount int64
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).
		Where("customer_id IS NOT NULL AND customer_id <> ''").
		Select("COUNT(DISTINCT customer_id)").Scan(&followedCount)
	h.FollowedCustomers = followedCount

	// 总线索数
	models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Count(&h.TotalLeads)
	// 已转化线索数
	models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("convert_customer_id IS NOT NULL AND convert_customer_id <> ''").Count(&h.ConvertedLeads)

	// 赢单/已结束商机
	models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("status = 'won'").Count(&h.WonOpp)
	models.DB.Model(&models.CrmOpportunity{}).Scopes(models.TenantScope(tenantID)).Where("status IN ('won','lost')").Count(&h.TotalClosedOpp)

	// 计算比率
	if h.TotalCustomers > 0 {
		h.FollowupRate = float64(h.FollowedCustomers) / float64(h.TotalCustomers) * 100
		h.ChurnRate = float64(h.ChurnedCustomers) / float64(h.TotalCustomers) * 100
	}
	if h.TotalLeads > 0 {
		h.ConversionRate = float64(h.ConvertedLeads) / float64(h.TotalLeads) * 100
	}
	if h.TotalClosedOpp > 0 {
		h.WonRate = float64(h.WonOpp) / float64(h.TotalClosedOpp) * 100
	}
	return h, nil
}

// StaffEfficiencyRow 员工效能行。
type StaffEfficiencyRow struct {
	UserID        string  `json:"user_id"`
	UserName      string  `json:"user_name"`
	CustomerCount int     `json:"customer_count"` // 负责客户数
	FollowupCount int     `json:"followup_count"` // 跟进次数
	OrderCount    int     `json:"order_count"`    // 订单数
	OrderAmount   float64 `json:"order_amount"`   // 订单金额
	AvgFollowup   float64 `json:"avg_followup"`   // 人均跟进次数(=跟进次数/客户数)
}

// StaffEfficiency 员工效能：人均客户数、人均跟进数、人均订单。
func (s CrmReportService) StaffEfficiency(ctx context.Context) ([]StaffEfficiencyRow, error) {
	tenantID := identity.TenantID(ctx)

	var users []models.User
	if err := models.MustFor(ctx).Where("status = 1").Find(&users).Error; err != nil {
		return nil, err
	}
	rows := make([]StaffEfficiencyRow, 0, len(users))
	for _, u := range users {
		row := StaffEfficiencyRow{UserID: u.ID, UserName: u.Name}
		// 负责客户数
		var custCnt int64
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ? AND deleted_at IS NULL", u.ID).Count(&custCnt)
		row.CustomerCount = int(custCnt)
		// 跟进次数
		var fuCnt int64
		models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ?", u.ID).Count(&fuCnt)
		row.FollowupCount = int(fuCnt)
		// 订单数
		var ordCnt int64
		models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND status <> 'cancelled'", u.ID).Count(&ordCnt)
		row.OrderCount = int(ordCnt)
		// 订单金额
		models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND status <> 'cancelled'", u.ID).
			Select("COALESCE(SUM(amount), 0)").Scan(&row.OrderAmount)
		// 人均跟进
		if row.CustomerCount > 0 {
			row.AvgFollowup = float64(row.FollowupCount) / float64(row.CustomerCount)
		}
		rows = append(rows, row)
	}
	// 按订单金额降序
	sort.Slice(rows, func(i, j int) bool { return rows[i].OrderAmount > rows[j].OrderAmount })
	return rows, nil
}

// RiskAlertItem 风险预警项。
type RiskAlertItem struct {
	Type       string `json:"type"`  // overdue_followup/churned_customer/no_followup
	Label      string `json:"label"` // 预警类型标签
	CustomerID string `json:"customer_id"`
	Customer   string `json:"customer"` // 客户名称
	OwnerID    string `json:"owner_id"`
	Owner      string `json:"owner"`  // 负责销售
	Days       int    `json:"days"`   // 逾期天数/未跟进天数
	Detail     string `json:"detail"` // 详情
}

// RiskAlerts 风险预警：逾期跟进、流失客户、长期未跟进客户。
func (s CrmReportService) RiskAlerts(ctx context.Context) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)
	now := time.Now()
	d30 := now.AddDate(0, 0, -30)
	d7 := now.AddDate(0, 0, -7)

	// 1. 逾期跟进：next_followup_at < 今天 且未完成
	var overdue []RiskAlertItem
	var overdueCustomers []models.CrmCustomer
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND next_followup_at IS NOT NULL AND next_followup_at < ?",
		now).Limit(20).Find(&overdueCustomers)
	for _, c := range overdueCustomers {
		if c.NextFollowupAt == nil {
			continue
		}
		days := int(now.Sub(*c.NextFollowupAt).Hours() / 24)
		if days < 0 {
			days = 0
		}
		overdue = append(overdue, RiskAlertItem{
			Type: "overdue_followup", Label: "逾期跟进",
			CustomerID: c.ID, Customer: c.Name, OwnerID: c.OwnerUserID, Days: days,
		})
	}

	// 2. 流失客户（近7天新增流失）
	var churned []RiskAlertItem
	var churnedCustomers []models.CrmCustomer
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND out_flow_status = 1 AND updated_at >= ?", d7).Limit(20).Find(&churnedCustomers)
	for _, c := range churnedCustomers {
		churned = append(churned, RiskAlertItem{
			Type: "churned_customer", Label: "客户流失",
			CustomerID: c.ID, Customer: c.Name, OwnerID: c.OwnerUserID,
		})
	}

	// 3. 长期未跟进（30天无跟进记录）
	var noFollowup []RiskAlertItem
	var noFollowupCustomers []models.CrmCustomer
	models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("deleted_at IS NULL AND updated_at < ?", d30).Limit(20).Find(&noFollowupCustomers)
	for _, c := range noFollowupCustomers {
		days := int(now.Sub(c.UpdatedAt).Hours() / 24)
		noFollowup = append(noFollowup, RiskAlertItem{
			Type: "no_followup", Label: "长期未跟进",
			CustomerID: c.ID, Customer: c.Name, OwnerID: c.OwnerUserID, Days: days,
		})
	}

	return map[string]any{
		"overdue_followup":  overdue,
		"churned_customer":  churned,
		"no_followup":       noFollowup,
		"overdue_count":     len(overdue),
		"churned_count":     len(churned),
		"no_followup_count": len(noFollowup),
	}, nil
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
	models.DB.Model(&models.CrmActivityRecord{}).Scopes(models.TenantScope(tenantID)).Where("activity_id = ?", id).Count(&recCnt)
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

	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmActivity{})
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

	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmChannelTag{})
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
	models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("customer_id = ? AND remark LIKE ?", customerID, "%"+activityID+"%").Count(&cnt)
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
	if err := models.MustFor(ctx).Where("id IN ?", ids).Find(&recs).Error; err != nil {
		return 0, err
	}
	if len(recs) == 0 {
		return 0, nil
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmActivityRecord{})
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
	return recomputeActivityCounters(models.DB.WithContext(ctx), identity.TenantID(ctx), activityID)
}

func recomputeActivityCounters(db *gorm.DB, tenantID, activityID string) error {

	var perType []struct {
		RecordType string
		N          int
	}
	if err := db.Model(&models.CrmActivityRecord{}).
		Select("record_type, COUNT(*) AS n").Scopes(models.TenantScope(tenantID)).Where("activity_id = ?", activityID).
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
	return db.Model(&models.CrmActivity{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", activityID).
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
	cnt := models.DB.Model(&models.CrmActivityRecord{}).Scopes(models.TenantScope(tenantID)).Where("activity_id = ?", activityID)
	if recordType != "" {
		cnt = cnt.Where("record_type = ?", recordType)
	}
	if err := cnt.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	db := models.DB.Table("crm_activity_record").
		Select("crm_activity_record.*, crm_customer.name AS customer_name").
		Joins("LEFT JOIN crm_customer ON crm_customer.id = crm_activity_record.customer_id").Scopes(models.TenantScope(tenantID)).Where("crm_activity_record.activity_id = ?", activityID)
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
		Joins("LEFT JOIN crm_activity ON crm_activity.id = crm_activity_record.activity_id").Scopes(models.TenantScope(tenantID)).Where("crm_activity_record.customer_id = ?", customerID).
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

	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmChannelInteraction{})
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
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if cfg.Channel == constants.ChannelWecom {
			if err := models.EnsureWecomTenantLink(tx, tenantID, cfg.CorpID); err != nil {
				return err
			}
		}
		return createCRM(tx, &cfg)
	}); err != nil {
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
	if cfg.Channel == constants.ChannelWecom && ((req.CorpID != "" && req.CorpID != cfg.CorpID) || (req.Channel != "" && req.Channel != cfg.Channel)) {
		return toChannelConfigVO(cfg), errors.New("企微配置的企业和渠道不可改绑，请新增配置并停用原配置")
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
	if err := validateChannelConfig(&cfg); err != nil {
		return toChannelConfigVO(cfg), err
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
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		if cfg.Channel == constants.ChannelWecom {
			if err := models.EnsureWecomTenantLink(tx, cfg.TenantID, cfg.CorpID); err != nil {
				return err
			}
		}
		return tx.Model(&models.CrmChannelConfig{}).Where("id = ? AND tenant_id = ?", id, cfg.TenantID).Updates(map[string]any{
			"channel": cfg.Channel, "corp_id": cfg.CorpID, "name": cfg.Name,
			"contact_secret": cfg.ContactSecret, "customer_secret": cfg.CustomerSecret,
			"main_agent_id": cfg.MainAgentID, "main_agent_secret": cfg.MainAgentSecret,
			"callback_token": cfg.CallbackToken, "callback_aes_key": cfg.CallbackAesKey,
			"callback_url": cfg.CallbackURL, "enabled": cfg.Enabled, "remark": cfg.Remark,
			"xhs_app_secret": cfg.XhsAppSecret,
			"gzh_app_id":     cfg.GzhAppID, "gzh_app_secret": cfg.GzhAppSecret,
			"douyin_app_id": cfg.DouyinAppID, "douyin_app_secret": cfg.DouyinAppSecret,
		}).Error
	}); err != nil {
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
	models.MustFor(ctx).Where("id IN ?", ids).Find(&cfgs)
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmChannelConfig{})
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
	cfg.CorpID = strings.TrimSpace(cfg.CorpID)
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

// RestoreChannelClients restores persisted WeCom configs after a process restart.
func RestoreChannelClients() error {
	we_work.EnsureClients()
	var configs []models.CrmChannelConfig
	if err := models.DB.Where("channel = ? AND enabled = ?", constants.ChannelWecom, 1).Find(&configs).Error; err != nil {
		return err
	}
	for _, cfg := range configs {
		registerWecomClient(cfg)
	}
	return nil
}
