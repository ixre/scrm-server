package services

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validateOrder(ctx context.Context, tx *gorm.DB, order *models.CrmOrder) error {
	if strings.TrimSpace(order.Title) == "" {
		return errors.New("请填写合同名称")
	}
	if order.CustomerID == "" {
		return errors.New("请选择合同客户")
	}
	db, err := customerAccessDB(ctx, tx, true)
	if err != nil {
		return err
	}
	var customer models.CrmCustomer
	if err = db.Clauses(clause.Locking{Strength: "SHARE"}).Where("id = ?", order.CustomerID).First(&customer).Error; err != nil {
		return ErrCustomerAccess
	}
	if customer.ReassignStatus == 1 {
		return errors.New("请先接收转交客户再创建合同")
	}
	if customer.PoolStatus == 1 {
		return errors.New("请先领取客户再创建合同")
	}
	if err = validateCustomerOwner(ctx, tx, order.UserID); err != nil {
		return err
	}
	if order.OpportunityID != "" {
		var opportunity models.CrmOpportunity
		if err = tx.Where("tenant_id = ? AND id = ? AND customer_id = ?", order.TenantID, order.OpportunityID, order.CustomerID).First(&opportunity).Error; err != nil {
			return errors.New("商机不存在或不属于合同客户")
		}
	}
	if err = validateOrderProductTotal(tx, *order, order.Status != "draft"); err != nil {
		return err
	}
	_, err = moneyCents(order.Amount)
	if err != nil {
		return err
	}
	switch order.Status {
	case "draft", "signed", "completed":
	default:
		return errors.New("合同状态无效，作废请使用专用操作")
	}
	return nil
}

// Persist order and coupon redemption in one transaction. A failed order never consumes a coupon.
func createOrderWithCoupon(ctx context.Context, tx *gorm.DB, order *models.CrmOrder, code string) error {
	var rec models.CrmCouponRecord
	var coupon models.CrmCoupon
	if code != "" {
		if err := tx.Where("tenant_id = ? AND code = ?", order.TenantID, code).First(&rec).Error; err != nil {
			return errors.New("优惠券不存在")
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", order.TenantID, rec.CouponID).First(&coupon).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", order.TenantID, rec.ID).First(&rec).Error; err != nil {
			return err
		}
		if rec.Status != "issued" {
			return errors.New("优惠券已使用或不可核销")
		}
		if rec.CustomerID != order.CustomerID {
			return errors.New("优惠券不属于合同客户")
		}
		if err := usableCoupon(coupon, time.Now()); err != nil {
			return err
		}
		if order.Amount < coupon.Threshold {
			return errors.New("合同金额未达到优惠券使用门槛")
		}
		order.CouponID = coupon.ID
		switch coupon.CouponType {
		case "cash":
			order.CouponAmount = math.Min(order.Amount, coupon.FaceValue)
		case "discount":
			if coupon.FaceValue <= 0 || coupon.FaceValue >= 1 {
				return errors.New("折扣券比例应大于 0 且小于 1")
			}
			order.CouponAmount = math.Round(order.Amount*(1-coupon.FaceValue)*100) / 100
		case "gift":
			order.CouponAmount = 0
		default:
			return errors.New("优惠券类型无效")
		}
	}
	due, err := orderDue(*order)
	if err != nil {
		return err
	}
	paid, err := moneyCents(order.PaidAmount)
	if err != nil {
		return err
	}
	if paid > due {
		return errors.New("已收金额不能超过合同应收金额")
	}
	order.PayStatus = paymentStatus(paid, due)
	if err = createCRM(tx, order); err != nil {
		return err
	}
	if code != "" {
		if err = tx.Model(&rec).Updates(map[string]any{"status": "used", "order_id": order.ID, "used_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Model(&coupon).UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error
	}
	return nil
}

func (s CrmOrderService) updateContract(ctx context.Context, id string, req models.CrmOrder) (models.CrmOrder, error) {
	var order models.CrmOrder
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		order, err = lockPaymentOrder(ctx, tx, id)
		if err != nil {
			return err
		}
		if order.Status == "cancelled" || order.Status == "completed" {
			return errors.New("已完成或已作废合同不能编辑")
		}
		if req.CustomerID == "" {
			req.CustomerID = order.CustomerID
		}
		if req.UserID == "" {
			req.UserID = order.UserID
		}
		if req.Status == "" {
			req.Status = order.Status
		}
		if req.CustomerID != order.CustomerID {
			var count int64
			if err = tx.Model(&models.CrmReceivablesPlan{}).Where("tenant_id = ? AND order_id = ?", order.TenantID, id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				return errors.New("已有回款计划的合同不能更换客户")
			}
			if err = tx.Model(&models.CrmReceivables{}).Where("tenant_id = ? AND order_id = ?", order.TenantID, id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 || order.CouponID != "" {
				return errors.New("已有回款或优惠券的合同不能更换客户")
			}
		}
		order.Title = req.Title
		order.Amount = req.Amount
		order.CustomerID = req.CustomerID
		order.UserID = req.UserID
		order.OpportunityID = req.OpportunityID
		order.Status = req.Status
		order.SignedAt = req.SignedAt
		order.Remark = req.Remark
		if err = validateOrder(ctx, tx, &order); err != nil {
			return err
		}
		due, err := orderDue(order)
		if err != nil {
			return err
		}
		paid, err := moneyCents(order.PaidAmount)
		if err != nil {
			return err
		}
		planned, err := receiptSum(tx.Model(&models.CrmReceivablesPlan{}).Where("tenant_id = ? AND order_id = ?", order.TenantID, id))
		if err != nil {
			return err
		}
		if due < paid || due < planned {
			return errors.New("合同应收金额不能低于已收金额或已制定的回款计划")
		}
		order.PayStatus = paymentStatus(paid, due)
		changes := map[string]any{"title": order.Title, "amount": order.Amount, "customer_id": order.CustomerID, "user_id": order.UserID, "opportunity_id": nil, "status": order.Status, "pay_status": order.PayStatus, "signed_at": order.SignedAt, "remark": order.Remark}
		if order.OpportunityID != "" {
			changes["opportunity_id"] = order.OpportunityID
		}
		return tx.Model(&order).Updates(changes).Error
	})
	return order, err
}

func newOrder(ctx context.Context, user string, req models.CrmOrder) models.CrmOrder {
	req.ID = id_generator.StringID()
	req.TenantID = identity.TenantID(ctx)
	if req.UserID == "" {
		req.UserID = user
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	req.CouponID = ""
	req.CouponAmount = 0
	if req.OrderNo == "" {
		req.OrderNo = "SO" + req.ID
	}
	return req
}

func (s CrmOrderService) Get(ctx context.Context, id string) (models.CrmOrder, error) {
	var order models.CrmOrder
	actor, manager, err := customerActor(ctx, models.DB)
	if err != nil {
		return order, err
	}
	db := models.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", actor.TenantID, id)
	if !manager {
		db = db.Where("user_id = ?", actor.ID)
	}
	err = db.First(&order).Error
	return order, err
}

// Sign changes only the signing fields of the current locked draft, preserving concurrent edits and receipts.
func (s CrmOrderService) Sign(ctx context.Context, id string, signedAt *time.Time) (models.CrmOrder, error) {
	var order models.CrmOrder
	if signedAt == nil {
		return order, errors.New("请选择签约日期")
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		order, err = lockPaymentOrder(ctx, tx, id)
		if err != nil {
			return err
		}
		if order.Status == "signed" {
			return nil
		}
		if order.Status != "draft" {
			return errors.New("仅草稿合同可以签约")
		}
		order.Status, order.SignedAt = "signed", signedAt
		if err = validateOrder(ctx, tx, &order); err != nil {
			return err
		}
		return tx.Model(&order).Updates(map[string]any{"status": "signed", "signed_at": signedAt}).Error
	})
	return order, err
}
