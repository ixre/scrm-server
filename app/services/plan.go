package services

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// PlanService 套餐管理服务（平台超管调用）。
type PlanService struct{}

func NewPlanService() *PlanService { return &PlanService{} }

// PlanCreateReq 创建套餐入参。
type PlanCreateReq struct {
	Code                string                         `json:"code"`
	Name                string                         `json:"name"`
	SortOrder           int                            `json:"sort_order"`
	Description         string                         `json:"description"`
	MaxUsers            int                            `json:"max_users"`
	MaxCustomers        int                            `json:"max_customers"`
	Modules             []string                       `json:"modules"`
	MonthlyPrice        int64                          `json:"monthly_price"`
	MonthlyRenewPrice   int64                          `json:"monthly_renew_price"`
	QuarterlyPrice      int64                          `json:"quarterly_price"`
	QuarterlyRenewPrice int64                          `json:"quarterly_renew_price"`
	YearlyPrice         int64                          `json:"yearly_price"`
	YearlyRenewPrice    int64                          `json:"yearly_renew_price"`
	MultiYearDiscounts  []models.PlanMultiYearDiscount `json:"multi_year_discounts"`
	IsDefault           int                            `json:"is_default"`
	IsTrial             int                            `json:"is_trial"`
	TrialDays           int                            `json:"trial_days"`
	IsFree              int                            `json:"is_free"`
	FreeDays            int                            `json:"free_days"`
}

// PlanUpdateReq 更新套餐入参（Code 不可改）。
type PlanUpdateReq struct {
	Name                string                         `json:"name"`
	SortOrder           int                            `json:"sort_order"`
	Status              int                            `json:"status"`
	Description         string                         `json:"description"`
	MaxUsers            int                            `json:"max_users"`
	MaxCustomers        int                            `json:"max_customers"`
	Modules             []string                       `json:"modules"`
	MonthlyPrice        int64                          `json:"monthly_price"`
	MonthlyRenewPrice   int64                          `json:"monthly_renew_price"`
	QuarterlyPrice      int64                          `json:"quarterly_price"`
	QuarterlyRenewPrice int64                          `json:"quarterly_renew_price"`
	YearlyPrice         int64                          `json:"yearly_price"`
	YearlyRenewPrice    int64                          `json:"yearly_renew_price"`
	MultiYearDiscounts  []models.PlanMultiYearDiscount `json:"multi_year_discounts"`
	IsDefault           int                            `json:"is_default"`
	IsTrial             int                            `json:"is_trial"`
	TrialDays           int                            `json:"trial_days"`
	IsFree              int                            `json:"is_free"`
	FreeDays            int                            `json:"free_days"`
}

// validModuleKeys 返回所有合法模块 key 集合
func validModuleKeys() map[string]bool {
	m := make(map[string]bool, len(models.PlanModuleDefs))
	for _, d := range models.PlanModuleDefs {
		m[d.Key] = true
	}
	return m
}

// validateModules 校验模块 key 合法性
func validateModules(modules []string) error {
	valid := validModuleKeys()
	for _, k := range modules {
		if !valid[k] {
			return errors.New("无效的模块: " + k)
		}
	}
	return nil
}

// normalizeModules 去重 + 排序
func normalizeModules(modules []string) []string {
	seen := make(map[string]bool, len(modules))
	out := make([]string, 0, len(modules))
	for _, k := range modules {
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}

// normalizeDiscounts 按年数排序、去重
func normalizeDiscounts(discounts []models.PlanMultiYearDiscount) []models.PlanMultiYearDiscount {
	seen := make(map[int]bool)
	out := make([]models.PlanMultiYearDiscount, 0, len(discounts))
	for _, d := range discounts {
		if d.Years < 1 {
			continue
		}
		if seen[d.Years] {
			continue
		}
		seen[d.Years] = true
		if d.Discount <= 0 || d.Discount > 100 {
			d.Discount = 100
		}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Years < out[j].Years })
	return out
}

// fillParsed 把 JSON 字段解析到 Parsed 字段（供前端使用）
func fillParsed(p *models.Plan) {
	if p.Modules != "" {
		_ = json.Unmarshal([]byte(p.Modules), &p.ModulesParsed)
	}
	if p.MultiYearDiscounts != "" {
		_ = json.Unmarshal([]byte(p.MultiYearDiscounts), &p.MultiYearDiscountsParsed)
	}
}

// Create 创建套餐
func (s PlanService) Create(req PlanCreateReq) (models.Plan, error) {
	var p models.Plan
	code := strings.TrimSpace(req.Code)
	name := strings.TrimSpace(req.Name)
	if code == "" || name == "" {
		return p, errors.New("套餐编码和名称不能为空")
	}
	// 校验编码唯一
	var cnt int64
	models.DB.Model(&models.Plan{}).Where("code = ?", code).Count(&cnt)
	if cnt > 0 {
		return p, errors.New("套餐编码已存在")
	}
	if err := validateModules(req.Modules); err != nil {
		return p, err
	}
	modules := normalizeModules(req.Modules)
	discounts := normalizeDiscounts(req.MultiYearDiscounts)
	modJSON, _ := json.Marshal(modules)
	discJSON, _ := json.Marshal(discounts)

	p = models.Plan{
		Model:               models.Model{ID: id_generator.StringID()},
		Code:                code,
		Name:                name,
		SortOrder:           req.SortOrder,
		Status:              1,
		Description:         req.Description,
		MaxUsers:            req.MaxUsers,
		MaxCustomers:        req.MaxCustomers,
		Modules:             string(modJSON),
		MonthlyPrice:        req.MonthlyPrice,
		MonthlyRenewPrice:   req.MonthlyRenewPrice,
		QuarterlyPrice:      req.QuarterlyPrice,
		QuarterlyRenewPrice: req.QuarterlyRenewPrice,
		YearlyPrice:         req.YearlyPrice,
		YearlyRenewPrice:    req.YearlyRenewPrice,
		MultiYearDiscounts:  string(discJSON),
		IsDefault:           req.IsDefault,
		IsTrial:             req.IsTrial,
		TrialDays:           req.TrialDays,
		IsFree:              req.IsFree,
		FreeDays:            req.FreeDays,
	}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		// 如果设为默认，先取消其它默认
		if req.IsDefault == 1 {
			tx.Model(&models.Plan{}).Where("is_default = 1").Update("is_default", 0)
		}
		return tx.Create(&p).Error
	})
	if err != nil {
		return p, err
	}
	fillParsed(&p)
	return p, nil
}

// List 列出所有套餐
func (s PlanService) List() ([]models.Plan, error) {
	var plans []models.Plan
	err := models.DB.Order("sort_order, id").Find(&plans).Error
	if err != nil {
		return nil, err
	}
	for i := range plans {
		fillParsed(&plans[i])
	}
	return plans, nil
}

// Get 按 ID 获取
func (s PlanService) Get(id string) (models.Plan, error) {
	var p models.Plan
	err := models.DB.Where("id = ?", id).First(&p).Error
	if err != nil {
		return p, err
	}
	fillParsed(&p)
	return p, nil
}

// Update 更新套餐
func (s PlanService) Update(id string, req PlanUpdateReq) error {
	if err := validateModules(req.Modules); err != nil {
		return err
	}
	modules := normalizeModules(req.Modules)
	discounts := normalizeDiscounts(req.MultiYearDiscounts)
	modJSON, _ := json.Marshal(modules)
	discJSON, _ := json.Marshal(discounts)

	status := req.Status
	if status != 1 && status != 2 {
		status = 1
	}

	return models.DB.Transaction(func(tx *gorm.DB) error {
		if req.IsDefault == 1 {
			tx.Model(&models.Plan{}).Where("is_default = 1 AND id != ?", id).Update("is_default", 0)
		}
		updates := map[string]any{
			"name":                  req.Name,
			"sort_order":            req.SortOrder,
			"status":                status,
			"description":           req.Description,
			"max_users":             req.MaxUsers,
			"max_customers":         req.MaxCustomers,
			"modules":               string(modJSON),
			"monthly_price":         req.MonthlyPrice,
			"monthly_renew_price":   req.MonthlyRenewPrice,
			"quarterly_price":       req.QuarterlyPrice,
			"quarterly_renew_price": req.QuarterlyRenewPrice,
			"yearly_price":          req.YearlyPrice,
			"yearly_renew_price":    req.YearlyRenewPrice,
			"multi_year_discounts":  string(discJSON),
			"is_default":            req.IsDefault,
			"is_trial":              req.IsTrial,
			"trial_days":            req.TrialDays,
			"is_free":               req.IsFree,
			"free_days":             req.FreeDays,
		}
		res := tx.Model(&models.Plan{}).Where("id = ?", id).Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("套餐不存在")
		}
		return nil
	})
}

// Delete 删除套餐（软删除）
func (s PlanService) Delete(id string) error {
	// 检查是否有租户在用
	var cnt int64
	models.DB.Model(&models.Tenant{}).Where("plan_code = ?", id).Count(&cnt)
	// 不阻断删除，因为 Tenant.Plan 存的是 code 字符串而非外键
	res := models.DB.Where("id = ?", id).Delete(&models.Plan{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("套餐不存在")
	}
	return nil
}

// SetStatus 启用/停用
func (s PlanService) SetStatus(id string, status int) error {
	if status != 1 && status != 2 {
		return errors.New("status 只能是 1(启用) 或 2(停用)")
	}
	res := models.DB.Model(&models.Plan{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("套餐不存在")
	}
	return nil
}

// ListModules 返回系统内置模块定义（供前端渲染开关）
func (s PlanService) ListModules() []models.PlanModuleDef {
	return models.PlanModuleDefs
}

// SeedDefaultPlans 幂等创建默认套餐（仅非生产环境）。
func SeedDefaultPlans() {
	var cnt int64
	models.DB.Model(&models.Plan{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	defaults := []PlanCreateReq{
		{
			Code: "trial", Name: "试用版", SortOrder: 0, Description: "14天免费试用专业版全部功能",
			MaxUsers: 10, MaxCustomers: 1000,
			Modules: []string{"workbench", "customer", "lead", "opportunity", "followup", "order", "receivable", "report",
				"channel_wecom", "channel_xhs", "channel_gzh", "channel_douyin",
				"marketing_coupon", "marketing_fission", "marketing_landing", "marketing_lottery",
				"marketing_bargain", "marketing_groupbuy", "marketing_groupsop", "marketing_ext",
				"risk_control", "company_mgmt", "role_perm", "staff_mgmt"},
			IsTrial: 1, TrialDays: 14,
		},
		{
			Code: "free", Name: "免费版", SortOrder: 1, Description: "永久免费，适合小团队基础使用",
			MaxUsers: 5, MaxCustomers: 500,
			Modules:     []string{"workbench", "customer", "lead", "followup", "channel_wecom"},
			YearlyPrice: 0,
			IsDefault:   1,
			IsFree:      1, FreeDays: 0, // 0=永久免费
		},
		{
			Code: "standard", Name: "标准版", SortOrder: 2, Description: "适合成长型企业",
			MaxUsers: 20, MaxCustomers: 5000,
			Modules: []string{"workbench", "customer", "lead", "opportunity", "followup", "order", "report",
				"channel_wecom", "channel_xhs", "marketing_coupon", "marketing_landing", "company_mgmt", "role_perm", "staff_mgmt"},
			MonthlyPrice: 9900, QuarterlyPrice: 26800, YearlyPrice: 96800,
			MonthlyRenewPrice: 8900, QuarterlyRenewPrice: 23800, YearlyRenewPrice: 86800,
			MultiYearDiscounts: []models.PlanMultiYearDiscount{
				{Years: 2, Discount: 95, GiftYears: 0, GiftRemark: ""},
				{Years: 3, Discount: 90, GiftYears: 0, GiftRemark: ""},
			},
		},
		{
			Code: "pro", Name: "专业版", SortOrder: 3, Description: "适合中大型企业全渠道运营",
			MaxUsers: 100, MaxCustomers: 50000,
			Modules: []string{"workbench", "customer", "lead", "opportunity", "followup", "order", "receivable", "report",
				"channel_wecom", "channel_xhs", "channel_gzh", "channel_douyin",
				"marketing_coupon", "marketing_fission", "marketing_landing", "marketing_lottery",
				"marketing_bargain", "marketing_groupbuy", "marketing_groupsop", "marketing_ext",
				"risk_control", "company_mgmt", "role_perm", "staff_mgmt"},
			MonthlyPrice: 29900, QuarterlyPrice: 79800, YearlyPrice: 288800,
			MonthlyRenewPrice: 26900, QuarterlyRenewPrice: 71800, YearlyRenewPrice: 258800,
			MultiYearDiscounts: []models.PlanMultiYearDiscount{
				{Years: 2, Discount: 92, GiftYears: 0, GiftRemark: "两年9.2折"},
				{Years: 3, Discount: 85, GiftYears: 1, GiftRemark: "买3年送1年"},
			},
		},
		{
			Code: "enterprise", Name: "企业版", SortOrder: 4, Description: "适合大型企业定制化需求",
			MaxUsers: 0, MaxCustomers: 0,
			Modules: []string{"workbench", "customer", "lead", "opportunity", "followup", "order", "receivable", "report",
				"channel_wecom", "channel_xhs", "channel_gzh", "channel_douyin",
				"marketing_coupon", "marketing_fission", "marketing_landing", "marketing_lottery",
				"marketing_bargain", "marketing_groupbuy", "marketing_groupsop", "marketing_ext",
				"risk_control", "msg_archive", "sensitive_word",
				"company_mgmt", "role_perm", "staff_mgmt"},
			MonthlyPrice: 0, QuarterlyPrice: 0, YearlyPrice: 0,
			MultiYearDiscounts: []models.PlanMultiYearDiscount{},
		},
	}
	svc := NewPlanService()
	for _, d := range defaults {
		_, _ = svc.Create(d)
	}
}
