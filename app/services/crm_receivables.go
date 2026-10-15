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
	if p == nil {
		return errors.New("回款计划不能为空")
	}
	plans := []models.CrmReceivablesPlan{*p}
	if err := s.BatchCreate(ctx, plans); err != nil {
		return err
	}
	*p = plans[0]
	return nil
}

// Update 更新回款计划。
func (s CrmReceivablesPlanService) Update(ctx context.Context, id string, p *models.CrmReceivablesPlan) error {
	if p == nil {
		return errors.New("回款计划不能为空")
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing models.CrmReceivablesPlan
		if err := tx.Where("tenant_id = ? AND id = ?", identity.TenantID(ctx), id).First(&existing).Error; err != nil {
			return err
		}
		if p.OrderID != "" && p.OrderID != existing.OrderID {
			return errors.New("已有计划不能转移到其他合同")
		}
		p.OrderID = existing.OrderID
		order, err := lockPaymentOrder(ctx, tx, existing.OrderID)
		if err != nil {
			return err
		}
		// Re-read after locking: a concurrent deletion must not become a silent success.
		if err = tx.Where("tenant_id = ? AND id = ?", order.TenantID, id).First(&existing).Error; err != nil {
			return err
		}
		if err = validatePlan(tx, order, p, id); err != nil {
			return err
		}
		return tx.Model(&existing).Updates(map[string]any{"period_no": p.PeriodNo, "amount": p.Amount, "plan_date": p.PlanDate, "status": p.Status, "remark": p.Remark}).Error
	})
}

// Delete 删除回款计划（软删除）。
func (s CrmReceivablesPlanService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var plans []models.CrmReceivablesPlan
		if err := tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Find(&plans).Error; err != nil {
			return err
		}
		if len(plans) != len(ids) {
			return errors.New("计划不存在或无权操作")
		}
		orders := []string{}
		for _, p := range plans {
			orders = append(orders, p.OrderID)
		}
		orders, _ = workflowIDs(orders)
		for _, id := range orders {
			if _, err := lockPaymentOrder(ctx, tx, id); err != nil {
				return err
			}
		}
		var count int64
		if err := tx.Model(&models.CrmReceivables{}).Where("tenant_id = ? AND plan_id IN ? AND status <> ?", identity.TenantID(ctx), ids, "void").Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("计划已有回款记录，请先处理回款记录")
		}
		return tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Delete(&models.CrmReceivablesPlan{}).Error
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// Get 读取单条回款计划。
func (s CrmReceivablesPlanService) Get(ctx context.Context, id string) (*models.CrmReceivablesPlan, error) {

	var item models.CrmReceivablesPlan
	db, err := receiptScope(ctx, models.DB)
	if err != nil {
		return nil, err
	}
	err = db.Where("id = ?", id).First(&item).Error
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
	db, scopeErr := receiptScope(ctx, models.DB)
	if scopeErr != nil {
		return nil, 0, scopeErr
	}
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
	if len(plans) == 0 {
		return errors.New("请至少添加一期回款计划")
	}
	orderIDs := make([]string, 0, len(plans))
	for _, p := range plans {
		orderIDs = append(orderIDs, p.OrderID)
	}
	ids, err := workflowIDs(orderIDs)
	if err != nil {
		return err
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		orders := map[string]models.CrmOrder{}
		for _, id := range ids {
			order, err := lockPaymentOrder(ctx, tx, id)
			if err != nil {
				return err
			}
			orders[id] = order
		}
		for i := range plans {
			p := &plans[i]
			p.ID = id_generator.StringID()
			p.TenantID = identity.TenantID(ctx)
			if err := validatePlan(tx, orders[p.OrderID], p, ""); err != nil {
				return err
			}
			if err := createCRM(tx, p); err != nil {
				return err
			}
		}
		return nil
	})
}

// MarkOverdue 标记逾期：PlanDate < now 且 status=pending → overdue。
func (s CrmReceivablesPlanService) MarkOverdue(ctx context.Context) error {
	tenantID := identity.TenantID(ctx)

	now := dayStart(time.Now())
	res := models.DB.Model(&models.CrmReceivablesPlan{}).Scopes(models.TenantScope(tenantID)).Where("status = ? AND plan_date < ?", "pending", now).
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
	Page          int
	PageSize      int
	OrderID       string
	CustomerID    string
	PlanID        string
	ApproveStatus string
	Status        string
}

type CrmReceivablesService struct{}

func NewCrmReceivables() *CrmReceivablesService { return &CrmReceivablesService{} }

// Create 创建回款登记。若指定 PlanID，事务内将计划状态置为 received。
func (s CrmReceivablesService) Create(ctx context.Context, r *models.CrmReceivables) error {
	if r == nil {
		return errors.New("回款登记不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = identity.TenantID(ctx)
	if r.Status == "" {
		r.Status = "confirmed"
	}
	if r.ApproveStatus == "" {
		r.ApproveStatus = "none"
	}
	if r.ApproveStatus != "none" && r.ApproveStatus != "pending" {
		return errors.New("新回款不能直接设置审批结果")
	}
	r.ApproverID = ""
	r.ApproveAt = nil
	r.ApproveRemark = ""
	if r.ApproveStatus == "pending" {
		r.Status = "pending"
	}
	if r.ReceivablesNo == "" {
		r.ReceivablesNo = "RC" + r.ID
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockPaymentOrder(ctx, tx, r.OrderID)
		if err != nil {
			return err
		}
		if err = validateReceipt(tx, order, r); err != nil {
			return err
		}
		baseline, err := paymentBaseline(tx, order)
		if err != nil {
			return err
		}
		if err = createCRM(tx, r); err != nil {
			return err
		}
		return reconcilePayments(tx, &order, baseline)
	})
	if err == nil {
		paymentEvent(ctx, *r)
	}
	return err
}

// Update 更新回款登记。
func (s CrmReceivablesService) Update(ctx context.Context, id string, r *models.CrmReceivables) error {
	if r == nil {
		return errors.New("回款登记不能为空")
	}
	return s.changeReceipt(ctx, id, func(tx *gorm.DB, current *models.CrmReceivables, order models.CrmOrder) error {
		if current.Status == "void" {
			return errors.New("已作废回款不能修改")
		}
		if current.ApproveStatus == "approved" {
			return errors.New("已审批回款不可修改，请作废后重新登记")
		}
		if r.OrderID != "" && r.OrderID != current.OrderID {
			return errors.New("已有回款不能转移到其他合同")
		}
		r.OrderID = current.OrderID
		r.ApproveStatus = current.ApproveStatus
		if r.Status == "" {
			r.Status = current.Status
		}
		if current.ApproveStatus == "pending" || current.ApproveStatus == "rejected" {
			r.Status = "pending"
		}
		if err := validateReceipt(tx, order, r); err != nil {
			return err
		}
		var plan any
		r.PlanID = strings.TrimSpace(r.PlanID)
		if r.PlanID != "" {
			plan = r.PlanID
		}
		err := tx.Model(current).Updates(map[string]any{"plan_id": plan, "amount": r.Amount, "received_at": r.ReceivedAt, "pay_method": r.PayMethod, "pay_account": r.PayAccount, "voucher_no": r.VoucherNo, "attachments": r.Attachments, "status": r.Status, "remark": r.Remark}).Error
		current.Status = r.Status
		return err
	})
}

// Delete 删除回款登记（软删除）。
func (s CrmReceivablesService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var records []models.CrmReceivables
		if err := tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Find(&records).Error; err != nil {
			return err
		}
		if len(records) != len(ids) {
			return errors.New("回款记录不存在或无权操作")
		}
		orders := []string{}
		for _, r := range records {
			orders = append(orders, r.OrderID)
		}
		orders, _ = workflowIDs(orders)
		for _, id := range orders {
			if _, err := lockPaymentOrder(ctx, tx, id); err != nil {
				return err
			}
		}
		if err := tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Find(&records).Error; err != nil {
			return err
		}
		for _, r := range records {
			if r.Status == "confirmed" {
				return errors.New("已确认回款不能直接删除，请先作废")
			}
		}
		return tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Delete(&models.CrmReceivables{}).Error
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// Get 读取单条回款登记。
func (s CrmReceivablesService) Get(ctx context.Context, id string) (*models.CrmReceivables, error) {

	var item models.CrmReceivables
	db, err := receiptScope(ctx, models.DB)
	if err != nil {
		return nil, err
	}
	err = db.Where("id = ?", id).First(&item).Error
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
	db, scopeErr := receiptScope(ctx, models.DB)
	if scopeErr != nil {
		return nil, 0, scopeErr
	}
	if req.OrderID != "" {
		db = db.Where("order_id = ?", req.OrderID)
	}
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if req.ApproveStatus != "" {
		db = db.Where("approve_status = ?", req.ApproveStatus)
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
	return s.changeReceipt(ctx, id, func(tx *gorm.DB, r *models.CrmReceivables, _ models.CrmOrder) error {
		if r.Status == "void" {
			return nil
		}
		r.Status = "void"
		return tx.Model(r).Update("status", "void").Error
	})
}
