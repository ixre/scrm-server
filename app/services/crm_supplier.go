package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
)

// ---------- 供应商 ----------

// SupplierQueryReq 供应商查询参数。
type SupplierQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
}

// CrmSupplierService 供应商服务。
type CrmSupplierService struct{}

func NewCrmSupplier() *CrmSupplierService { return &CrmSupplierService{} }

// Create 创建供应商。
func (s CrmSupplierService) Create(ctx context.Context, sup *models.CrmSupplier) error {
	tenantID := identity.TenantID(ctx)


	if sup == nil {
		return errors.New("供应商不能为空")
	}
	if strings.TrimSpace(sup.Name) == "" {
		return errors.New("供应商名称不能为空")
	}
	sup.ID = id_generator.StringID()
	sup.TenantID = tenantID
	sup.Name = strings.TrimSpace(sup.Name)
	return createCRM(models.DB, sup)
}

// Update 更新供应商。
func (s CrmSupplierService) Update(ctx context.Context, id string, sup *models.CrmSupplier) error {


	if sup == nil {
		return errors.New("供应商不能为空")
	}
	m := map[string]any{
		"name":    sup.Name,
		"contact": sup.Contact,
		"phone":   sup.Phone,
		"address": sup.Address,
		"remark":  sup.Remark,
	}
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除供应商。
func (s CrmSupplierService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmSupplier{})
	return res.RowsAffected, res.Error
}

// Get 获取供应商详情。
func (s CrmSupplierService) Get(ctx context.Context, id string) (*models.CrmSupplier, error) {


	var item models.CrmSupplier
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Query 分页查询供应商。
func (s CrmSupplierService) Query(ctx context.Context, req SupplierQueryReq) ([]models.CrmSupplier, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR contact LIKE ? OR phone LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmSupplier{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSupplier
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- 采购订单 ----------

// PurchaseOrderQueryReq 采购订单查询参数。
type PurchaseOrderQueryReq struct {
	Page       int
	PageSize   int
	SupplierID string
	Status     string
	Keyword    string
}

// CrmPurchaseOrderService 采购订单服务。
type CrmPurchaseOrderService struct{}

func NewCrmPurchaseOrder() *CrmPurchaseOrderService { return &CrmPurchaseOrderService{} }

// Create 创建采购订单。
func (s CrmPurchaseOrderService) Create(ctx context.Context, o *models.CrmPurchaseOrder) error {
	tenantID := identity.TenantID(ctx)


	if o == nil {
		return errors.New("采购订单不能为空")
	}
	if strings.TrimSpace(o.No) == "" {
		return errors.New("采购单号不能为空")
	}
	if o.SupplierID == "" {
		return errors.New("supplier_id 不能为空")
	}
	o.ID = id_generator.StringID()
	o.TenantID = tenantID
	o.No = strings.TrimSpace(o.No)
	if o.Status == "" {
		o.Status = "draft"
	}
	return createCRM(models.DB, o, "SupplierID", "WarehouseID")
}

// Update 更新采购订单。
func (s CrmPurchaseOrderService) Update(ctx context.Context, id string, o *models.CrmPurchaseOrder) error {


	if o == nil {
		return errors.New("采购订单不能为空")
	}
	m := map[string]any{
		"no":           o.No,
		"supplier_id":  o.SupplierID,
		"warehouse_id": o.WarehouseID,
		"amount":       o.Amount,
		"paid_amount":  o.PaidAmount,
		"status":       o.Status,
		"order_date":   o.OrderDate,
		"remark":       o.Remark,
	}
	updatesCRM(m, "supplier_id", "warehouse_id")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除采购订单。
func (s CrmPurchaseOrderService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmPurchaseOrder{})
	return res.RowsAffected, res.Error
}

// Get 获取采购订单详情。
func (s CrmPurchaseOrderService) Get(ctx context.Context, id string) (*models.CrmPurchaseOrder, error) {


	var item models.CrmPurchaseOrder
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Query 分页查询采购订单。
func (s CrmPurchaseOrderService) Query(ctx context.Context, req PurchaseOrderQueryReq) ([]models.CrmPurchaseOrder, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.SupplierID != "" {
		db = db.Where("supplier_id = ?", req.SupplierID)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("no LIKE ?", like)
	}
	var total int64
	if err := db.Model(&models.CrmPurchaseOrder{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmPurchaseOrder
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 变更采购订单状态。
func (s CrmPurchaseOrderService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	if status == "" {
		return errors.New("状态不能为空")
	}
	return models.DB.Model(&models.CrmPurchaseOrder{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Update("status", status).Error
}
