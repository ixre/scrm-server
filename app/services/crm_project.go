package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
)

// ProjectQueryReq 项目查询参数。
type ProjectQueryReq struct {
	Page       int
	PageSize   int
	Status     string
	CustomerID string
	Keyword    string
}

// CrmProjectService 项目服务。
type CrmProjectService struct{}

func NewCrmProject() *CrmProjectService { return &CrmProjectService{} }

// Create 创建项目。
func (s CrmProjectService) Create(ctx context.Context, p *models.CrmProject) error {
	tenantID := identity.TenantID(ctx)

	if p == nil {
		return errors.New("项目不能为空")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("项目名称不能为空")
	}
	p.ID = id_generator.StringID()
	p.TenantID = tenantID
	p.Name = strings.TrimSpace(p.Name)
	if p.Status == "" {
		p.Status = "planning"
	}
	if p.Progress < 0 {
		p.Progress = 0
	}
	if p.Progress > 100 {
		p.Progress = 100
	}
	return createCRM(models.DB, p, "CustomerID", "OrderID", "ManagerID")
}

// Update 更新项目。
func (s CrmProjectService) Update(ctx context.Context, id string, p *models.CrmProject) error {

	if p == nil {
		return errors.New("项目不能为空")
	}
	if strings.TrimSpace(p.Name) == "" {
		return errors.New("项目名称不能为空")
	}
	if p.Progress < 0 {
		p.Progress = 0
	}
	if p.Progress > 100 {
		p.Progress = 100
	}
	// 已完成/已取消的项目不允许通过编辑改 status，必须用 ChangeStatus 操作。
	var existing models.CrmProject
	if err := models.MustFor(ctx).Where("id = ?", id).First(&existing).Error; err != nil {
		return errors.New("项目不存在")
	}
	statusVal := p.Status
	if existing.Status == "completed" || existing.Status == "cancelled" {
		statusVal = existing.Status
	}
	m := map[string]any{
		"name":        p.Name,
		"type":        p.Type,
		"customer_id": p.CustomerID,
		"order_id":    p.OrderID,
		"manager_id":  p.ManagerID,
		"amount":      p.Amount,
		"start_date":  p.StartDate,
		"end_date":    p.EndDate,
		"status":      statusVal,
		"progress":    p.Progress,
		"remark":      p.Remark,
	}
	updatesCRM(m, "customer_id", "order_id", "manager_id")
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除项目。
func (s CrmProjectService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmProject{})
	return res.RowsAffected, res.Error
}

// Get 获取项目详情。
func (s CrmProjectService) Get(ctx context.Context, id string) (*models.CrmProject, error) {

	var item models.CrmProject
	err := models.MustFor(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Query 分页查询项目。
func (s CrmProjectService) Query(ctx context.Context, req ProjectQueryReq) ([]models.CrmProject, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR remark LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmProject{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmProject
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 变更项目状态。
func (s CrmProjectService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)

	if status == "" {
		return errors.New("状态不能为空")
	}
	return models.DB.Model(&models.CrmProject{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Update("status", status).Error
}

// ---------- 项目任务 ----------

// CrmProjectTaskService 项目任务服务。
type CrmProjectTaskService struct{}

func NewCrmProjectTask() *CrmProjectTaskService { return &CrmProjectTaskService{} }

// Create 创建项目任务。
func (s CrmProjectTaskService) Create(ctx context.Context, t *models.CrmProjectTask) error {
	tenantID := identity.TenantID(ctx)

	if t == nil {
		return errors.New("任务不能为空")
	}
	if strings.TrimSpace(t.Title) == "" {
		return errors.New("任务标题不能为空")
	}
	if t.ProjectID == "" {
		return errors.New("project_id 不能为空")
	}
	t.ID = id_generator.StringID()
	t.TenantID = tenantID
	t.Title = strings.TrimSpace(t.Title)
	if t.Status == "" {
		t.Status = "pending"
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	return createCRM(models.DB, t, "ProjectID", "AssigneeID")
}

// Update 更新项目任务。
func (s CrmProjectTaskService) Update(ctx context.Context, id string, t *models.CrmProjectTask) error {

	if t == nil {
		return errors.New("任务不能为空")
	}
	if strings.TrimSpace(t.Title) == "" {
		return errors.New("任务标题不能为空")
	}
	m := map[string]any{
		"title":       t.Title,
		"assignee_id": t.AssigneeID,
		"status":      t.Status,
		"priority":    t.Priority,
		"due_date":    t.DueDate,
		"sort":        t.Sort,
	}
	updatesCRM(m, "assignee_id")
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除项目任务。
func (s CrmProjectTaskService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmProjectTask{})
	return res.RowsAffected, res.Error
}

// ListByProject 按项目列出任务。
func (s CrmProjectTaskService) ListByProject(ctx context.Context, projectID string) ([]models.CrmProjectTask, error) {

	var items []models.CrmProjectTask
	err := models.MustFor(ctx).Where("project_id = ?", projectID).
		Order("sort asc, id desc").Find(&items).Error
	return items, err
}

// UpdateStatus 更新任务状态。
func (s CrmProjectTaskService) UpdateStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)

	if status == "" {
		return errors.New("状态不能为空")
	}
	return models.DB.Model(&models.CrmProjectTask{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Update("status", status).Error
}
