package services

import (
	"context"
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QueryReq struct {
	Page, PageSize                  int
	CustomerID, Keyword, FilterMode string
}
type CrmContactService struct{}

func NewCrmContact() *CrmContactService { return &CrmContactService{} }

type ContactWithCustomerName struct {
	models.CrmContact
	CustomerName string `json:"customer_name"`
}

func contactAccessDB(ctx context.Context, tx *gorm.DB, write bool) (*gorm.DB, error) {
	customers, err := customerAccessDB(ctx, tx, write)
	if err != nil {
		return nil, err
	}
	// A contact inherits its customer's scope; unlinked contacts form a shared tenant pool.
	return tx.WithContext(ctx).Model(&models.CrmContact{}).Where("crm_contact.tenant_id = ?", identity.TenantID(ctx)).Where("(crm_contact.customer_id IS NULL OR crm_contact.customer_id = 0 OR crm_contact.customer_id IN (?))", customers.Model(&models.CrmCustomer{}).Select("id")), nil
}
func contactWriteLock(ctx context.Context, tx *gorm.DB) error {
	// Serializes primary-contact changes and moves within a tenant, including the unlinked pool.
	return lockSalesKey(ctx, tx, "contact-directory", "write")
}
func lockContact(ctx context.Context, tx *gorm.DB, id string) (models.CrmContact, error) {
	var c models.CrmContact
	if id == "" {
		return c, errors.New("请选择联系人")
	}
	db, err := contactAccessDB(ctx, tx, true)
	if err != nil {
		return c, err
	}
	err = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("crm_contact.id = ?", id).First(&c).Error
	if err != nil {
		return c, errors.New("联系人不存在或无权操作")
	}
	return c, nil
}
func validateContact(ctx context.Context, tx *gorm.DB, c *models.CrmContact) error {
	c.Name, c.Phone = strings.TrimSpace(c.Name), strings.TrimSpace(c.Phone)
	if c.Name == "" {
		return errors.New("请填写联系人姓名")
	}
	if c.CustomerID == "0" {
		c.CustomerID = ""
	}
	if c.Gender < 0 || c.Gender > 2 {
		return errors.New("性别选项无效")
	}
	if c.CustomerID == "" {
		c.IsPrimary = false
		return nil
	}
	_, err := activeSalesCustomer(ctx, tx, c.CustomerID)
	return err
}
func setPrimaryContact(tx *gorm.DB, c models.CrmContact) error {
	if !c.IsPrimary || c.CustomerID == "" {
		return nil
	}
	return tx.Model(&models.CrmContact{}).Where("tenant_id = ? AND customer_id = ? AND id <> ? AND is_primary = true", c.TenantID, c.CustomerID, c.ID).Update("is_primary", false).Error
}
func contactHasHistory(tx *gorm.DB, c models.CrmContact) error {
	for _, model := range []any{&models.CrmFollowup{}, &models.CrmQuote{}} {
		var count int64
		if err := tx.Model(model).Where("tenant_id = ? AND contact_id = ?", c.TenantID, c.ID).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("联系人已有跟进或报价记录，不能删除或更换所属客户")
		}
	}
	return nil
}
func (s CrmContactService) Create(ctx context.Context, c *models.CrmContact) error {
	if c == nil {
		return errors.New("联系人不能为空")
	}
	c.ID, c.TenantID = id_generator.StringID(), identity.TenantID(ctx)
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, _, err := customerActor(ctx, tx); err != nil {
			return err
		}
		if err := contactWriteLock(ctx, tx); err != nil {
			return err
		}
		if err := validateContact(ctx, tx, c); err != nil {
			return err
		}
		if err := setPrimaryContact(tx, *c); err != nil {
			return err
		}
		return createCRM(tx, c)
	})
}
func (s CrmContactService) Update(ctx context.Context, id string, c *models.CrmContact) error {
	if c == nil {
		return errors.New("联系人不能为空")
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := contactWriteLock(ctx, tx); err != nil {
			return err
		}
		old, err := lockContact(ctx, tx, id)
		if err != nil {
			return err
		}
		c.ID, c.TenantID = old.ID, old.TenantID
		if err = validateContact(ctx, tx, c); err != nil {
			return err
		}
		if c.CustomerID != old.CustomerID {
			if err = contactHasHistory(tx, old); err != nil {
				return err
			}
		}
		if err = setPrimaryContact(tx, *c); err != nil {
			return err
		}
		return tx.Model(&old).Updates(map[string]any{
			"customer_id": nullableSalesID(c.CustomerID), "name": c.Name, "title": c.Title, "phone": c.Phone, "email": c.Email, "wechat": c.Wechat, "is_primary": c.IsPrimary, "is_decision_maker": c.IsDecisionMaker, "remark": c.Remark, "department": c.Department, "address": c.Address, "province": c.Province, "city": c.City, "district": c.District, "birthday": c.Birthday, "gender": c.Gender,
		}).Error
	})
}
func (s CrmContactService) Delete(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := contactWriteLock(ctx, tx); err != nil {
			return err
		}
		for _, id := range ids {
			c, err := lockContact(ctx, tx, id)
			if err != nil {
				return err
			}
			if err = contactHasHistory(tx, c); err != nil {
				return err
			}
			if err = recycleEntity(ctx, tx, "contact", c.ID, c.Name); err != nil {
				return err
			}
			if err = tx.Delete(&c).Error; err != nil {
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
func (s CrmContactService) Get(ctx context.Context, id string) (*models.CrmContact, error) {
	db, err := contactAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, err
	}
	var c models.CrmContact
	if err = db.Where("crm_contact.id = ?", id).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}
func (s CrmContactService) Query(ctx context.Context, req QueryReq) ([]ContactWithCustomerName, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 500 {
		req.PageSize = 500
	}
	db, err := contactAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	db = db.Joins("LEFT JOIN crm_customer ON crm_customer.id = crm_contact.customer_id AND crm_customer.tenant_id = crm_contact.tenant_id AND crm_customer.deleted_at IS NULL")
	if req.CustomerID != "" {
		db = db.Where("crm_contact.customer_id = ?", req.CustomerID)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("crm_contact.name LIKE ? OR crm_contact.phone LIKE ? OR crm_contact.email LIKE ? OR crm_contact.wechat LIKE ?", like, like, like, like)
	}
	switch req.FilterMode {
	case "linked":
		db = db.Where("crm_contact.customer_id IS NOT NULL AND crm_contact.customer_id <> 0")
	case "pool":
		db = db.Where("(crm_contact.customer_id IS NULL OR crm_contact.customer_id = 0)")
	}
	var total int64
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []ContactWithCustomerName
	err = db.Select("crm_contact.*, crm_customer.name AS customer_name").Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("crm_contact.id desc").Find(&items).Error
	return items, total, err
}
func (s CrmContactService) ListByCustomer(ctx context.Context, customerID string) ([]models.CrmContact, error) {
	if _, err := NewCrmCustomer().Get(ctx, customerID); err != nil {
		return nil, err
	}
	db, err := contactAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, err
	}
	var items []models.CrmContact
	err = db.Where("customer_id = ?", customerID).Order("is_primary desc, id desc").Find(&items).Error
	return items, err
}
