package services

import (
	"context"
	"core/app/models"
	"core/common/identity"
	"errors"
	"gorm.io/gorm"
	"time"
)

// RunWecomEvent serializes one external entity and acknowledges only completed
// events. Failed attempts leave no receipt and remain retryable.
func RunWecomEvent(ctx context.Context, entityKey, receiptKey string, run func() error) error {
	tenantID := identity.TenantID(ctx)
	if tenantID == "" {
		return models.ErrMissingTenant
	}
	if err := models.RequirePlanModules(models.DB.WithContext(ctx), tenantID, "channel_wecom"); err != nil {
		return err
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", entityKey).Error; err != nil {
			return err
		}
		var receipt models.WecomEventReceipt
		err := tx.Where("receipt_key=? AND tenant_id=?", receiptKey, tenantID).Take(&receipt).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := run(); err != nil {
			return err
		}
		return tx.Create(&models.WecomEventReceipt{ReceiptKey: receiptKey, TenantID: tenantID, CreatedAt: time.Now()}).Error
	})
}
