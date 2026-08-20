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

// AftersalesQueryReq 售后查询参数。
type AftersalesQueryReq struct {
	Page       int
	PageSize   int
	Status     string
	CustomerID string
	Type       string
	Keyword    string
}

// aftersalesStatusFlow 允许的状态流转。
var aftersalesStatusFlow = map[string]map[string]bool{
	"pending":    {"processing": true, "done": true, "rejected": true},
	"processing": {"done": true, "rejected": true},
	"done":       {},
	"rejected":   {"pending": true},
}

// genAftersalesNo 生成售后单号：AS + yyyyMMdd + 4位序号。
func genAftersalesNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "AS" + day
	var cnt int64
	models.DB.Model(&models.CrmAftersales{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?",  prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// CrmAftersalesService 售后服务。
type CrmAftersalesService struct{}

func NewCrmAftersales() *CrmAftersalesService { return &CrmAftersalesService{} }

// Create 创建售后单，自动生成 No。
func (s CrmAftersalesService) Create(ctx context.Context, a *models.CrmAftersales) error {
	tenantID := identity.TenantID(ctx)


	if a == nil {
		return errors.New("售后单不能为空")
	}
	if strings.TrimSpace(a.Reason) == "" {
		return errors.New("售后原因不能为空")
	}
	a.ID = id_generator.StringID()
	a.TenantID = tenantID
	a.Reason = strings.TrimSpace(a.Reason)
	if a.Status == "" {
		a.Status = "pending"
	}
	if a.No == "" {
		a.No = genAftersalesNo(tenantID)
	}
	if err := createCRM(models.DB, a, "CustomerID", "OrderID", "HandlerID"); err != nil {
		log.Sugar.Errorw("crm aftersales create failed", "err", err)
		return err
	}
	return nil
}

// Update 更新售后单。
func (s CrmAftersalesService) Update(ctx context.Context, id string, a *models.CrmAftersales) error {
	tenantID := identity.TenantID(ctx)


	if a == nil {
		return errors.New("售后单不能为空")
	}
	var exist models.CrmAftersales
	if err := models.MustFor(ctx).Where("id = ?", id).First(&exist).Error; err != nil {
		return errors.New("售后单不存在")
	}
	upd := updatesCRM(map[string]any{
		"type":        a.Type,
		"customer_id": a.CustomerID,
		"order_id":    a.OrderID,
		"amount":      a.Amount,
		"reason":      a.Reason,
		"handler_id":  a.HandlerID,
		"result":      a.Result,
	}, "customer_id", "order_id", "handler_id")
	if err := models.DB.Model(&models.CrmAftersales{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Updates(upd).Error; err != nil {
		log.Sugar.Errorw("crm aftersales update failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除售后单。
func (s CrmAftersalesService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmAftersales{})
	return res.RowsAffected, res.Error
}

// Query 分页查询售后单。
func (s CrmAftersalesService) Query(ctx context.Context, req AftersalesQueryReq) ([]models.CrmAftersales, int64, error) {


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
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("reason LIKE ? OR no LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmAftersales{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmAftersales
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改售后单状态，校验状态流转。
func (s CrmAftersalesService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	if _, ok := aftersalesStatusFlow[status]; !ok {
		return errors.New("无效的售后状态")
	}
	var a models.CrmAftersales
	if err := models.MustFor(ctx).Where("id = ?", id).First(&a).Error; err != nil {
		return errors.New("售后单不存在")
	}
	if a.Status == status {
		return nil
	}
	transitions, ok := aftersalesStatusFlow[a.Status]
	if !ok || !transitions[status] {
		return fmt.Errorf("不允许从 %s 流转到 %s", a.Status, status)
	}
	upd := map[string]any{"status": status}
	if status == "done" {
		now := time.Now()
		upd["done_at"] = &now
	}
	if err := models.DB.Model(&models.CrmAftersales{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(upd).Error; err != nil {
		log.Sugar.Errorw("crm aftersales change status failed", "err", err)
		return err
	}
	return nil
}
