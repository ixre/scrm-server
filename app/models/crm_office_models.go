package models

import (
	"core/app/constants"
	"time"
)

// CrmTodo 待办/消息聚合。
type CrmTodo struct {
	Model
	Timestamp
	TenantID  string     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	UserID    string     `gorm:"type:bigint;index;comment:归属用户ID" json:"user_id"`
	Type      string     `gorm:"type:varchar(32);index;comment:类型 followup/contract_expire/receivables_expire/loss/timeout/task" json:"type"`
	Title     string     `gorm:"type:varchar(255);comment:标题" json:"title"`
	Content   string     `gorm:"type:text;comment:内容" json:"content"`
	RefType   string     `gorm:"type:varchar(32);comment:关联类型 customer/order/lead/opportunity" json:"ref_type"`
	RefID     string     `gorm:"type:bigint;comment:关联ID" json:"ref_id"`
	DueAt     *time.Time `gorm:"comment:到期时间" json:"due_at"`
	Status    string     `gorm:"type:varchar(16);index;default:pending;comment:pending(待处理)/done(已处理)/ignored(已忽略)/cancelled(已失效)" json:"status"`
	SourceID  string     `gorm:"type:bigint;comment:提醒源ID(回款计划或客户)" json:"source_id"`
	EventKey  string     `gorm:"type:varchar(160);not null;default:'';comment:同一提醒事件去重键" json:"-"`
	HandledAt *time.Time `gorm:"comment:处理或失效时间" json:"handled_at"`
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

// CrmKnowledge 知识库。
type CrmKnowledge struct {
	Model
	Timestamp
	TenantID        string                     `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	KnowledgeTypeID string                     `gorm:"type:bigint;index;comment:知识库分类ID(关联CrmKnowledgeType)" json:"knowledge_type_id"`
	Title           string                     `gorm:"type:varchar(255);comment:标题" json:"title"`
	Content         string                     `gorm:"type:text;comment:内容(富文本)" json:"content"`
	Attachments     constants.StringArrayField `gorm:"type:jsonb;comment:附件列表" json:"attachments"`
	Visibility      string                     `gorm:"type:varchar(16);default:company;comment:可见范围 company(全公司)/department(部门)/role(角色)/person(指定人)" json:"visibility"`
	AuthorID        string                     `gorm:"type:bigint;comment:作者" json:"author_id"`
	Status          string                     `gorm:"type:varchar(16);default:published;comment:draft/published/archived" json:"status"`
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
