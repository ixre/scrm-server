package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
)

// ApprovalInstanceQuery 审批实例查询参数。
type ApprovalInstanceQuery struct {
	Page        int
	PageSize    int
	BizType     string
	Status      string
	ApplicantID string
}

// CrmApprovalService 审批引擎服务。
type CrmApprovalService struct{}

func NewCrmApproval() *CrmApprovalService { return &CrmApprovalService{} }

// ---------- 审批流定义 ----------

// ListFlows 按业务类型列出审批流定义。
func (s CrmApprovalService) ListFlows(ctx context.Context, bizType string) ([]models.CrmApprovalFlow, error) {


	var items []models.CrmApprovalFlow
	db := models.MustFor(ctx)
	if bizType != "" {
		db = db.Where("biz_type = ?", bizType)
	}
	err := db.Order("id desc").Find(&items).Error
	return items, err
}

// CreateFlow 创建审批流定义。
func (s CrmApprovalService) CreateFlow(ctx context.Context, f *models.CrmApprovalFlow) error {
	tenantID := identity.TenantID(ctx)


	if f == nil {
		return errors.New("审批流不能为空")
	}
	if strings.TrimSpace(f.Name) == "" {
		return errors.New("审批流名称不能为空")
	}
	if f.BizType == "" {
		return errors.New("业务类型不能为空")
	}
	f.ID = id_generator.StringID()
	f.TenantID = tenantID
	f.Name = strings.TrimSpace(f.Name)
	return createCRM(models.DB, f)
}

// UpdateFlow 更新审批流定义。
func (s CrmApprovalService) UpdateFlow(ctx context.Context, id string, f *models.CrmApprovalFlow) error {


	if f == nil {
		return errors.New("审批流不能为空")
	}
	if strings.TrimSpace(f.Name) == "" {
		return errors.New("审批流名称不能为空")
	}
	m := map[string]any{
		"biz_type":   f.BizType,
		"name":       strings.TrimSpace(f.Name),
		"nodes":      f.Nodes,
		"is_enabled": f.IsEnabled,
	}
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// DeleteFlow 批量删除审批流定义。
func (s CrmApprovalService) DeleteFlow(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmApprovalFlow{})
	return res.RowsAffected, res.Error
}

// ---------- 审批实例 ----------

// Submit 根据业务类型查找启用的审批流，创建审批实例。
func (s CrmApprovalService) Submit(ctx context.Context, bizType, bizID, applicantID string) (*models.CrmApprovalInstance, error) {
	tenantID := identity.TenantID(ctx)


	if bizType == "" {
		return nil, errors.New("业务类型不能为空")
	}
	if bizID == "" {
		return nil, errors.New("业务单据ID不能为空")
	}
	if applicantID == "" {
		return nil, errors.New("申请人不能为空")
	}
	var flow models.CrmApprovalFlow
	if err := models.MustFor(ctx).Where("biz_type = ? AND is_enabled = ?",  bizType,  true).
		First(&flow).Error; err != nil {
		return nil, errors.New("未找到启用的审批流")
	}
	if len(flow.Nodes) == 0 {
		return nil, errors.New("审批流节点为空")
	}
	inst := models.CrmApprovalInstance{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		FlowID:      flow.ID,
		BizType:     bizType,
		BizID:       bizID,
		ApplicantID: applicantID,
		CurrentNode: 0,
		Status:      "pending",
	}
	if err := createCRM(models.DB, &inst, "FlowID", "ApplicantID"); err != nil {
		return nil, err
	}
	return &inst, nil
}

// ListInstances 分页查询审批实例，支持按 biz_type/status/applicant_id 筛选。
func (s CrmApprovalService) ListInstances(ctx context.Context, req ApprovalInstanceQuery) ([]models.CrmApprovalInstance, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.BizType != "" {
		db = db.Where("biz_type = ?", req.BizType)
	}
	if req.Status != "" {
		db = db.Where("status = ?", req.Status)
	}
	if req.ApplicantID != "" {
		db = db.Where("applicant_id = ?", req.ApplicantID)
	}
	var total int64
	if err := db.Model(&models.CrmApprovalInstance{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmApprovalInstance
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// Approve 通过当前节点，推进到下一节点或完成审批。
func (s CrmApprovalService) Approve(ctx context.Context, instanceID, approverID, comment string) error {
	tenantID := identity.TenantID(ctx)


	if approverID == "" {
		return errors.New("审批人不能为空")
	}
	var inst models.CrmApprovalInstance
	if err := models.MustFor(ctx).Where("id = ?",  instanceID).First(&inst).Error; err != nil {
		return errors.New("审批实例不存在")
	}
	if inst.Status != "pending" {
		return errors.New("该审批实例不在审批中")
	}
	var flow models.CrmApprovalFlow
	if err := models.MustFor(ctx).Where("id = ?",  inst.FlowID).First(&flow).Error; err != nil {
		return errors.New("审批流不存在")
	}
	nodeCount := len(flow.Nodes)
	nextNode := inst.CurrentNode + 1
	// 写入审批记录
	rec := models.CrmApprovalRecord{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		InstanceID: instanceID,
		NodeIndex:  inst.CurrentNode,
		ApproverID: approverID,
		Action:     "approve",
		Comment:    comment,
	}
	if err := createCRM(models.DB, &rec, "ApproverID"); err != nil {
		return err
	}
	// 推进到下一节点或完成
	if nextNode >= nodeCount {
		// 全部节点通过，审批完成
		return models.DB.Model(&models.CrmApprovalInstance{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  instanceID).
			Updates(map[string]any{"current_node": nextNode, "status": "approved"}).Error
	}
	return models.DB.Model(&models.CrmApprovalInstance{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  instanceID).
		Updates(map[string]any{"current_node": nextNode}).Error
}

// Reject 驳回当前节点，审批终止。
func (s CrmApprovalService) Reject(ctx context.Context, instanceID, approverID, comment string) error {
	tenantID := identity.TenantID(ctx)


	if approverID == "" {
		return errors.New("审批人不能为空")
	}
	var inst models.CrmApprovalInstance
	if err := models.MustFor(ctx).Where("id = ?",  instanceID).First(&inst).Error; err != nil {
		return errors.New("审批实例不存在")
	}
	if inst.Status != "pending" {
		return errors.New("该审批实例不在审批中")
	}
	rec := models.CrmApprovalRecord{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		InstanceID: instanceID,
		NodeIndex:  inst.CurrentNode,
		ApproverID: approverID,
		Action:     "reject",
		Comment:    comment,
	}
	if err := createCRM(models.DB, &rec, "ApproverID"); err != nil {
		return err
	}
	return models.DB.Model(&models.CrmApprovalInstance{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  instanceID).
		Updates(map[string]any{"status": "rejected"}).Error
}

// Void 撤回审批实例（申请人撤回）。
func (s CrmApprovalService) Void(ctx context.Context, instanceID, approverID string) error {
	tenantID := identity.TenantID(ctx)


	var inst models.CrmApprovalInstance
	if err := models.MustFor(ctx).Where("id = ?",  instanceID).First(&inst).Error; err != nil {
		return errors.New("审批实例不存在")
	}
	if inst.Status != "pending" {
		return errors.New("该审批实例不在审批中")
	}
	rec := models.CrmApprovalRecord{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		InstanceID: instanceID,
		NodeIndex:  inst.CurrentNode,
		ApproverID: approverID,
		Action:     "void",
	}
	if err := createCRM(models.DB, &rec, "ApproverID"); err != nil {
		return err
	}
	return models.DB.Model(&models.CrmApprovalInstance{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  instanceID).
		Updates(map[string]any{"status": "void"}).Error
}

// ListRecords 查询审批实例的操作记录。
func (s CrmApprovalService) ListRecords(ctx context.Context, instanceID string) ([]models.CrmApprovalRecord, error) {


	var items []models.CrmApprovalRecord
	err := models.MustFor(ctx).Where("instance_id = ?",  instanceID).
		Order("id asc").Find(&items).Error
	return items, err
}
