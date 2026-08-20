package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// ---------- 合同订单产品明细 ----------

type CrmOrderProductService struct{}

func NewCrmOrderProduct() *CrmOrderProductService { return &CrmOrderProductService{} }

// Create 新增订单产品明细。
func (s CrmOrderProductService) Create(ctx context.Context, p *models.CrmOrderProduct) error {
	tenantID := identity.TenantID(ctx)


	p.ID = id_generator.StringID()
	p.TenantID = tenantID
	if err := models.DB.Create(p).Error; err != nil {
		log.Sugar.Errorw("order product create failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除订单产品明细。
func (s CrmOrderProductService) Delete(ctx context.Context, ids []string) (int64, error) {


	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmOrderProduct{})
	return res.RowsAffected, res.Error
}

// ListByOrder 按订单列出产品明细。
func (s CrmOrderProductService) ListByOrder(ctx context.Context, orderID string) ([]models.CrmOrderProduct, error) {


	var items []models.CrmOrderProduct
	err := models.MustFor(ctx).Where("order_id = ?",  orderID).
		Order("id asc").Find(&items).Error
	return items, err
}

// ---------- 合同订单附件 ----------

type CrmOrderAttachmentService struct{}

func NewCrmOrderAttachment() *CrmOrderAttachmentService { return &CrmOrderAttachmentService{} }

// Create 新增订单附件。
func (s CrmOrderAttachmentService) Create(ctx context.Context, a *models.CrmOrderAttachment) error {
	tenantID := identity.TenantID(ctx)


	a.ID = id_generator.StringID()
	a.TenantID = tenantID
	if err := models.DB.Create(a).Error; err != nil {
		log.Sugar.Errorw("order attachment create failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除订单附件。
func (s CrmOrderAttachmentService) Delete(ctx context.Context, ids []string) (int64, error) {


	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmOrderAttachment{})
	return res.RowsAffected, res.Error
}

// ListByOrder 按订单列出附件。
func (s CrmOrderAttachmentService) ListByOrder(ctx context.Context, orderID string) ([]models.CrmOrderAttachment, error) {


	var items []models.CrmOrderAttachment
	err := models.MustFor(ctx).Where("order_id = ?",  orderID).
		Order("id desc").Find(&items).Error
	return items, err
}

// ---------- 合同订单进度节点 ----------

type CrmOrderProgressService struct{}

func NewCrmOrderProgress() *CrmOrderProgressService { return &CrmOrderProgressService{} }

// Create 新增进度节点。
func (s CrmOrderProgressService) Create(ctx context.Context, p *models.CrmOrderProgress) error {
	tenantID := identity.TenantID(ctx)


	p.ID = id_generator.StringID()
	p.TenantID = tenantID
	if err := models.DB.Create(p).Error; err != nil {
		log.Sugar.Errorw("order progress create failed", "err", err)
		return err
	}
	return nil
}

// Update 更新进度节点。
func (s CrmOrderProgressService) Update(ctx context.Context, id string, p *models.CrmOrderProgress) error {
	tenantID := identity.TenantID(ctx)


	updates := map[string]any{
		"node_name": p.NodeName, "sort": p.Sort, "status": p.Status,
		"plan_date": p.PlanDate, "remark": p.Remark,
	}
	res := models.DB.Model(&models.CrmOrderProgress{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		log.Sugar.Errorw("order progress update failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("进度节点不存在")
	}
	return nil
}

// Delete 删除进度节点。
func (s CrmOrderProgressService) Delete(ctx context.Context, ids []string) (int64, error) {


	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmOrderProgress{})
	return res.RowsAffected, res.Error
}

// ListByOrder 按订单列出进度节点。
func (s CrmOrderProgressService) ListByOrder(ctx context.Context, orderID string) ([]models.CrmOrderProgress, error) {


	var items []models.CrmOrderProgress
	err := models.MustFor(ctx).Where("order_id = ?",  orderID).
		Order("sort asc, id asc").Find(&items).Error
	return items, err
}

// UpdateStatus 更新进度节点状态，done 时设置 DoneAt 为当前时间。
func (s CrmOrderProgressService) UpdateStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	updates := map[string]any{"status": status}
	if status == "done" {
		now := time.Now()
		updates["done_at"] = &now
	}
	res := models.DB.Model(&models.CrmOrderProgress{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		log.Sugar.Errorw("order progress status update failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("进度节点不存在")
	}
	return nil
}
