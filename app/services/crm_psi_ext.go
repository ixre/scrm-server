package services

import (
	"core/common/identity"
	
	"context"
	
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// psiItem 进销存单据明细项（入库/出库/调拨通用）。
type psiItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Spec      string  `json:"spec"`
	Qty       int     `json:"qty"`
	Price     float64 `json:"price"`
	Amount    float64 `json:"amount"`
}

// stockCheckItem 盘点明细项。
type stockCheckItem struct {
	ProductID string `json:"product_id"`
	Name      string `json:"name"`
	BookQty   int    `json:"book_qty"`
	ActualQty int    `json:"actual_qty"`
	Diff      int    `json:"diff"`
}

// parsePSIItems 解析 StringArrayField 明细。
func parsePSIItems(items constants.StringArrayField) ([]psiItem, error) {
	var out []psiItem
	for _, s := range items {
		var it psiItem
		if err := json.Unmarshal([]byte(s), &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// parseStockCheckItems 解析盘点明细。
func parseStockCheckItems(items constants.StringArrayField) ([]stockCheckItem, error) {
	var out []stockCheckItem
	for _, s := range items {
		var it stockCheckItem
		if err := json.Unmarshal([]byte(s), &it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// adjustProductStock 调整产品库存（delta 可正可负）。
func adjustProductStock(tx *gorm.DB, tenantID, productID string, delta int) error {
	return tx.Model(&models.CrmProduct{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  productID).
		UpdateColumn("stock_qty", gorm.Expr("stock_qty + ?", delta)).Error
}

// ==================== 入库单 ====================

// StockInQueryReq 入库单查询参数。
type StockInQueryReq struct {
	Page        int
	PageSize    int
	WarehouseID string
	Type        string
	Status      string
}

// CrmStockInService 入库单服务。
type CrmStockInService struct{}

func NewCrmStockIn() *CrmStockInService { return &CrmStockInService{} }

// genStockInNo 生成入库单号：SI + yyyyMMdd + 4位序号。
func genStockInNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "SI" + day
	var cnt int64
	models.DB.Model(&models.CrmStockIn{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?",  prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建入库单（自动生成 No）。
func (s CrmStockInService) Create(ctx context.Context, r *models.CrmStockIn) error {
	tenantID := identity.TenantID(ctx)


	if r == nil {
		return errors.New("入库单不能为空")
	}
	if r.WarehouseID == "" {
		return errors.New("仓库ID不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	r.No = genStockInNo(tenantID)
	if r.Status == "" {
		r.Status = "draft"
	}
	return createCRM(models.DB, r, "WarehouseID", "OperatorID")
}

// Update 更新入库单。
func (s CrmStockInService) Update(ctx context.Context, id string, r *models.CrmStockIn) error {


	if r == nil {
		return errors.New("入库单不能为空")
	}
	m := map[string]any{
		"warehouse_id": r.WarehouseID,
		"type":         r.Type,
		"ref_no":       r.RefNo,
		"total_amount": r.TotalAmount,
		"occur_date":   r.OccurDate,
		"operator_id":  r.OperatorID,
		"status":       r.Status,
		"remark":       r.Remark,
		"items":        r.Items,
	}
	updatesCRM(m, "warehouse_id", "operator_id")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除入库单。
func (s CrmStockInService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmStockIn{})
	return res.RowsAffected, res.Error
}

// Query 分页查询入库单（按 warehouse_id/type/status）。
func (s CrmStockInService) Query(ctx context.Context, req StockInQueryReq) ([]models.CrmStockIn, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.WarehouseID != "" {
		db = db.Where("warehouse_id = ?", req.WarehouseID)
	}
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmStockIn{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmStockIn
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改入库单状态；confirmed 时增加仓库库存。
func (s CrmStockInService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	if status != "draft" && status != "confirmed" && status != "cancelled" {
		return errors.New("无效的状态")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmStockIn
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		its, err := parsePSIItems(r.Items)
		if err != nil {
			return err
		}
		// 离开 confirmed：回滚（减少库存）
		if r.Status == "confirmed" && status != "confirmed" {
			for _, it := range its {
				if err := adjustProductStock(tx, tenantID, it.ProductID, -it.Qty); err != nil {
					return err
				}
			}
		}
		// 进入 confirmed：增加库存
		if status == "confirmed" && r.Status != "confirmed" {
			for _, it := range its {
				if err := adjustProductStock(tx, tenantID, it.ProductID, it.Qty); err != nil {
					return err
				}
			}
		}
		return tx.Model(&models.CrmStockIn{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
			Update("status", status).Error
	})
}

// ==================== 出库单 ====================

// StockOutQueryReq 出库单查询参数。
type StockOutQueryReq struct {
	Page        int
	PageSize    int
	WarehouseID string
	Type        string
	Status      string
}

// CrmStockOutService 出库单服务。
type CrmStockOutService struct{}

func NewCrmStockOut() *CrmStockOutService { return &CrmStockOutService{} }

// genStockOutNo 生成出库单号：SO + yyyyMMdd + 4位序号。
func genStockOutNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "SO" + day
	var cnt int64
	models.DB.Model(&models.CrmStockOut{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?",  prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建出库单（自动生成 No）。
func (s CrmStockOutService) Create(ctx context.Context, r *models.CrmStockOut) error {
	tenantID := identity.TenantID(ctx)


	if r == nil {
		return errors.New("出库单不能为空")
	}
	if r.WarehouseID == "" {
		return errors.New("仓库ID不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	r.No = genStockOutNo(tenantID)
	if r.Status == "" {
		r.Status = "draft"
	}
	return createCRM(models.DB, r, "WarehouseID", "OperatorID")
}

// Update 更新出库单。
func (s CrmStockOutService) Update(ctx context.Context, id string, r *models.CrmStockOut) error {


	if r == nil {
		return errors.New("出库单不能为空")
	}
	m := map[string]any{
		"warehouse_id": r.WarehouseID,
		"type":         r.Type,
		"ref_no":       r.RefNo,
		"total_amount": r.TotalAmount,
		"occur_date":   r.OccurDate,
		"operator_id":  r.OperatorID,
		"status":       r.Status,
		"remark":       r.Remark,
		"items":        r.Items,
	}
	updatesCRM(m, "warehouse_id", "operator_id")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除出库单。
func (s CrmStockOutService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmStockOut{})
	return res.RowsAffected, res.Error
}

// Query 分页查询出库单（按 warehouse_id/type/status）。
func (s CrmStockOutService) Query(ctx context.Context, req StockOutQueryReq) ([]models.CrmStockOut, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.WarehouseID != "" {
		db = db.Where("warehouse_id = ?", req.WarehouseID)
	}
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmStockOut{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmStockOut
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改出库单状态；confirmed 时减少仓库库存。
func (s CrmStockOutService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	if status != "draft" && status != "confirmed" && status != "cancelled" {
		return errors.New("无效的状态")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmStockOut
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		its, err := parsePSIItems(r.Items)
		if err != nil {
			return err
		}
		// 离开 confirmed：回滚（增加库存）
		if r.Status == "confirmed" && status != "confirmed" {
			for _, it := range its {
				if err := adjustProductStock(tx, tenantID, it.ProductID, it.Qty); err != nil {
					return err
				}
			}
		}
		// 进入 confirmed：减少库存
		if status == "confirmed" && r.Status != "confirmed" {
			for _, it := range its {
				if err := adjustProductStock(tx, tenantID, it.ProductID, -it.Qty); err != nil {
					return err
				}
			}
		}
		return tx.Model(&models.CrmStockOut{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
			Update("status", status).Error
	})
}

// ==================== 盘点单 ====================

// StockCheckQueryReq 盘点单查询参数。
type StockCheckQueryReq struct {
	Page        int
	PageSize    int
	WarehouseID string
	Status      string
}

// CrmStockCheckService 盘点单服务。
type CrmStockCheckService struct{}

func NewCrmStockCheck() *CrmStockCheckService { return &CrmStockCheckService{} }

// genStockCheckNo 生成盘点单号：SC + yyyyMMdd + 4位序号。
func genStockCheckNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "SC" + day
	var cnt int64
	models.DB.Model(&models.CrmStockCheck{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?",  prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建盘点单（自动生成 No）。
func (s CrmStockCheckService) Create(ctx context.Context, r *models.CrmStockCheck) error {
	tenantID := identity.TenantID(ctx)


	if r == nil {
		return errors.New("盘点单不能为空")
	}
	if r.WarehouseID == "" {
		return errors.New("仓库ID不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	r.No = genStockCheckNo(tenantID)
	if r.Status == "" {
		r.Status = "draft"
	}
	return createCRM(models.DB, r, "WarehouseID", "OperatorID")
}

// Update 更新盘点单。
func (s CrmStockCheckService) Update(ctx context.Context, id string, r *models.CrmStockCheck) error {


	if r == nil {
		return errors.New("盘点单不能为空")
	}
	m := map[string]any{
		"warehouse_id": r.WarehouseID,
		"occur_date":   r.OccurDate,
		"operator_id":  r.OperatorID,
		"status":       r.Status,
		"remark":       r.Remark,
		"items":        r.Items,
	}
	updatesCRM(m, "warehouse_id", "operator_id")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除盘点单。
func (s CrmStockCheckService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmStockCheck{})
	return res.RowsAffected, res.Error
}

// Query 分页查询盘点单。
func (s CrmStockCheckService) Query(ctx context.Context, req StockCheckQueryReq) ([]models.CrmStockCheck, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.WarehouseID != "" {
		db = db.Where("warehouse_id = ?", req.WarehouseID)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmStockCheck{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmStockCheck
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改盘点单状态；confirmed 时按 diff（actual_qty - book_qty）调整库存。
func (s CrmStockCheckService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	if status != "draft" && status != "confirmed" && status != "cancelled" {
		return errors.New("无效的状态")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmStockCheck
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		its, err := parseStockCheckItems(r.Items)
		if err != nil {
			return err
		}
		// 离开 confirmed：回滚（按 -diff 调整）
		if r.Status == "confirmed" && status != "confirmed" {
			for _, it := range its {
				if err := adjustProductStock(tx, tenantID, it.ProductID, -it.Diff); err != nil {
					return err
				}
			}
		}
		// 进入 confirmed：按 diff 调整库存
		if status == "confirmed" && r.Status != "confirmed" {
			for _, it := range its {
				if err := adjustProductStock(tx, tenantID, it.ProductID, it.Diff); err != nil {
					return err
				}
			}
		}
		return tx.Model(&models.CrmStockCheck{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
			Update("status", status).Error
	})
}

// ==================== 调拨单 ====================

// StockTransferQueryReq 调拨单查询参数。
type StockTransferQueryReq struct {
	Page     int
	PageSize int
	Status   string
}

// CrmStockTransferService 调拨单服务。
type CrmStockTransferService struct{}

func NewCrmStockTransfer() *CrmStockTransferService { return &CrmStockTransferService{} }

// genStockTransferNo 生成调拨单号：ST + yyyyMMdd + 4位序号。
func genStockTransferNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "ST" + day
	var cnt int64
	models.DB.Model(&models.CrmStockTransfer{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?",  prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建调拨单（自动生成 No）。
func (s CrmStockTransferService) Create(ctx context.Context, r *models.CrmStockTransfer) error {
	tenantID := identity.TenantID(ctx)


	if r == nil {
		return errors.New("调拨单不能为空")
	}
	if r.FromWarehouse == "" || r.ToWarehouse == "" {
		return errors.New("调出/调入仓库不能为空")
	}
	if r.FromWarehouse == r.ToWarehouse {
		return errors.New("调出与调入仓库不能相同")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	r.No = genStockTransferNo(tenantID)
	if r.Status == "" {
		r.Status = "draft"
	}
	return createCRM(models.DB, r, "FromWarehouse", "ToWarehouse", "OperatorID")
}

// Update 更新调拨单。
func (s CrmStockTransferService) Update(ctx context.Context, id string, r *models.CrmStockTransfer) error {


	if r == nil {
		return errors.New("调拨单不能为空")
	}
	m := map[string]any{
		"from_warehouse": r.FromWarehouse,
		"to_warehouse":   r.ToWarehouse,
		"occur_date":     r.OccurDate,
		"operator_id":    r.OperatorID,
		"status":         r.Status,
		"remark":         r.Remark,
		"items":          r.Items,
	}
	updatesCRM(m, "from_warehouse", "to_warehouse", "operator_id")
	if err := models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除调拨单。
func (s CrmStockTransferService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmStockTransfer{})
	return res.RowsAffected, res.Error
}

// Query 分页查询调拨单。
func (s CrmStockTransferService) Query(ctx context.Context, req StockTransferQueryReq) ([]models.CrmStockTransfer, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmStockTransfer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmStockTransfer
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改调拨单状态；confirmed 时从 FromWarehouse 减到 ToWarehouse。
// 注：当前库存按产品总量维护（CrmProduct.StockQty），调拨仅做总量不变的处理，
// 如需分仓库存可扩展库存快照表。此处按明细记录流水，总量不变。
func (s CrmStockTransferService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	if status != "draft" && status != "confirmed" && status != "cancelled" {
		return errors.New("无效的状态")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmStockTransfer
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		// 调拨不改变产品总库存（仅仓库间移动），此处仅记录流水。
		// 如需分仓库存，可在 confirmed 时写入 CrmStockRecord 流水。
		if status == "confirmed" && r.Status != "confirmed" {
			its, err := parsePSIItems(r.Items)
			if err != nil {
				return err
			}
			now := time.Now()
			for _, it := range its {
				rec := models.CrmStockRecord{
					TenantID:    tenantID,
					WarehouseID: r.FromWarehouse,
					ProductID:   it.ProductID,
					Type:        "transfer",
					Qty:         -it.Qty,
					OrderNo:     r.No,
					OccurAt:     &now,
					OperatorID:  r.OperatorID,
					Remark:      "调拨出库",
				}
				rec.ID = id_generator.StringID()
				if err := tx.Create(&rec).Error; err != nil {
					return err
				}
				rec2 := models.CrmStockRecord{
					TenantID:    tenantID,
					WarehouseID: r.ToWarehouse,
					ProductID:   it.ProductID,
					Type:        "transfer",
					Qty:         it.Qty,
					OrderNo:     r.No,
					OccurAt:     &now,
					OperatorID:  r.OperatorID,
					Remark:      "调拨入库",
				}
				rec2.ID = id_generator.StringID()
				if err := tx.Create(&rec2).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&models.CrmStockTransfer{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
			Update("status", status).Error
	})
}
