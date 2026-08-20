package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 回款计划 ----------

// PlanQueryReq 回款计划查询参数。
type PlanQueryReq struct {
	Page       int
	PageSize   int
	OrderID    string
	CustomerID string
	Status     string
}

type CrmReceivablesPlanService struct{}

func NewCrmReceivablesPlan() *CrmReceivablesPlanService { return &CrmReceivablesPlanService{} }

// Create 创建单期回款计划。
func (s CrmReceivablesPlanService) Create(ctx context.Context, p *models.CrmReceivablesPlan) error {
	tenantID := identity.TenantID(ctx)


	if p == nil {
		return errors.New("回款计划不能为空")
	}
	if p.OrderID == "" {
		return errors.New("order_id 不能为空")
	}
	p.ID = id_generator.StringID()
	p.TenantID = tenantID
	if p.Status == "" {
		p.Status = "pending"
	}
	return createCRM(models.DB, p, "OrderID", "CustomerID")
}

// Update 更新回款计划。
func (s CrmReceivablesPlanService) Update(ctx context.Context, id string, p *models.CrmReceivablesPlan) error {


	if p == nil {
		return errors.New("回款计划不能为空")
	}
	m := map[string]any{
		"order_id":    p.OrderID,
		"customer_id": p.CustomerID,
		"period_no":   p.PeriodNo,
		"amount":      p.Amount,
		"plan_date":   p.PlanDate,
		"status":      p.Status,
		"remark":      p.Remark,
	}
	updatesCRM(m, "OrderID", "CustomerID")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除回款计划（软删除）。
func (s CrmReceivablesPlanService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmReceivablesPlan{})
	return res.RowsAffected, res.Error
}

// Get 读取单条回款计划。
func (s CrmReceivablesPlanService) Get(ctx context.Context, id string) (*models.CrmReceivablesPlan, error) {


	var item models.CrmReceivablesPlan
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Query 分页查询回款计划，支持按 order_id/customer_id/status 筛选。
func (s CrmReceivablesPlanService) Query(ctx context.Context, req PlanQueryReq) ([]models.CrmReceivablesPlan, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.OrderID != "" {
		db = db.Where("order_id = ?", req.OrderID)
	}
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmReceivablesPlan{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmReceivablesPlan
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("period_no asc, id desc").Find(&items).Error
	return items, total, err
}

// BatchCreate 批量创建多期回款计划（事务）。
func (s CrmReceivablesPlanService) BatchCreate(ctx context.Context, plans []models.CrmReceivablesPlan) error {
	tenantID := identity.TenantID(ctx)


	if len(plans) == 0 {
		return errors.New("plans 不能为空")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		for i := range plans {
			plans[i].ID = id_generator.StringID()
			plans[i].TenantID = tenantID
			if plans[i].Status == "" {
				plans[i].Status = "pending"
			}
			if err := createCRM(tx, &plans[i], "OrderID", "CustomerID"); err != nil {
				return err
			}
		}
		return nil
	})
}

// MarkOverdue 标记逾期：PlanDate < now 且 status=pending → overdue。
func (s CrmReceivablesPlanService) MarkOverdue(ctx context.Context) error {
	tenantID := identity.TenantID(ctx)


	now := time.Now()
	res := models.DB.Model(&models.CrmReceivablesPlan{}).Scopes(models.TenantScope(tenantID)).Where("status = ? AND plan_date < ?",  "pending",  now).
		Update("status", "overdue")
	if res.Error != nil {
		log.Sugar.Errorw("crm receivables plan mark overdue failed", "tenant", tenantID, "err", res.Error)
		return res.Error
	}
	if res.RowsAffected > 0 {
		log.Sugar.Infow("crm receivables plan marked overdue", "tenant", tenantID, "count", res.RowsAffected)
	}
	return nil
}

// ---------- 回款登记 ----------

// RecvQueryReq 回款登记查询参数。
type RecvQueryReq struct {
	Page       int
	PageSize   int
	OrderID    string
	CustomerID string
	PlanID     string
	Status     string
}

type CrmReceivablesService struct{}

func NewCrmReceivables() *CrmReceivablesService { return &CrmReceivablesService{} }

// Create 创建回款登记。若指定 PlanID，事务内将计划状态置为 received。
func (s CrmReceivablesService) Create(ctx context.Context, r *models.CrmReceivables) error {
	tenantID := identity.TenantID(ctx)


	if r == nil {
		return errors.New("回款登记不能为空")
	}
	if r.OrderID == "" {
		return errors.New("order_id 不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	if r.Status == "" {
		r.Status = "confirmed"
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := createCRM(tx, r, "OrderID", "CustomerID", "PlanID"); err != nil {
			return err
		}
		if r.PlanID != "" {
			if err := tx.Model(&models.CrmReceivablesPlan{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  r.PlanID).
				Update("status", "received").Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// Update 更新回款登记。
func (s CrmReceivablesService) Update(ctx context.Context, id string, r *models.CrmReceivables) error {


	if r == nil {
		return errors.New("回款登记不能为空")
	}
	m := map[string]any{
		"order_id":    r.OrderID,
		"customer_id": r.CustomerID,
		"plan_id":     r.PlanID,
		"amount":      r.Amount,
		"received_at": r.ReceivedAt,
		"pay_method":  r.PayMethod,
		"pay_account": r.PayAccount,
		"voucher_no":  r.VoucherNo,
		"attachments": r.Attachments,
		"status":      r.Status,
		"remark":      r.Remark,
	}
	updatesCRM(m, "OrderID", "CustomerID", "PlanID")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除回款登记（软删除）。
func (s CrmReceivablesService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmReceivables{})
	return res.RowsAffected, res.Error
}

// Get 读取单条回款登记。
func (s CrmReceivablesService) Get(ctx context.Context, id string) (*models.CrmReceivables, error) {


	var item models.CrmReceivables
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Query 分页查询回款登记，支持按 order_id/customer_id/plan_id/status 筛选。
func (s CrmReceivablesService) Query(ctx context.Context, req RecvQueryReq) ([]models.CrmReceivables, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.OrderID != "" {
		db = db.Where("order_id = ?", req.OrderID)
	}
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if req.PlanID != "" {
		db = db.Where("plan_id = ?", req.PlanID)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmReceivables{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmReceivables
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("received_at desc, id desc").Find(&items).Error
	return items, total, err
}

// Void 作废回款登记。若关联 PlanID，事务内将计划状态恢复为 pending。
func (s CrmReceivablesService) Void(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)


	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmReceivables
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == "void" {
			return nil
		}
		if err := tx.Model(&models.CrmReceivables{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
			Update("status", "void").Error; err != nil {
			return err
		}
		if r.PlanID != "" {
			if err := tx.Model(&models.CrmReceivablesPlan{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  r.PlanID).
				Update("status", "pending").Error; err != nil {
				return err
			}
		}
		return nil
	})
}
