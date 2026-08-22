package services

import (
	"core/common/identity"

	"context"

	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// ==================== 收支类别 ====================

// IncomeCategoryQueryReq 收支类别查询参数。
type IncomeCategoryQueryReq struct {
	Page     int
	PageSize int
	Type     string
	Status   int
}

// CrmIncomeCategoryService 收支类别服务。
type CrmIncomeCategoryService struct{}

func NewCrmIncomeCategory() *CrmIncomeCategoryService { return &CrmIncomeCategoryService{} }

// Create 创建收支类别。
func (s CrmIncomeCategoryService) Create(ctx context.Context, c *models.CrmIncomeCategory) error {
	tenantID := identity.TenantID(ctx)

	if c == nil {
		return errors.New("类别不能为空")
	}
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("类别名称不能为空")
	}
	c.ID = id_generator.StringID()
	c.TenantID = tenantID
	c.Name = strings.TrimSpace(c.Name)
	if c.Type == "" {
		c.Type = "income"
	}
	return models.DB.Create(c).Error
}

// Update 更新收支类别。
func (s CrmIncomeCategoryService) Update(ctx context.Context, id string, c *models.CrmIncomeCategory) error {

	if c == nil {
		return errors.New("类别不能为空")
	}
	m := map[string]any{
		"name":   c.Name,
		"type":   c.Type,
		"status": c.Status,
	}
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除收支类别。
func (s CrmIncomeCategoryService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmIncomeCategory{})
	return res.RowsAffected, res.Error
}

// Query 分页查询收支类别（按 type）。
func (s CrmIncomeCategoryService) Query(ctx context.Context, req IncomeCategoryQueryReq) ([]models.CrmIncomeCategory, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if req.Status >= 0 {
		db = db.Where("status = ?", req.Status)
	}
	var total int64
	if err := db.Model(&models.CrmIncomeCategory{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmIncomeCategory
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ==================== 其他收入单 ====================

// OtherIncomeQueryReq 其他收入单查询参数。
type OtherIncomeQueryReq struct {
	Page       int
	PageSize   int
	Status     string
	CategoryID string
}

// CrmOtherIncomeService 其他收入单服务。
type CrmOtherIncomeService struct{}

func NewCrmOtherIncome() *CrmOtherIncomeService { return &CrmOtherIncomeService{} }

// genOtherIncomeNo 生成其他收入单号：OI + yyyyMMdd + 4位序号。
func genOtherIncomeNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "OI" + day
	var cnt int64
	models.DB.Model(&models.CrmOtherIncome{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?", prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建其他收入单（自动生成 No）。
func (s CrmOtherIncomeService) Create(ctx context.Context, r *models.CrmOtherIncome) error {
	tenantID := identity.TenantID(ctx)

	if r == nil {
		return errors.New("收入单不能为空")
	}
	if r.Amount <= 0 {
		return errors.New("金额必须大于 0")
	}
	if r.AccountID == "" {
		return errors.New("account_id 不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	r.No = genOtherIncomeNo(tenantID)
	if r.Status == "" {
		r.Status = "draft"
	}
	return createCRM(models.DB, r, "CategoryID", "AccountID", "CustomerID")
}

// Update 更新其他收入单。
func (s CrmOtherIncomeService) Update(ctx context.Context, id string, r *models.CrmOtherIncome) error {

	if r == nil {
		return errors.New("收入单不能为空")
	}
	// 已作废的单据不允许通过编辑改 status，必须用 ChangeStatus。
	var existing models.CrmOtherIncome
	if err := models.MustFor(ctx).Where("id = ?", id).First(&existing).Error; err != nil {
		return errors.New("收入单不存在")
	}
	statusVal := r.Status
	if existing.Status == "cancelled" {
		statusVal = existing.Status
	}
	m := map[string]any{
		"category_id": r.CategoryID,
		"account_id":  r.AccountID,
		"amount":      r.Amount,
		"occur_date":  r.OccurDate,
		"customer_id": r.CustomerID,
		"remark":      r.Remark,
		"status":      statusVal,
	}
	updatesCRM(m, "category_id", "account_id", "customer_id")
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除其他收入单。
func (s CrmOtherIncomeService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmOtherIncome{})
	return res.RowsAffected, res.Error
}

// Query 分页查询其他收入单（按 status/category_id）。
func (s CrmOtherIncomeService) Query(ctx context.Context, req OtherIncomeQueryReq) ([]models.CrmOtherIncome, int64, error) {

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
	if req.CategoryID != "" {
		db = db.Where("category_id = ?", req.CategoryID)
	}
	var total int64
	if err := db.Model(&models.CrmOtherIncome{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmOtherIncome
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改其他收入单状态；confirmed 时增加账户余额，cancelled 时回滚。
func (s CrmOtherIncomeService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)

	if status != "draft" && status != "confirmed" && status != "cancelled" {
		return errors.New("无效的状态")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmOtherIncome
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		// 离开 confirmed：回滚余额
		if r.Status == "confirmed" && status != "confirmed" {
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
				UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error; err != nil {
				return err
			}
		}
		// 进入 confirmed：增加余额
		if status == "confirmed" && r.Status != "confirmed" {
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
				UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.CrmOtherIncome{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
			Update("status", status).Error
	})
}

// ==================== 其他支出单 ====================

// OtherExpenseQueryReq 其他支出单查询参数。
type OtherExpenseQueryReq struct {
	Page       int
	PageSize   int
	Status     string
	CategoryID string
}

// CrmOtherExpenseService 其他支出单服务。
type CrmOtherExpenseService struct{}

func NewCrmOtherExpense() *CrmOtherExpenseService { return &CrmOtherExpenseService{} }

// genOtherExpenseNo 生成其他支出单号：OE + yyyyMMdd + 4位序号。
func genOtherExpenseNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "OE" + day
	var cnt int64
	models.DB.Model(&models.CrmOtherExpense{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?", prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建其他支出单（自动生成 No）。
func (s CrmOtherExpenseService) Create(ctx context.Context, r *models.CrmOtherExpense) error {
	tenantID := identity.TenantID(ctx)

	if r == nil {
		return errors.New("支出单不能为空")
	}
	if r.Amount <= 0 {
		return errors.New("金额必须大于 0")
	}
	if r.AccountID == "" {
		return errors.New("account_id 不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	r.No = genOtherExpenseNo(tenantID)
	if r.Status == "" {
		r.Status = "draft"
	}
	return createCRM(models.DB, r, "CategoryID", "AccountID", "CustomerID")
}

// Update 更新其他支出单。
func (s CrmOtherExpenseService) Update(ctx context.Context, id string, r *models.CrmOtherExpense) error {

	if r == nil {
		return errors.New("支出单不能为空")
	}
	// 已作废的单据不允许通过编辑改 status，必须用 ChangeStatus。
	var existing models.CrmOtherExpense
	if err := models.MustFor(ctx).Where("id = ?", id).First(&existing).Error; err != nil {
		return errors.New("支出单不存在")
	}
	statusVal := r.Status
	if existing.Status == "cancelled" {
		statusVal = existing.Status
	}
	m := map[string]any{
		"category_id": r.CategoryID,
		"account_id":  r.AccountID,
		"amount":      r.Amount,
		"occur_date":  r.OccurDate,
		"customer_id": r.CustomerID,
		"remark":      r.Remark,
		"status":      statusVal,
	}
	updatesCRM(m, "category_id", "account_id", "customer_id")
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除其他支出单。
func (s CrmOtherExpenseService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmOtherExpense{})
	return res.RowsAffected, res.Error
}

// Query 分页查询其他支出单（按 status/category_id）。
func (s CrmOtherExpenseService) Query(ctx context.Context, req OtherExpenseQueryReq) ([]models.CrmOtherExpense, int64, error) {

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
	if req.CategoryID != "" {
		db = db.Where("category_id = ?", req.CategoryID)
	}
	var total int64
	if err := db.Model(&models.CrmOtherExpense{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmOtherExpense
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改其他支出单状态；confirmed 时扣减账户余额，cancelled 时回滚。
func (s CrmOtherExpenseService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)

	if status != "draft" && status != "confirmed" && status != "cancelled" {
		return errors.New("无效的状态")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmOtherExpense
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		if r.Status == "confirmed" && status != "confirmed" {
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
				UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error; err != nil {
				return err
			}
		}
		if status == "confirmed" && r.Status != "confirmed" {
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
				UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.CrmOtherExpense{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
			Update("status", status).Error
	})
}

// ==================== 资金转账单 ====================

// FundTransferQueryReq 资金转账单查询参数。
type FundTransferQueryReq struct {
	Page     int
	PageSize int
	Status   string
}

// CrmFundTransferService 资金转账单服务。
type CrmFundTransferService struct{}

func NewCrmFundTransfer() *CrmFundTransferService { return &CrmFundTransferService{} }

// genFundTransferNo 生成资金转账单号：FT + yyyyMMdd + 4位序号。
func genFundTransferNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "FT" + day
	var cnt int64
	models.DB.Model(&models.CrmFundTransfer{}).Scopes(models.TenantScope(tenantID)).Where("no LIKE ?", prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建资金转账单（自动生成 No）。
func (s CrmFundTransferService) Create(ctx context.Context, r *models.CrmFundTransfer) error {
	tenantID := identity.TenantID(ctx)

	if r == nil {
		return errors.New("转账单不能为空")
	}
	if r.Amount <= 0 {
		return errors.New("金额必须大于 0")
	}
	if r.FromAccount == "" || r.ToAccount == "" {
		return errors.New("转出/转入账户不能为空")
	}
	if r.FromAccount == r.ToAccount {
		return errors.New("转出与转入账户不能相同")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	r.No = genFundTransferNo(tenantID)
	if r.Status == "" {
		r.Status = "draft"
	}
	return createCRM(models.DB, r, "FromAccount", "ToAccount")
}

// Update 更新资金转账单。
func (s CrmFundTransferService) Update(ctx context.Context, id string, r *models.CrmFundTransfer) error {

	if r == nil {
		return errors.New("转账单不能为空")
	}
	// 已作废的单据不允许通过编辑改 status，必须用 ChangeStatus。
	var existing models.CrmFundTransfer
	if err := models.MustFor(ctx).Where("id = ?", id).First(&existing).Error; err != nil {
		return errors.New("转账单不存在")
	}
	statusVal := r.Status
	if existing.Status == "cancelled" {
		statusVal = existing.Status
	}
	m := map[string]any{
		"from_account": r.FromAccount,
		"to_account":   r.ToAccount,
		"amount":       r.Amount,
		"occur_date":   r.OccurDate,
		"remark":       r.Remark,
		"status":       statusVal,
	}
	updatesCRM(m, "from_account", "to_account")
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除资金转账单。
func (s CrmFundTransferService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmFundTransfer{})
	return res.RowsAffected, res.Error
}

// Query 分页查询资金转账单。
func (s CrmFundTransferService) Query(ctx context.Context, req FundTransferQueryReq) ([]models.CrmFundTransfer, int64, error) {

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
	if err := db.Model(&models.CrmFundTransfer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmFundTransfer
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改资金转账单状态；confirmed 时扣减 FromAccount 余额、增加 ToAccount 余额。
func (s CrmFundTransferService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)

	if status != "draft" && status != "confirmed" && status != "cancelled" {
		return errors.New("无效的状态")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmFundTransfer
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == status {
			return nil
		}
		// 离开 confirmed：回滚（FromAccount +，ToAccount -）
		if r.Status == "confirmed" && status != "confirmed" {
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.FromAccount).
				UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.ToAccount).
				UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error; err != nil {
				return err
			}
		}
		// 进入 confirmed：扣减 FromAccount、增加 ToAccount
		if status == "confirmed" && r.Status != "confirmed" {
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.FromAccount).
				UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.ToAccount).
				UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error; err != nil {
				return err
			}
		}
		return tx.Model(&models.CrmFundTransfer{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
			Update("status", status).Error
	})
}
