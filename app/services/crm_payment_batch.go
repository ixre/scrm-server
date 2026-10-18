package services

import (
	"core/app/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

const maxPaymentPlanBatch = 1000

// The caller holds the parent contract lock throughout validation and updates.
// Restrict the aggregate before grouping, so unrelated tenants/contracts are never scanned.
const planCollectionTotalsSQL = `SELECT r.plan_id, SUM(r.amount) AS paid
 FROM crm_receivables r
 WHERE r.tenant_id = ? AND r.order_id = ? AND r.deleted_at IS NULL AND ` + effectiveBusinessReceipt + `
 GROUP BY r.plan_id`

func validatePlanCollections(tx *gorm.DB, order models.CrmOrder) error {
	var result struct {
		InvalidAmount bool
		Overpaid      bool
	}
	err := tx.Raw(`WITH collected AS (`+planCollectionTotalsSQL+`)
 SELECT COALESCE(bool_or(p.amount < 0 OR p.amount > 90000000000000 OR c.paid < 0 OR c.paid > 90000000000000),false) AS invalid_amount,
        COALESCE(bool_or(COALESCE(c.paid,0) > COALESCE(p.amount,0)),false) AS overpaid
 FROM crm_receivables_plan p LEFT JOIN collected c ON c.plan_id = p.id
 WHERE p.tenant_id = ? AND p.order_id = ? AND p.deleted_at IS NULL`, order.TenantID, order.ID, order.TenantID, order.ID).Scan(&result).Error
	if err != nil {
		return err
	}
	if result.InvalidAmount {
		return errors.New("金额必须是有效的非负数")
	}
	if result.Overpaid {
		return errors.New("本期累计回款不能超过计划金额")
	}
	return nil
}

func reconcilePlanStatuses(tx *gorm.DB, order models.CrmOrder) error {
	now := time.Now()
	return tx.Exec(`WITH collected AS (`+planCollectionTotalsSQL+`), states AS (
 SELECT p.id, CASE WHEN ? THEN 'cancelled'
                   WHEN p.amount > 0 AND COALESCE(c.paid,0) >= p.amount THEN 'received'
                   WHEN p.plan_date < ? THEN 'overdue' ELSE 'pending' END AS next_status
 FROM crm_receivables_plan p LEFT JOIN collected c ON c.plan_id = p.id
 WHERE p.tenant_id = ? AND p.order_id = ? AND p.deleted_at IS NULL)
 UPDATE crm_receivables_plan p SET status = s.next_status, updated_at = ?
 FROM states s WHERE p.id = s.id AND p.tenant_id = ? AND p.order_id = ? AND p.deleted_at IS NULL
 AND p.status IS DISTINCT FROM s.next_status`, order.TenantID, order.ID, order.Status == "cancelled", dayStart(now), order.TenantID, order.ID, now, order.TenantID, order.ID).Error
}
