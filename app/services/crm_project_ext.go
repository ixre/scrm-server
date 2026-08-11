package services

import (
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
)

// ---------- 项目事件（里程碑） ----------

// CrmProjectEventService 项目事件服务。
type CrmProjectEventService struct{}

func NewCrmProjectEvent() *CrmProjectEventService { return &CrmProjectEventService{} }

// Create 创建项目事件。
func (s CrmProjectEventService) Create(tenantID string, e *models.CrmProjectEvent) error {
	if e == nil {
		return errors.New("事件不能为空")
	}
	if strings.TrimSpace(e.Title) == "" {
		return errors.New("事件标题不能为空")
	}
	if e.ProjectID == "" {
		return errors.New("project_id 不能为空")
	}
	e.ID = id_generator.StringID()
	e.TenantID = tenantID
	e.Title = strings.TrimSpace(e.Title)
	if e.Status == "" {
		e.Status = "pending"
	}
	return createCRM(models.DB, e, "ProjectID")
}

// Update 更新项目事件。
func (s CrmProjectEventService) Update(tenantID, id string, e *models.CrmProjectEvent) error {
	if e == nil {
		return errors.New("事件不能为空")
	}
	m := map[string]any{
		"title":     e.Title,
		"sort":      e.Sort,
		"status":    e.Status,
		"plan_date": e.PlanDate,
		"remark":    e.Remark,
	}
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除项目事件。
func (s CrmProjectEventService) Delete(tenantID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmProjectEvent{})
	return res.RowsAffected, res.Error
}

// ListByProject 按项目列出事件。
func (s CrmProjectEventService) ListByProject(tenantID, projectID string) ([]models.CrmProjectEvent, error) {
	var items []models.CrmProjectEvent
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("project_id = ?",  projectID).
		Order("sort asc, id desc").Find(&items).Error
	return items, err
}

// CompleteEvent 完成项目事件。
func (s CrmProjectEventService) CompleteEvent(tenantID, id string) error {
	now := time.Now()
	return models.DB.Model(&models.CrmProjectEvent{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Updates(map[string]any{"status": "done", "done_at": &now}).Error
}

// ---------- 项目文档 ----------

// CrmProjectFileService 项目文档服务。
type CrmProjectFileService struct{}

func NewCrmProjectFile() *CrmProjectFileService { return &CrmProjectFileService{} }

// Create 上传项目文档。
func (s CrmProjectFileService) Create(tenantID, uploadBy string, f *models.CrmProjectFile) error {
	if f == nil {
		return errors.New("文档不能为空")
	}
	if strings.TrimSpace(f.FileName) == "" {
		return errors.New("文件名不能为空")
	}
	if f.ProjectID == "" {
		return errors.New("project_id 不能为空")
	}
	f.ID = id_generator.StringID()
	f.TenantID = tenantID
	f.FileName = strings.TrimSpace(f.FileName)
	f.UploadBy = uploadBy
	return createCRM(models.DB, f, "ProjectID", "UploadBy")
}

// Delete 删除项目文档。
func (s CrmProjectFileService) Delete(tenantID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmProjectFile{})
	return res.RowsAffected, res.Error
}

// ListByProject 按项目列出文档。
func (s CrmProjectFileService) ListByProject(tenantID, projectID string) ([]models.CrmProjectFile, error) {
	var items []models.CrmProjectFile
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("project_id = ?",  projectID).
		Order("id desc").Find(&items).Error
	return items, err
}
