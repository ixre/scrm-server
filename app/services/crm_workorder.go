package services

import (
	"core/common/identity"

	"context"

	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// ---------- 工单 ----------

type CrmWorkorderService struct{}

func NewCrmWorkorder() *CrmWorkorderService { return &CrmWorkorderService{} }

// WorkorderQueryReq 工单查询请求。
type WorkorderQueryReq struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	Status     string `json:"status"`
	CustomerID string `json:"customer_id"`
	Type       string `json:"type"`
	Priority   string `json:"priority"`
	Keyword    string `json:"keyword"`
}

// workorderStatusFlow 允许的状态流转：from -> [to...]
var workorderStatusFlow = map[string]map[string]bool{
	"open":       {"processing": true, "done": true, "cancelled": true},
	"processing": {"done": true, "cancelled": true},
	"done":       {},
	"cancelled":  {"open": true},
}

// genWorkorderNo 生成工单号：WO + yyyyMMdd + 4位当日序号。
func genWorkorderNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "WO" + day
	var cnt int64
	models.DB.Model(&models.CrmWorkorder{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?", prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建工单，自动生成 No。
func (s CrmWorkorderService) Create(ctx context.Context, w *models.CrmWorkorder) error {
	tenantID := identity.TenantID(ctx)

	if w == nil {
		return errors.New("工单不能为空")
	}
	if strings.TrimSpace(w.Title) == "" {
		return errors.New("标题不能为空")
	}
	w.ID = id_generator.StringID()
	w.TenantID = tenantID
	w.Title = strings.TrimSpace(w.Title)
	if w.Status == "" {
		w.Status = "open"
	}
	if w.Priority == "" {
		w.Priority = "medium"
	}
	if w.No == "" {
		w.No = genWorkorderNo(tenantID)
	}
	if err := createCRM(models.DB, w, "CustomerID", "OrderID", "AssigneeID"); err != nil {
		log.Sugar.Errorw("crm workorder create failed", "err", err)
		return err
	}
	return nil
}

// Update 更新工单。
func (s CrmWorkorderService) Update(ctx context.Context, id string, w *models.CrmWorkorder) error {
	tenantID := identity.TenantID(ctx)

	if w == nil {
		return errors.New("工单不能为空")
	}
	var exist models.CrmWorkorder
	if err := models.MustFor(ctx).Where("id = ?", id).First(&exist).Error; err != nil {
		return errors.New("工单不存在")
	}
	upd := updatesCRM(map[string]any{
		"title":       w.Title,
		"type":        w.Type,
		"customer_id": w.CustomerID,
		"order_id":    w.OrderID,
		"priority":    w.Priority,
		"assignee_id": w.AssigneeID,
		"description": w.Description,
	}, "customer_id", "order_id", "assignee_id")
	selectCols := []string{"title", "type", "customer_id", "order_id", "priority", "assignee_id", "description"}
	if err := models.DB.Model(&models.CrmWorkorder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Select(selectCols).Updates(upd).Error; err != nil {
		log.Sugar.Errorw("crm workorder update failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除工单。
func (s CrmWorkorderService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmWorkorder{})
	return res.RowsAffected, res.Error
}

// Get 查单个工单。
func (s CrmWorkorderService) Get(ctx context.Context, id string) (*models.CrmWorkorder, error) {

	var w models.CrmWorkorder
	if err := models.MustFor(ctx).Where("id = ?", id).First(&w).Error; err != nil {
		return nil, err
	}
	return &w, nil
}

// Query 列出工单，支持按 status/customer_id/type/priority/keyword 筛选。
func (s CrmWorkorderService) Query(ctx context.Context, req WorkorderQueryReq) ([]models.CrmWorkorder, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	db := models.MustFor(ctx)
	if req.Status != "" {
		db = db.Where("status = ?", req.Status)
	}
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if req.Type != "" {
		db = db.Where("type = ?", req.Type)
	}
	if req.Priority != "" {
		db = db.Where("priority = ?", req.Priority)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("title LIKE ? OR no LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmWorkorder{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmWorkorder
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改工单状态，校验状态流转；done 时设 DoneAt。
func (s CrmWorkorderService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)

	if _, ok := workorderStatusFlow[status]; !ok {
		return errors.New("无效的工单状态")
	}
	var w models.CrmWorkorder
	if err := models.MustFor(ctx).Where("id = ?", id).First(&w).Error; err != nil {
		return errors.New("工单不存在")
	}
	if w.Status == status {
		return nil
	}
	transitions, ok := workorderStatusFlow[w.Status]
	if !ok || !transitions[status] {
		return fmt.Errorf("不允许从 %s 流转到 %s", w.Status, status)
	}
	upd := map[string]any{"status": status}
	if status == "done" {
		now := time.Now()
		upd["done_at"] = &now
	}
	if err := models.DB.Model(&models.CrmWorkorder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(upd).Error; err != nil {
		log.Sugar.Errorw("crm workorder change status failed", "err", err)
		return err
	}
	return nil
}

// Transfer 转派工单处理人。
func (s CrmWorkorderService) Transfer(ctx context.Context, id, toUserID string) error {
	tenantID := identity.TenantID(ctx)

	if toUserID == "" {
		return errors.New("目标处理人不能为空")
	}
	res := models.DB.Model(&models.CrmWorkorder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Update("assignee_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("crm workorder transfer failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("工单不存在")
	}
	return nil
}
