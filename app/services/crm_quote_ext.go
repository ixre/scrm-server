package services

import (
	"context"
	"core/app/models"
	"core/common/identity"
	"errors"
	"gorm.io/gorm"
	"strings"
)

type CrmQuoteExtService struct{}

func NewCrmQuoteExt() *CrmQuoteExtService { return &CrmQuoteExtService{} }

// GenerateNo also persists a missing number when an existing quote ID is supplied.
func (s CrmQuoteExtService) GenerateNo(ctx context.Context, ids ...string) (string, error) {
	if _, _, err := customerActor(ctx, models.DB); err != nil {
		return "", err
	}
	if len(ids) == 0 || ids[0] == "" {
		return genQuoteNo(identity.TenantID(ctx)), nil
	}
	var no string
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q, err := lockSalesRecord[models.CrmQuote](ctx, tx, ids[0], false)
		if err != nil {
			return err
		}
		no = q.QuoteNo
		if no != "" {
			return nil
		}
		if q.Status != "draft" {
			return errors.New("仅草稿报价可生成单号")
		}
		no = genQuoteNo(q.TenantID)
		return tx.Model(&q).Update("quote_no", no).Error
	})
	return no, err
}
func (s CrmQuoteExtService) Void(ctx context.Context, id, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return errors.New("请填写作废原因")
	}
	return NewCrmQuote().changeStatus(ctx, id, "void", reason)
}
func (s CrmQuoteExtService) Transfer(ctx context.Context, id, toUserID string) error {
	return s.BatchTransfer(ctx, []string{id}, toUserID)
}
func (s CrmQuoteExtService) BatchTransfer(ctx context.Context, ids []string, toUserID string) error {
	ids, err := workflowIDs(ids)
	if err != nil {
		return err
	}
	if toUserID == "" {
		return errors.New("请选择接收销售")
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateCustomerOwner(ctx, tx, toUserID); err != nil {
			return err
		}
		for _, id := range ids {
			q, err := lockSalesRecord[models.CrmQuote](ctx, tx, id, false)
			if err != nil {
				return err
			}
			if q.OrderID != "" {
				return errors.New("已转合同的报价不支持单独转移")
			}
			if err = tx.Model(&q).Update("owner_user_id", toUserID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
