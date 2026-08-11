package services

import (
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// ---------- 产品档案 ----------

// ProductQueryReq 产品查询参数。
type ProductQueryReq struct {
	Page     int
	PageSize int
	Category string
	Status   string
	Keyword  string
}

type CrmProductService struct{}

func NewCrmProduct() *CrmProductService { return &CrmProductService{} }

// Create 新增产品档案。
func (s CrmProductService) Create(tenantID string, p *models.CrmProduct) error {
	if p == nil {
		return errors.New("产品不能为空")
	}
	if p.Name == "" {
		return errors.New("产品名称不能为空")
	}
	p.ID = id_generator.StringID()
	p.TenantID = tenantID
	if p.Status == "" {
		p.Status = "active"
	}
	if err := models.DB.Create(p).Error; err != nil {
		log.Sugar.Errorw("product create failed", "err", err)
		return err
	}
	return nil
}

// Update 更新产品档案。
func (s CrmProductService) Update(tenantID, id string, p *models.CrmProduct) error {
	if p == nil {
		return errors.New("产品不能为空")
	}
	updates := map[string]any{
		"name":     p.Name,
		"category": p.Category,
		"spec":     p.Spec,
		"unit":     p.Unit,
		"price":    p.Price,
		"cost":     p.Cost,
		"sku":      p.SKU,
		"status":   p.Status,
		"remark":   p.Remark,
	}
	res := models.DB.Model(&models.CrmProduct{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		log.Sugar.Errorw("product update failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("产品不存在")
	}
	return nil
}

// Delete 删除产品档案。
func (s CrmProductService) Delete(tenantID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmProduct{})
	return res.RowsAffected, res.Error
}

// Get 读取单条产品档案。
func (s CrmProductService) Get(tenantID, id string) (*models.CrmProduct, error) {
	var item models.CrmProduct
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Query 分页查询产品档案，支持按 category/status/keyword 筛选。
func (s CrmProductService) Query(tenantID string, req ProductQueryReq) ([]models.CrmProduct, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.DB.Scopes(models.TenantScope(tenantID))
	if req.Category != "" {
		db = db.Where("category = ?", req.Category)
	}
	if status := strings.TrimSpace(req.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR sku LIKE ? OR spec LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmProduct{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmProduct
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- 仓库 ----------

// WarehouseQueryReq 仓库查询参数。
type WarehouseQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
	Status   string
}

type CrmWarehouseService struct{}

func NewCrmWarehouse() *CrmWarehouseService { return &CrmWarehouseService{} }

// Create 新增仓库。
func (s CrmWarehouseService) Create(tenantID string, w *models.CrmWarehouse) error {
	if w == nil {
		return errors.New("仓库不能为空")
	}
	if w.Name == "" {
		return errors.New("仓库名称不能为空")
	}
	w.ID = id_generator.StringID()
	w.TenantID = tenantID
	if w.Status == "" {
		w.Status = "active"
	}
	if err := models.DB.Create(w).Error; err != nil {
		log.Sugar.Errorw("warehouse create failed", "err", err)
		return err
	}
	return nil
}

// Update 更新仓库。
func (s CrmWarehouseService) Update(tenantID, id string, w *models.CrmWarehouse) error {
	if w == nil {
		return errors.New("仓库不能为空")
	}
	updates := map[string]any{
		"name":    w.Name,
		"address": w.Address,
		"manager": w.Manager,
		"status":  w.Status,
	}
	res := models.DB.Model(&models.CrmWarehouse{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		log.Sugar.Errorw("warehouse update failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("仓库不存在")
	}
	return nil
}

// Delete 删除仓库。
func (s CrmWarehouseService) Delete(tenantID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmWarehouse{})
	return res.RowsAffected, res.Error
}

// Query 分页查询仓库。
func (s CrmWarehouseService) Query(tenantID string, req WarehouseQueryReq) ([]models.CrmWarehouse, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.DB.Scopes(models.TenantScope(tenantID))
	if status := strings.TrimSpace(req.Status); status != "" {
		db = db.Where("status = ?", status)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		db = db.Where("name LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := db.Model(&models.CrmWarehouse{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmWarehouse
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- 库存流水 ----------

// StockRecordQueryReq 库存流水查询参数。
type StockRecordQueryReq struct {
	Page        int
	PageSize    int
	WarehouseID string
	ProductID   string
	Type        string
}

type CrmStockRecordService struct{}

func NewCrmStockRecord() *CrmStockRecordService { return &CrmStockRecordService{} }

// Create 新增库存流水记录。
func (s CrmStockRecordService) Create(tenantID string, r *models.CrmStockRecord) error {
	if r == nil {
		return errors.New("库存记录不能为空")
	}
	if r.WarehouseID == "" || r.ProductID == "" {
		return errors.New("仓库ID和产品ID不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	if err := models.DB.Create(r).Error; err != nil {
		log.Sugar.Errorw("stock record create failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除库存流水记录。
func (s CrmStockRecordService) Delete(tenantID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmStockRecord{})
	return res.RowsAffected, res.Error
}

// Query 分页查询库存流水，支持按 warehouse_id/product_id/type 筛选。
func (s CrmStockRecordService) Query(tenantID string, req StockRecordQueryReq) ([]models.CrmStockRecord, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.DB.Scopes(models.TenantScope(tenantID))
	if req.WarehouseID != "" {
		db = db.Where("warehouse_id = ?", req.WarehouseID)
	}
	if req.ProductID != "" {
		db = db.Where("product_id = ?", req.ProductID)
	}
	if req.Type != "" {
		db = db.Where("type = ?", req.Type)
	}
	var total int64
	if err := db.Model(&models.CrmStockRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmStockRecord
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("id desc").Find(&items).Error
	return items, total, err
}

// ListByWarehouse 按仓库列出库存流水（不分页）。
func (s CrmStockRecordService) ListByWarehouse(tenantID, warehouseID string) ([]models.CrmStockRecord, error) {
	var items []models.CrmStockRecord
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("warehouse_id = ?",  warehouseID).
		Order("id desc").Find(&items).Error
	return items, err
}
