package models

import (
	"core/app/constants"
	"time"
)

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
	EventKey   string     `gorm:"type:varchar(64);not null;default:'';comment:渠道事件去重摘要" json:"-"`
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

// CrmChannelConfig 渠道接入配置（一租户一渠道账号）。
// 每个 (tenant, channel) 一条：存该租户在外部渠道的企业账号凭据。
// CorpID 字段语义按渠道不同：wecom=corp_id, xiaohongshu=app_id。
// secrets 一律经 util.AESEncode 加密落库，json:"-" 永不序列化，读接口只回掩码。
type CrmChannelConfig struct {
	Model
	Timestamp
	TenantID           string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Channel            string `gorm:"type:varchar(32);index;comment:渠道标识 wecom/xiaohongshu" json:"channel"`
	CorpID             string `gorm:"type:varchar(64);index;uniqueIndex:idx_corp;comment:渠道外部账号ID(企微corp_id/小红书app_id,全局唯一)" json:"corp_id"`
	Name               string `gorm:"type:varchar(128);comment:企业/应用名称" json:"name"`
	ContactSecret      string `gorm:"type:text;comment:通讯录secret(加密)" json:"-"`
	CustomerSecret     string `gorm:"type:text;comment:客户联系secret(加密)" json:"-"`
	MainAgentID        int64  `gorm:"type:bigint;comment:主应用AgentID" json:"main_agent_id"`
	MainAgentSecret    string `gorm:"type:text;comment:主应用secret(加密)" json:"-"`
	CallbackToken      string `gorm:"type:varchar(256);comment:回调Token(加密)" json:"-"`
	CallbackAesKey     string `gorm:"type:text;comment:回调AesKey(加密)" json:"-"`
	CallbackURL        string `gorm:"type:varchar(255);comment:回调URL(展示)" json:"callback_url"`
	Enabled            int    `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"enabled"`
	Remark             string `gorm:"type:varchar(512);comment:备注" json:"remark"`
	MsgArchEnabled     int    `gorm:"default:2" json:"msg_arch_enabled"`
	MsgArchSecret      string `gorm:"type:text" json:"-"`
	MsgArchPrivateKeys string `gorm:"type:text" json:"-"`
	AuthMode           string `gorm:"default:self_build" json:"auth_mode"`
	SuiteID            string `json:"suite_id"`
	PermanentCode      string `gorm:"type:text" json:"-"`
	AuthStatus         string `json:"auth_status"`
	AuthorizedAt       int64  `json:"authorized_at"`
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

// CrmChannelTask 跨渠道引流任务（如小红书留资→企微添加）。
type CrmChannelTask struct {
	Model
	Timestamp
	TenantID     string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	CustomerID   string     `gorm:"type:bigint;index;comment:客户ID" json:"customer_id"`
	FromChannel  string     `gorm:"type:varchar(32);comment:来源渠道" json:"from_channel"`
	ToChannel    string     `gorm:"type:varchar(32);comment:目标渠道" json:"to_channel"`
	Action       string     `gorm:"type:varchar(32);comment:动作类型 add_wecom/call_back" json:"action"`
	Status       string     `gorm:"type:varchar(16);index;default:pending;comment:pending/timeout/done/skipped" json:"status"`
	AssignUserID string     `gorm:"type:bigint;comment:分配给哪个销售" json:"assign_user_id"`
	Remark       string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	DueAt        *time.Time `gorm:"comment:处理截止时间" json:"due_at"`
	HandledAt    *time.Time `gorm:"comment:最后处理时间" json:"handled_at"`
	HandledBy    string     `gorm:"type:bigint;comment:最后处理人" json:"handled_by"`
	Resolution   string     `gorm:"type:varchar(512);comment:处理结果或跳过原因" json:"resolution"`
	Version      int64      `gorm:"not null;default:1;comment:并发版本" json:"version"`
	DedupKey     string     `gorm:"type:varchar(160);not null;default:'';comment:来源任务去重标识" json:"-"`
}

// CrmChannelTaskLog records assignment and state transitions without foreign keys.
type CrmChannelTaskLog struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;not null" json:"tenant_id"`
	TaskID     string `gorm:"type:bigint;not null" json:"task_id"`
	ActorID    string `gorm:"type:bigint" json:"actor_id"`
	Action     string `gorm:"type:varchar(32);not null" json:"action"`
	FromStatus string `gorm:"type:varchar(16)" json:"from_status"`
	ToStatus   string `gorm:"type:varchar(16)" json:"to_status"`
	FromUserID string `gorm:"type:bigint" json:"from_user_id"`
	ToUserID   string `gorm:"type:bigint" json:"to_user_id"`
	Note       string `gorm:"type:varchar(512)" json:"note"`
}

// ---- C4 智能名片 ----

// CrmSmartCard 智能名片（销售社交工具）。
type CrmSmartCard struct {
	Model
	Timestamp
	TenantID     string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID       string                     `gorm:"type:bigint;index;comment:销售用户ID" json:"user_id"`
	Name         string                     `gorm:"type:varchar(64);comment:姓名" json:"name"`
	Title        string                     `gorm:"type:varchar(128);comment:职位/头衔" json:"title"`
	Company      string                     `gorm:"type:varchar(128);comment:公司名" json:"company"`
	CompanyIntro string                     `gorm:"type:text;comment:公司简介" json:"company_intro"`
	Address      string                     `gorm:"type:varchar(512);comment:详细地址" json:"address"`
	Province     string                     `gorm:"type:varchar(32);comment:省" json:"province"`
	City         string                     `gorm:"type:varchar(32);comment:市" json:"city"`
	District     string                     `gorm:"type:varchar(32);comment:区/县" json:"district"`
	Phone        string                     `gorm:"type:varchar(32);comment:电话" json:"phone"`
	Email        string                     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Avatar       string                     `gorm:"type:varchar(512);comment:头像URL" json:"avatar"`
	Intro        string                     `gorm:"type:text;comment:个人简介" json:"intro"`
	Tags         constants.StringArrayField `gorm:"type:jsonb;comment:专业标签" json:"tags"`
	Products     constants.StringArrayField `gorm:"type:jsonb;comment:产品/服务列表JSON[{name,desc}]" json:"products"`
	Cases        constants.StringArrayField `gorm:"type:jsonb;comment:案例列表JSON[{name,desc}]" json:"cases"`
	WechatQR     string                     `gorm:"type:varchar(512);comment:企业微信二维码URL" json:"wechat_qr"`
	ViewCount    int                        `gorm:"type:int;default:0;comment:浏览数" json:"view_count"`
	ShareCount   int                        `gorm:"type:int;default:0;comment:分享数" json:"share_count"`
	LikeCount    int                        `gorm:"type:int;default:0;comment:点赞数" json:"like_count"`
	ShareCode    string                     `gorm:"type:varchar(32);index;comment:分享码(短链标识)" json:"share_code"`
	Status       string                     `gorm:"type:varchar(16);index;default:active;comment:active/disabled" json:"status"`
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
