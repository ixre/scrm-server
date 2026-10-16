package models

import (
	"core/app/constants"
)

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
