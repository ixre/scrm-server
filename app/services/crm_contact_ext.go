package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
)

type CrmContactExtService struct{}

func NewCrmContactExt() *CrmContactExtService { return &CrmContactExtService{} }

func (s CrmContactExtService) ConvertToCustomer(ctx context.Context, userID, contactID string) (*models.CrmCustomer, error) {
	if userID == "" || userID != identity.UserID(ctx) {
		return nil, ErrCustomerAccess
	}
	var customer models.CrmCustomer
	created := false
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := contactWriteLock(ctx, tx); err != nil {
			return err
		}
		c, err := lockContact(ctx, tx, contactID)
		if err != nil {
			return err
		}
		if c.CustomerID != "" && c.CustomerID != "0" {
			db, err := customerAccessDB(ctx, tx, false)
			if err != nil {
				return err
			}
			return db.Where("id = ?", c.CustomerID).First(&customer).Error
		}
		phone := strings.TrimSpace(c.Phone)
		if phone != "" {
			// Share the same logical key with lead conversion.
			if err = lockSalesKey(ctx, tx, "lead-conversion-mobile", phone); err != nil {
				return err
			}
			var matches []models.CrmCustomer
			if err = tx.Where("tenant_id = ? AND mobile = ?", c.TenantID, phone).Limit(2).Find(&matches).Error; err != nil {
				return err
			}
			if len(matches) > 1 {
				return errors.New("手机号存在重复客户，请先合并")
			}
			if len(matches) == 1 {
				customer, err = activeSalesCustomer(ctx, tx, matches[0].ID)
				if err != nil {
					return errors.New("该手机号已有客户，请联系负责人或先领取公海客户")
				}
			}
		}
		if customer.ID == "" {
			now := time.Now()
			customer = models.CrmCustomer{Model: models.Model{ID: id_generator.StringID()}, TenantID: c.TenantID, Name: strings.TrimSpace(c.Name), Mobile: phone, Email: c.Email, Position: c.Title, Address: c.Address, Province: c.Province, City: c.City, District: c.District, Source: "manual", OwnerUserID: userID, ClaimedAt: &now, Level: 1}
			if customer.Name == "" {
				customer.Name = phone
			}
			if customer.Name == "" {
				return errors.New("请先完善联系人姓名或手机号")
			}
			if err = createCRM(tx, &customer); err != nil {
				return err
			}
			created = true
		}
		var primaries int64
		if err = tx.Model(&models.CrmContact{}).Where("tenant_id = ? AND customer_id = ? AND is_primary = true", c.TenantID, customer.ID).Count(&primaries).Error; err != nil {
			return err
		}
		return tx.Model(&c).Updates(map[string]any{"customer_id": customer.ID, "is_primary": primaries == 0}).Error
	})
	if err != nil {
		return nil, err
	}
	if created {
		NewCrmSop().Trigger(ctx, "new_customer", map[string]string{"customer_id": customer.ID, "source": customer.Source})
	}
	return &customer, nil
}
func (s CrmContactExtService) ListPool(ctx context.Context, req PageReq) ([]models.CrmContact, int64, error) {
	p := req.pager()
	rows, total, err := NewCrmContact().Query(ctx, QueryReq{Page: p.Page, PageSize: p.PageSize, FilterMode: "pool"})
	items := make([]models.CrmContact, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.CrmContact)
	}
	return items, total, err
}
