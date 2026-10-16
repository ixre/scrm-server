package models

import (
	"core/app/constants"
	"time"
)

// CrmCustomer 本地客户（CRM 客户 360 规范记录），带来源渠道维度。
type CrmCustomer struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:客户名称" json:"name"`
	Mobile   string `gorm:"type:varchar(32);index;comment:手机号" json:"mobile"`
	Email    string `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Company  string `gorm:"type:varchar(255);comment:公司名称" json:"company"`
	Industry string `gorm:"type:varchar(64);index;comment:所属行业" json:"industry"`
	Position string `gorm:"type:varchar(64);comment:职位" json:"position"`
	Gender   int    `gorm:"type:smallint;default:0;comment:0-未知 1-男 2-女" json:"gender"`
	// Birthday 生日。用 varchar 而非 date：支持仅存月日(MM-DD)的匿名客户场景，crm_care 用 RIGHT(birthday,5) 做月日匹配。
	Birthday       string                     `gorm:"type:varchar(32);comment:生日(支持YYYY-MM-DD或MM-DD)" json:"birthday"`
	Level          int                        `gorm:"type:smallint;default:0;comment:客户等级 0-未知 1-潜在 2-普通 3-重要 4-重点 5-VIP" json:"level"`
	Address        string                     `gorm:"type:varchar(255);comment:详细地址" json:"address"`
	Province       string                     `gorm:"type:varchar(32);comment:省" json:"province"`
	City           string                     `gorm:"type:varchar(32);comment:市" json:"city"`
	District       string                     `gorm:"type:varchar(32);comment:区/县" json:"district"`
	Source         string                     `gorm:"type:varchar(32);index;comment:来源渠道 wecom/xiaohongshu/manual/import/web_form" json:"source"`
	OwnerUserID    string                     `gorm:"type:bigint;index;comment:归属销售用户ID" json:"owner_user_id"`
	NextFollowupAt *time.Time                 `gorm:"comment:下次跟进时间" json:"next_followup_at"`
	Remark         string                     `gorm:"type:varchar(1024);comment:备注" json:"remark"`
	Tags           constants.StringArrayField `gorm:"type:jsonb;comment:客户标签(内部标签ID数组)" json:"tags"`
	// ---- 渠道维度快照 ----
	// 设计说明：这些字段是渠道侧客户身份在 CRM 主档上的"快照"，用于快速检索和展示。
	// 渠道原始数据的完整映射由 ChannelLink 表维护（entity_type=customer）。
	// 快照字段在渠道回调/同步时回填，避免每次查询都需 JOIN ChannelLink。
	// 若一个客户关联多个渠道，以最近活跃渠道的快照为准；历史渠道数据通过 ChannelLink 可溯。
	ExtCustomerID string                     `gorm:"type:varchar(128);index;comment:渠道用户ID快照(企微external_userid/小红书user_id)" json:"ext_customer_id"`
	ExtCorpID     string                     `gorm:"type:varchar(128);index;comment:渠道账号ID(企微corp_id / 小红书企业号)" json:"ext_corp_id"`
	AddWay        int                        `gorm:"type:int;default:0;comment:渠道内添加/承接方式(wecom 0-9/201/202；xiaohongshu 0-4)" json:"add_way"`
	AccountType   string                     `gorm:"type:varchar(32);comment:账号类型 微信/企业微信/小红书" json:"account_type"`
	OutFlowStatus int                        `gorm:"type:smallint;default:0;comment:流失状态 0-正常 1-已流失" json:"out_flow_status"`
	ChannelTags   constants.StringArrayField `gorm:"type:jsonb;comment:渠道标签快照(标签名数组)" json:"channel_tags"`
	// ---- 公海池 ----
	PoolStatus      int        `gorm:"type:smallint;default:0;index;comment:公海状态 0-非公海 1-公海待认领" json:"pool_status"`
	PoolAt          *time.Time `gorm:"comment:进入公海时间" json:"pool_at"`
	PrevOwnerUserID string     `gorm:"type:varchar(64);index;comment:上一个归属销售用户ID(公海前负责人)" json:"prev_owner_user_id"`
	ClaimedAt       *time.Time `gorm:"comment:最近一次认领时间(用于保护期判断)" json:"claimed_at"`
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
	Address         string `gorm:"type:varchar(255);comment:详细地址" json:"address"`
	Province        string `gorm:"type:varchar(32);comment:省" json:"province"`
	City            string `gorm:"type:varchar(32);comment:市" json:"city"`
	District        string `gorm:"type:varchar(32);comment:区/县" json:"district"`
	// Birthday 生日。用 varchar 支持 MM-DD 匿名场景，详见 CrmCustomer.Birthday 注释。
	Birthday string `gorm:"type:varchar(32);comment:生日(支持YYYY-MM-DD或MM-DD)" json:"birthday"`
	Gender   int    `gorm:"type:smallint;default:0;comment:0-未知 1-男 2-女" json:"gender"`
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

// CrmQuickRemark 常用备注（话术库）。
type CrmQuickRemark struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID   string `gorm:"type:bigint;index;comment:归属用户ID(空=全员共用)" json:"user_id"`
	Content  string `gorm:"type:varchar(512);comment:备注内容" json:"content"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
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

// ---- Phase 1 销售域深化模型 ----

// CrmPoolRule 公海回收规则（租户级，单条）。
type CrmPoolRule struct {
	Model
	Timestamp
	TenantID          string `gorm:"type:bigint;uniqueIndex;comment:归属租户ID" json:"tenant_id"`
	RecycleDays       int    `gorm:"type:int;default:0;comment:超N天未跟进自动回收,0=不自动回收" json:"recycle_days"`
	NoDealDays        int    `gorm:"type:int;default:0;comment:超N天未成单自动回收,0=不自动回收" json:"no_deal_days"`
	Enabled           bool   `gorm:"comment:是否启用自动回收" json:"enabled"`
	ClaimLimit        int    `gorm:"type:int;default:0;comment:每人最多领取公海客户数,0=不限" json:"claim_limit"`
	ClaimCooldownDays int    `gorm:"type:int;default:0;comment:前负责人领取冷却天数,0=不限制" json:"claim_cooldown_days"`
	ProtectDays       int    `gorm:"type:int;default:0;comment:领取后保护期天数,保护期内不被自动回收,0=不保护" json:"protect_days"`
}

// CrmPoolLog 公海操作记录（认领/回收/分配）。
type CrmPoolLog struct {
	Model
	Timestamp
	TenantID     string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID   string `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	CustomerName string `gorm:"type:varchar(128);comment:客户名称(快照)" json:"customer_name"`
	Action       string `gorm:"type:varchar(16);index;comment:claim/recycle/assign" json:"action"`
	OperatorID   string `gorm:"type:varchar(64);index;comment:操作人用户ID" json:"operator_id"`
	OperatorName string `gorm:"type:varchar(64);comment:操作人姓名(快照)" json:"operator_name"`
	TargetUserID string `gorm:"type:varchar(64);comment:分配目标用户ID(assign时)" json:"target_user_id"`
	Remark       string `gorm:"type:varchar(512);comment:备注" json:"remark"`
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

// CrmCustomerTag 客户标签（租户级，管理员在设置页维护）。
// CrmCustomer.tags 存储本标签的 ID 数组。
type CrmCustomerTag struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(64);comment:标签名称" json:"name"`
	Group    string `gorm:"column:tag_group;type:varchar(64);index;comment:标签分组" json:"group"`
	Color    string `gorm:"type:varchar(16);default:arcoblue;comment:标签颜色" json:"color"`
	Sort     int    `gorm:"type:int;default:0;comment:排序(越小越靠前)" json:"sort"`
	Enabled  bool   `gorm:"type:bool;default:true;comment:是否启用" json:"enabled"`
}

// CrmCareTemplate 客户关怀模板（生日/节日/续约关怀）。
type CrmCareTemplate struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name        string `gorm:"type:varchar(128);comment:模板名称" json:"name"`
	Type        string `gorm:"type:varchar(32);index;comment:关怀类型 birthday/festival/renew/custom" json:"type"`
	Content     string `gorm:"type:text;comment:关怀内容/话术" json:"content"`
	AdvanceDays int    `gorm:"type:int;default:0;comment:提前几天提醒,0=当天" json:"advance_days"`
	Enabled     bool   `gorm:"type:bool;default:true;comment:是否启用" json:"enabled"`
}

// CrmCustomerSegment 客户分群（保存的筛选视图）。
type CrmCustomerSegment struct {
	Model
	Timestamp
	TenantID    string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name        string `gorm:"type:varchar(128);comment:分群名称" json:"name"`
	Description string `gorm:"type:varchar(512);comment:描述" json:"description"`
	Conditions  string `gorm:"type:text;comment:筛选条件JSON" json:"conditions"`
	IsDynamic   bool   `gorm:"type:bool;default:false;comment:是否动态分群" json:"is_dynamic"`
	CreatorID   string `gorm:"type:bigint;index;comment:创建人ID" json:"creator_id"`
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

// CrmIndustryDict 行业字典（平台级，平台超管在后台维护，所有租户共享）。
type CrmIndustryDict struct {
	Model
	Timestamp
	Name    string `gorm:"type:varchar(64);comment:行业名称" json:"name"`
	Sort    int    `gorm:"type:int;default:0;comment:排序(越小越靠前)" json:"sort"`
	Enabled bool   `gorm:"type:bool;default:true;comment:是否启用" json:"enabled"`
}

// CrmSeasType 公海类型（多公海池配置）。
// 与 CrmPoolRule 的区别：CrmPoolRule 是租户级单条自动回收规则（全局策略），
// CrmSeasType 支持多个公海池，每个池有独立的可见部门/员工和回收规则。
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
// 注意：公海回收天数由 CrmPoolRule.RecycleDays 统一管理，不在本表重复。
type CrmCustomerSetting struct {
	Model
	Timestamp
	TenantID     string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	MaxCustomers int    `gorm:"type:int;default:0;comment:每人客户上限(0=不限)" json:"max_customers"`
	MaxLocked    int    `gorm:"type:int;default:0;comment:每人锁定客户上限" json:"max_locked"`
	LockDays     int    `gorm:"type:int;default:0;comment:锁定期天数" json:"lock_days"`
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
