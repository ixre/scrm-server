package models

import (
	"core/app/constants"
	"time"
)

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
