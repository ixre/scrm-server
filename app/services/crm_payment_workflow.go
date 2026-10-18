package services

import (
	"context"
	"errors"
	"math"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Use integer cents for comparisons, while keeping the existing DECIMAL storage contract.
func moneyCents(value float64) (int64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 9e13 {
		return 0, errors.New("金额必须是有效的非负数")
	}
	cents := math.Round(value * 100)
	if math.Abs(value*100-cents) > 0.001 {
		return 0, errors.New("金额最多保留两位小数")
	}
	return int64(cents), nil
}
func orderDue(order models.CrmOrder) (int64, error) {
	amount, err := moneyCents(order.Amount)
	if err != nil {
		return 0, err
	}
	discount, err := moneyCents(order.CouponAmount)
	if err != nil {
		return 0, err
	}
	if discount > amount {
		return 0, errors.New("优惠金额不能超过合同金额")
	}
	return amount - discount, nil
}
func paymentStatus(paid, due int64) string {
	if paid >= due && due > 0 {
		return "paid"
	}
	if paid > 0 {
		return "partial"
	}
	return "unpaid"
}
func dayStart(now time.Time) time.Time {
	now = now.In(constants.PRCLocation)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, constants.PRCLocation)
}
func planStatus(plan models.CrmReceivablesPlan, paid int64) string {
	amount, _ := moneyCents(plan.Amount)
	if amount > 0 && paid >= amount {
		return "received"
	}
	if plan.PlanDate != nil && plan.PlanDate.Before(dayStart(time.Now())) {
		return "overdue"
	}
	return "pending"
}
func confirmedReceipts(tx *gorm.DB, tenant, order string) *gorm.DB {
	return tx.Model(&models.CrmReceivables{}).Where("tenant_id = ? AND order_id = ? AND status = ? AND (approve_status IN ? OR approve_status IS NULL)", tenant, order, "confirmed", []string{"none", "approved", ""})
}
func receiptSum(db *gorm.DB) (int64, error) {
	var amount float64
	err := db.Select("COALESCE(SUM(amount),0)").Scan(&amount).Error
	if err != nil {
		return 0, err
	}
	return moneyCents(amount)
}
func paymentBaseline(tx *gorm.DB, order models.CrmOrder) (int64, error) {
	total, err := receiptSum(confirmedReceipts(tx, order.TenantID, order.ID))
	if err != nil {
		return 0, err
	}
	paid, err := moneyCents(order.PaidAmount)
	if err != nil {
		return 0, err
	}
	// Preserve historical amounts entered directly on the order before receipts existed.
	if paid > total {
		return paid - total, nil
	}
	return 0, nil
}
func reconcilePayments(tx *gorm.DB, order *models.CrmOrder, baseline int64) error {
	total, err := receiptSum(confirmedReceipts(tx, order.TenantID, order.ID))
	if err != nil {
		return err
	}
	total += baseline
	due, err := orderDue(*order)
	if err != nil {
		return err
	}
	if total > due {
		return errors.New("累计回款不能超过合同应收金额")
	}
	if err := validatePlanCollections(tx, *order); err != nil {
		return err
	}
	order.PaidAmount = float64(total) / 100
	order.PayStatus = paymentStatus(total, due)
	if err = tx.Model(order).Updates(map[string]any{"paid_amount": order.PaidAmount, "pay_status": order.PayStatus}).Error; err != nil {
		return err
	}
	return reconcilePlanStatuses(tx, *order)
}

// All receipt and plan writers lock the parent order first, so concurrent amounts cannot overshoot.
func lockPaymentOrder(ctx context.Context, tx *gorm.DB, id string) (models.CrmOrder, error) {
	var order models.CrmOrder
	if id == "" {
		return order, errors.New("请选择合同")
	}
	actor, manager, err := customerActor(ctx, tx)
	if err != nil {
		return order, err
	}
	if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", actor.TenantID, id).First(&order).Error; err != nil {
		return order, errors.New("合同不存在或无权操作")
	}
	if !manager && order.UserID != actor.ID {
		return order, errors.New("只能操作自己负责的合同")
	}
	return order, nil
}
func validateReceipt(tx *gorm.DB, order models.CrmOrder, r *models.CrmReceivables) error {
	if order.Status == "cancelled" {
		return errors.New("已作废合同不能登记或修改回款")
	}
	if r.CustomerID != "" && r.CustomerID != order.CustomerID {
		return errors.New("回款客户与合同客户不一致")
	}
	r.CustomerID = order.CustomerID
	amount, err := moneyCents(r.Amount)
	if err != nil {
		return err
	}
	if amount <= 0 {
		return errors.New("回款金额必须大于零")
	}
	if r.ReceivedAt == nil {
		return errors.New("请选择实际回款日期")
	}
	if r.Status != "pending" && r.Status != "confirmed" {
		return errors.New("回款状态只能为待确认或已确认")
	}
	if r.PlanID != "" {
		var plan models.CrmReceivablesPlan
		if err = tx.Where("tenant_id = ? AND id = ? AND order_id = ?", order.TenantID, r.PlanID, order.ID).First(&plan).Error; err != nil {
			return errors.New("回款计划不属于当前合同")
		}
	}
	return nil
}

// validatePlanFields is shared by single-plan updates and batch creation.
func validatePlanFields(order models.CrmOrder, p *models.CrmReceivablesPlan) (int64, error) {
	if order.Status == "cancelled" {
		return 0, errors.New("已作废合同不能维护回款计划")
	}
	if p.CustomerID != "" && p.CustomerID != order.CustomerID {
		return 0, errors.New("计划客户与合同客户不一致")
	}
	p.CustomerID = order.CustomerID
	amount, err := moneyCents(p.Amount)
	if err != nil {
		return 0, err
	}
	if amount <= 0 {
		return 0, errors.New("计划金额必须大于零")
	}
	if p.PeriodNo <= 0 || p.PeriodNo > math.MaxInt32 {
		return 0, errors.New("期次必须为有效正整数")
	}
	if p.PlanDate == nil {
		return 0, errors.New("请选择计划回款日期")
	}
	return amount, nil
}

func validatePlan(tx *gorm.DB, order models.CrmOrder, p *models.CrmReceivablesPlan, exclude string) error {
	amount, err := validatePlanFields(order, p)
	if err != nil {
		return err
	}
	var count int64
	if err = tx.Model(&models.CrmReceivablesPlan{}).Where("tenant_id = ? AND order_id = ? AND period_no = ? AND id <> ?", order.TenantID, order.ID, p.PeriodNo, zeroID(exclude)).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return errors.New("该合同已有相同期次的回款计划")
	}
	total, err := receiptSum(tx.Model(&models.CrmReceivablesPlan{}).Where("tenant_id = ? AND order_id = ? AND id <> ?", order.TenantID, order.ID, zeroID(exclude)))
	if err != nil {
		return err
	}
	due, err := orderDue(order)
	if err != nil {
		return err
	}
	if total+amount > due {
		return errors.New("计划总金额不能超过合同应收金额")
	}
	paid := int64(0)
	if exclude != "" {
		paid, err = receiptSum(confirmedReceipts(tx, order.TenantID, order.ID).Where("plan_id = ?", exclude))
		if err != nil {
			return err
		}
	}
	if paid > amount {
		return errors.New("计划金额不能低于本期已收金额")
	}
	p.Status = planStatus(*p, paid)
	return nil
}
func zeroID(id string) string {
	if id == "" {
		return "0"
	}
	return id
}

func receiptScope(ctx context.Context, db *gorm.DB) (*gorm.DB, error) {
	actor, manager, err := customerActor(ctx, db)
	if err != nil {
		return nil, err
	}
	q := db.WithContext(ctx).Where("tenant_id = ?", actor.TenantID)
	if !manager {
		q = q.Where("order_id IN (?)", db.Model(&models.CrmOrder{}).Select("id").Where("tenant_id = ? AND user_id = ?", actor.TenantID, actor.ID))
	}
	return q, nil
}

func paymentEvent(ctx context.Context, r models.CrmReceivables) {
	if r.Status == "confirmed" && (r.ApproveStatus == "none" || r.ApproveStatus == "approved" || r.ApproveStatus == "") && r.CustomerID != "" {
		NewCrmSop().Trigger(ctx, "payment_received", map[string]string{"customer_id": r.CustomerID, "order_id": r.OrderID})
	}
}

func (s CrmReceivablesService) changeReceipt(ctx context.Context, id string, change func(*gorm.DB, *models.CrmReceivables, models.CrmOrder) error) error {
	var changed models.CrmReceivables
	var notify bool
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rec models.CrmReceivables
		if err := tx.Where("tenant_id = ? AND id = ?", identity.TenantID(ctx), id).First(&rec).Error; err != nil {
			return err
		}
		order, err := lockPaymentOrder(ctx, tx, rec.OrderID)
		if err != nil {
			return err
		}
		if err = tx.Where("tenant_id = ? AND id = ?", order.TenantID, id).First(&rec).Error; err != nil {
			return err
		}
		baseline, err := paymentBaseline(tx, order)
		if err != nil {
			return err
		}
		before := rec.Status
		if err = change(tx, &rec, order); err != nil {
			return err
		}
		if err = reconcilePayments(tx, &order, baseline); err != nil {
			return err
		}
		changed = rec
		notify = before != "confirmed" && rec.Status == "confirmed"
		return nil
	})
	if err == nil && notify {
		paymentEvent(ctx, changed)
	}
	return err
}
