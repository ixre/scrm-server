package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// ==================== 财务账户 ====================

// AccountQueryReq 财务账户查询参数。
type AccountQueryReq struct {
	Page     int
	PageSize int
	Type     string
	Status   string
	Keyword  string
}

// CrmAccountService 财务账户服务。
type CrmAccountService struct{}

func NewCrmAccount() *CrmAccountService { return &CrmAccountService{} }

// Create 创建财务账户。
func (s CrmAccountService) Create(ctx context.Context, a *models.CrmAccount) error {
	tenantID := identity.TenantID(ctx)

	if a == nil {
		return errors.New("账户不能为空")
	}
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("账户名称不能为空")
	}
	a.ID = id_generator.StringID()
	a.TenantID = tenantID
	a.Name = strings.TrimSpace(a.Name)
	if a.Status == "" {
		a.Status = "active"
	}
	return models.DB.Create(a).Error
}

// Update 更新财务账户。
func (s CrmAccountService) Update(ctx context.Context, id string, a *models.CrmAccount) error {

	if a == nil {
		return errors.New("账户不能为空")
	}
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("账户名称不能为空")
	}
	m := map[string]any{
		"name":   a.Name,
		"type":   a.Type,
		"number": a.Number,
		"status": a.Status,
		"remark": a.Remark,
	}
	if err := models.MustFor(ctx).Model(&models.CrmAccount{}).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除财务账户。
func (s CrmAccountService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmAccount{})
	return res.RowsAffected, res.Error
}

// Get 获取财务账户详情。
func (s CrmAccountService) Get(ctx context.Context, id string) (*models.CrmAccount, error) {

	var item models.CrmAccount
	err := models.MustFor(ctx).Where("id = ?", id).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// Query 分页查询财务账户。
func (s CrmAccountService) Query(ctx context.Context, req AccountQueryReq) ([]models.CrmAccount, int64, error) {

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
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("name LIKE ? OR number LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmAccount{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmAccount
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ==================== 财务收支记录 ====================

// FinanceRecordQueryReq 财务收支记录查询参数。
type FinanceRecordQueryReq struct {
	Page      int
	PageSize  int
	Type      string
	Category  string
	AccountID string
}

// CrmFinanceRecordService 财务收支记录服务。
type CrmFinanceRecordService struct{}

func NewCrmFinanceRecord() *CrmFinanceRecordService { return &CrmFinanceRecordService{} }

// Create 创建财务收支记录（事务内更新账户余额）。
func (s CrmFinanceRecordService) Create(ctx context.Context, r *models.CrmFinanceRecord) error {
	tenantID := identity.TenantID(ctx)

	if r == nil {
		return errors.New("收支记录不能为空")
	}
	if r.Type == "" {
		return errors.New("类型不能为空")
	}
	if r.Amount <= 0 {
		return errors.New("金额必须大于 0")
	}
	if r.AccountID == "" {
		return errors.New("account_id 不能为空")
	}
	if r.Type == "transfer" && r.ToAccountID == "" {
		return errors.New("转账必须指定转入账户")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	if r.Status == "" {
		r.Status = "confirmed"
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := createCRM(tx, r, "AccountID", "ToAccountID", "OrderID", "CustomerID"); err != nil {
			return err
		}
		if r.Status != "confirmed" {
			return nil
		}
		return applyFinanceEffect(tx, tenantID, r)
	})
}

// Update 更新财务收支记录（仅允许 pending 状态编辑；已确认记录需先作废再重建）。
func (s CrmFinanceRecordService) Update(ctx context.Context, id string, r *models.CrmFinanceRecord) error {

	if r == nil {
		return errors.New("收支记录不能为空")
	}
	if r.Amount <= 0 {
		return errors.New("金额必须大于 0")
	}
	// 已作废的记录不允许通过编辑改 status。
	var existing models.CrmFinanceRecord
	if err := models.MustFor(ctx).Where("id = ?", id).First(&existing).Error; err != nil {
		return errors.New("收支记录不存在")
	}
	statusVal := r.Status
	if existing.Status == "void" {
		statusVal = existing.Status
	}
	m := map[string]any{
		"type":          r.Type,
		"category":      r.Category,
		"account_id":    r.AccountID,
		"to_account_id": r.ToAccountID,
		"amount":        r.Amount,
		"occur_date":    r.OccurDate,
		"order_id":      r.OrderID,
		"customer_id":   r.CustomerID,
		"status":        statusVal,
		"remark":        r.Remark,
	}
	updatesCRM(m, "account_id", "to_account_id", "order_id", "customer_id")
	if err := models.MustFor(ctx).Model(&models.CrmFinanceRecord{}).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除财务收支记录。
func (s CrmFinanceRecordService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmFinanceRecord{})
	return res.RowsAffected, res.Error
}

// Query 分页查询财务收支记录。
func (s CrmFinanceRecordService) Query(ctx context.Context, req FinanceRecordQueryReq) ([]models.CrmFinanceRecord, int64, error) {

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
	if c := strings.TrimSpace(req.Category); c != "" {
		db = db.Where("category = ?", c)
	}
	if req.AccountID != "" {
		db = db.Where("account_id = ?", req.AccountID)
	}
	var total int64
	if err := db.Model(&models.CrmFinanceRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmFinanceRecord
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("occur_date desc, id desc").Find(&items).Error
	return items, total, err
}

// Void 作废财务收支记录（回滚账户余额变动）。
func (s CrmFinanceRecordService) Void(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)

	return models.DB.Transaction(func(tx *gorm.DB) error {
		var r models.CrmFinanceRecord
		if err := tx.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&r).Error; err != nil {
			return err
		}
		if r.Status == "void" {
			return nil
		}
		// 若此前为已确认状态，先回滚余额变动。
		if r.Status == "confirmed" {
			if err := reverseFinanceEffect(tx, tenantID, &r); err != nil {
				return err
			}
		}
		return tx.Model(&models.CrmFinanceRecord{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
			Update("status", "void").Error
	})
}

// applyFinanceEffect 对已确认记录应用账户余额变动。
func applyFinanceEffect(tx *gorm.DB, tenantID string, r *models.CrmFinanceRecord) error {
	switch r.Type {
	case "income":
		return tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
			UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error
	case "expense":
		return tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
			UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error
	case "transfer":
		if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
			UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error; err != nil {
			return err
		}
		return tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.ToAccountID).
			UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error
	}
	return nil
}

// reverseFinanceEffect 回滚 applyFinanceEffect 的余额变动。
func reverseFinanceEffect(tx *gorm.DB, tenantID string, r *models.CrmFinanceRecord) error {
	switch r.Type {
	case "income":
		return tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
			UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error
	case "expense":
		return tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
			UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error
	case "transfer":
		if err := tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.AccountID).
			UpdateColumn("balance", gorm.Expr("balance + ?", r.Amount)).Error; err != nil {
			return err
		}
		return tx.Model(&models.CrmAccount{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", r.ToAccountID).
			UpdateColumn("balance", gorm.Expr("balance - ?", r.Amount)).Error
	}
	return nil
}

// ==================== 发票管理 ====================

// InvoiceQueryReq 发票查询参数。
type InvoiceQueryReq struct {
	Page       int
	PageSize   int
	CustomerID string
	Status     string
	Keyword    string
}

// CrmInvoiceService 发票服务。
type CrmInvoiceService struct{}

func NewCrmInvoice() *CrmInvoiceService { return &CrmInvoiceService{} }

// Create 创建发票。
func (s CrmInvoiceService) Create(ctx context.Context, i *models.CrmInvoice) error {
	tenantID := identity.TenantID(ctx)

	if i == nil {
		return errors.New("发票不能为空")
	}
	if strings.TrimSpace(i.InvoiceNo) == "" {
		return errors.New("发票号不能为空")
	}
	if i.Amount <= 0 {
		return errors.New("开票金额必须大于 0")
	}
	i.ID = id_generator.StringID()
	i.TenantID = tenantID
	i.InvoiceNo = strings.TrimSpace(i.InvoiceNo)
	if i.Status == "" {
		i.Status = "pending"
	}
	return createCRM(models.DB, i, "OrderID", "CustomerID")
}

// Update 更新发票。
func (s CrmInvoiceService) Update(ctx context.Context, id string, i *models.CrmInvoice) error {

	if i == nil {
		return errors.New("发票不能为空")
	}
	if strings.TrimSpace(i.InvoiceNo) == "" {
		return errors.New("发票号不能为空")
	}
	// 已作废的发票不允许通过编辑改 status。
	var existing models.CrmInvoice
	if err := models.MustFor(ctx).Where("id = ?", id).First(&existing).Error; err != nil {
		return errors.New("发票不存在")
	}
	statusVal := i.Status
	if existing.Status == "void" {
		statusVal = existing.Status
	}
	m := map[string]any{
		"invoice_no":  i.InvoiceNo,
		"order_id":    i.OrderID,
		"customer_id": i.CustomerID,
		"type":        i.Type,
		"amount":      i.Amount,
		"tax_rate":    i.TaxRate,
		"title":       i.Title,
		"tax_no":      i.TaxNo,
		"issue_date":  i.IssueDate,
		"status":      statusVal,
	}
	updatesCRM(m, "order_id", "customer_id")
	if err := models.MustFor(ctx).Model(&models.CrmInvoice{}).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除发票。
func (s CrmInvoiceService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmInvoice{})
	return res.RowsAffected, res.Error
}

// Query 分页查询发票。
func (s CrmInvoiceService) Query(ctx context.Context, req InvoiceQueryReq) ([]models.CrmInvoice, int64, error) {

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
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("invoice_no LIKE ? OR title LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmInvoice{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmInvoice
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// Void 作废发票。
func (s CrmInvoiceService) Void(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)

	var i models.CrmInvoice
	if err := models.MustFor(ctx).Where("id = ?", id).First(&i).Error; err != nil {
		return err
	}
	if i.Status == "void" {
		return nil
	}
	return models.DB.Model(&models.CrmInvoice{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Update("status", "void").Error
}
