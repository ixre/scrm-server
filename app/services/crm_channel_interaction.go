package services

import (
	"context"
	"errors"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Manual interaction writes use the same customer ownership rules as follow-ups.
func (s CrmChannelInteractionService) Create(ctx context.Context, userID string, req models.CrmChannelInteraction) (models.CrmChannelInteraction, error) {
	if userID == "" || userID != identity.UserID(ctx) {
		return req, ErrCustomerAccess
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := activeSalesCustomer(ctx, tx, req.CustomerID); err != nil {
			return err
		}
		req.ID, req.TenantID = id_generator.StringID(), identity.TenantID(ctx)
		req.Timestamp = models.Timestamp{}
		req.EventKey = ""
		if req.Channel == "" {
			req.Channel = constants.ChannelGeneral
		}
		if req.ActionType == "" {
			req.ActionType = "other"
		}
		if req.HappenedAt == nil {
			now := time.Now()
			req.HappenedAt = &now
		}
		return createCRM(tx, &req)
	})
	return req, err
}

func (s CrmChannelInteractionService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	var count int64
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		customers, err := customerAccessDB(ctx, tx, true)
		if err != nil {
			return err
		}
		tid := identity.TenantID(ctx)
		var rows []models.CrmChannelInteraction
		if err = tx.Where("tenant_id = ? AND id IN ?", tid, ids).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return ErrCustomerAccess
		}
		customerIDs := map[string]bool{}
		for _, row := range rows {
			customerIDs[row.CustomerID] = true
		}
		keys := make([]string, 0, len(customerIDs))
		for key := range customerIDs {
			keys = append(keys, key)
		}
		// Parent locks precede child writes, consistent with customer merge/transfer.
		var allowed []models.CrmCustomer
		if err = customers.Clauses(clause.Locking{Strength: "SHARE"}).Where("id IN ?", keys).Order("id").Find(&allowed).Error; err != nil {
			return err
		}
		if len(allowed) != len(keys) {
			return ErrCustomerAccess
		}
		res := tx.Where("tenant_id = ? AND id IN ? AND customer_id IN ?", tid, ids, keys).Delete(&models.CrmChannelInteraction{})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != int64(len(ids)) {
			return errors.New("互动记录已变化，请刷新后重试")
		}
		count = res.RowsAffected
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s CrmChannelInteractionService) Query(ctx context.Context, customerID, channel string, pager *app.Pager) ([]models.CrmChannelInteraction, int64, error) {
	customers, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	if customerID != "" {
		if _, err := NewCrmCustomer().Get(ctx, customerID); err != nil {
			return nil, 0, err
		}
	}
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.WithContext(ctx).Model(&models.CrmChannelInteraction{}).
		Where("tenant_id = ? AND customer_id IN (?)", identity.TenantID(ctx), customers.Model(&models.CrmCustomer{}).Select("id"))
	if customerID != "" {
		db = db.Where("customer_id = ?", customerID)
	}
	if channel != "" {
		db = db.Where("channel = ?", channel)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.CrmChannelInteraction
	err = db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("happened_at desc, id desc").Find(&rows).Error
	return rows, total, err
}
