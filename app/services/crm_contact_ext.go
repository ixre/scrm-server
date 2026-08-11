package services

import (
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// CrmContactExtService 联系人扩展服务（转客户 / 联系人池）。
type CrmContactExtService struct{}

func NewCrmContactExt() *CrmContactExtService { return &CrmContactExtService{} }

// ConvertToCustomer 联系人转客户：创建 CrmCustomer（姓名/电话/邮箱），回写联系人 CustomerID。
// 幂等：联系人已关联客户则直接返回已有客户。
func (s CrmContactExtService) ConvertToCustomer(tenantID, userID, contactID string) (*models.CrmCustomer, error) {
	if contactID == "" {
		return nil, errors.New("contact_id 不能为空")
	}
	var c models.CrmContact
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", contactID).First(&c).Error; err != nil {
		return nil, errors.New("联系人不存在")
	}
	// 已关联客户则直接返回。
	if c.CustomerID != "" {
		var exist models.CrmCustomer
		if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", c.CustomerID).First(&exist).Error; err == nil {
			return &exist, nil
		}
	}
	// 去重：同手机号已有客户则直接关联。
	if c.Phone != "" {
		var exist models.CrmCustomer
		if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("mobile = ?",  c.Phone).First(&exist).Error; err == nil {
			models.DB.Model(&models.CrmContact{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", contactID).Update("customer_id", exist.ID)
			return &exist, nil
		}
	}
	cust := models.CrmCustomer{
		Model:    models.Model{ID: id_generator.StringID()},
		TenantID: tenantID,
		Name:     strings.TrimSpace(c.Name),
		Mobile:   c.Phone,
		Email:    c.Email,
		Company:  c.Department,
		Position: c.Title,
		Source:   "manual",
		Level:    1,
	}
	if cust.Name == "" {
		cust.Name = c.Phone
	}
	if err := createCRM(models.DB, &cust, "OwnerUserID", "ReassignFrom"); err != nil {
		log.Sugar.Errorw("contact convert: create customer failed", "err", err)
		return nil, err
	}
	if err := models.DB.Model(&models.CrmContact{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", contactID).Update("customer_id", cust.ID).Error; err != nil {
		log.Sugar.Errorw("contact convert: write back customer_id failed", "err", err)
		return &cust, err
	}
	return &cust, nil
}

// ListPool 联系人池：CustomerID 为空的联系人。
func (s CrmContactExtService) ListPool(tenantID string, req PageReq) ([]models.CrmContact, int64, error) {
	p := req.pager()
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("(customer_id IS NULL OR customer_id = '0')", )
	var total int64
	if err := db.Model(&models.CrmContact{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmContact
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("created_at desc, id desc").Find(&items).Error
	return items, total, err
}
