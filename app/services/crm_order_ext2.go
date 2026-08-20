package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"time"

	"core/app/models"
	"core/common/log"

	"gorm.io/gorm"
)

// CrmOrderExtService 合同扩展服务（作废 / 变更 / 关联项目工单）。
type CrmOrderExtService struct{}

func NewCrmOrderExt() *CrmOrderExtService { return &CrmOrderExtService{} }

// Revoke 合同作废：Status=cancelled, RevokeReason, RevokeAt=now。
func (s CrmOrderExtService) Revoke(ctx context.Context, id, reason string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status <> ?", id,  "cancelled").
		Updates(map[string]any{"status": "cancelled", "revoke_reason": reason, "revoke_at": now})
	if res.Error != nil {
		log.Sugar.Errorw("order revoke failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("合同不存在或已作废")
	}
	return nil
}

// Change 合同变更：通用字段更新（仅 draft/signed 状态可变更）。
func (s CrmOrderExtService) Change(ctx context.Context, id string, updates map[string]any) error {
	tenantID := identity.TenantID(ctx)


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
	res := models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status IN ?", id,  []string{"draft", "signed"}).
		Updates(cleaned)
	if res.Error != nil {
		log.Sugar.Errorw("order change failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("合同不存在或当前状态不可变更")
	}
	return nil
}

// LinkProject 关联项目。
func (s CrmOrderExtService) LinkProject(ctx context.Context, id, projectID string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	if projectID == "" {
		return errors.New("project_id 不能为空")
	}
	res := models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Update("project_id", projectID)
	if res.Error != nil {
		log.Sugar.Errorw("order link project failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// LinkWorkorder 关联工单。
func (s CrmOrderExtService) LinkWorkorder(ctx context.Context, id, workorderID string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	if workorderID == "" {
		return errors.New("workorder_id 不能为空")
	}
	res := models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Update("workorder_id", workorderID)
	if res.Error != nil {
		log.Sugar.Errorw("order link workorder failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
