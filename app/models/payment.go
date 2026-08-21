package models

import (
	"time"
)

// 支付方式常量
const (
	PaymentMethodWechat = "wechat"
	PaymentMethodAlipay = "alipay"
)

// 订单类型
const (
	OrderTypePurchase = "purchase" // 新购
	OrderTypeRenew    = "renew"    // 续费
	OrderTypeUpgrade  = "upgrade"  // 升级
)

// 计费周期
const (
	BillingMonthly   = "monthly"
	BillingQuarterly = "quarterly"
	BillingYearly    = "yearly"
)

// 支付状态
const (
	PayStatusPending  = 1 // 待支付
	PayStatusPaid     = 2 // 已支付
	PayStatusCancelled = 3 // 已取消
	PayStatusExpired  = 4 // 已过期
)

// PaymentConfig 平台级支付方式配置（平台超管维护）。
type PaymentConfig struct {
	Model
	Timestamp
	// 支付方式编码 wechat / alipay
	Method string `gorm:"type:varchar(32);uniqueIndex;comment:支付方式编码" json:"method"`
	// 显示名称
	Name string `gorm:"type:varchar(64);comment:显示名称" json:"name"`
	// 状态 1-启用 2-停用
	Status int `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
	// 排序
	SortOrder int `gorm:"type:int;default:0;comment:排序" json:"sort_order"`
	// 是否沙箱模式 1=是 0=否
	IsSandbox int `gorm:"type:smallint;default:0;comment:是否沙箱模式" json:"is_sandbox"`
	// 支付配置（JSON），不同支付方式字段不同
	// wechat: {app_id, mch_id, api_v3_key, cert_serial, private_key, notify_url}
	// alipay: {app_id, private_key, alipay_public_key, gateway_url, notify_url}
	Config string `gorm:"type:jsonb;default:'{}';comment:支付配置(JSON)" json:"-"`
	// ConfigParsed 前端友好的配置对象
	ConfigParsed map[string]string `gorm:"-" json:"config"`
}

// TableName 表名
func (PaymentConfig) TableName() string { return "payment_config" }

// TenantOrder 租户套餐订单（租户端发起升级/续费时创建）。
type TenantOrder struct {
	Model
	Timestamp
	// 租户 ID
	TenantID string `gorm:"type:varchar(32);index;comment:租户ID" json:"tenant_id"`
	// 订单号（全局唯一）
	OrderNo string `gorm:"type:varchar(64);uniqueIndex;comment:订单号" json:"order_no"`
	// 订单类型 purchase/renew/upgrade
	Type string `gorm:"type:varchar(16);comment:订单类型" json:"type"`
	// 套餐 ID
	PlanID string `gorm:"type:varchar(32);comment:套餐ID" json:"plan_id"`
	// 套餐编码（快照）
	PlanCode string `gorm:"type:varchar(32);comment:套餐编码快照" json:"plan_code"`
	// 套餐名称（快照）
	PlanName string `gorm:"type:varchar(64);comment:套餐名称快照" json:"plan_name"`
	// 计费周期 monthly/quarterly/yearly
	BillingPeriod string `gorm:"type:varchar(16);comment:计费周期" json:"billing_period"`
	// 购买年数
	Years int `gorm:"type:int;default:1;comment:购买年数" json:"years"`
	// 赠送年数（来自折扣规则）
	GiftYears int `gorm:"type:int;default:0;comment:赠送年数" json:"gift_years"`
	// 原始金额（分）
	OriginalAmount int64 `gorm:"type:bigint;default:0;comment:原始金额(分)" json:"original_amount"`
	// 折扣金额（分）
	DiscountAmount int64 `gorm:"type:bigint;default:0;comment:折扣金额(分)" json:"discount_amount"`
	// 实付金额（分）
	Amount int64 `gorm:"type:bigint;default:0;comment:实付金额(分)" json:"amount"`
	// 支付方式 wechat/alipay
	PaymentMethod string `gorm:"type:varchar(32);comment:支付方式" json:"payment_method"`
	// 支付状态 1-待支付 2-已支付 3-已取消 4-已过期
	PayStatus int `gorm:"type:smallint;default:1;comment:支付状态" json:"pay_status"`
	// 支付二维码内容（URL 或 code_url）
	QrCodeContent string `gorm:"type:varchar(512);comment:二维码内容" json:"qr_code_content"`
	// 第三方支付流水号
	TransactionID string `gorm:"type:varchar(128);comment:第三方支付流水号" json:"transaction_id"`
	// 支付时间
	PaidAt *time.Time `gorm:"comment:支付时间" json:"paid_at"`
	// 订单过期时间（待支付状态超时自动关闭）
	ExpireAt *time.Time `gorm:"comment:订单过期时间" json:"expire_at"`
	// 备注
	Remark string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// TableName 表名
func (TenantOrder) TableName() string { return "tenant_order" }

var _ = time.Now
