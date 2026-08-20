package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
)

// QueryReq 联系人查询参数。
type QueryReq struct {
	Page       int
	PageSize   int
	CustomerID string
	Keyword    string
}

type CrmContactService struct{}

func NewCrmContact() *CrmContactService { return &CrmContactService{} }

func (s CrmContactService) Create(ctx context.Context, c *models.CrmContact) error {
	tenantID := identity.TenantID(ctx)


	if c == nil {
		return errors.New("联系人不能为空")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("姓名不能为空")
	}
	if c.CustomerID == "" {
		return errors.New("customer_id 不能为空")
	}
	c.ID = id_generator.StringID()
	c.TenantID = tenantID
	c.Name = strings.TrimSpace(c.Name)
	return createCRM(models.DB, c, "CustomerID")
}

func (s CrmContactService) Update(ctx context.Context, id string, c *models.CrmContact) error {


	if c == nil {
		return errors.New("联系人不能为空")
	}
	m := map[string]any{
		"customer_id":        c.CustomerID,
		"name":               c.Name,
		"title":              c.Title,
		"phone":              c.Phone,
		"email":              c.Email,
		"wechat":             c.Wechat,
		"is_primary":         c.IsPrimary,
		"is_decision_maker":  c.IsDecisionMaker,
		"remark":             c.Remark,
	}
	updatesCRM(m, "CustomerID")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

func (s CrmContactService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmContact{})
	return res.RowsAffected, res.Error
}

func (s CrmContactService) Get(ctx context.Context, id string) (*models.CrmContact, error) {


	var item models.CrmContact
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s CrmContactService) Query(ctx context.Context, req QueryReq) ([]models.CrmContact, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR phone LIKE ? OR email LIKE ? OR wechat LIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmContact{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmContact
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmContactService) ListByCustomer(ctx context.Context, customerID string) ([]models.CrmContact, error) {


	var items []models.CrmContact
	err := models.MustFor(ctx).Where("customer_id = ?",  customerID).
		Order("is_primary desc, id desc").Find(&items).Error
	return items, err
}
