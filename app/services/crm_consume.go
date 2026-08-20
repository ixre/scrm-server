package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// ConsumeQueryReq 客户消费记录查询参数。
type ConsumeQueryReq struct {
	Page       int
	PageSize   int
	CustomerID string
	Type       string
	Status     string
}

// CrmConsumeService 客户消费记录服务。
type CrmConsumeService struct{}

func NewCrmConsume() *CrmConsumeService { return &CrmConsumeService{} }

// Create 创建消费记录。
func (s CrmConsumeService) Create(ctx context.Context, c *models.CrmConsume) error {
	tenantID := identity.TenantID(ctx)


	if c == nil {
		return errors.New("消费记录不能为空")
	}
	if c.CustomerID == "" {
		return errors.New("customer_id 不能为空")
	}
	if c.Amount <= 0 {
		return errors.New("消费金额必须大于 0")
	}
	if c.Type == "" {
		return errors.New("消费类型不能为空")
	}
	c.ID = id_generator.StringID()
	c.TenantID = tenantID
	if c.Status == "" {
		c.Status = "normal"
	}
	return createCRM(models.DB, c, "OrderID")
}

// Update 更新消费记录。
func (s CrmConsumeService) Update(ctx context.Context, id string, c *models.CrmConsume) error {
	tenantID := identity.TenantID(ctx)


	if c == nil {
		return errors.New("消费记录不能为空")
	}
	if c.CustomerID == "" {
		return errors.New("customer_id 不能为空")
	}
	m := map[string]any{
		"customer_id": c.CustomerID,
		"order_id":    c.OrderID,
		"amount":      c.Amount,
		"type":        c.Type,
		"consumed_at": c.ConsumedAt,
		"status":      c.Status,
		"remark":      c.Remark,
	}
	updatesCRM(m, "order_id")
	if err := models.DB.Model(&models.CrmConsume{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).Updates(m).Error; err != nil {
		log.Sugar.Errorw("crm consume update failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除消费记录。
func (s CrmConsumeService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmConsume{})
	return res.RowsAffected, res.Error
}

// Query 分页查询消费记录。
func (s CrmConsumeService) Query(ctx context.Context, req ConsumeQueryReq) ([]models.CrmConsume, int64, error) {


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
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmConsume{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmConsume
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("consumed_at desc, id desc").Find(&items).Error
	return items, total, err
}

// Revoke 撤销消费记录（status -> revoked）。
func (s CrmConsumeService) Revoke(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)


	var c models.CrmConsume
	if err := models.MustFor(ctx).Where("id = ?",  id).First(&c).Error; err != nil {
		return err
	}
	if c.Status == "revoked" {
		return nil
	}
	return models.DB.Model(&models.CrmConsume{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).
		Update("status", "revoked").Error
}

// Summary 按消费类型汇总金额（仅统计 normal 状态）。
func (s CrmConsumeService) Summary(ctx context.Context, customerID string) (map[string]float64, error) {
	tenantID := identity.TenantID(ctx)


	type row struct {
		Type   string
		Amount float64
	}
	var rows []row
	err := models.DB.Model(&models.CrmConsume{}).
		Select("type, COALESCE(SUM(amount), 0) as amount").Scopes(models.TenantScope(tenantID)).Where("customer_id = ? AND status = ?",  customerID,  "normal").
		Group("type").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		out[r.Type] = r.Amount
	}
	// 汇总总额。
	var total float64
	tErr := models.DB.Model(&models.CrmConsume{}).Scopes(models.TenantScope(tenantID)).Where("customer_id = ? AND status = ?",  customerID,  "normal").
		Select("COALESCE(SUM(amount), 0)").Scan(&total).Error
	if tErr != nil {
		return out, tErr
	}
	out["total"] = total
	return out, nil
}
