package models

import (
	"core/app/constants"
	"time"
)

// CrmLead 线索。
type CrmLead struct {
	Model
	Timestamp
	TenantID          string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID        string     `gorm:"type:bigint;index;comment:关联客户ID(可空)" json:"customer_id"`
	OwnerUserID       string     `gorm:"type:bigint;index;comment:归属销售用户ID" json:"owner_user_id"`
	Title             string     `gorm:"type:varchar(255);comment:线索标题" json:"title"`
	Company           string     `gorm:"type:varchar(255);comment:公司名称" json:"company"`
	Name              string     `gorm:"type:varchar(128);comment:联系人姓名" json:"name"`
	Mobile            string     `gorm:"type:varchar(32);comment:联系电话" json:"mobile"`
	Source            string     `gorm:"type:varchar(32);comment:线索来源" json:"source"`
	Status            string     `gorm:"type:varchar(32);index;comment:new/contacted/qualified/lost" json:"status"`
	Remark            string     `gorm:"type:varchar(1024);comment:备注" json:"remark"`
	PoolStatus        int        `gorm:"type:smallint;default:0;index;comment:线索池状态 0-非池 1-池中待领" json:"pool_status"`
	PoolAt            *time.Time `gorm:"comment:进入线索池时间" json:"pool_at"`
	ConvertCustomerID string     `gorm:"type:bigint;index;comment:转化后的客户ID" json:"convert_customer_id"`
	ConvertAt         *time.Time `gorm:"comment:转化时间" json:"convert_at"`
	ConvertRemark     string     `gorm:"type:varchar(512);comment:转化备注" json:"convert_remark"`
	Email             string     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Position          string     `gorm:"type:varchar(64);comment:职位" json:"position"`
	Address           string     `gorm:"type:varchar(255);comment:详细地址" json:"address"`
	Province          string     `gorm:"type:varchar(32);comment:省" json:"province"`
	City              string     `gorm:"type:varchar(32);comment:市" json:"city"`
	District          string     `gorm:"type:varchar(32);comment:区/县" json:"district"`
	NextFollowupAt    *time.Time `gorm:"comment:线索下次跟进时间" json:"next_followup_at"`
	NextAction        string     `gorm:"type:varchar(512);comment:线索下一步行动" json:"next_action"`
}

// CrmLead 生命周期状态。
const (
	LeadStatusNew       = "new"       // 新线索
	LeadStatusContacted = "contacted" // 已联系
	LeadStatusQualified = "qualified" // 已转客户（终态）
	LeadStatusLost      = "lost"      // 已流失（可恢复）
)

// CrmLead 线索池状态。
const (
	LeadPoolNone = 0 // 非池（在归属销售名下）
	LeadPoolIn   = 1 // 池中待领
)

// IsConverted 线索已转化为客户（终态）：已产生客户关联或状态为 qualified。
func (l CrmLead) IsConverted() bool {
	return l.ConvertCustomerID != "" || l.Status == LeadStatusQualified
}

// IsLost 线索已流失（终态，可通过恢复跟进回到跟进中）。
func (l CrmLead) IsLost() bool { return l.Status == LeadStatusLost }

// InPool 线索在线索池中待领取。
func (l CrmLead) InPool() bool { return l.PoolStatus == LeadPoolIn }

// Followable 线索处于可跟进/可领取状态：新建、已联系或尚未填写状态。
func (l CrmLead) Followable() bool {
	return l.Status == "" || l.Status == LeadStatusNew || l.Status == LeadStatusContacted
}

// CrmPipelineStage 销售阶段定义。
type CrmPipelineStage struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name        string `gorm:"type:varchar(64);comment:阶段名称" json:"name"`
	Sort        int    `gorm:"type:int;comment:排序" json:"sort"`
	Probability int    `gorm:"type:int;default:0;comment:成交概率(%)" json:"probability"`
	IsWon       bool   `gorm:"comment:是否赢单终态" json:"is_won"`
	IsLost      bool   `gorm:"comment:是否输单终态" json:"is_lost"`
	Color       string `gorm:"type:varchar(16);default:#1890ff;comment:阶段颜色" json:"color"`
	Description string `gorm:"type:varchar(512);comment:阶段描述/退出标准" json:"description"`
}

// CrmOpportunity 商机。
type CrmOpportunity struct {
	Model
	Timestamp
	TenantID          string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID        string                     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	LeadID            string                     `gorm:"type:bigint;index;comment:来源线索ID" json:"lead_id"`
	OwnerUserID       string                     `gorm:"type:bigint;index;comment:归属销售用户ID" json:"owner_user_id"`
	StageID           string                     `gorm:"type:bigint;index;comment:当前阶段ID" json:"stage_id"`
	Title             string                     `gorm:"type:varchar(255);comment:商机名称" json:"title"`
	Product           string                     `gorm:"type:varchar(128);comment:产品/服务" json:"product"`
	Amount            float64                    `gorm:"type:decimal(16,2);comment:预计金额" json:"amount"`
	ExpectedCloseDate *time.Time                 `gorm:"comment:预计成交日期" json:"expected_close_date"`
	Status            string                     `gorm:"type:varchar(32);index;comment:open/won/lost" json:"status"`
	Remark            string                     `gorm:"type:varchar(1024);comment:备注" json:"remark"`
	ProductDetails    constants.StringArrayField `gorm:"type:jsonb;comment:产品明细JSON数组[{name,spec,qty,price,amount}]" json:"product_details"`
	Probability       int                        `gorm:"type:int;default:0;comment:成交概率(%)" json:"probability"`
	ExpectedRevenue   float64                    `gorm:"type:decimal(16,2);comment:预计收入" json:"expected_revenue"`
	Competitor        string                     `gorm:"type:varchar(255);comment:竞争对手" json:"competitor"`
	NextStep          string                     `gorm:"type:varchar(512);comment:下一步行动" json:"next_step"`
	LostReason        string                     `gorm:"type:varchar(512);comment:输单原因" json:"lost_reason"`
	WonReason         string                     `gorm:"type:varchar(512);comment:赢单原因" json:"won_reason"`
	ActualAmount      float64                    `gorm:"type:decimal(16,2);comment:实际成交金额" json:"actual_amount"`
	WonAt             *time.Time                 `gorm:"comment:赢单时间" json:"won_at"`
	LostAt            *time.Time                 `gorm:"comment:输单时间" json:"lost_at"`
}

// CrmOrder 合同/订单。
type CrmOrder struct {
	Model
	Timestamp
	TenantID      string     `gorm:"type:bigint;index;uniqueIndex:idx_tenant_order_no,priority:1;comment:归属租户ID" json:"tenant_id"`
	CustomerID    string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	OpportunityID string     `gorm:"type:bigint;index;comment:来源商机ID" json:"opportunity_id"`
	UserID        string     `gorm:"type:bigint;index;comment:签约销售用户ID" json:"user_id"`
	OrderNo       string     `gorm:"type:varchar(64);uniqueIndex:idx_tenant_order_no,priority:2;comment:合同/订单号(租户内唯一)" json:"order_no"`
	Title         string     `gorm:"type:varchar(255);comment:合同/订单名称" json:"title"`
	Amount        float64    `gorm:"type:decimal(16,2);comment:合同金额" json:"amount"`
	PaidAmount    float64    `gorm:"type:decimal(16,2);default:0;comment:已收金额" json:"paid_amount"`
	Status        string     `gorm:"type:varchar(32);index;comment:draft/signed/completed/cancelled" json:"status"`
	PayStatus     string     `gorm:"type:varchar(32);index;comment:收款状态 unpaid/partial/paid" json:"pay_status"`
	SignedAt      *time.Time `gorm:"comment:签约时间" json:"signed_at"`
	CouponID      string     `gorm:"type:bigint;index;comment:关联优惠券ID(可空)" json:"coupon_id"`
	CouponAmount  float64    `gorm:"type:decimal(16,2);default:0;comment:优惠券抵扣金额" json:"coupon_amount"`
	Remark        string     `gorm:"type:varchar(1024);comment:备注" json:"remark"`
	// ---- 合同扩展字段（对照青东CRM） ----
	ContractType   string     `gorm:"type:varchar(32);comment:合同类型" json:"contract_type"`
	DiscountAmount float64    `gorm:"type:decimal(16,2);default:0;comment:折扣金额" json:"discount_amount"`
	TaxAmount      float64    `gorm:"type:decimal(16,2);default:0;comment:税额" json:"tax_amount"`
	DeliveryDate   *time.Time `gorm:"comment:交付日期" json:"delivery_date"`
	EffectiveDate  *time.Time `gorm:"comment:生效日期" json:"effective_date"`
	ExpireDate     *time.Time `gorm:"comment:到期日期" json:"expire_date"`
	RevokeReason   string     `gorm:"type:varchar(512);comment:作废原因" json:"revoke_reason"`
	RevokeAt       *time.Time `gorm:"comment:作废时间" json:"revoke_at"`
	ProjectID      string     `gorm:"type:bigint;index;comment:关联项目ID(可空)" json:"project_id"`
	WorkorderID    string     `gorm:"type:bigint;index;comment:关联工单ID(可空)" json:"workorder_id"`
}

// CrmSalesRule 销售规则（租户级，单条）：阶段流转校验等。
type CrmSalesRule struct {
	Model
	Timestamp
	TenantID        string `gorm:"type:bigint;uniqueIndex;comment:归属租户ID" json:"tenant_id"`
	RequireFollowup bool   `gorm:"comment:移动阶段是否强制校验跟进" json:"require_followup"`
	FollowupDays    int    `gorm:"type:int;default:3;comment:近N天需有跟进记录" json:"followup_days"`
}

// CrmQuota 销售业绩目标（按月按销售）。
type CrmQuota struct {
	Model
	Timestamp
	TenantID     string  `gorm:"type:bigint;index;uniqueIndex:idx_tenant_user_month,priority:1;comment:归属租户ID" json:"tenant_id"`
	UserID       string  `gorm:"type:bigint;index;uniqueIndex:idx_tenant_user_month,priority:2;comment:销售用户ID" json:"user_id"`
	YearMonth    string  `gorm:"type:char(7);index;uniqueIndex:idx_tenant_user_month,priority:3;comment:YYYY-MM" json:"year_month"`
	TargetAmount float64 `gorm:"type:decimal(16,2);default:0;comment:目标金额" json:"target_amount"`
}

// CrmQuote 报价单。
type CrmQuote struct {
	Model
	Timestamp
	TenantID      string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	QuoteNo       string                     `gorm:"type:varchar(32);index;comment:报价单号" json:"quote_no"`
	CustomerID    string                     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	ContactID     string                     `gorm:"type:bigint;comment:关联联系人ID" json:"contact_id"`
	OpportunityID string                     `gorm:"type:bigint;comment:关联商机ID(可空)" json:"opportunity_id"`
	OrderID       string                     `gorm:"type:bigint;index;comment:报价转成的合同ID" json:"order_id"`
	Subject       string                     `gorm:"type:varchar(255);comment:报价主题" json:"subject"`
	Amount        float64                    `gorm:"type:decimal(16,2);comment:报价金额" json:"amount"`
	Discount      float64                    `gorm:"type:decimal(16,2);comment:折扣金额" json:"discount"`
	FinalAmount   float64                    `gorm:"type:decimal(16,2);comment:最终金额" json:"final_amount"`
	Items         constants.StringArrayField `gorm:"type:jsonb;comment:报价明细(JSON数组)" json:"items"`
	ValidUntil    *time.Time                 `gorm:"comment:报价有效期" json:"valid_until"`
	Status        string                     `gorm:"type:varchar(16);index;default:draft;comment:draft(草稿)/sent(已发送)/accepted(已接受)/rejected(已拒绝)/void(已作废)" json:"status"`
	OwnerUserID   string                     `gorm:"type:bigint;index;comment:负责人" json:"owner_user_id"`
	Remark        string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	TaxRate       float64                    `gorm:"type:decimal(6,4);default:0;comment:税率" json:"tax_rate"`
	TaxAmount     float64                    `gorm:"type:decimal(16,2);default:0;comment:税额" json:"tax_amount"`
	VoidReason    string                     `gorm:"type:varchar(512);comment:作废原因" json:"void_reason"`
	VoidAt        *time.Time                 `gorm:"comment:作废时间" json:"void_at"`
}

// CrmOrderProduct 合同订单产品明细。
type CrmOrderProduct struct {
	Model
	Timestamp
	TenantID    string  `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OrderID     string  `gorm:"type:bigint;index;comment:关联订单ID" json:"order_id"`
	ProductName string  `gorm:"type:varchar(255);comment:产品名称" json:"product_name"`
	Spec        string  `gorm:"type:varchar(255);comment:规格" json:"spec"`
	Unit        string  `gorm:"type:varchar(32);comment:单位" json:"unit"`
	Qty         int     `gorm:"type:int;comment:数量" json:"qty"`
	UnitPrice   float64 `gorm:"type:decimal(16,2);comment:单价" json:"unit_price"`
	Discount    float64 `gorm:"type:decimal(16,2);comment:折扣" json:"discount"`
	Amount      float64 `gorm:"type:decimal(16,2);comment:小计金额" json:"amount"`
	Remark      string  `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmOrderAttachment 合同订单附件。
type CrmOrderAttachment struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OrderID    string `gorm:"type:bigint;index;comment:关联订单ID" json:"order_id"`
	FileName   string `gorm:"type:varchar(255);comment:文件名" json:"file_name"`
	FileURL    string `gorm:"type:varchar(512);comment:文件URL" json:"file_url"`
	FileSize   int64  `gorm:"type:bigint;comment:文件大小(字节)" json:"file_size"`
	FileType   string `gorm:"type:varchar(32);comment:文件类型" json:"file_type"`
	UploadedBy string `gorm:"type:bigint;comment:上传人" json:"uploaded_by"`
	StorageKey string `gorm:"type:varchar(255);uniqueIndex:idx_crm_attachment_storage_key,where:storage_key <> '';comment:私有文件存储键" json:"storage_key"`
	SHA256     string `gorm:"type:varchar(64);comment:文件内容摘要" json:"sha256"`
}

// CrmOrderProgress 合同进度节点。
type CrmOrderProgress struct {
	Model
	Timestamp
	TenantID string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OrderID  string     `gorm:"type:bigint;index;comment:关联订单ID" json:"order_id"`
	NodeName string     `gorm:"type:varchar(128);comment:节点名称" json:"node_name"`
	Sort     int        `gorm:"type:int;comment:排序" json:"sort"`
	Status   string     `gorm:"type:varchar(16);default:pending;comment:pending(待执行)/doing(执行中)/done(已完成)/skipped(已跳过)" json:"status"`
	PlanDate *time.Time `gorm:"comment:计划日期" json:"plan_date"`
	DoneAt   *time.Time `gorm:"comment:完成日期" json:"done_at"`
	Remark   string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}
