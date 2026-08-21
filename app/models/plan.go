package models

import (
	"time"
)

// PlanModuleDef 系统内置模块定义（前端硬编码展示，后端校验 key 合法性）。
// 新增模块时在此追加，前端套餐管理页会自动展示开关项。
var PlanModuleDefs = []PlanModuleDef{
	{Key: "workbench", Name: "工作台", Group: "基础"},
	{Key: "customer", Name: "客户管理", Group: "基础"},
	{Key: "lead", Name: "线索管理", Group: "基础"},
	{Key: "opportunity", Name: "商机管理", Group: "基础"},
	{Key: "followup", Name: "跟进管理", Group: "基础"},
	{Key: "order", Name: "合同订单", Group: "基础"},
	{Key: "receivable", Name: "应收账款", Group: "基础"},
	{Key: "report", Name: "数据报表", Group: "基础"},
	{Key: "channel_wecom", Name: "企业微信渠道", Group: "渠道"},
	{Key: "channel_xhs", Name: "小红书渠道", Group: "渠道"},
	{Key: "channel_gzh", Name: "公众号渠道", Group: "渠道"},
	{Key: "channel_douyin", Name: "抖音渠道", Group: "渠道"},
	{Key: "marketing_coupon", Name: "优惠券", Group: "营销"},
	{Key: "marketing_fission", Name: "裂变活动", Group: "营销"},
	{Key: "marketing_landing", Name: "落地页", Group: "营销"},
	{Key: "marketing_lottery", Name: "抽奖活动", Group: "营销"},
	{Key: "marketing_bargain", Name: "砍价活动", Group: "营销"},
	{Key: "marketing_groupbuy", Name: "拼团活动", Group: "营销"},
	{Key: "marketing_groupsop", Name: "群SOP", Group: "营销"},
	{Key: "marketing_ext", Name: "营销扩展", Group: "营销"},
	{Key: "risk_control", Name: "风控合规", Group: "风控"},
	{Key: "msg_archive", Name: "会话存档", Group: "风控"},
	{Key: "sensitive_word", Name: "敏感词管理", Group: "风控"},
	{Key: "company_mgmt", Name: "企业管理", Group: "管理"},
	{Key: "role_perm", Name: "角色权限", Group: "管理"},
	{Key: "staff_mgmt", Name: "员工管理", Group: "管理"},
}

// PlanModuleDef 模块定义（仅用于展示和校验，不入库）。
type PlanModuleDef struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

// PlanBillingPeriod 计费周期。
const (
	BillingMonthly   = "monthly"   // 按月
	BillingQuarterly = "quarterly" // 按季
	BillingYearly    = "yearly"    // 按年
)

// PlanPrice 单个计费周期的价格配置。
type PlanPrice struct {
	// 新购价格（分）
	Price int64 `gorm:"type:bigint;default:0;comment:新购价格(分)" json:"price"`
	// 续费价格（分），0 表示同新购价
	RenewPrice int64 `gorm:"type:bigint;default:0;comment:续费价格(分,0=同新购)" json:"renew_price"`
}

// PlanMultiYearDiscount 多年购买/续费折扣配置。
// 例如：买2年打9折、买3年打8折；续2年送1年等。
type PlanMultiYearDiscount struct {
	Years      int    `gorm:"type:smallint;comment:购买年数" json:"years"`
	Discount   int    `gorm:"type:smallint;comment:折扣(百分比如90=9折,100=不打折)" json:"discount"`
	GiftYears  int    `gorm:"type:smallint;default:0;comment:赠送年数(如续2年送1年则gift_years=1)" json:"gift_years"`
	GiftRemark string `gorm:"type:varchar(128);comment:赠送说明" json:"gift_remark"`
}

// Plan 套餐/版本定义。
// 平台超管维护，租户创建/编辑时选择关联。
type Plan struct {
	Model
	Timestamp
	// 套餐编码（全局唯一），如 free/standard/pro/enterprise
	Code string `gorm:"type:varchar(32);uniqueIndex;comment:套餐编码" json:"code"`
	// 套餐名称
	Name string `gorm:"type:varchar(64);comment:套餐名称" json:"name"`
	// 排序（越小越靠前）
	SortOrder int `gorm:"type:int;default:0;comment:排序" json:"sort_order"`
	// 状态 1-启用 2-停用
	Status int `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
	// 套餐描述
	Description string `gorm:"type:varchar(512);comment:套餐描述" json:"description"`
	// 最大用户数（0=不限）
	MaxUsers int `gorm:"type:int;default:0;comment:最大用户数(0=不限)" json:"max_users"`
	// 最大客户数（0=不限）
	MaxCustomers int `gorm:"type:int;default:0;comment:最大客户数(0=不限)" json:"max_customers"`
	// 启用的模块 key 列表（JSON 数组）
	Modules string `gorm:"type:jsonb;default:'[]';comment:启用模块key列表" json:"-" `
	// ModulesParsed 前端友好的模块数组（运行时从 Modules 解析）
	ModulesParsed []string `gorm:"-" json:"modules"`

	// 按月价格
	MonthlyPrice      int64 `gorm:"type:bigint;default:0;comment:按月新购价格(分)" json:"monthly_price"`
	MonthlyRenewPrice int64 `gorm:"type:bigint;default:0;comment:按月续费价格(分,0=同新购)" json:"monthly_renew_price"`

	// 按季价格
	QuarterlyPrice      int64 `gorm:"type:bigint;default:0;comment:按季新购价格(分)" json:"quarterly_price"`
	QuarterlyRenewPrice int64 `gorm:"type:bigint;default:0;comment:按季续费价格(分,0=同新购)" json:"quarterly_renew_price"`

	// 按年价格
	YearlyPrice      int64 `gorm:"type:bigint;default:0;comment:按年新购价格(分)" json:"yearly_price"`
	YearlyRenewPrice int64 `gorm:"type:bigint;default:0;comment:按年续费价格(分,0=同新购)" json:"yearly_renew_price"`

	// 多年折扣配置（JSON 数组，元素为 PlanMultiYearDiscount）
	MultiYearDiscounts string `gorm:"type:jsonb;default:'[]';comment:多年折扣配置" json:"-"`
	// MultiYearDiscountsParsed 前端友好的折扣数组
	MultiYearDiscountsParsed []PlanMultiYearDiscount `gorm:"-" json:"multi_year_discounts"`

	// 是否为默认套餐（新租户未指定时使用，全局仅一个）
	IsDefault int `gorm:"type:smallint;default:0;comment:是否默认套餐(1=是)" json:"is_default"`

	// 是否为试用套餐（1=是）。试用套餐免费、有期限，到期后需转正或停用。
	IsTrial int `gorm:"type:smallint;default:0;comment:是否试用套餐(1=是)" json:"is_trial"`
	// 试用天数（如 7/14/30），仅 IsTrial=1 时有效
	TrialDays int `gorm:"type:int;default:0;comment:试用天数(仅试用套餐)" json:"trial_days"`

	// 是否为免费套餐（1=是）。免费套餐不收费，可永久免费或限定期限。
	// 与试用套餐区别：试用是短期体验全功能，免费是长期受限使用。
	IsFree int `gorm:"type:smallint;default:0;comment:是否免费套餐(1=是)" json:"is_free"`
	// 免费天数（0=永久免费，N=免费N天后需升级），仅 IsFree=1 时有效
	FreeDays int `gorm:"type:int;default:0;comment:免费天数(0=永久免费,仅免费套餐)" json:"free_days"`
}

// TableName 指定表名
func (Plan) TableName() string { return "plan" }

// _ 引用 time 防止未使用
var _ = time.Now
