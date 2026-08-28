package services

import (
	"context"

	"encoding/json"
	"errors"
	"strings"
	"time"

	"core/app/models"

	"gorm.io/gorm"
)

// CrmOrderExtService 合同扩展服务（作废 / 变更 / 关联项目工单）。
type CrmOrderExtService struct{}

func NewCrmOrderExt() *CrmOrderExtService { return &CrmOrderExtService{} }

// Revoke 合同作废：Status=cancelled, RevokeReason, RevokeAt=now。
func (s CrmOrderExtService) Revoke(ctx context.Context, id, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("请填写合同作废原因")
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockPaymentOrder(ctx, tx, id)
		if err != nil {
			return err
		}
		if order.Status == "cancelled" {
			return nil
		}
		var count int64
		if err = tx.Model(&models.CrmReceivables{}).Where("tenant_id = ? AND order_id = ? AND status <> ?", order.TenantID, id, "void").Count(&count).Error; err != nil {
			return err
		}
		if order.PaidAmount > 0 || count > 0 {
			return errors.New("合同仍有回款记录，请先完成收款冲销后再作废")
		}
		if err = tx.Model(&order).Updates(map[string]any{"status": "cancelled", "revoke_reason": reason, "revoke_at": time.Now()}).Error; err != nil {
			return err
		}
		return tx.Model(&models.CrmReceivablesPlan{}).Where("tenant_id = ? AND order_id = ?", order.TenantID, id).Update("status", "cancelled").Error
	})
}

// Change 合同变更：通用字段更新（仅 draft/signed 状态可变更）。
func (s CrmOrderExtService) Change(ctx context.Context, id string, updates map[string]any) error {

	if id == "" {
		return errors.New("id 不能为空")
	}
	if len(updates) == 0 {
		return errors.New("updates 不能为空")
	}
	// 允许变更的字段白名单。
	allowed := map[string]bool{
		"title": true, "amount": true, "contract_type": true, "discount_amount": true,
		"tax_amount": true, "delivery_date": true, "effective_date": true, "expire_date": true,
		"signed_at": true, "remark": true, "customer_id": true, "opportunity_id": true,
		"user_id": true,
	}
	cleaned := map[string]any{}
	for k, v := range updates {
		if allowed[k] {
			cleaned[k] = v
		}
	}
	if len(cleaned) == 0 {
		return errors.New("无可变更字段")
	}
	// 剔除空字符串外键（bigint 列不允许空串）。
	updatesCRM(cleaned, "customer_id", "opportunity_id", "user_id")
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockPaymentOrder(ctx, tx, id)
		if err != nil {
			return err
		}
		if order.Status != "draft" && order.Status != "signed" {
			return errors.New("当前合同状态不可变更")
		}
		core := map[string]any{}
		for _, key := range []string{"title", "amount", "customer_id", "opportunity_id", "user_id"} {
			if value, ok := cleaned[key]; ok {
				core[key] = value
			}
		}
		oldCustomer := order.CustomerID
		raw, err := json.Marshal(core)
		if err != nil {
			return err
		}
		if err = json.Unmarshal(raw, &order); err != nil {
			return errors.New("合同字段格式错误")
		}
		if oldCustomer != order.CustomerID {
			return errors.New("更换客户请使用合同编辑并核对关联记录")
		}
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
			return errors.New("合同应收金额不能低于已收金额或回款计划")
		}
		cleaned["pay_status"] = paymentStatus(paid, due)
		return tx.Model(&order).Updates(cleaned).Error
	})
}

// LinkProject 关联项目。
func (s CrmOrderExtService) LinkProject(ctx context.Context, id, targetID string) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := editableOrder(ctx, tx, id, false)
		if err != nil {
			return err
		}
		var target models.CrmProject
		if err := tx.Where("tenant_id = ? AND id = ? AND customer_id = ?", order.TenantID, targetID, order.CustomerID).First(&target).Error; err != nil {
			return errors.New("关联记录不存在或不属于合同客户")
		}
		if target.OrderID != "" && target.OrderID != id {
			return errors.New("关联记录已绑定其他合同")
		}
		return tx.Model(&order).Update("project_id", targetID).Error
	})
}

// LinkWorkorder 关联工单。
func (s CrmOrderExtService) LinkWorkorder(ctx context.Context, id, targetID string) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := editableOrder(ctx, tx, id, false)
		if err != nil {
			return err
		}
		var target models.CrmWorkorder
		if err := tx.Where("tenant_id = ? AND id = ? AND customer_id = ?", order.TenantID, targetID, order.CustomerID).First(&target).Error; err != nil {
			return errors.New("关联记录不存在或不属于合同客户")
		}
		if target.OrderID != "" && target.OrderID != id {
			return errors.New("关联记录已绑定其他合同")
		}
		return tx.Model(&order).Update("workorder_id", targetID).Error
	})
}
