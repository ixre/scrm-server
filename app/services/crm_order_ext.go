package services

import (
	"context"
	"errors"
	"math"
	"net/url"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// All order child mutations serialize on the parent contract, including signing and revocation.
func editableOrder(ctx context.Context, tx *gorm.DB, id string, draftOnly bool) (models.CrmOrder, error) {
	order, err := lockPaymentOrder(ctx, tx, id)
	if err != nil {
		return order, err
	}
	if draftOnly && order.Status != "draft" {
		return order, errors.New("仅草稿合同可以修改产品或删除附件")
	}
	if order.Status != "draft" && order.Status != "signed" {
		return order, errors.New("已完成或作废的合同不可修改")
	}
	return order, nil
}

func deleteOrderChildren(ctx context.Context, model any, ids []string, draftOnly bool) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var children []struct {
			ID      string
			OrderID string
		}
		if err := tx.Model(model).Select("id", "order_id").Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Find(&children).Error; err != nil {
			return err
		}
		if len(children) != len(ids) {
			return errors.New("合同明细不存在或已删除")
		}
		parents := []string{}
		for _, c := range children {
			parents = append(parents, c.OrderID)
		}
		parents, err = workflowIDs(parents)
		if err != nil {
			return err
		}
		for _, id := range parents {
			if _, err := editableOrder(ctx, tx, id, draftOnly); err != nil {
				return err
			}
		}
		res := tx.Where("tenant_id = ? AND id IN ?", identity.TenantID(ctx), ids).Delete(model)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected != int64(len(ids)) {
			return errors.New("记录已变化，请刷新后重试")
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

func validateOrderProductTotal(tx *gorm.DB, order models.CrmOrder, exact bool) error {
	var lines []models.CrmOrderProduct
	if err := tx.Where("tenant_id = ? AND order_id = ?", order.TenantID, order.ID).Find(&lines).Error; err != nil {
		return err
	}
	var total int64
	for _, line := range lines {
		amount, err := moneyCents(line.Amount)
		if err != nil {
			return err
		}
		total += amount
	}
	amount, err := moneyCents(order.Amount)
	if err != nil {
		return err
	}
	if total > amount {
		return errors.New("产品明细合计超过合同金额，请先核对合同金额")
	}
	if exact && len(lines) > 0 && total != amount {
		return errors.New("产品明细合计与合同金额不一致，请补齐明细后签约")
	}
	return nil
}

type CrmOrderProductService struct{}

func NewCrmOrderProduct() *CrmOrderProductService { return &CrmOrderProductService{} }
func (s CrmOrderProductService) Create(ctx context.Context, p *models.CrmOrderProduct) error {
	p.ProductName = strings.TrimSpace(p.ProductName)
	if p.ProductName == "" || p.Qty <= 0 {
		return errors.New("请填写产品名称和大于零的数量")
	}
	if _, err := moneyCents(p.UnitPrice); err != nil {
		return err
	}
	if math.IsNaN(p.Discount) || math.IsInf(p.Discount, 0) || p.Discount < 0 || p.Discount > 100 || math.Abs(p.Discount*100-math.Round(p.Discount*100)) > 0.00001 {
		return errors.New("折扣须为 0 到 100 的百分数，最多两位小数")
	}
	p.Amount = math.Round(float64(p.Qty)*p.UnitPrice*p.Discount) / 100
	if _, err := moneyCents(p.Amount); err != nil {
		return err
	}
	p.ID, p.TenantID = id_generator.StringID(), identity.TenantID(ctx)
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := editableOrder(ctx, tx, p.OrderID, true)
		if err != nil {
			return err
		}
		if err = createCRM(tx, p); err != nil {
			return err
		}
		return validateOrderProductTotal(tx, order, false)
	})
}
func (s CrmOrderProductService) Delete(ctx context.Context, ids []string) (int64, error) {
	return deleteOrderChildren(ctx, &models.CrmOrderProduct{}, ids, true)
}
func (s CrmOrderProductService) ListByOrder(ctx context.Context, orderID string) ([]models.CrmOrderProduct, error) {
	if _, err := NewCrmOrder().Get(ctx, orderID); err != nil {
		return nil, err
	}
	var rows []models.CrmOrderProduct
	err := models.MustFor(ctx).Where("order_id = ?", orderID).Order("id asc").Find(&rows).Error
	return rows, err
}

type CrmOrderAttachmentService struct{}

func NewCrmOrderAttachment() *CrmOrderAttachmentService { return &CrmOrderAttachmentService{} }
func (s CrmOrderAttachmentService) Create(ctx context.Context, a *models.CrmOrderAttachment) error {
	if a.StorageKey != "" || a.SHA256 != "" {
		return errors.New("私有附件请通过文件上传入口提交")
	}
	a.FileName, a.FileURL = strings.TrimSpace(a.FileName), strings.TrimSpace(a.FileURL)
	u, err := url.Parse(a.FileURL)
	if err != nil || u.User != nil || !((u.Scheme == "https" || u.Scheme == "http") && u.Host != "" || u.Scheme == "" && u.Host == "" && strings.HasPrefix(u.Path, "/uploads/")) {
		return errors.New("附件地址须为有效的 HTTP(S) 地址或上传文件路径")
	}
	if a.FileName == "" || a.FileSize < 0 {
		return errors.New("请填写文件名称及有效的文件大小")
	}
	a.ID, a.TenantID, a.UploadedBy = id_generator.StringID(), identity.TenantID(ctx), identity.UserID(ctx)
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := editableOrder(ctx, tx, a.OrderID, false); err != nil {
			return err
		}
		return createCRM(tx, a)
	})
}
func (s CrmOrderAttachmentService) Delete(ctx context.Context, ids []string) (int64, error) {
	return deleteOrderChildren(ctx, &models.CrmOrderAttachment{}, ids, true)
}
func (s CrmOrderAttachmentService) ListByOrder(ctx context.Context, orderID string) ([]models.CrmOrderAttachment, error) {
	if _, err := NewCrmOrder().Get(ctx, orderID); err != nil {
		return nil, err
	}
	var rows []models.CrmOrderAttachment
	err := models.MustFor(ctx).Where("order_id = ?", orderID).Order("id desc").Find(&rows).Error
	return rows, err
}

type CrmOrderProgressService struct{}

func NewCrmOrderProgress() *CrmOrderProgressService { return &CrmOrderProgressService{} }
func validateOrderProgress(p *models.CrmOrderProgress) error {
	p.NodeName = strings.TrimSpace(p.NodeName)
	if p.NodeName == "" || p.Sort < 0 {
		return errors.New("请填写节点名称及有效排序")
	}
	if p.Status == "" {
		p.Status = "pending"
	}
	switch p.Status {
	case "pending", "doing", "skipped":
		p.DoneAt = nil
	case "done":
		if p.DoneAt == nil {
			now := time.Now()
			p.DoneAt = &now
		}
	default:
		return errors.New("进度节点状态无效")
	}
	return nil
}
func (s CrmOrderProgressService) Create(ctx context.Context, p *models.CrmOrderProgress) error {
	p.ID, p.TenantID, p.DoneAt = id_generator.StringID(), identity.TenantID(ctx), nil
	if err := validateOrderProgress(p); err != nil {
		return err
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := editableOrder(ctx, tx, p.OrderID, false); err != nil {
			return err
		}
		return createCRM(tx, p)
	})
}
func mutateOrderProgress(ctx context.Context, id string, fn func(*models.CrmOrderProgress) error) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var row models.CrmOrderProgress
		if err := tx.Where("tenant_id = ? AND id = ?", identity.TenantID(ctx), id).First(&row).Error; err != nil {
			return errors.New("进度节点不存在")
		}
		if _, err := editableOrder(ctx, tx, row.OrderID, false); err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", identity.TenantID(ctx), id).First(&row).Error; err != nil {
			return err
		}
		if err := fn(&row); err != nil {
			return err
		}
		if err := validateOrderProgress(&row); err != nil {
			return err
		}
		return tx.Model(&row).Updates(map[string]any{"node_name": row.NodeName, "sort": row.Sort, "status": row.Status, "plan_date": row.PlanDate, "done_at": row.DoneAt, "remark": row.Remark}).Error
	})
}
func (s CrmOrderProgressService) Update(ctx context.Context, id string, p *models.CrmOrderProgress) error {
	return mutateOrderProgress(ctx, id, func(row *models.CrmOrderProgress) error {
		if p.OrderID != "" && p.OrderID != row.OrderID {
			return errors.New("不可更换节点所属合同")
		}
		row.NodeName, row.Sort, row.Status, row.PlanDate, row.Remark = p.NodeName, p.Sort, p.Status, p.PlanDate, p.Remark
		return nil
	})
}
func (s CrmOrderProgressService) Delete(ctx context.Context, ids []string) (int64, error) {
	return deleteOrderChildren(ctx, &models.CrmOrderProgress{}, ids, false)
}
func (s CrmOrderProgressService) ListByOrder(ctx context.Context, orderID string) ([]models.CrmOrderProgress, error) {
	if _, err := NewCrmOrder().Get(ctx, orderID); err != nil {
		return nil, err
	}
	var rows []models.CrmOrderProgress
	err := models.MustFor(ctx).Where("order_id = ?", orderID).Order("sort asc, id asc").Find(&rows).Error
	return rows, err
}
func (s CrmOrderProgressService) UpdateStatus(ctx context.Context, id, status string) error {
	return mutateOrderProgress(ctx, id, func(row *models.CrmOrderProgress) error { row.Status = status; return nil })
}
