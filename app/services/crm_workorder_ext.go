package services

import (
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// ==================== 工单类型 ====================

// WorkorderTypeQueryReq 工单类型查询参数。
type WorkorderTypeQueryReq struct {
	Page     int
	PageSize int
	Status   int
}

// CrmWorkorderTypeService 工单类型服务。
type CrmWorkorderTypeService struct{}

func NewCrmWorkorderType() *CrmWorkorderTypeService { return &CrmWorkorderTypeService{} }

// Create 创建工单类型。
func (s CrmWorkorderTypeService) Create(tenantID string, t *models.CrmWorkorderType) error {
	if t == nil {
		return errors.New("工单类型不能为空")
	}
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("类型名称不能为空")
	}
	t.ID = id_generator.StringID()
	t.TenantID = tenantID
	t.Name = strings.TrimSpace(t.Name)
	return models.DB.Create(t).Error
}

// Update 更新工单类型。
func (s CrmWorkorderTypeService) Update(tenantID, id string, t *models.CrmWorkorderType) error {
	if t == nil {
		return errors.New("工单类型不能为空")
	}
	m := map[string]any{
		"name":   t.Name,
		"sort":   t.Sort,
		"status": t.Status,
		"remark": t.Remark,
	}
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除工单类型。
func (s CrmWorkorderTypeService) Delete(tenantID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmWorkorderType{})
	return res.RowsAffected, res.Error
}

// Query 分页查询工单类型。
func (s CrmWorkorderTypeService) Query(tenantID string, req WorkorderTypeQueryReq) ([]models.CrmWorkorderType, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.DB.Scopes(models.TenantScope(tenantID))
	db = db.Where("status = ?", req.Status)
	var total int64
	if err := db.Model(&models.CrmWorkorderType{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmWorkorderType
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("sort asc, id desc").Find(&items).Error
	return items, total, err
}

// ==================== 工单池 ====================

// WorkorderPoolQueryReq 工单池查询参数。
type WorkorderPoolQueryReq struct {
	Page     int
	PageSize int
}

// CrmWorkorderPoolService 工单池服务。
type CrmWorkorderPoolService struct{}

func NewCrmWorkorderPool() *CrmWorkorderPoolService { return &CrmWorkorderPoolService{} }

// MoveToPool 批量入工单池：为每个工单创建池记录（幂等：已存在则跳过）。
func (s CrmWorkorderPoolService) MoveToPool(tenantID string, ids []string) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	for _, wid := range ids {
		var cnt int64
		models.DB.Model(&models.CrmWorkorderPool{}).Scopes(models.TenantScope(tenantID)).Where("workorder_id = ?",  wid).Count(&cnt)
		if cnt > 0 {
			continue
		}
		p := models.CrmWorkorderPool{
			TenantID:    tenantID,
			WorkorderID: wid,
			Source:      "manual",
		}
		p.ID = id_generator.StringID()
		if err := models.DB.Create(&p).Error; err != nil {
			log.Sugar.Errorw("crm workorder move to pool failed", "err", err, "workorder_id", wid)
			return err
		}
	}
	return nil
}

// Receive 从工单池领取单个工单：删除池记录，将工单 assignee_id 设为 userID。
func (s CrmWorkorderPoolService) Receive(tenantID, userID, id string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("workorder_id = ?",  id).
		Delete(&models.CrmWorkorderPool{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("工单不在池中")
	}
	return models.DB.Model(&models.CrmWorkorder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Update("assignee_id", userID).Error
}

// BatchReceive 批量领取工单。
func (s CrmWorkorderPoolService) BatchReceive(tenantID, userID string, ids []string) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	for _, id := range ids {
		if err := s.Receive(tenantID, userID, id); err != nil {
			log.Sugar.Errorw("crm workorder batch receive failed", "err", err, "workorder_id", id)
		}
	}
	return nil
}

// ListPool 工单池列表（分页）。
func (s CrmWorkorderPoolService) ListPool(tenantID string, req WorkorderPoolQueryReq) ([]models.CrmWorkorderPool, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.DB.Scopes(models.TenantScope(tenantID))
	var total int64
	if err := db.Model(&models.CrmWorkorderPool{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmWorkorderPool
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ==================== 工单日志 ====================

// CrmWorkorderLogService 工单日志服务。
type CrmWorkorderLogService struct{}

func NewCrmWorkorderLog() *CrmWorkorderLogService { return &CrmWorkorderLogService{} }

// ListByWorkorder 按工单列出日志。
func (s CrmWorkorderLogService) ListByWorkorder(tenantID, workorderID string) ([]models.CrmWorkorderLog, error) {
	var items []models.CrmWorkorderLog
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("workorder_id = ?",  workorderID).
		Order("id desc").Find(&items).Error
	return items, err
}

// Add 新增工单日志。
func (s CrmWorkorderLogService) Add(tenantID, workorderID, userID, action, content string) error {
	if workorderID == "" {
		return errors.New("workorder_id 不能为空")
	}
	l := models.CrmWorkorderLog{
		TenantID:    tenantID,
		WorkorderID: workorderID,
		UserID:      userID,
		Action:      action,
		Content:     content,
	}
	l.ID = id_generator.StringID()
	if err := models.DB.Create(&l).Error; err != nil {
		log.Sugar.Errorw("crm workorder log add failed", "err", err)
		return err
	}
	return nil
}
