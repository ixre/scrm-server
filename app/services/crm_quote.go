package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
)

type CrmQuoteService struct{}

func NewCrmQuote() *CrmQuoteService { return &CrmQuoteService{} }

type QuoteQueryReq struct {
	Page          int    `json:"page"`
	PageSize      int    `json:"page_size"`
	CustomerID    string `json:"customer_id"`
	OpportunityID string `json:"opportunity_id"`
	Status        string `json:"status"`
	Keyword       string `json:"keyword"`
}

var quoteStatusFlow = map[string]map[string]bool{
	"draft":    {"sent": true, "void": true},
	"sent":     {"accepted": true, "rejected": true, "void": true},
	"accepted": {}, "rejected": {}, "void": {},
}

// Snowflake IDs avoid duplicate numbers from concurrent creation or deleted rows.
func genQuoteNo(_ string) string {
	return "Q" + time.Now().Format("20060102") + id_generator.StringID()
}

func validateQuote(ctx context.Context, tx *gorm.DB, q *models.CrmQuote) error {
	if q.ContactID != "" {
		if err := contactWriteLock(ctx, tx); err != nil {
			return err
		}
	}
	q.Subject = strings.TrimSpace(q.Subject)
	if q.Subject == "" {
		return errors.New("请填写报价主题")
	}
	if _, err := activeSalesCustomer(ctx, tx, q.CustomerID); err != nil {
		return err
	}
	if err := validateCustomerOwner(ctx, tx, q.OwnerUserID); err != nil {
		return err
	}
	amount, err := moneyCents(q.Amount)
	if err != nil {
		return err
	}
	discount, err := moneyCents(q.Discount)
	if err != nil {
		return err
	}
	if discount > amount {
		return errors.New("优惠金额不能超过报价金额")
	}
	q.FinalAmount = float64(amount-discount) / 100
	if q.OpportunityID != "" {
		db, err := salesAccessDB(ctx, tx, false)
		if err != nil {
			return err
		}
		var o models.CrmOpportunity
		if err = db.Where("id = ? AND customer_id = ?", q.OpportunityID, q.CustomerID).First(&o).Error; err != nil {
			return errors.New("商机不存在、无权操作或不属于报价客户")
		}
	}
	if q.ContactID != "" {
		var c models.CrmContact
		if err := tx.Where("tenant_id = ? AND id = ? AND customer_id = ?", q.TenantID, q.ContactID, q.CustomerID).First(&c).Error; err != nil {
			return errors.New("联系人不属于报价客户")
		}
	}
	return nil
}

func (s CrmQuoteService) Create(ctx context.Context, q *models.CrmQuote) error {
	if q == nil {
		return errors.New("报价单不能为空")
	}
	if q.Status != "" && q.Status != "draft" {
		return errors.New("新报价必须先保存为草稿")
	}
	q.ID, q.TenantID, q.Status = id_generator.StringID(), identity.TenantID(ctx), "draft"
	q.OrderID, q.VoidReason, q.VoidAt = "", "", nil
	if q.OwnerUserID == "" {
		q.OwnerUserID = identity.UserID(ctx)
	}
	q.QuoteNo = strings.TrimSpace(q.QuoteNo)
	if q.QuoteNo == "" {
		q.QuoteNo = genQuoteNo(q.TenantID)
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := validateQuote(ctx, tx, q); err != nil {
			return err
		}
		if err := lockSalesKey(ctx, tx, "quote-number", q.QuoteNo); err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&models.CrmQuote{}).Where("tenant_id = ? AND quote_no = ?", q.TenantID, q.QuoteNo).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("报价单号已存在")
		}
		return createCRM(tx, q)
	})
}

func (s CrmQuoteService) Update(ctx context.Context, id string, q *models.CrmQuote) error {
	if q == nil {
		return errors.New("报价单不能为空")
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old, err := lockSalesRecord[models.CrmQuote](ctx, tx, id, false)
		if err != nil {
			return err
		}
		if old.Status != "draft" {
			return errors.New("仅草稿报价可以编辑")
		}
		q.ID, q.TenantID, q.QuoteNo, q.Status, q.OrderID = old.ID, old.TenantID, old.QuoteNo, old.Status, old.OrderID
		if q.OwnerUserID == "" {
			q.OwnerUserID = old.OwnerUserID
		}
		if err := validateQuote(ctx, tx, q); err != nil {
			return err
		}
		changes := map[string]any{"customer_id": q.CustomerID, "contact_id": nullableSalesID(q.ContactID), "opportunity_id": nullableSalesID(q.OpportunityID), "subject": q.Subject, "amount": q.Amount, "discount": q.Discount, "final_amount": q.FinalAmount, "items": q.Items, "valid_until": q.ValidUntil, "owner_user_id": q.OwnerUserID, "remark": q.Remark}
		return tx.Model(&old).Updates(changes).Error
	})
}

func nullableSalesID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func (s CrmQuoteService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			q, err := lockSalesRecord[models.CrmQuote](ctx, tx, id, false)
			if err != nil {
				return err
			}
			if q.Status != "draft" || q.OrderID != "" {
				return errors.New("仅未转合同的草稿报价可以删除")
			}
			if err := tx.Delete(&q).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func (s CrmQuoteService) Get(ctx context.Context, id string) (*models.CrmQuote, error) {
	db, err := salesAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, err
	}
	var q models.CrmQuote
	err = db.Where("id = ?", id).First(&q).Error
	return &q, err
}

func (s CrmQuoteService) Query(ctx context.Context, req QuoteQueryReq) ([]models.CrmQuote, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	db, err := salesAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if req.OpportunityID != "" {
		db = db.Where("opportunity_id = ?", req.OpportunityID)
	}
	if req.Status != "" {
		db = db.Where("status = ?", req.Status)
	}
	if req.Keyword != "" {
		like := "%" + strings.TrimSpace(req.Keyword) + "%"
		db = db.Where("subject LIKE ? OR quote_no LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmQuote{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmQuote
	err = db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmQuoteService) ChangeStatus(ctx context.Context, id, status string) error {
	if status == "void" {
		return errors.New("请使用作废操作并填写原因")
	}
	return s.changeStatus(ctx, id, status, "")
}

func (s CrmQuoteService) changeStatus(ctx context.Context, id, status, reason string) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q, err := lockSalesRecord[models.CrmQuote](ctx, tx, id, false)
		if err != nil {
			return err
		}
		if q.Status == status {
			return nil
		}
		if !quoteStatusFlow[q.Status][status] {
			return fmt.Errorf("当前报价状态不允许此操作（%s → %s）", q.Status, status)
		}
		if status == "sent" || status == "accepted" {
			if q.ValidUntil != nil && q.ValidUntil.Before(dayStart(time.Now())) {
				return errors.New("报价已过有效期，请重新报价")
			}
			final := q.FinalAmount
			if err = validateQuote(ctx, tx, &q); err != nil {
				return err
			}
			if final != q.FinalAmount {
				return errors.New("报价金额不一致，请先修正草稿或重新报价")
			}
		}
		changes := map[string]any{"status": status}
		if status == "void" {
			changes["void_reason"], changes["void_at"] = reason, time.Now()
		}
		return tx.Model(&q).Updates(changes).Error
	})
}

func (s CrmQuoteService) Transfer(ctx context.Context, id, toUserID string) error {
	return NewCrmQuoteExt().BatchTransfer(ctx, []string{id}, toUserID)
}

// ConvertToOrder creates exactly one draft contract and persists the backlink in the same transaction.
func (s CrmQuoteService) ConvertToOrder(ctx context.Context, id string) (*models.CrmOrder, error) {
	var order models.CrmOrder
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q, err := lockSalesRecord[models.CrmQuote](ctx, tx, id, false)
		if err != nil {
			return err
		}
		if q.OrderID != "" {
			order, err = lockPaymentOrder(ctx, tx, q.OrderID)
			return err
		}
		if q.Status != "accepted" {
			return errors.New("仅客户已接受的报价可以转合同")
		}
		final := q.FinalAmount
		if err = validateQuote(ctx, tx, &q); err != nil {
			return err
		}
		if final != q.FinalAmount {
			return errors.New("历史报价金额不一致，请核对后重新报价")
		}
		order = newOrder(ctx, q.OwnerUserID, models.CrmOrder{Title: q.Subject, CustomerID: q.CustomerID, OpportunityID: q.OpportunityID, Amount: q.FinalAmount, DiscountAmount: q.Discount, Remark: "来源报价：" + q.QuoteNo})
		if err = validateOrder(ctx, tx, &order); err != nil {
			return err
		}
		if err = createOrderWithCoupon(ctx, tx, &order, ""); err != nil {
			return err
		}
		return tx.Model(&q).Update("order_id", order.ID).Error
	})
	if err != nil {
		return nil, err
	}
	return &order, nil
}
