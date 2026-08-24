package models

import (
	"time"

	"core/app/constants"
)

// CRM 原生模型 —— 渠道无关的客户/员工/销售管线底座。
//
// 与企微模型的区别：主键均为本地 ID（id_generator 生成的 bigint），
// 顶层分区是 TenantID（本地租户），而不是 ExtCorpID（企微 corp_id）。
// 企微 external id 一律落到 ChannelLink 映射，不作为主键。
// 这样"去掉企微也能用"：CRM 核心不依赖任何企微字段。

// Tenant 企业/组织（本地租户），取代 corp_id 作为顶层数据分区。
type Tenant struct {
	Model
	Timestamp
	Name         string     `gorm:"type:varchar(128);comment:企业/组织名称" json:"name"`
	Code         string     `gorm:"type:varchar(64);uniqueIndex;comment:租户编码" json:"code"`
	Status       int        `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
	Remark       string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Plan         string     `gorm:"type:varchar(32);default:free;comment:套餐 free/standard/pro/enterprise" json:"plan"`
	ExpireAt     *time.Time `gorm:"comment:套餐到期时间(NULL=永久)" json:"expire_at"`
	ContactName  string     `gorm:"type:varchar(64);comment:联系人姓名" json:"contact_name"`
	ContactPhone string     `gorm:"type:varchar(32);comment:联系人手机" json:"contact_phone"`
	ContactEmail string     `gorm:"type:varchar(128);comment:联系人邮箱" json:"contact_email"`
	Industry     string     `gorm:"type:varchar(64);comment:所属行业" json:"industry"`
	Logo         string     `gorm:"type:varchar(255);comment:企业Logo URL" json:"logo"`
	MaxUsers     int        `gorm:"type:int;default:0;comment:最大用户数(0=不限)" json:"max_users"`
	MaxCustomers int        `gorm:"type:int;default:0;comment:最大客户数(0=不限)" json:"max_customers"`
}

// PlatformAdmin 平台超管（独立于租户，管理所有租户）。
// 与租户内 User 解耦：不归属任何 tenant，专用登录入口，JWT 标记 is_platform。
type PlatformAdmin struct {
	Model
	Timestamp
	Username  string     `gorm:"type:varchar(64);uniqueIndex;comment:登录名(全局唯一)" json:"username"`
	Password  string     `gorm:"type:varchar(256);comment:登录密码(哈希)" json:"-"`
	Name      string     `gorm:"type:varchar(128);comment:显示名" json:"name"`
	Avatar    string     `gorm:"type:varchar(512);comment:头像URL" json:"avatar"`
	Mobile    string     `gorm:"type:varchar(32);comment:手机号" json:"mobile"`
	Email     string     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Role      string     `gorm:"type:varchar(32);default:super_admin;comment:平台角色 super_admin/operator/finance/support" json:"role"`
	Status    int        `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
	LastLogin *time.Time `gorm:"comment:最后登录时间" json:"last_login_at"`
}

// PlatformOperationLog 平台操作日志（记录平台超管对租户/套餐/支付等资源的操作）。
type PlatformOperationLog struct {
	Model
	Timestamp
	AdminID    string `gorm:"type:bigint;index;comment:操作人(PlatformAdmin.ID)" json:"admin_id"`
	AdminName  string `gorm:"type:varchar(128);comment:操作人姓名" json:"admin_name"`
	AdminRole  string `gorm:"type:varchar(32);comment:操作人角色" json:"admin_role"`
	Action     string `gorm:"type:varchar(64);index;comment:操作类型(如 tenant.create/plan.delete)" json:"action"`
	Resource   string `gorm:"type:varchar(32);comment:资源类型(tenant/plan/payment/order/admin)" json:"resource"`
	ResourceID string `gorm:"type:varchar(64);comment:资源ID" json:"resource_id"`
	Detail     string `gorm:"type:text;comment:操作详情(JSON)" json:"detail"`
	IP         string `gorm:"type:varchar(64);comment:操作IP" json:"ip"`
}

// User 本地用户（销售/员工账号），取代企微 staff 作为鉴权与归属单元。
type User struct {
	Model
	Timestamp
	TenantID     string     `gorm:"type:bigint;index;uniqueIndex:idx_tenant_username,priority:1,where:deleted_at IS NULL;comment:归属租户ID" json:"tenant_id"`
	Username     string     `gorm:"type:varchar(64);index;uniqueIndex:idx_tenant_username,priority:2,where:deleted_at IS NULL;comment:登录名(租户内唯一)" json:"username"`
	Password     string     `gorm:"type:varchar(256);comment:登录密码(哈希)" json:"-"`
	Name         string     `gorm:"type:varchar(128);comment:显示名" json:"name"`
	Mobile       string     `gorm:"type:varchar(32);index;comment:手机号" json:"mobile"`
	Email        string     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	RoleID       string     `gorm:"type:bigint;index;comment:角色ID" json:"role_id"`
	Position     string     `gorm:"type:varchar(64);comment:职位" json:"position"`
	DepartmentID string     `gorm:"type:bigint;index;comment:部门ID" json:"department_id"`
	Avatar       string     `gorm:"type:varchar(255);comment:头像" json:"avatar"`
	LastLoginAt  *time.Time `gorm:"comment:最后登录时间" json:"last_login_at"`
	Status       int        `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
}

// ChannelLink 渠道映射：把本地实体(租户/用户/客户)与外部渠道 ID(如企微 ext id)关联。
// 这是企微 external id 从"主键"降级为"渠道外部键"的载体。
type ChannelLink struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:本地租户ID" json:"tenant_id"`
	Channel    string `gorm:"type:varchar(32);index;uniqueIndex:idx_channel_ext,priority:1;comment:渠道标识,如 wecom" json:"channel"`
	EntityType string `gorm:"type:varchar(32);index;comment:实体类型 tenant/user/customer" json:"entity_type"`
	EntityID   string `gorm:"type:bigint;index;comment:本地实体主键" json:"entity_id"`
	ExtID      string `gorm:"type:varchar(128);index;uniqueIndex:idx_channel_ext,priority:3;comment:渠道外部ID(如企微 external_userid)" json:"ext_id"`
	ExtCorpID  string `gorm:"type:varchar(128);index;uniqueIndex:idx_channel_ext,priority:2;comment:渠道外部企业ID(如企微 corp_id)" json:"ext_corp_id"`
}

// CrmCustomer 本地客户（CRM 客户 360 规范记录），带来源渠道维度。
type CrmCustomer struct {
	Model
	Timestamp
	TenantID       string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name           string                     `gorm:"type:varchar(128);comment:客户名称" json:"name"`
	Mobile         string                     `gorm:"type:varchar(32);index;comment:手机号" json:"mobile"`
	Email          string                     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Company        string                     `gorm:"type:varchar(255);comment:公司名称" json:"company"`
	Industry       string                     `gorm:"type:varchar(64);index;comment:所属行业" json:"industry"`
	Position       string                     `gorm:"type:varchar(64);comment:职位" json:"position"`
	Gender         int                        `gorm:"type:smallint;default:0;comment:0-未知 1-男 2-女" json:"gender"`
	Birthday       string                     `gorm:"type:varchar(32);comment:生日" json:"birthday"`
	Level          int                        `gorm:"type:smallint;default:0;comment:客户等级 0-未知 1-潜在 2-普通 3-重要 4-重点 5-VIP" json:"level"`
	Address        string                     `gorm:"type:varchar(255);comment:地址" json:"address"`
	Source         string                     `gorm:"type:varchar(32);index;comment:来源渠道 wecom/xiaohongshu/manual/import/web_form" json:"source"`
	OwnerUserID    string                     `gorm:"type:bigint;index;comment:归属销售用户ID" json:"owner_user_id"`
	NextFollowupAt *time.Time                 `gorm:"comment:下次跟进时间" json:"next_followup_at"`
	Remark         string                     `gorm:"type:varchar(1024);comment:备注" json:"remark"`
	Tags           constants.StringArrayField `gorm:"type:jsonb;comment:客户标签(内部标签ID数组)" json:"tags"`
	// ---- 渠道维度快照（source 为渠道时回填，作为该渠道客户在 CRM 主档上的身份快照，经 ChannelLink 关联渠道原始数据）----
	ExtCustomerID string                     `gorm:"type:varchar(128);index;comment:渠道用户ID(企微external_userid / 小红书user_id)" json:"ext_customer_id"`
	ExtCorpID     string                     `gorm:"type:varchar(128);index;comment:渠道账号ID(企微corp_id / 小红书企业号)" json:"ext_corp_id"`
	AddWay        int                        `gorm:"type:int;default:0;comment:渠道内添加/承接方式(wecom 0-9/201/202；xiaohongshu 0-4)" json:"add_way"`
	AccountType   string                     `gorm:"type:varchar(32);comment:账号类型 微信/企业微信/小红书" json:"account_type"`
	OutFlowStatus int                        `gorm:"type:smallint;default:0;comment:流失状态 0-正常 1-已流失" json:"out_flow_status"`
	ChannelTags   constants.StringArrayField `gorm:"type:jsonb;comment:渠道标签快照(标签名数组)" json:"channel_tags"`
	// ---- 公海池 ----
	PoolStatus int        `gorm:"type:smallint;default:0;index;comment:公海状态 0-非公海 1-公海待认领" json:"pool_status"`
	PoolAt     *time.Time `gorm:"comment:进入公海时间" json:"pool_at"`
	// ---- Phase 4 客户评分 ----
	Score          int        `gorm:"type:int;default:0;index;comment:客户评分(0-100,自动计算)" json:"score"`
	ScoreUpdatedAt *time.Time `gorm:"comment:评分更新时间" json:"score_updated_at"`
	// ---- 客户池扩展（对照青东CRM） ----
	IsLocked        bool                       `gorm:"type:bool;default:false;index;comment:是否锁定(锁客防公海回收)" json:"is_locked"`
	LockReason      string                     `gorm:"type:varchar(256);comment:锁定原因" json:"lock_reason"`
	LockAt          *time.Time                 `gorm:"comment:锁定时间" json:"lock_at"`
	IsBlacklist     bool                       `gorm:"type:bool;default:false;index;comment:是否黑名单" json:"is_blacklist"`
	BlacklistReason string                     `gorm:"type:varchar(256);comment:拉黑原因" json:"blacklist_reason"`
	BlacklistAt     *time.Time                 `gorm:"comment:拉黑时间" json:"blacklist_at"`
	IsKey           bool                       `gorm:"type:bool;default:false;index;comment:是否重点客户" json:"is_key"`
	SharedStaffIDs  constants.StringArrayField `gorm:"type:jsonb;comment:共享可见人用户ID列表" json:"shared_staff_ids"`
	ReassignStatus  int                        `gorm:"type:smallint;default:0;comment:再分配状态 0-正常 1-待确认接收" json:"reassign_status"`
	ReassignFrom    string                     `gorm:"type:bigint;comment:转出人用户ID(再分配时)" json:"reassign_from"`
}

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
	Address           string     `gorm:"type:varchar(255);comment:地址" json:"address"`
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

// CrmFollowup 跟进记录。
type CrmFollowup struct {
	Model
	Timestamp
	TenantID       string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID     string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	LeadID         string     `gorm:"type:bigint;index;comment:关联线索ID(可空)" json:"lead_id"`
	OpportunityID  string     `gorm:"type:bigint;index;comment:关联商机ID(可空)" json:"opportunity_id"`
	UserID         string     `gorm:"type:bigint;index;comment:跟进人用户ID" json:"user_id"`
	Type           string     `gorm:"type:varchar(32);comment:call/visit/message/meeting/other" json:"type"`
	Content        string     `gorm:"type:text;comment:跟进内容" json:"content"`
	NextFollowupAt *time.Time `gorm:"comment:下次跟进时间" json:"next_followup_at"`
	ContactID      string     `gorm:"type:bigint;index;comment:关联联系人ID(可空)" json:"contact_id"`
	Duration       int        `gorm:"type:int;comment:跟进时长(分钟)" json:"duration"`
	Result         string     `gorm:"type:varchar(32);comment:跟进结果 positive/neutral/negative" json:"result"`
	NextAction     string     `gorm:"type:varchar(512);comment:下一步行动" json:"next_action"`
	Location       string     `gorm:"type:varchar(255);comment:签到位置(拜访时)" json:"location"`
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
	// CouponCode 临时字段：创建订单时传入优惠券码，不入库（gorm:"-"）。
	CouponCode string `gorm:"-" json:"coupon_code"`
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

// CrmRole 角色（CRM 原生权限模型底座，tenant 分区）。
// 与企微 role 无关：本地 User.role_id 引用本表，支撑租户内权限校验（如管理员重置密码）。
type CrmRole struct {
	Model
	Timestamp
	TenantID  string                     `gorm:"type:bigint;index;uniqueIndex:idx_tenant_role_code,priority:1;comment:归属租户ID" json:"tenant_id"`
	Code      string                     `gorm:"type:varchar(64);uniqueIndex:idx_tenant_role_code,priority:2;comment:角色编码 admin/manager/sales" json:"code"`
	Name      string                     `gorm:"type:varchar(128);comment:角色名称" json:"name"`
	Perms     constants.StringArrayField `gorm:"type:jsonb;comment:权限点编码数组" json:"perms"`
	IsDefault bool                       `gorm:"comment:是否默认角色(新用户自动分配)" json:"is_default"`
}

// CrmActivity 营销活动。
// 运营创建营销活动（优惠券/抽奖/秒杀/H5落地页），跟踪触达-参与-转化效果，并接入统一客户主档。
// Channel 为主渠道（向后兼容），Channels 为投放渠道多选（有赞/微盟式多渠道投放）。
type CrmActivity struct {
	Model
	Timestamp
	TenantID         string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OwnerUserID      string                     `gorm:"type:bigint;index;comment:创建人用户ID" json:"owner_user_id"`
	Name             string                     `gorm:"type:varchar(128);comment:活动名称" json:"name"`
	ActivityType     string                     `gorm:"type:varchar(32);index;comment:活动类型 coupon/lottery/seckill/h5/group_buy/bargain/full_reduction/red_packet/sign_in/other" json:"activity_type"`
	Status           string                     `gorm:"type:varchar(32);index;comment:draft/active/ended/off" json:"status"`
	Channel          string                     `gorm:"type:varchar(32);comment:主渠道 wecom/xiaohongshu/general" json:"channel"`
	Channels         constants.StringArrayField `gorm:"type:jsonb;comment:投放渠道多选(wecom/xiaohongshu/general)" json:"channels"`
	StartAt          *time.Time                 `gorm:"comment:开始时间" json:"start_at"`
	EndAt            *time.Time                 `gorm:"comment:结束时间" json:"end_at"`
	Cost             float64                    `gorm:"type:decimal(16,2);default:0;comment:活动预算/成本" json:"cost"`
	ReachCount       int                        `gorm:"type:int;default:0;comment:触达数" json:"reach_count"`
	ParticipateCount int                        `gorm:"type:int;default:0;comment:参与数" json:"participate_count"`
	ConversionCount  int                        `gorm:"type:int;default:0;comment:转化数(拉新客户)" json:"conversion_count"`
	RewardDesc       string                     `gorm:"type:text;comment:奖品/优惠说明" json:"reward_desc"`
	Remark           string                     `gorm:"type:varchar(1024);comment:备注" json:"remark"`
}

// CrmChannelInteraction 渠道互动记录（非企微渠道的行为数据，如小红书留资/评论/私信）。
// 企微互动仍走 chat_msg（消息存档），本表承载企微之外渠道的互动时间线。
type CrmChannelInteraction struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	Channel    string     `gorm:"type:varchar(32);index;comment:渠道标识" json:"channel"`
	ActionType string     `gorm:"type:varchar(32);comment:互动类型 lead/comment/dm/other" json:"action_type"`
	Content    string     `gorm:"type:text;comment:互动内容" json:"content"`
	HappenedAt *time.Time `gorm:"comment:互动发生时间" json:"happened_at"`
	ExtRef     string     `gorm:"type:varchar(255);comment:渠道侧引用(笔记ID/会话ID等)" json:"ext_ref"`
}

// CrmChannelTag 渠道标签目录（参考 SCRM 标签中心，按渠道组织预设标签）。
// 客户打标结果仍落在 CrmCustomer.channel_tags 快照（标签名数组），目录驱动选择、快照为准。
type CrmChannelTag struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Channel  string `gorm:"type:varchar(32);index;comment:渠道标识" json:"channel"`
	Name     string `gorm:"type:varchar(64);comment:标签名" json:"name"`
	Category string `gorm:"type:varchar(64);comment:标签分类/分组" json:"category"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
}

// CrmActivityRecord 活动参与明细：把活动与客户主档关联，支撑触达/参与/转化漏斗的可溯源性。
// 语义：每个 (activity, customer) 仅保留一行，RecordType 取最高档（reached<participated<converted）。
// 活动上的 reach/participate/conversion_count 由本表派生（COUNT(customer) per type），避免手填失真。
type CrmActivityRecord struct {
	Model
	Timestamp
	TenantID       string     `gorm:"type:bigint;index;uniqueIndex:idx_activity_customer,priority:1;comment:归属租户ID" json:"tenant_id"`
	ActivityID     string     `gorm:"type:bigint;index;uniqueIndex:idx_activity_customer,priority:2;comment:关联活动ID" json:"activity_id"`
	CustomerID     string     `gorm:"type:bigint;index;uniqueIndex:idx_activity_customer,priority:3;comment:关联客户ID" json:"customer_id"`
	Channel        string     `gorm:"type:varchar(32);index;comment:参与渠道" json:"channel"`
	RecordType     string     `gorm:"type:varchar(32);index;comment:触达档位 reached/participated/converted" json:"record_type"`
	HappenedAt     *time.Time `gorm:"comment:参与时间" json:"happened_at"`
	Remark         string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	OperatorUserID string     `gorm:"type:bigint;comment:录入人用户ID" json:"operator_user_id"`
}

// CrmChannelConfig 渠道接入配置（一租户一渠道账号）。
// 每个 (tenant, channel) 一条：存该租户在外部渠道的企业账号凭据。
// CorpID 字段语义按渠道不同：wecom=corp_id, xiaohongshu=app_id。
// secrets 一律经 util.AESEncode 加密落库，json:"-" 永不序列化，读接口只回掩码。
type CrmChannelConfig struct {
	Model
	Timestamp
	TenantID        string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Channel         string `gorm:"type:varchar(32);index;comment:渠道标识 wecom/xiaohongshu" json:"channel"`
	CorpID          string `gorm:"type:varchar(64);index;uniqueIndex:idx_corp;comment:渠道外部账号ID(企微corp_id/小红书app_id,全局唯一)" json:"corp_id"`
	Name            string `gorm:"type:varchar(128);comment:企业/应用名称" json:"name"`
	ContactSecret   string `gorm:"type:text;comment:通讯录secret(加密)" json:"-"`
	CustomerSecret  string `gorm:"type:text;comment:客户联系secret(加密)" json:"-"`
	MainAgentID     int64  `gorm:"type:bigint;comment:主应用AgentID" json:"main_agent_id"`
	MainAgentSecret string `gorm:"type:text;comment:主应用secret(加密)" json:"-"`
	CallbackToken   string `gorm:"type:varchar(256);comment:回调Token(加密)" json:"-"`
	CallbackAesKey  string `gorm:"type:text;comment:回调AesKey(加密)" json:"-"`
	CallbackURL     string `gorm:"type:varchar(255);comment:回调URL(展示)" json:"callback_url"`
	Enabled         int    `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"enabled"`
	Remark          string `gorm:"type:varchar(512);comment:备注" json:"remark"`
	// ---- 小红书专用 ----
	XhsAppSecret string `gorm:"type:text;comment:小红书AppSecret(加密)" json:"-"`
	// ---- 公众号专用 ----
	GzhAppID     string `gorm:"type:varchar(64);comment:公众号AppID(与corp_id相同,冗余便于识别)" json:"gzh_app_id"`
	GzhAppSecret string `gorm:"type:text;comment:公众号AppSecret(加密)" json:"-"`
	// ---- 抖音专用 ----
	DouyinAppID     string `gorm:"type:varchar(64);comment:抖音应用AppID(与corp_id相同)" json:"douyin_app_id"`
	DouyinAppSecret string `gorm:"type:text;comment:抖音应用AppSecret(加密)" json:"-"`
}

// CrmWecomEventLog 企微回调事件日志（多租户排障：corp→租户、事件来了没有、处理结果）。
type CrmWecomEventLog struct {
	Model
	Timestamp
	TenantID      string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CorpID        string `gorm:"type:varchar(18);index;comment:企微 corp_id" json:"corp_id"`
	Event         string `gorm:"type:varchar(64);comment:事件类型(EventType 值)" json:"event"`
	ChangeType    string `gorm:"type:varchar(64);index;comment:变更类型(ChangeType 值)" json:"change_type"`
	ExtUserID     string `gorm:"type:varchar(128);comment:企微员工 userid" json:"ext_user_id"`
	ExtCustomerID string `gorm:"type:varchar(128);comment:企微外部联系人 external_userid" json:"ext_customer_id"`
	Handled       bool   `gorm:"comment:是否命中并处理" json:"handled"`
	Error         string `gorm:"type:varchar(512);comment:处理失败原因" json:"error"`
}

// ---- Phase 1 销售域深化模型 ----

// CrmPoolRule 公海回收规则（租户级，单条）。
type CrmPoolRule struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;uniqueIndex;comment:归属租户ID" json:"tenant_id"`
	RecycleDays int    `gorm:"type:int;default:0;comment:超N天未跟进自动回收,0=不自动回收" json:"recycle_days"`
	Enabled     bool   `gorm:"comment:是否启用自动回收" json:"enabled"`
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

// CrmTransferLog 客户转移历史。
type CrmTransferLog struct {
	Model
	Timestamp
	TenantID       string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID     string `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	FromUserID     string `gorm:"type:bigint;comment:原归属销售ID" json:"from_user_id"`
	ToUserID       string `gorm:"type:bigint;comment:新归属销售ID" json:"to_user_id"`
	Reason         string `gorm:"type:varchar(512);comment:转移原因" json:"reason"`
	OperatorUserID string `gorm:"type:bigint;comment:操作人ID" json:"operator_user_id"`
}

// CrmChannelTask 跨渠道引流任务（如小红书留资→企微添加）。
type CrmChannelTask struct {
	Model
	Timestamp
	TenantID     string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID   string `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	FromChannel  string `gorm:"type:varchar(32);comment:来源渠道" json:"from_channel"`
	ToChannel    string `gorm:"type:varchar(32);comment:目标渠道" json:"to_channel"`
	Action       string `gorm:"type:varchar(32);comment:动作类型 add_wecom/call_back" json:"action"`
	Status       string `gorm:"type:varchar(16);index;default:pending;comment:pending/done/skipped" json:"status"`
	AssignUserID string `gorm:"type:bigint;comment:分配给哪个销售" json:"assign_user_id"`
	Remark       string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// ---- Phase 3 营销活动落地 ----

// CrmCoupon 优惠券模板。
type CrmCoupon struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID  string     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name        string     `gorm:"type:varchar(128);comment:券名称" json:"name"`
	CouponType  string     `gorm:"type:varchar(32);comment:类型 discount(折扣)/cash(满减)/gift(兑换)" json:"coupon_type"`
	FaceValue   float64    `gorm:"type:decimal(16,2);comment:面值(折扣=折扣率如0.8,满减=减免金额)" json:"face_value"`
	Threshold   float64    `gorm:"type:decimal(16,2);default:0;comment:使用门槛(满X元可用)" json:"threshold"`
	TotalCount  int        `gorm:"type:int;default:0;comment:发放总量(0=不限)" json:"total_count"`
	IssuedCount int        `gorm:"type:int;default:0;comment:已发放数" json:"issued_count"`
	UsedCount   int        `gorm:"type:int;default:0;comment:已核销数" json:"used_count"`
	StartAt     *time.Time `gorm:"comment:生效时间" json:"start_at"`
	EndAt       *time.Time `gorm:"comment:失效时间" json:"end_at"`
	Status      string     `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmCouponRecord 优惠券发放/核销记录（一张券一个 code）。
type CrmCouponRecord struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CouponID   string     `gorm:"type:bigint;index;comment:优惠券ID" json:"coupon_id"`
	CustomerID string     `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	Code       string     `gorm:"type:varchar(64);uniqueIndex;comment:券码(唯一)" json:"code"`
	Status     string     `gorm:"type:varchar(16);index;default:issued;comment:issued/used/expired" json:"status"`
	IssuedAt   *time.Time `gorm:"comment:发放时间" json:"issued_at"`
	UsedAt     *time.Time `gorm:"comment:核销时间" json:"used_at"`
	OrderID    string     `gorm:"type:bigint;comment:核销关联订单ID(可空)" json:"order_id"`
	Remark     string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmFission 裂变活动配置。
type CrmFission struct {
	Model
	Timestamp
	TenantID         string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID       string `gorm:"type:bigint;index;comment:关联活动ID" json:"activity_id"`
	Name             string `gorm:"type:varchar(128);comment:裂变活动名称" json:"name"`
	RewardType       string `gorm:"type:varchar(32);comment:奖励类型 coupon/cash/gift" json:"reward_type"`
	RewardThreshold  int    `gorm:"type:int;default:1;comment:邀请几人触发奖励" json:"reward_threshold"`
	MaxRewardPerUser int    `gorm:"type:int;default:1;comment:每人最多奖励次数" json:"max_reward_per_user"`
	Status           string `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
	Remark           string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmFissionInvite 裁变邀请关系记录。
type CrmFissionInvite struct {
	Model
	Timestamp
	TenantID          string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	FissionID         string `gorm:"type:bigint;index;comment:裂变活动ID" json:"fission_id"`
	InviterCustomerID string `gorm:"type:bigint;index;comment:邀请人客户ID" json:"inviter_customer_id"`
	InviteeCustomerID string `gorm:"type:bigint;index;comment:被邀请人客户ID" json:"invitee_customer_id"`
	RewardTriggered   bool   `gorm:"comment:是否已触发奖励" json:"reward_triggered"`
	Remark            string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmLandingPage H5 落地页配置。
type CrmLandingPage struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID  string                     `gorm:"type:bigint;index;comment:关联活动ID" json:"activity_id"`
	Title       string                     `gorm:"type:varchar(255);comment:落地页标题" json:"title"`
	Slug        string                     `gorm:"type:varchar(64);uniqueIndex;comment:URL slug(唯一)" json:"slug"`
	Template    string                     `gorm:"type:varchar(32);default:default;comment:模板 default/simple/form" json:"template"`
	FormFields  constants.StringArrayField `gorm:"type:jsonb;comment:表单字段(name/mobile/company)" json:"form_fields"`
	SubmitCount int                        `gorm:"type:int;default:0;comment:提交数" json:"submit_count"`
	Status      string                     `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
}

// ---- Phase 4 自动化与智能化 ----

// CrmSopRule SOP 自动化规则：触发条件 → 执行动作。
// 触发条件：stage_change(商机阶段变化)/tag_added(打标签)/pool_claim(公海认领)/new_customer(新客户)。
// 执行动作：create_followup(创建跟进任务)/send_message(发消息提醒销售)。
type CrmSopRule struct {
	Model
	Timestamp
	TenantID      string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name          string `gorm:"type:varchar(128);comment:规则名称" json:"name"`
	TriggerType   string `gorm:"type:varchar(32);index;comment:触发类型 stage_change/tag_added/pool_claim/new_customer" json:"trigger_type"`
	TriggerCond   string `gorm:"type:varchar(255);comment:触发条件(如stage_id或tag_name)" json:"trigger_cond"`
	ActionType    string `gorm:"type:varchar(32);comment:动作类型 create_followup/send_message" json:"action_type"`
	ActionContent string `gorm:"type:text;comment:动作内容(跟进内容模板/消息内容)" json:"action_content"`
	FollowupType  string `gorm:"type:varchar(32);comment:跟进类型(call/visit/message/meeting)" json:"followup_type"`
	DelayHours    int    `gorm:"type:int;default:0;comment:延迟执行小时数" json:"delay_hours"`
	Status        string `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
	Remark        string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmSopTask SOP 触发的任务实例。
type CrmSopTask struct {
	Model
	Timestamp
	TenantID      string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	RuleID        string     `gorm:"type:bigint;index;comment:规则ID" json:"rule_id"`
	CustomerID    string     `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	OpportunityID string     `gorm:"type:bigint;comment:商机ID(可空)" json:"opportunity_id"`
	ActionType    string     `gorm:"type:varchar(32);comment:动作类型" json:"action_type"`
	ActionContent string     `gorm:"type:text;comment:动作内容" json:"action_content"`
	Status        string     `gorm:"type:varchar(16);index;default:pending;comment:pending/done/skipped" json:"status"`
	ExecuteAt     *time.Time `gorm:"comment:计划执行时间" json:"execute_at"`
	DoneAt        *time.Time `gorm:"comment:完成时间" json:"done_at"`
	AssignUserID  string     `gorm:"type:bigint;comment:分配销售ID" json:"assign_user_id"`
}

// CrmAutoTagRule 自动打标签规则：渠道行为 → 自动打标签。
type CrmAutoTagRule struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name        string `gorm:"type:varchar(128);comment:规则名称" json:"name"`
	Channel     string `gorm:"type:varchar(32);comment:渠道标识" json:"channel"`
	TriggerType string `gorm:"type:varchar(32);comment:触发类型 interaction_count/order_amount/has_followup" json:"trigger_type"`
	TriggerCond string `gorm:"type:varchar(255);comment:触发条件(如>=3或>=1000)" json:"trigger_cond"`
	TagName     string `gorm:"type:varchar(64);comment:要打的标签名" json:"tag_name"`
	Status      string `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
	Remark      string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmSalesScript 销售话术库：按阶段/行业/场景分类。
type CrmSalesScript struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Title    string `gorm:"type:varchar(255);comment:话术标题" json:"title"`
	Stage    string `gorm:"type:varchar(32);index;comment:适用阶段(prospect/qualify/propose/negotiate/close)" json:"stage"`
	Industry string `gorm:"type:varchar(64);index;comment:适用行业(可空=通用)" json:"industry"`
	Scene    string `gorm:"type:varchar(32);comment:场景 cold_call/follow_up/objection/closing" json:"scene"`
	Content  string `gorm:"type:text;comment:话术内容" json:"content"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
	Status   string `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
}

// ---- Phase 6 营销活动深化 ----

// CrmSeckill 秒杀活动。
type CrmSeckill struct {
	Model
	Timestamp
	TenantID      string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID    string     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name          string     `gorm:"type:varchar(128);comment:秒杀活动名称" json:"name"`
	ProductName   string     `gorm:"type:varchar(255);comment:商品名称" json:"product_name"`
	OriginalPrice float64    `gorm:"type:decimal(16,2);comment:原价" json:"original_price"`
	SeckillPrice  float64    `gorm:"type:decimal(16,2);comment:秒杀价" json:"seckill_price"`
	TotalStock    int        `gorm:"type:int;comment:总库存" json:"total_stock"`
	SoldCount     int        `gorm:"type:int;default:0;comment:已售数" json:"sold_count"`
	LimitPerUser  int        `gorm:"type:int;default:1;comment:每人限购" json:"limit_per_user"`
	StartAt       *time.Time `gorm:"comment:开始时间" json:"start_at"`
	EndAt         *time.Time `gorm:"comment:结束时间" json:"end_at"`
	Status        string     `gorm:"type:varchar(16);index;default:active;comment:active/ended/disabled" json:"status"`
	Remark        string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmSeckillRecord 秒杀记录。
type CrmSeckillRecord struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	SeckillID  string     `gorm:"type:bigint;index;comment:秒杀活动ID" json:"seckill_id"`
	CustomerID string     `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	Price      float64    `gorm:"type:decimal(16,2);comment:成交价(秒杀价)" json:"price"`
	OrderID    string     `gorm:"type:bigint;comment:关联订单ID(可空)" json:"order_id"`
	Status     string     `gorm:"type:varchar(16);index;default:grabbed;comment:grabbed/paid/cancelled" json:"status"`
	GrabbedAt  *time.Time `gorm:"comment:抢购时间" json:"grabbed_at"`
	Remark     string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmLottery 抽奖活动。
type CrmLottery struct {
	Model
	Timestamp
	TenantID   string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID string                     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name       string                     `gorm:"type:varchar(128);comment:抽奖活动名称" json:"name"`
	Prizes     constants.StringArrayField `gorm:"type:jsonb;comment:奖品配置JSON[{name,type,weight,count,issued}]" json:"prizes"`
	DailyLimit int                        `gorm:"type:int;default:1;comment:每人每日抽奖次数" json:"daily_limit"`
	TotalLimit int                        `gorm:"type:int;default:0;comment:总抽奖次数(0=不限)" json:"total_limit"`
	DrawnCount int                        `gorm:"type:int;default:0;comment:已抽次数" json:"drawn_count"`
	StartAt    *time.Time                 `gorm:"comment:开始时间" json:"start_at"`
	EndAt      *time.Time                 `gorm:"comment:结束时间" json:"end_at"`
	Status     string                     `gorm:"type:varchar(16);index;default:active;comment:active/ended/disabled" json:"status"`
	Remark     string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmLotteryRecord 抽奖记录。
type CrmLotteryRecord struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	LotteryID  string     `gorm:"type:bigint;index;comment:抽奖活动ID" json:"lottery_id"`
	CustomerID string     `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	PrizeName  string     `gorm:"type:varchar(128);comment:奖品名称" json:"prize_name"`
	PrizeType  string     `gorm:"type:varchar(32);comment:奖品类型 coupon/cash/gift/thanks" json:"prize_type"`
	IsWin      bool       `gorm:"comment:是否中奖(thanks=未中奖)" json:"is_win"`
	DrawnAt    *time.Time `gorm:"comment:抽奖时间" json:"drawn_at"`
	Remark     string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// ---- A 组：SCRM 风控与客户资产保护 ----

// CrmSensitiveWord 敏感词库（会话风控）。
type CrmSensitiveWord struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Word     string `gorm:"type:varchar(128);comment:敏感词" json:"word"`
	Category string `gorm:"type:varchar(32);index;comment:分类 fly_single(飞单)/abuse(辱骂)/privacy(隐私)/other" json:"category"`
	Action   string `gorm:"type:varchar(16);default:warn;comment:动作 warn(提醒)/block(拦截)" json:"action"`
	Status   string `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
}

// CrmSensitiveAlert 敏感词触发告警记录。
type CrmSensitiveAlert struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID     string `gorm:"type:bigint;index;comment:触发员工ID" json:"user_id"`
	CustomerID string `gorm:"type:bigint;index;comment:客户ID(可空)" json:"customer_id"`
	Word       string `gorm:"type:varchar(128);comment:触发的敏感词" json:"word"`
	Category   string `gorm:"type:varchar(32);comment:敏感词分类" json:"category"`
	Content    string `gorm:"type:text;comment:触发上下文(消息片段)" json:"content"`
	Action     string `gorm:"type:varchar(16);comment:动作 warn/block" json:"action"`
	Status     string `gorm:"type:varchar(16);default:pending;comment:pending/handled/ignored" json:"status"`
}

// CrmInheritance 客户继承记录（离职/在职继承）。
type CrmInheritance struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	FromUserID  string                     `gorm:"type:bigint;index;comment:原归属销售ID" json:"from_user_id"`
	ToUserID    string                     `gorm:"type:bigint;index;comment:新归属销售ID" json:"to_user_id"`
	Type        string                     `gorm:"type:varchar(16);comment:resign(离职继承)/transfer(在职继承)" json:"type"`
	CustomerIDs constants.StringArrayField `gorm:"type:jsonb;comment:继承的客户ID列表" json:"customer_ids"`
	CustomerCnt int                        `gorm:"type:int;comment:继承客户数" json:"customer_cnt"`
	Status      string                     `gorm:"type:varchar(16);default:done;comment:done/failed" json:"status"`
	Remark      string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmCustomerLoss 客户流失/删除提醒记录。
type CrmCustomerLoss struct {
	Model
	Timestamp
	TenantID     string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID   string `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	CustomerName string `gorm:"type:varchar(128);comment:客户名(快照)" json:"customer_name"`
	UserID       string `gorm:"type:bigint;index;comment:原归属销售ID" json:"user_id"`
	LossType     string `gorm:"type:varchar(32);index;comment:deleted(被删)/unfollowed(长期未跟进)/churned(流失)" json:"loss_type"`
	Days         int    `gorm:"type:int;comment:未跟进天数(loss_type=unfollowed时)" json:"days"`
	Status       string `gorm:"type:varchar(16);default:pending;comment:pending/handled" json:"status"`
	Remark       string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmReplyTimeout 超时未回复提醒记录。
type CrmReplyTimeout struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID      string     `gorm:"type:bigint;index;comment:应回复销售ID" json:"user_id"`
	CustomerID  string     `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	Channel     string     `gorm:"type:varchar(32);comment:渠道" json:"channel"`
	LastMsgAt   *time.Time `gorm:"comment:客户最后消息时间" json:"last_msg_at"`
	TimeoutMins int        `gorm:"type:int;comment:超时分钟数" json:"timeout_mins"`
	Status      string     `gorm:"type:varchar(16);default:pending;comment:pending/replied/escalated" json:"status"`
}

// CrmGroupSop 群SOP规则（群运营标准化）。
type CrmGroupSop struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name        string                     `gorm:"type:varchar(128);comment:规则名称" json:"name"`
	GroupIDs    constants.StringArrayField `gorm:"type:jsonb;comment:目标群聊ID列表(空=全部群)" json:"group_ids"`
	TriggerType string                     `gorm:"type:varchar(32);comment:trigger: daily(每日定时)/new_member(新成员入群)/keyword(关键词)" json:"trigger_type"`
	TriggerData string                     `gorm:"type:text;comment:触发配置JSON(如每日时间/关键词列表)" json:"trigger_data"`
	ActionType  string                     `gorm:"type:varchar(32);comment:动作 send_message(发消息)/send_material(发素材)" json:"action_type"`
	ActionData  string                     `gorm:"type:text;comment:动作内容(消息文本/素材ID)" json:"action_data"`
	Status      string                     `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
}

// CrmGroupSopTask 群SOP执行任务。
type CrmGroupSopTask struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	RuleID     string     `gorm:"type:bigint;index;comment:群SOP规则ID" json:"rule_id"`
	GroupID    string     `gorm:"type:bigint;index;comment:群聊ID" json:"group_id"`
	UserID     string     `gorm:"type:bigint;index;comment:执行人(群主)ID" json:"user_id"`
	ActionType string     `gorm:"type:varchar(32);comment:动作类型" json:"action_type"`
	ActionData string     `gorm:"type:text;comment:动作内容" json:"action_data"`
	ExecuteAt  *time.Time `gorm:"comment:应执行时间" json:"execute_at"`
	Status     string     `gorm:"type:varchar(16);index;default:pending;comment:pending/done/skipped" json:"status"`
	DoneAt     *time.Time `gorm:"comment:实际执行时间" json:"done_at"`
}

// ---- B 组：营销活动补充 ----

// CrmGroupBuy 拼团活动。
type CrmGroupBuy struct {
	Model
	Timestamp
	TenantID      string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID    string     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name          string     `gorm:"type:varchar(128);comment:拼团活动名称" json:"name"`
	ProductName   string     `gorm:"type:varchar(255);comment:商品名称" json:"product_name"`
	OriginalPrice float64    `gorm:"type:decimal(16,2);comment:原价" json:"original_price"`
	GroupPrice    float64    `gorm:"type:decimal(16,2);comment:拼团价" json:"group_price"`
	GroupSize     int        `gorm:"type:int;comment:成团人数" json:"group_size"`
	TimeLimit     int        `gorm:"type:int;comment:成团时限(小时)" json:"time_limit"`
	MaxGroups     int        `gorm:"type:int;default:0;comment:最大开团数(0=不限)" json:"max_groups"`
	OpenedGroups  int        `gorm:"type:int;default:0;comment:已开团数" json:"opened_groups"`
	StartAt       *time.Time `gorm:"comment:开始时间" json:"start_at"`
	EndAt         *time.Time `gorm:"comment:结束时间" json:"end_at"`
	Status        string     `gorm:"type:varchar(16);index;default:active;comment:active/ended/disabled" json:"status"`
	Remark        string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmGroupBuyRecord 拼团记录（一个团一条）。
type CrmGroupBuyRecord struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	GroupBuyID  string                     `gorm:"type:bigint;index;comment:拼团活动ID" json:"group_buy_id"`
	LeaderID    string                     `gorm:"type:bigint;index;comment:团长客户ID" json:"leader_id"`
	MemberIDs   constants.StringArrayField `gorm:"type:jsonb;comment:参团客户ID列表" json:"member_ids"`
	CurrentSize int                        `gorm:"type:int;default:1;comment:当前参团人数" json:"current_size"`
	Status      string                     `gorm:"type:varchar(16);index;default:open;comment:open(进行中)/success(成团)/failed(超时未成团)" json:"status"`
	ExpireAt    *time.Time                 `gorm:"comment:成团截止时间" json:"expire_at"`
	SuccessAt   *time.Time                 `gorm:"comment:成团时间" json:"success_at"`
}

// CrmBargain 砍价活动。
type CrmBargain struct {
	Model
	Timestamp
	TenantID      string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID    string     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name          string     `gorm:"type:varchar(128);comment:砍价活动名称" json:"name"`
	ProductName   string     `gorm:"type:varchar(255);comment:商品名称" json:"product_name"`
	OriginalPrice float64    `gorm:"type:decimal(16,2);comment:原价" json:"original_price"`
	FloorPrice    float64    `gorm:"type:decimal(16,2);comment:底价(砍到最低)" json:"floor_price"`
	MaxHelpers    int        `gorm:"type:int;comment:最多帮砍人数" json:"max_helpers"`
	PerCutMin     float64    `gorm:"type:decimal(16,2);comment:每人最少砍金额" json:"per_cut_min"`
	PerCutMax     float64    `gorm:"type:decimal(16,2);comment:每人最多砍金额" json:"per_cut_max"`
	TimeLimit     int        `gorm:"type:int;comment:砍价时限(小时)" json:"time_limit"`
	StartAt       *time.Time `gorm:"comment:开始时间" json:"start_at"`
	EndAt         *time.Time `gorm:"comment:结束时间" json:"end_at"`
	Status        string     `gorm:"type:varchar(16);index;default:active;comment:active/ended/disabled" json:"status"`
	Remark        string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmBargainRecord 砍价记录（一个客户发起一条）。
type CrmBargainRecord struct {
	Model
	Timestamp
	TenantID     string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	BargainID    string                     `gorm:"type:bigint;index;comment:砍价活动ID" json:"bargain_id"`
	CustomerID   string                     `gorm:"type:bigint;index;comment:发起人客户ID" json:"customer_id"`
	CurrentPrice float64                    `gorm:"type:decimal(16,2);comment:当前价格" json:"current_price"`
	HelperIDs    constants.StringArrayField `gorm:"type:jsonb;comment:帮砍客户ID列表" json:"helper_ids"`
	HelperCnt    int                        `gorm:"type:int;default:0;comment:帮砍人数" json:"helper_cnt"`
	Status       string                     `gorm:"type:varchar(16);index;default:active;comment:active/success(砍到底价)/expired/purchased" json:"status"`
	ExpireAt     *time.Time                 `gorm:"comment:截止时间" json:"expire_at"`
}

// CrmFullReduction 满减满件活动。
type CrmFullReduction struct {
	Model
	Timestamp
	TenantID   string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID string                     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name       string                     `gorm:"type:varchar(128);comment:活动名称" json:"name"`
	RuleType   string                     `gorm:"type:varchar(16);comment:full_amount(满额)/full_count(满件)" json:"rule_type"`
	Rules      constants.StringArrayField `gorm:"type:jsonb;comment:阶梯规则JSON[{threshold,discount}]" json:"rules"`
	StartAt    *time.Time                 `gorm:"comment:开始时间" json:"start_at"`
	EndAt      *time.Time                 `gorm:"comment:结束时间" json:"end_at"`
	Status     string                     `gorm:"type:varchar(16);index;default:active;comment:active/ended/disabled" json:"status"`
	Remark     string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmRedPacket 红包获客活动。
type CrmRedPacket struct {
	Model
	Timestamp
	TenantID       string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID     string     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name           string     `gorm:"type:varchar(128);comment:红包活动名称" json:"name"`
	TotalAmount    float64    `gorm:"type:decimal(16,2);comment:红包总金额" json:"total_amount"`
	PerAmount      float64    `gorm:"type:decimal(16,2);comment:单个红包金额" json:"per_amount"`
	TotalCount     int        `gorm:"type:int;comment:红包总个数" json:"total_count"`
	IssuedCount    int        `gorm:"type:int;default:0;comment:已发放个数" json:"issued_count"`
	InviteRequired int        `gorm:"type:int;default:0;comment:需邀请人数(0=无门槛)" json:"invite_required"`
	StartAt        *time.Time `gorm:"comment:开始时间" json:"start_at"`
	EndAt          *time.Time `gorm:"comment:结束时间" json:"end_at"`
	Status         string     `gorm:"type:varchar(16);index;default:active;comment:active/ended/disabled" json:"status"`
	Remark         string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmRedPacketRecord 红包领取记录。
type CrmRedPacketRecord struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	RedPacketID string     `gorm:"type:bigint;index;comment:红包活动ID" json:"red_packet_id"`
	CustomerID  string     `gorm:"type:bigint;index;comment:领取客户ID" json:"customer_id"`
	Amount      float64    `gorm:"type:decimal(16,2);comment:领取金额" json:"amount"`
	InviteCnt   int        `gorm:"type:int;default:0;comment:邀请人数" json:"invite_cnt"`
	Status      string     `gorm:"type:varchar(16);default:issued;comment:issued/received/expired" json:"status"`
	IssuedAt    *time.Time `gorm:"comment:发放时间" json:"issued_at"`
}

// CrmSignIn 签到送券活动。
type CrmSignIn struct {
	Model
	Timestamp
	TenantID          string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ActivityID        string                     `gorm:"type:bigint;index;comment:关联活动ID(可空)" json:"activity_id"`
	Name              string                     `gorm:"type:varchar(128);comment:签到活动名称" json:"name"`
	DailyCouponID     string                     `gorm:"type:bigint;comment:每日签到送券ID" json:"daily_coupon_id"`
	ContinuousRewards constants.StringArrayField `gorm:"type:jsonb;comment:连续签到奖励JSON[{days,coupon_id}]" json:"continuous_rewards"`
	StartAt           *time.Time                 `gorm:"comment:开始时间" json:"start_at"`
	EndAt             *time.Time                 `gorm:"comment:结束时间" json:"end_at"`
	Status            string                     `gorm:"type:varchar(16);index;default:active;comment:active/ended/disabled" json:"status"`
	Remark            string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmSignInRecord 签到记录。
type CrmSignInRecord struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	SignInID   string `gorm:"type:bigint;index;comment:签到活动ID" json:"sign_in_id"`
	CustomerID string `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	Day        string `gorm:"type:varchar(10);comment:签到日期 YYYY-MM-DD" json:"day"`
	Continuous int    `gorm:"type:int;default:1;comment:连续签到天数" json:"continuous"`
	CouponID   string `gorm:"type:bigint;comment:获得券ID" json:"coupon_id"`
}

// ---- C4 智能名片 ----

// CrmSmartCard 智能名片（销售社交工具）。
type CrmSmartCard struct {
	Model
	Timestamp
	TenantID   string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID     string                     `gorm:"type:bigint;index;comment:销售用户ID" json:"user_id"`
	Name       string                     `gorm:"type:varchar(64);comment:姓名" json:"name"`
	Title      string                     `gorm:"type:varchar(128);comment:职位/头衔" json:"title"`
	Company    string                     `gorm:"type:varchar(128);comment:公司名" json:"company"`
	Phone      string                     `gorm:"type:varchar(32);comment:电话" json:"phone"`
	Email      string                     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Avatar     string                     `gorm:"type:varchar(512);comment:头像URL" json:"avatar"`
	Intro      string                     `gorm:"type:text;comment:个人简介" json:"intro"`
	Tags       constants.StringArrayField `gorm:"type:jsonb;comment:专业标签" json:"tags"`
	Products   constants.StringArrayField `gorm:"type:jsonb;comment:产品/服务列表JSON[{name,desc}]" json:"products"`
	Cases      constants.StringArrayField `gorm:"type:jsonb;comment:案例列表JSON[{name,desc}]" json:"cases"`
	ViewCount  int                        `gorm:"type:int;default:0;comment:浏览数" json:"view_count"`
	ShareCount int                        `gorm:"type:int;default:0;comment:分享数" json:"share_count"`
	ShareCode  string                     `gorm:"type:varchar(32);index;comment:分享码(短链标识)" json:"share_code"`
	Status     string                     `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
}

// CrmSmartCardView 名片浏览记录（访客轨迹）。
type CrmSmartCardView struct {
	Model
	Timestamp
	TenantID  string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CardID    string `gorm:"type:bigint;index;comment:名片ID" json:"card_id"`
	VisitorID string `gorm:"type:bigint;index;comment:访客客户ID(可空)" json:"visitor_id"`
	VisitorIP string `gorm:"type:varchar(64);comment:访客IP" json:"visitor_ip"`
	VisitorUA string `gorm:"type:varchar(512);comment:访客User-Agent" json:"visitor_ua"`
	Source    string `gorm:"type:varchar(32);comment:来源 wecom/xhs/share/qr" json:"source"`
	Duration  int    `gorm:"type:int;comment:停留秒数" json:"duration"`
}

// ==================== P1: CRM 核心补全 ====================

// CrmContact 客户联系人（客户下多联系人：决策人/经办人）。
type CrmContact struct {
	Model
	Timestamp
	TenantID        string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID      string `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	Name            string `gorm:"type:varchar(64);comment:姓名" json:"name"`
	Title           string `gorm:"type:varchar(64);comment:职位" json:"title"`
	Phone           string `gorm:"type:varchar(32);comment:电话" json:"phone"`
	Email           string `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Wechat          string `gorm:"type:varchar(64);comment:微信号" json:"wechat"`
	IsPrimary       bool   `gorm:"type:bool;default:false;comment:是否主联系人" json:"is_primary"`
	IsDecisionMaker bool   `gorm:"type:bool;default:false;comment:是否决策人" json:"is_decision_maker"`
	Remark          string `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Department      string `gorm:"type:varchar(128);comment:所属部门" json:"department"`
	Address         string `gorm:"type:varchar(255);comment:地址" json:"address"`
	Birthday        string `gorm:"type:varchar(32);comment:生日" json:"birthday"`
	Gender          int    `gorm:"type:smallint;default:0;comment:0-未知 1-男 2-女" json:"gender"`
}

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
	TenantID      string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OrderID       string                     `gorm:"type:bigint;index;comment:关联合同订单ID" json:"order_id"`
	CustomerID    string                     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	PlanID        string                     `gorm:"type:bigint;index;comment:关联回款计划ID(可空)" json:"plan_id"`
	Amount        float64                    `gorm:"type:decimal(16,2);comment:回款金额" json:"amount"`
	ReceivedAt    *time.Time                 `gorm:"comment:实际回款日期" json:"received_at"`
	PayMethod     string                     `gorm:"type:varchar(32);comment:收款方式 cash/transfer/wechat/alipay" json:"pay_method"`
	PayAccount    string                     `gorm:"type:varchar(128);comment:收款账户" json:"pay_account"`
	VoucherNo     string                     `gorm:"type:varchar(64);comment:凭证号" json:"voucher_no"`
	Attachments   constants.StringArrayField `gorm:"type:jsonb;comment:附件URL列表" json:"attachments"`
	Status        string                     `gorm:"type:varchar(16);index;default:confirmed;comment:pending(待确认)/confirmed(已确认)/void(已作废)" json:"status"`
	Remark        string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	ReceivablesNo string                     `gorm:"type:varchar(32);index;comment:回款单号" json:"receivables_no"`
	ApproverID    string                     `gorm:"type:bigint;comment:审批人ID" json:"approver_id"`
	ApproveStatus string                     `gorm:"type:varchar(16);default:none;comment:none(无需审批)/pending(待审)/approved(通过)/rejected(驳回)" json:"approve_status"`
	ApproveAt     *time.Time                 `gorm:"comment:审批时间" json:"approve_at"`
	ApproveRemark string                     `gorm:"type:varchar(512);comment:审批意见" json:"approve_remark"`
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

// CrmTodo 待办/消息聚合。
type CrmTodo struct {
	Model
	Timestamp
	TenantID string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID   string     `gorm:"type:bigint;index;comment:归属用户ID" json:"user_id"`
	Type     string     `gorm:"type:varchar(32);index;comment:类型 followup/contract_expire/receivables_expire/loss/timeout/task" json:"type"`
	Title    string     `gorm:"type:varchar(255);comment:标题" json:"title"`
	Content  string     `gorm:"type:text;comment:内容" json:"content"`
	RefType  string     `gorm:"type:varchar(32);comment:关联类型 customer/order/lead/opportunity" json:"ref_type"`
	RefID    string     `gorm:"type:bigint;comment:关联ID" json:"ref_id"`
	DueAt    *time.Time `gorm:"comment:到期时间" json:"due_at"`
	Status   string     `gorm:"type:varchar(16);index;default:pending;comment:pending(待处理)/done(已处理)/ignored(已忽略)" json:"status"`
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

// CrmSignin 拜访签到。
type CrmSignin struct {
	Model
	Timestamp
	TenantID   string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID     string                     `gorm:"type:bigint;index;comment:签到人" json:"user_id"`
	CustomerID string                     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	Latitude   float64                    `gorm:"type:decimal(10,7);comment:纬度" json:"latitude"`
	Longitude  float64                    `gorm:"type:decimal(10,7);comment:经度" json:"longitude"`
	Address    string                     `gorm:"type:varchar(255);comment:签到地址" json:"address"`
	Photos     constants.StringArrayField `gorm:"type:jsonb;comment:签到照片URL列表" json:"photos"`
	Remark     string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmQuickFollowup 快捷跟进话术（个人常用）。
type CrmQuickFollowup struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID   string `gorm:"type:bigint;index;comment:归属用户" json:"user_id"`
	Content  string `gorm:"type:varchar(512);comment:跟进内容" json:"content"`
	Type     string `gorm:"type:varchar(32);comment:跟进类型 phone/visit/wechat/email" json:"type"`
	Sort     int    `gorm:"type:int;comment:排序" json:"sort"`
}

// ==================== P2: 重要增强 ====================

// CrmFieldDef 动态字段定义（客户/线索/商机/合同可配置字段）。
type CrmFieldDef struct {
	Model
	Timestamp
	TenantID     string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	EntityType   string                     `gorm:"type:varchar(32);index;comment:实体类型 customer/lead/opportunity/order" json:"entity_type"`
	FieldKey     string                     `gorm:"type:varchar(64);comment:字段标识" json:"field_key"`
	FieldLabel   string                     `gorm:"type:varchar(64);comment:字段名称" json:"field_label"`
	FieldType    string                     `gorm:"type:varchar(32);comment:字段类型 text/number/select/multiselect/date/datetime/textarea/file" json:"field_type"`
	Options      constants.StringArrayField `gorm:"type:jsonb;comment:选项列表(select/multiselect用)" json:"options"`
	IsRequired   bool                       `gorm:"type:bool;default:false;comment:是否必填" json:"is_required"`
	IsSearchable bool                       `gorm:"type:bool;default:false;comment:是否可搜索" json:"is_searchable"`
	DefaultValue string                     `gorm:"type:varchar(512);comment:默认值" json:"default_value"`
	Sort         int                        `gorm:"type:int;comment:排序" json:"sort"`
	Status       string                     `gorm:"type:varchar(16);default:active;comment:active/disabled" json:"status"`
}

// CrmFieldValue 动态字段值。
type CrmFieldValue struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	EntityType string `gorm:"type:varchar(32);index;comment:实体类型" json:"entity_type"`
	EntityID   string `gorm:"type:bigint;index;comment:实体ID" json:"entity_id"`
	FieldKey   string `gorm:"type:varchar(64);index;comment:字段标识" json:"field_key"`
	Value      string `gorm:"type:text;comment:字段值" json:"value"`
}

// CrmScene 列表场景（自定义筛选条件集合）。
type CrmScene struct {
	Model
	Timestamp
	TenantID   string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID     string                     `gorm:"type:bigint;index;comment:归属用户(0=租户级)" json:"user_id"`
	EntityType string                     `gorm:"type:varchar(32);index;comment:实体类型 customer/lead/opportunity" json:"entity_type"`
	Name       string                     `gorm:"type:varchar(64);comment:场景名称" json:"name"`
	Conditions constants.StringArrayField `gorm:"type:jsonb;comment:筛选条件JSON" json:"conditions"`
	IsDefault  bool                       `gorm:"type:bool;default:false;comment:是否默认场景" json:"is_default"`
	IsShared   bool                       `gorm:"type:bool;default:false;comment:是否共享给团队" json:"is_shared"`
	Sort       int                        `gorm:"type:int;comment:排序" json:"sort"`
}

// CrmApprovalFlow 审批流定义。
type CrmApprovalFlow struct {
	Model
	Timestamp
	TenantID  string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	BizType   string                     `gorm:"type:varchar(32);index;comment:业务类型 contract/quote/receivables/expense" json:"biz_type"`
	Name      string                     `gorm:"type:varchar(128);comment:审批流名称" json:"name"`
	Nodes     constants.StringArrayField `gorm:"type:jsonb;comment:审批节点JSON[{name,approver_type,approver_ids}]" json:"nodes"`
	IsEnabled bool                       `gorm:"type:bool;default:true;comment:是否启用" json:"is_enabled"`
}

// CrmApprovalInstance 审批实例。
type CrmApprovalInstance struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	FlowID      string `gorm:"type:bigint;index;comment:审批流ID" json:"flow_id"`
	BizType     string `gorm:"type:varchar(32);index;comment:业务类型" json:"biz_type"`
	BizID       string `gorm:"type:bigint;index;comment:业务单据ID" json:"biz_id"`
	ApplicantID string `gorm:"type:bigint;comment:申请人" json:"applicant_id"`
	CurrentNode int    `gorm:"type:int;comment:当前节点序号" json:"current_node"`
	Status      string `gorm:"type:varchar(16);index;default:pending;comment:pending(审批中)/approved(通过)/rejected(驳回)/void(已撤回)" json:"status"`
}

// CrmApprovalRecord 审批操作记录。
type CrmApprovalRecord struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	InstanceID string `gorm:"type:bigint;index;comment:审批实例ID" json:"instance_id"`
	NodeIndex  int    `gorm:"type:int;comment:节点序号" json:"node_index"`
	ApproverID string `gorm:"type:bigint;comment:审批人" json:"approver_id"`
	Action     string `gorm:"type:varchar(16);comment:动作 approve/reject/void" json:"action"`
	Comment    string `gorm:"type:varchar(512);comment:审批意见" json:"comment"`
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

// ==================== P3: 大模块扩展 ====================

// CrmProject 项目。
type CrmProject struct {
	Model
	Timestamp
	TenantID   string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name       string                     `gorm:"type:varchar(128);comment:项目名称" json:"name"`
	Type       string                     `gorm:"type:varchar(32);comment:项目类型" json:"type"`
	CustomerID string                     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	OrderID    string                     `gorm:"type:bigint;comment:关联合同ID" json:"order_id"`
	ManagerID  string                     `gorm:"type:bigint;index;comment:项目经理" json:"manager_id"`
	Amount     float64                    `gorm:"type:decimal(16,2);comment:项目金额" json:"amount"`
	StartDate  *time.Time                 `gorm:"comment:开始日期" json:"start_date"`
	EndDate    *time.Time                 `gorm:"comment:计划结束日期" json:"end_date"`
	Status     string                     `gorm:"type:varchar(16);index;default:planning;comment:planning(规划中)/active(进行中)/suspended(暂停)/completed(已完成)/cancelled(已取消)" json:"status"`
	Progress   int                        `gorm:"type:int;default:0;comment:进度百分比" json:"progress"`
	Remark     string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	ProjectNo  string                     `gorm:"type:varchar(32);index;comment:项目编号" json:"project_no"`
	ProcessID  string                     `gorm:"type:bigint;comment:流程模板ID" json:"process_id"`
	Stage      string                     `gorm:"type:varchar(64);comment:当前阶段" json:"stage"`
	TeamIDs    constants.StringArrayField `gorm:"type:jsonb;comment:团队成员ID列表" json:"team_ids"`
}

// CrmProjectTask 项目任务。
type CrmProjectTask struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ProjectID  string     `gorm:"type:bigint;index;comment:关联项目ID" json:"project_id"`
	Title      string     `gorm:"type:varchar(255);comment:任务标题" json:"title"`
	AssigneeID string     `gorm:"type:bigint;comment:负责人" json:"assignee_id"`
	Status     string     `gorm:"type:varchar(16);default:pending;comment:pending/doing/done" json:"status"`
	Priority   string     `gorm:"type:varchar(16);default:medium;comment:low/medium/high" json:"priority"`
	DueDate    *time.Time `gorm:"comment:截止日期" json:"due_date"`
	Sort       int        `gorm:"type:int;comment:排序" json:"sort"`
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

// CrmProduct 产品档案。
type CrmProduct struct {
	Model
	Timestamp
	TenantID   string  `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name       string  `gorm:"type:varchar(255);comment:产品名称" json:"name"`
	Category   string  `gorm:"type:varchar(64);comment:分类" json:"category"`
	Spec       string  `gorm:"type:varchar(255);comment:规格" json:"spec"`
	Unit       string  `gorm:"type:varchar(32);comment:单位" json:"unit"`
	Price      float64 `gorm:"type:decimal(16,2);comment:售价" json:"price"`
	Cost       float64 `gorm:"type:decimal(16,2);comment:成本价" json:"cost"`
	SKU        string  `gorm:"type:varchar(64);index;comment:SKU编码" json:"sku"`
	Status     string  `gorm:"type:varchar(16);default:active;comment:active/disabled" json:"status"`
	Remark     string  `gorm:"type:varchar(512);comment:备注" json:"remark"`
	ProductNo  string  `gorm:"type:varchar(32);index;comment:产品编号" json:"product_no"`
	CategoryID string  `gorm:"type:bigint;index;comment:产品分类ID" json:"category_id"`
	StockQty   int     `gorm:"type:int;default:0;comment:库存数量" json:"stock_qty"`
	MinStock   int     `gorm:"type:int;default:0;comment:最小库存(预警下限)" json:"min_stock"`
}

// CrmWarehouse 仓库。
type CrmWarehouse struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name        string `gorm:"type:varchar(128);comment:仓库名称" json:"name"`
	Address     string `gorm:"type:varchar(255);comment:仓库地址" json:"address"`
	Manager     string `gorm:"type:bigint;comment:仓库管理员" json:"manager"`
	Status      string `gorm:"type:varchar(16);default:active;comment:active/disabled" json:"status"`
	WarehouseNo string `gorm:"type:varchar(32);index;comment:仓库编号" json:"warehouse_no"`
}

// CrmStockRecord 库存出入库记录。
type CrmStockRecord struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	WarehouseID string     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	ProductID   string     `gorm:"type:bigint;index;comment:产品ID" json:"product_id"`
	Type        string     `gorm:"type:varchar(16);index;comment:类型 instock(入库)/outstock(出库)/transfer(调拨)/check(盘点)" json:"type"`
	Qty         int        `gorm:"type:int;comment:数量(正入库/负出库)" json:"qty"`
	OrderNo     string     `gorm:"type:varchar(64);comment:关联单号" json:"order_no"`
	OccurAt     *time.Time `gorm:"comment:发生时间" json:"occur_at"`
	OperatorID  string     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmKnowledge 知识库。
type CrmKnowledge struct {
	Model
	Timestamp
	TenantID        string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CategoryID      string                     `gorm:"type:bigint;index;comment:分类ID" json:"category_id"`
	Title           string                     `gorm:"type:varchar(255);comment:标题" json:"title"`
	Content         string                     `gorm:"type:text;comment:内容(富文本)" json:"content"`
	Attachments     constants.StringArrayField `gorm:"type:jsonb;comment:附件列表" json:"attachments"`
	Visibility      string                     `gorm:"type:varchar(16);default:company;comment:company(全公司)/department(部门)/role(角色)/person(指定人)" json:"visibility"`
	AuthorID        string                     `gorm:"type:bigint;comment:作者" json:"author_id"`
	Status          string                     `gorm:"type:varchar(16);default:published;comment:draft/published/archived" json:"status"`
	KnowledgeTypeID string                     `gorm:"type:bigint;index;comment:知识库分类ID" json:"knowledge_type_id"`
	PublicType      string                     `gorm:"type:varchar(16);default:company;comment:公开类型 company/department/role/person" json:"public_type"`
}

// CrmNotice 公告。
type CrmNotice struct {
	Model
	Timestamp
	TenantID  string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Title     string     `gorm:"type:varchar(255);comment:标题" json:"title"`
	Content   string     `gorm:"type:text;comment:内容" json:"content"`
	Type      string     `gorm:"type:varchar(16);default:info;comment:info/warning/urgent" json:"type"`
	PublishAt *time.Time `gorm:"comment:发布时间" json:"publish_at"`
	ExpireAt  *time.Time `gorm:"comment:过期时间" json:"expire_at"`
	Status    string     `gorm:"type:varchar(16);default:draft;comment:draft/published/expired" json:"status"`
}

// CrmWorkReport 工作报告。
type CrmWorkReport struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID     string     `gorm:"type:bigint;index;comment:提交人" json:"user_id"`
	Type       string     `gorm:"type:varchar(16);comment:类型 daily/weekly/monthly" json:"type"`
	ReportDate *time.Time `gorm:"comment:报告日期" json:"report_date"`
	Content    string     `gorm:"type:text;comment:报告内容" json:"content"`
	Plan       string     `gorm:"type:text;comment:下期计划" json:"plan"`
	Status     string     `gorm:"type:varchar(16);default:submitted;comment:submitted(已提交)/reviewed(已审阅)" json:"status"`
	ReviewerID string     `gorm:"type:bigint;comment:审阅人" json:"reviewer_id"`
}

// CrmWorkorder 工单。
type CrmWorkorder struct {
	Model
	Timestamp
	TenantID        string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No              string     `gorm:"type:varchar(32);index;comment:工单号" json:"no"`
	Title           string     `gorm:"type:varchar(255);comment:标题" json:"title"`
	Type            string     `gorm:"type:varchar(32);comment:工单类型" json:"type"`
	CustomerID      string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	OrderID         string     `gorm:"type:bigint;comment:关联合同ID" json:"order_id"`
	Priority        string     `gorm:"type:varchar(16);default:medium;comment:low/medium/high/urgent" json:"priority"`
	AssigneeID      string     `gorm:"type:bigint;index;comment:处理人" json:"assignee_id"`
	Description     string     `gorm:"type:text;comment:问题描述" json:"description"`
	Status          string     `gorm:"type:varchar(16);index;default:open;comment:open(待处理)/processing(处理中)/done(已完成)/cancelled(已取消)" json:"status"`
	CreatedAt2      *time.Time `gorm:"comment:创建时间" json:"created_at2"`
	DoneAt          *time.Time `gorm:"comment:完成时间" json:"done_at"`
	WorkorderTypeID string     `gorm:"type:bigint;index;comment:工单类型ID" json:"workorder_type_id"`
	ProcessID       string     `gorm:"type:bigint;comment:流程ID" json:"process_id"`
	CurrentStage    string     `gorm:"type:varchar(64);comment:当前阶段" json:"current_stage"`
	Source          string     `gorm:"type:varchar(32);comment:来源 manual/wechat/system" json:"source"`
	Channel         string     `gorm:"type:varchar(32);comment:渠道" json:"channel"`
	ContactID       string     `gorm:"type:bigint;comment:联系人ID" json:"contact_id"`
	Satisfaction    int        `gorm:"type:smallint;default:0;comment:满意度1-5" json:"satisfaction"`
	HandlerRemark   string     `gorm:"type:text;comment:处理备注" json:"handler_remark"`
}

// CrmCallRecord 通话记录。
type CrmCallRecord struct {
	Model
	Timestamp
	TenantID     string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID       string     `gorm:"type:bigint;index;comment:呼叫人" json:"user_id"`
	CustomerID   string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	ContactID    string     `gorm:"type:bigint;comment:关联联系人ID" json:"contact_id"`
	Direction    string     `gorm:"type:varchar(16);comment:方向 outbound/inbound" json:"direction"`
	Phone        string     `gorm:"type:varchar(32);comment:对方号码" json:"phone"`
	Duration     int        `gorm:"type:int;comment:通话时长(秒)" json:"duration"`
	StartAt      *time.Time `gorm:"comment:通话开始时间" json:"start_at"`
	RecordingURL string     `gorm:"type:varchar(512);comment:录音URL" json:"recording_url"`
	Status       string     `gorm:"type:varchar(16);comment:状态 connected/no_answer/failed/busy" json:"status"`
	Remark       string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmAiAnalysis AI销售分析结果。
type CrmAiAnalysis struct {
	Model
	Timestamp
	TenantID     string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID   string                     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	Type         string                     `gorm:"type:varchar(32);index;comment:分析类型 customer_profile/call_analysis/objection/pain_point/emotion/keyword" json:"type"`
	Summary      string                     `gorm:"type:text;comment:分析摘要" json:"summary"`
	Details      constants.StringArrayField `gorm:"type:jsonb;comment:详细分析结果JSON" json:"details"`
	Score        float64                    `gorm:"type:decimal(8,2);comment:评分" json:"score"`
	Status       string                     `gorm:"type:varchar(16);default:completed;comment:pending/completed/failed" json:"status"`
	CallRecordID string                     `gorm:"type:bigint;index;comment:关联通话记录ID" json:"call_record_id"`
	StaffID      string                     `gorm:"type:bigint;index;comment:关联员工ID" json:"staff_id"`
	AnalysisAt   *time.Time                 `gorm:"comment:分析时间" json:"analysis_at"`
	ModelVersion string                     `gorm:"type:varchar(32);comment:模型版本" json:"model_version"`
}

// CrmAftersales 售后工单。
type CrmAftersales struct {
	Model
	Timestamp
	TenantID   string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No         string     `gorm:"type:varchar(32);index;comment:售后单号" json:"no"`
	OrderID    string     `gorm:"type:bigint;index;comment:关联订单ID" json:"order_id"`
	CustomerID string     `gorm:"type:bigint;index;comment:关联客户ID" json:"customer_id"`
	Type       string     `gorm:"type:varchar(32);comment:售后类型 refund/return/exchange/repair" json:"type"`
	Reason     string     `gorm:"type:text;comment:售后原因" json:"reason"`
	Amount     float64    `gorm:"type:decimal(16,2);comment:涉及金额" json:"amount"`
	Status     string     `gorm:"type:varchar(16);index;default:pending;comment:pending/processing/done/rejected" json:"status"`
	HandlerID  string     `gorm:"type:bigint;comment:处理人" json:"handler_id"`
	Result     string     `gorm:"type:text;comment:处理结果" json:"result"`
	DoneAt     *time.Time `gorm:"comment:完成时间" json:"done_at"`
	Remark     string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// ==================== 补全模型 ====================

// CrmLeadPool 线索池。
type CrmLeadPool struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	LeadID   string `gorm:"type:bigint;index;comment:线索ID" json:"lead_id"`
	Source   string `gorm:"type:varchar(32);comment:入池来源 manual/auto" json:"source"`
}

// CrmRecycle 回收站。
type CrmRecycle struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	EntityType string `gorm:"type:varchar(32);index;comment:实体类型 customer/lead/opportunity/order/contact" json:"entity_type"`
	EntityID   string `gorm:"type:bigint;index;comment:实体ID" json:"entity_id"`
	EntityName string `gorm:"type:varchar(255);comment:实体名称(冗余便于展示)" json:"entity_name"`
	DeletedBy  string `gorm:"type:bigint;comment:删除人用户ID" json:"deleted_by"`
}

// CrmBatchOp 批量操作记录。
type CrmBatchOp struct {
	Model
	Timestamp
	TenantID     string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OpType       string                     `gorm:"type:varchar(32);index;comment:操作类型 change/seas/lock/blacklist/edit_field" json:"op_type"`
	EntityType   string                     `gorm:"type:varchar(32);comment:实体类型" json:"entity_type"`
	EntityIDs    constants.StringArrayField `gorm:"type:jsonb;comment:实体ID列表" json:"entity_ids"`
	Status       string                     `gorm:"type:varchar(16);default:done;comment:pending/done/failed" json:"status"`
	SuccessCount int                        `gorm:"type:int;comment:成功数" json:"success_count"`
	FailCount    int                        `gorm:"type:int;comment:失败数" json:"fail_count"`
	OperatorID   string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Remark       string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmProjectEvent 项目事件（里程碑）。
type CrmProjectEvent struct {
	Model
	Timestamp
	TenantID  string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ProjectID string     `gorm:"type:bigint;index;comment:关联项目ID" json:"project_id"`
	Title     string     `gorm:"type:varchar(255);comment:事件标题" json:"title"`
	Sort      int        `gorm:"type:int;comment:排序" json:"sort"`
	Status    string     `gorm:"type:varchar(16);default:pending;comment:pending/done/cancelled" json:"status"`
	PlanDate  *time.Time `gorm:"comment:计划日期" json:"plan_date"`
	DoneAt    *time.Time `gorm:"comment:完成日期" json:"done_at"`
	Remark    string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmProjectFile 项目文档。
type CrmProjectFile struct {
	Model
	Timestamp
	TenantID  string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ProjectID string `gorm:"type:bigint;index;comment:关联项目ID" json:"project_id"`
	FileName  string `gorm:"type:varchar(255);comment:文件名" json:"file_name"`
	FileURL   string `gorm:"type:varchar(512);comment:文件URL" json:"file_url"`
	FileSize  int64  `gorm:"type:bigint;comment:文件大小" json:"file_size"`
	UploadBy  string `gorm:"type:bigint;comment:上传人" json:"upload_by"`
}

// CrmSupplier 供应商。
type CrmSupplier struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(255);comment:供应商名称" json:"name"`
	Contact  string `gorm:"type:varchar(64);comment:联系人" json:"contact"`
	Phone    string `gorm:"type:varchar(32);comment:电话" json:"phone"`
	Address  string `gorm:"type:varchar(255);comment:地址" json:"address"`
	Remark   string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmPurchaseOrder 采购订单。
type CrmPurchaseOrder struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string     `gorm:"type:varchar(32);index;comment:采购单号" json:"no"`
	SupplierID  string     `gorm:"type:bigint;index;comment:供应商ID" json:"supplier_id"`
	WarehouseID string     `gorm:"type:bigint;comment:入库仓库ID" json:"warehouse_id"`
	Amount      float64    `gorm:"type:decimal(16,2);comment:采购金额" json:"amount"`
	PaidAmount  float64    `gorm:"type:decimal(16,2);default:0;comment:已付金额" json:"paid_amount"`
	Status      string     `gorm:"type:varchar(16);default:draft;comment:draft/ordered/instocked/paid/cancelled" json:"status"`
	OrderDate   *time.Time `gorm:"comment:下单日期" json:"order_date"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmAttendance 考勤记录。
type CrmAttendance struct {
	Model
	Timestamp
	TenantID  string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID    string     `gorm:"type:bigint;index;comment:员工ID" json:"user_id"`
	Type      string     `gorm:"type:varchar(16);comment:类型 signin(签到)/signout(签退)/out(外勤)" json:"type"`
	ClockAt   *time.Time `gorm:"comment:打卡时间" json:"clock_at"`
	Latitude  float64    `gorm:"type:decimal(10,7);comment:纬度" json:"latitude"`
	Longitude float64    `gorm:"type:decimal(10,7);comment:经度" json:"longitude"`
	Address   string     `gorm:"type:varchar(255);comment:打卡地址" json:"address"`
	Remark    string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmLeave 请假记录。
type CrmLeave struct {
	Model
	Timestamp
	TenantID string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID   string     `gorm:"type:bigint;index;comment:员工ID" json:"user_id"`
	Type     string     `gorm:"type:varchar(32);comment:请假类型 sick/personal/annual/other" json:"type"`
	StartAt  *time.Time `gorm:"comment:开始时间" json:"start_at"`
	EndAt    *time.Time `gorm:"comment:结束时间" json:"end_at"`
	Days     float64    `gorm:"type:decimal(8,2);comment:请假天数" json:"days"`
	Reason   string     `gorm:"type:varchar(512);comment:请假原因" json:"reason"`
	Status   string     `gorm:"type:varchar(16);default:pending;comment:pending/approved/rejected" json:"status"`
}

// CrmAutoAllocation 自动分配规则。
type CrmAutoAllocation struct {
	Model
	Timestamp
	TenantID   string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	EntityType string                     `gorm:"type:varchar(32);comment:实体类型 customer/lead" json:"entity_type"`
	Name       string                     `gorm:"type:varchar(128);comment:规则名称" json:"name"`
	Conditions constants.StringArrayField `gorm:"type:jsonb;comment:条件JSON" json:"conditions"`
	Strategy   string                     `gorm:"type:varchar(32);comment:分配策略 round_robin/weighted/random" json:"strategy"`
	TargetIDs  constants.StringArrayField `gorm:"type:jsonb;comment:目标用户ID列表" json:"target_ids"`
	IsEnabled  bool                       `gorm:"type:bool;default:true;comment:是否启用" json:"is_enabled"`
	Priority   int                        `gorm:"type:int;comment:优先级" json:"priority"`
}

// CrmLeadSource 线索来源选项（租户级，管理员在设置页维护）。
type CrmLeadSource struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(64);comment:来源名称" json:"name"`
	Sort     int    `gorm:"type:int;default:0;comment:排序(越小越靠前)" json:"sort"`
	Enabled  bool   `gorm:"type:bool;default:true;comment:是否启用" json:"enabled"`
}

// CrmReasonDict 赢单/输单原因字典（租户级，管理员在设置页维护）。
// Category: "win" 赢单原因 / "lose" 输单原因。
type CrmReasonDict struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Category string `gorm:"type:varchar(16);index;comment:win/lose" json:"category"`
	Name     string `gorm:"type:varchar(64);comment:原因名称" json:"name"`
	Sort     int    `gorm:"type:int;default:0;comment:排序(越小越靠前)" json:"sort"`
	Enabled  bool   `gorm:"type:bool;default:true;comment:是否启用" json:"enabled"`
}

// CrmSeasType 公海类型。
type CrmSeasType struct {
	Model
	Timestamp
	TenantID  string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name      string                     `gorm:"type:varchar(128);comment:公海名称" json:"name"`
	Rules     constants.StringArrayField `gorm:"type:jsonb;comment:回收规则JSON" json:"rules"`
	GroupIDs  constants.StringArrayField `gorm:"type:jsonb;comment:可见部门ID列表" json:"group_ids"`
	StaffIDs  constants.StringArrayField `gorm:"type:jsonb;comment:可见员工ID列表" json:"staff_ids"`
	IsDefault bool                       `gorm:"type:bool;default:false;comment:是否默认公海(兜底)" json:"is_default"`
	Remark    string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmCustomerSetting 客户设置（拥有客户数限制等）。
type CrmCustomerSetting struct {
	Model
	Timestamp
	TenantID     string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	MaxCustomers int    `gorm:"type:int;default:0;comment:每人客户上限(0=不限)" json:"max_customers"`
	MaxLocked    int    `gorm:"type:int;default:0;comment:每人锁定客户上限" json:"max_locked"`
	LockDays     int    `gorm:"type:int;default:0;comment:锁定期天数" json:"lock_days"`
	RecycleDays  int    `gorm:"type:int;default:0;comment:公海回收天数" json:"recycle_days"`
}

// ==================== 财务扩展 ====================

// CrmIncomeCategory 收支类别。
type CrmIncomeCategory struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:类别名称" json:"name"`
	Type     string `gorm:"type:varchar(16);index;comment:income/expense" json:"type"`
	Status   int    `gorm:"type:smallint;default:1;comment:1-启用 0-禁用" json:"status"`
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

// ==================== 进销存单据 ====================

// CrmStockIn 入库单。
type CrmStockIn struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string                     `gorm:"type:varchar(32);index;comment:入库单号" json:"no"`
	WarehouseID string                     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	Type        string                     `gorm:"type:varchar(16);comment:purchase/return/transfer_in/other" json:"type"`
	RefNo       string                     `gorm:"type:varchar(64);comment:关联单号(采购单号等)" json:"ref_no"`
	TotalAmount float64                    `gorm:"type:decimal(16,2);comment:总金额" json:"total_amount"`
	OccurDate   *time.Time                 `gorm:"comment:入库日期" json:"occur_date"`
	OperatorID  string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status      string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark      string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items       constants.StringArrayField `gorm:"type:jsonb;comment:明细JSON[{product_id,name,spec,qty,price,amount}]" json:"items"`
}

// CrmStockOut 出库单。
type CrmStockOut struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string                     `gorm:"type:varchar(32);index;comment:出库单号" json:"no"`
	WarehouseID string                     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	Type        string                     `gorm:"type:varchar(16);comment:sales/return/transfer_out/other" json:"type"`
	RefNo       string                     `gorm:"type:varchar(64);comment:关联单号" json:"ref_no"`
	TotalAmount float64                    `gorm:"type:decimal(16,2);comment:总金额" json:"total_amount"`
	OccurDate   *time.Time                 `gorm:"comment:出库日期" json:"occur_date"`
	OperatorID  string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status      string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark      string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items       constants.StringArrayField `gorm:"type:jsonb;comment:明细JSON" json:"items"`
}

// CrmStockCheck 库存盘点单。
type CrmStockCheck struct {
	Model
	Timestamp
	TenantID    string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No          string                     `gorm:"type:varchar(32);index;comment:盘点单号" json:"no"`
	WarehouseID string                     `gorm:"type:bigint;index;comment:仓库ID" json:"warehouse_id"`
	OccurDate   *time.Time                 `gorm:"comment:盘点日期" json:"occur_date"`
	OperatorID  string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status      string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark      string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items       constants.StringArrayField `gorm:"type:jsonb;comment:盘点明细JSON[{product_id,name,book_qty,actual_qty,diff}]" json:"items"`
}

// CrmStockTransfer 库存调拨单。
type CrmStockTransfer struct {
	Model
	Timestamp
	TenantID      string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	No            string                     `gorm:"type:varchar(32);index;comment:调拨单号" json:"no"`
	FromWarehouse string                     `gorm:"type:bigint;index;comment:调出仓库ID" json:"from_warehouse"`
	ToWarehouse   string                     `gorm:"type:bigint;index;comment:调入仓库ID" json:"to_warehouse"`
	OccurDate     *time.Time                 `gorm:"comment:调拨日期" json:"occur_date"`
	OperatorID    string                     `gorm:"type:bigint;comment:操作人" json:"operator_id"`
	Status        string                     `gorm:"type:varchar(16);default:draft;comment:draft/confirmed/cancelled" json:"status"`
	Remark        string                     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Items         constants.StringArrayField `gorm:"type:jsonb;comment:调拨明细JSON" json:"items"`
}

// ==================== 工单扩展 ====================

// CrmWorkorderType 工单类型。
type CrmWorkorderType struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:类型名称" json:"name"`
	Sort     int    `gorm:"type:int;comment:排序" json:"sort"`
	Status   int    `gorm:"type:smallint;default:1;comment:1-启用 0-禁用" json:"status"`
	Remark   string `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// CrmWorkorderPool 工单池。
type CrmWorkorderPool struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	WorkorderID string `gorm:"type:bigint;index;comment:工单ID" json:"workorder_id"`
	Source      string `gorm:"type:varchar(32);comment:入池来源 manual/auto" json:"source"`
}

// CrmWorkorderLog 工单日志。
type CrmWorkorderLog struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	WorkorderID string `gorm:"type:bigint;index;comment:工单ID" json:"workorder_id"`
	UserID      string `gorm:"type:bigint;comment:操作人" json:"user_id"`
	Action      string `gorm:"type:varchar(32);comment:create/transfer/complete/cancel/comment" json:"action"`
	Content     string `gorm:"type:text;comment:日志内容" json:"content"`
}

// ==================== 呼叫中心扩展 ====================

// CrmCallList 待拨电话列表。
type CrmCallList struct {
	Model
	Timestamp
	TenantID    string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID      string     `gorm:"type:bigint;index;comment:分配的员工ID" json:"user_id"`
	CustomerID  string     `gorm:"type:bigint;comment:关联客户ID(可空)" json:"customer_id"`
	ContactName string     `gorm:"type:varchar(128);comment:联系人名" json:"contact_name"`
	Phone       string     `gorm:"type:varchar(32);index;comment:电话号码" json:"phone"`
	Status      string     `gorm:"type:varchar(16);default:pending;index;comment:pending/dialed/skipped" json:"status"`
	DialedAt    *time.Time `gorm:"comment:拨打时间" json:"dialed_at"`
	Remark      string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
}

// ==================== 跟进扩展 ====================

// CrmFollowupComment 跟进评论。
type CrmFollowupComment struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	FollowupID string `gorm:"type:bigint;index;comment:跟进记录ID" json:"followup_id"`
	UserID     string `gorm:"type:bigint;comment:评论人" json:"user_id"`
	Content    string `gorm:"type:text;comment:评论内容" json:"content"`
}

// CrmQuickRemark 常用备注（话术库）。
type CrmQuickRemark struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID   string `gorm:"type:bigint;index;comment:归属用户ID(空=全员共用)" json:"user_id"`
	Content  string `gorm:"type:varchar(512);comment:备注内容" json:"content"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
}

// ==================== 办公扩展 ====================

// CrmKnowledgeType 知识库分类。
type CrmKnowledgeType struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:分类名称" json:"name"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
}

// CrmWorkReportComment 工作报告评论。
type CrmWorkReportComment struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	ReportID string `gorm:"type:bigint;index;comment:报告ID" json:"report_id"`
	UserID   string `gorm:"type:bigint;comment:评论人" json:"user_id"`
	Content  string `gorm:"type:text;comment:评论内容" json:"content"`
}

// ==================== 售后扩展 ====================

// CrmAftersalesType 售后类型。
type CrmAftersalesType struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:类型名称" json:"name"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
	Status   int    `gorm:"type:smallint;default:1;comment:1-启用 0-禁用" json:"status"`
}

// CrmAftersalesComment 售后评论。
type CrmAftersalesComment struct {
	Model
	Timestamp
	TenantID     string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	AftersalesID string `gorm:"type:bigint;index;comment:售后单ID" json:"aftersales_id"`
	UserID       string `gorm:"type:bigint;comment:评论人" json:"user_id"`
	Content      string `gorm:"type:text;comment:评论内容" json:"content"`
}
