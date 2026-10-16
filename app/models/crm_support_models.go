package models

import (
	"time"
)

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

// ==================== 工单扩展 ====================

// CrmWorkorderType 工单类型。
type CrmWorkorderType struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:类型名称" json:"name"`
	Sort     int    `gorm:"type:int;comment:排序" json:"sort"`
	Status   bool   `gorm:"type:bool;default:true;comment:是否启用" json:"status"`
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

// ==================== 售后扩展 ====================

// CrmAftersalesType 售后类型。
type CrmAftersalesType struct {
	Model
	Timestamp
	TenantID string `gorm:"type:bigint;index;comment:归属租户ID" json:"tenant_id"`
	Name     string `gorm:"type:varchar(128);comment:类型名称" json:"name"`
	Sort     int    `gorm:"type:int;default:0;comment:排序" json:"sort"`
	Status   bool   `gorm:"type:bool;default:true;comment:是否启用" json:"status"`
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
