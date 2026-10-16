package models

import (
	"core/app/constants"
	"time"
)

// CrmActivity 营销活动。
// 运营创建营销活动（优惠券/抽奖/秒杀/H5落地页），跟踪触达-参与-转化效果，并接入统一客户主档。
// Channel 为主渠道（向后兼容），Channels 为投放渠道多选（有赞/微盟式多渠道投放）。
type CrmActivity struct {
	Model
	Timestamp
	TenantID     string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	OwnerUserID  string `gorm:"type:bigint;index;comment:创建人用户ID" json:"owner_user_id"`
	Name         string `gorm:"type:varchar(128);comment:活动名称" json:"name"`
	ActivityType string `gorm:"type:varchar(32);index;comment:活动类型 coupon/lottery/seckill/h5/group_buy/bargain/full_reduction/red_packet/sign_in/other" json:"activity_type"`
	Status       string `gorm:"type:varchar(32);index;comment:draft/active/ended/off" json:"status"`
	// Channel 主渠道（派生字段）：由 Channels[0] 自动填充，normalizeActivityChannels 维护一致性。
	// 保留用于快速筛选和列表展示；新增/编辑时以 Channels 多选为准。
	Channel          string                     `gorm:"type:varchar(32);comment:主渠道(派生自Channels[0]) wecom/xiaohongshu/general" json:"channel"`
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
	Status      string                     `gorm:"type:varchar(16);index:idx_group_buy_status_expire,priority:1;default:open;comment:open(进行中)/success(成团)/failed(超时未成团)" json:"status"`
	ExpireAt    *time.Time                 `gorm:"index:idx_group_buy_status_expire,priority:2;comment:成团截止时间" json:"expire_at"`
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
	Status       string                     `gorm:"type:varchar(16);index:idx_bargain_status_expire,priority:1;default:active;comment:active/success(砍到底价)/expired/purchased" json:"status"`
	ExpireAt     *time.Time                 `gorm:"index:idx_bargain_status_expire,priority:2;comment:截止时间" json:"expire_at"`
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
