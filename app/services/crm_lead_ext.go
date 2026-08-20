package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// LeadPoolQueryReq 线索池查询参数。
type LeadPoolQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
}

func (r LeadPoolQueryReq) pager() *app.Pager {
	p := &app.Pager{Page: r.Page, PageSize: r.PageSize}
	p.SetDefault()
	return p
}

type CrmLeadExtService struct{}

func NewCrmLeadExt() *CrmLeadExtService { return &CrmLeadExtService{} }

// MoveToPool 批量入线索池：PoolStatus=1, PoolAt=now。
func (s CrmLeadExtService) MoveToPool(ctx context.Context, ids []string) error {
	tenantID := identity.TenantID(ctx)


	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Updates(map[string]any{"pool_status": 1, "pool_at": now, "owner_user_id": ""})
	if res.Error != nil {
		log.Sugar.Errorw("crm lead move to pool failed", "err", res.Error)
	}
	return res.Error
}

// ReceiveFromPool 从线索池领取：PoolStatus=0, OwnerUserID=userID。
func (s CrmLeadExtService) ReceiveFromPool(ctx context.Context, userID, id string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND pool_status = ?",  id,  1).
		Updates(map[string]any{"pool_status": 0, "pool_at": nil, "owner_user_id": userID})
	if res.Error != nil {
		log.Sugar.Errorw("crm lead receive from pool failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListPool 线索池列表（PoolStatus=1）。
func (s CrmLeadExtService) ListPool(ctx context.Context, req LeadPoolQueryReq) ([]models.CrmLead, int64, error) {


	p := req.pager()
	db := models.MustFor(ctx).Where("pool_status = ?",  1)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("title LIKE ? OR name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmLead{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmLead
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("pool_at desc, id desc").Find(&items).Error
	return items, total, err
}

// Transfer 线索转移：改 OwnerUserID。
func (s CrmLeadExtService) Transfer(ctx context.Context, id, toUserID string) error {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return errors.New("id 不能为空")
	}
	if toUserID == "" {
		return errors.New("to_user_id 不能为空")
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Update("owner_user_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("crm lead transfer failed", "err", res.Error)
	}
	return res.Error
}

// ConvertToCustomer 线索转客户：创建 CrmCustomer，更新 CrmLead 的 ConvertCustomerID/ConvertAt/Status。
// 幂等：线索已转化（ConvertCustomerID 非空）则直接返回已有客户。
func (s CrmLeadExtService) ConvertToCustomer(ctx context.Context, userID, id, remark string) (*models.CrmCustomer, error) {
	tenantID := identity.TenantID(ctx)


	if id == "" {
		return nil, errors.New("id 不能为空")
	}
	var l models.CrmLead
	if err := models.MustFor(ctx).Where("id = ?", id).First(&l).Error; err != nil {
		return nil, err
	}
	// 已转化则直接返回已有客户。
	if l.ConvertCustomerID != "" {
		var c models.CrmCustomer
		if err := models.MustFor(ctx).Where("id = ?", l.ConvertCustomerID).First(&c).Error; err != nil {
			return nil, err
		}
		return &c, nil
	}
	// 去重：同手机号已有客户则直接关联。
	if l.Mobile != "" {
		var exist models.CrmCustomer
		if err := models.MustFor(ctx).Where("mobile = ?",  l.Mobile).First(&exist).Error; err == nil {
			now := time.Now()
			models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
				Updates(map[string]any{"convert_customer_id": exist.ID, "convert_at": now, "status": "qualified", "convert_remark": remark})
			return &exist, nil
		}
	}
	cust := models.CrmCustomer{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		Name:        l.Name,
		Company:     l.Company,
		Mobile:      l.Mobile,
		Email:       l.Email,
		Position:    l.Position,
		Address:     l.Address,
		Source:      l.Source,
		OwnerUserID: userID,
		Level:       1,
	}
	if err := createCRM(models.DB, &cust, "OwnerUserID", "ReassignFrom", "OpportunityID", "LeadID"); err != nil {
		log.Sugar.Errorw("crm lead convert: create customer failed", "err", err)
		return nil, err
	}
	now := time.Now()
	if err := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Updates(map[string]any{"convert_customer_id": cust.ID, "convert_at": now, "status": "qualified", "convert_remark": remark}).Error; err != nil {
		return &cust, err
	}
	return &cust, nil
}
