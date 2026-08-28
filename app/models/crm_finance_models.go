package models

import (
	"core/app/constants"
	"time"
)

// CrmReceivablesPlan 回款计划（合同拆分多期收款计划）。
type CrmReceivablesPlan struct {
	Model
	Timestamp
	TenantID      string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OrderID       string     `gorm:"type:bigint;index;comment:关联合同订单ID" json:"order_id"`
	CustomerID    string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	PeriodNo      int        `gorm:"type:int;comment:期次" json:"period_no"`
	Amount        float64    `gorm:"type:decimal(16,2);comment:计划回款金额" json:"amount"`
	PlanDate      *time.Time `gorm:"comment:计划回款日期" json:"plan_date"`
	Status        string     `gorm:"type:varchar(16);index;default:pending;comment:pending(待回款)/received(已回款)/overdue(逾期)" json:"status"`
	Remark        string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	ContractRatio float64    `gorm:"type:decimal(8,4);comment:合同分摊比例" json:"contract_ratio"`
}

// CrmReceivables 回款登记（实际收款记录）。
type CrmReceivables struct {
	Model
	Timestamp
	TenantID        string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OrderID         string                     `gorm:"type:bigint;index;comment:关联合同订单ID" json:"order_id"`
	CustomerID      string                     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	PlanID          string                     `gorm:"type:bigint;index;comment:关联回款计划ID(可空)" json:"plan_id"`
	Amount          float64                    `gorm:"type:decimal(16,2);comment:回款金额" json:"amount"`
	ReceivedAt      *time.Time                 `gorm:"comment:实际回款日期" json:"received_at"`
	PayMethod       string                     `gorm:"type:varchar(32);comment:收款方式 cash/transfer/wechat/alipay" json:"pay_method"`
	PayAccount      string                     `gorm:"type:varchar(128);comment:收款账户" json:"pay_account"`
	VoucherNo       string                     `gorm:"type:varchar(64);comment:凭证号" json:"voucher_no"`
	Attachments     constants.StringArrayField `gorm:"type:jsonb;comment:附件URL列表" json:"attachments"`
	Status          string                     `gorm:"type:varchar(16);index;default:confirmed;comment:pending(待确认)/confirmed(已确认)/void(已作废)" json:"status"`
	Remark          string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	ReceivablesNo   string                     `gorm:"type:varchar(32);index;comment:回款单号" json:"receivables_no"`
	ClientRequestID string                     `gorm:"type:varchar(64);comment:客户端登记请求标识" json:"client_request_id,omitempty"`
	RequestDigest   string                     `gorm:"type:varchar(64);comment:首次登记请求摘要" json:"-"`
	ApproverID      string                     `gorm:"type:bigint;comment:审批人ID" json:"approver_id"`
	ApproveStatus   string                     `gorm:"type:varchar(16);default:none;comment:none(无需审批)/pending(待审)/approved(通过)/rejected(驳回)" json:"approve_status"`
	ApproveAt       *time.Time                 `gorm:"comment:审批时间" json:"approve_at"`
	ApproveRemark   string                     `gorm:"type:varchar(512);comment:审批意见" json:"approve_remark"`
}

// CrmConsume 客户消费记录。
type CrmConsume struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	OrderID    string     `gorm:"type:bigint;comment:关联订单ID(可空)" json:"order_id"`
	Amount     float64    `gorm:"type:decimal(16,2);comment:消费金额" json:"amount"`
	Type       string     `gorm:"type:varchar(32);comment:消费类型 purchase/renewal/upgrade/service" json:"type"`
	ConsumedAt *time.Time `gorm:"comment:消费时间" json:"consumed_at"`
	Status     string     `gorm:"type:varchar(16);default:normal;comment:normal(正常)/revoked(已撤销)" json:"status"`
	Remark     string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmAccount 财务账户。
type CrmAccount struct {
	Model
	Timestamp
	TenantID string  `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string  `gorm:"type:varchar(128);comment:账户名称" json:"name"`
	Type     string  `gorm:"type:varchar(32);comment:账户类型 cash/bank/alipay/wechat/other" json:"type"`
	Balance  float64 `gorm:"type:decimal(16,2);default:0;comment:余额" json:"balance"`
	Number   string  `gorm:"type:varchar(64);comment:账号" json:"number"`
	Status   string  `gorm:"type:varchar(16);default:active;comment:active/disabled" json:"status"`
	Remark   string  `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmFinanceRecord 财务收支记录。
type CrmFinanceRecord struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Type        string     `gorm:"type:varchar(16);index;comment:类型 income/expense/transfer" json:"type"`
	Category    string     `gorm:"type:varchar(64);comment:收支类别" json:"category"`
	AccountID   string     `gorm:"type:bigint;index;comment:关联账户ID" json:"account_id"`
	ToAccountID string     `gorm:"type:bigint;comment:转入账户(转账用)" json:"to_account_id"`
	Amount      float64    `gorm:"type:decimal(16,2);comment:金额" json:"amount"`
	OccurDate   *time.Time `gorm:"comment:发生日期" json:"occur_date"`
	OrderID     string     `gorm:"type:bigint;comment:关联合同(可空)" json:"order_id"`
	CustomerID  string     `gorm:"type:bigint;comment:关联客户(可空)" json:"customer_id"`
	Status      string     `gorm:"type:varchar(16);default:confirmed;comment:pending/confirmed/void" json:"status"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmInvoice 发票管理。
type CrmInvoice struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	InvoiceNo  string     `gorm:"type:varchar(64);index;comment:发票号" json:"invoice_no"`
	OrderID    string     `gorm:"type:bigint;index;comment:关联合同ID" json:"order_id"`
	CustomerID string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	Type       string     `gorm:"type:varchar(16);comment:发票类型 normal/special/electronic" json:"type"`
	Amount     float64    `gorm:"type:decimal(16,2);comment:开票金额" json:"amount"`
	TaxRate    float64    `gorm:"type:decimal(6,4);comment:税率" json:"tax_rate"`
	Title      string     `gorm:"type:varchar(255);comment:发票抬头" json:"title"`
	TaxNo      string     `gorm:"type:varchar(64);comment:税号" json:"tax_no"`
	IssueDate  *time.Time `gorm:"comment:开票日期" json:"issue_date"`
	Status     string     `gorm:"type:varchar(16);default:pending;comment:pending(待开)/issued(已开)/void(已作废)" json:"status"`
}

// ==================== 财务扩展 ====================

// CrmIncomeCategory 收支类别。
type CrmIncomeCategory struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:类别名称" json:"name"`
	Type     string `gorm:"type:varchar(16);index;comment:income/expense" json:"type"`
	Status   bool   `gorm:"type:bool;default:true;comment:是否启用" json:"status"`
}

// CrmOtherIncome 其他收入单。
type CrmOtherIncome struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No         string     `gorm:"type:varchar(32);index;comment:单号" json:"no"`
	CategoryID string     `gorm:"type:bigint;index;comment:收支类别ID" json:"category_id"`
	AccountID  string     `gorm:"type:bigint;index;comment:账户ID" json:"account_id"`
	Amount     float64    `gorm:"type:decimal(16,2);comment:金额" json:"amount"`
	OccurDate  *time.Time `gorm:"comment:发生日期" json:"occur_date"`
	CustomerID string     `gorm:"type:bigint;comment:关联客户ID(可空)" json:"customer_id"`
	Remark     string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Status     string     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
}

// CrmOtherExpense 其他支出单。
type CrmOtherExpense struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No         string     `gorm:"type:varchar(32);index;comment:单号" json:"no"`
	CategoryID string     `gorm:"type:bigint;index;comment:收支类别ID" json:"category_id"`
	AccountID  string     `gorm:"type:bigint;index;comment:账户ID" json:"account_id"`
	Amount     float64    `gorm:"type:decimal(16,2);comment:金额" json:"amount"`
	OccurDate  *time.Time `gorm:"comment:发生日期" json:"occur_date"`
	CustomerID string     `gorm:"type:bigint;comment:关联客户ID(可空)" json:"customer_id"`
	Remark     string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Status     string     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
}

// CrmFundTransfer 资金转账单。
type CrmFundTransfer struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string     `gorm:"type:varchar(32);index;comment:单号" json:"no"`
	FromAccount string     `gorm:"type:bigint;index;comment:转出账户ID" json:"from_account"`
	ToAccount   string     `gorm:"type:bigint;index;comment:转入账户ID" json:"to_account"`
	Amount      float64    `gorm:"type:decimal(16,2);comment:转账金额" json:"amount"`
	OccurDate   *time.Time `gorm:"comment:发生日期" json:"occur_date"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Status      string     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
}
