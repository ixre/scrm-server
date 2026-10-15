package services

import (
	"context"
	"errors"
	"fmt"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RecycleQueryReq struct {
	Page, PageSize int
	Keyword        string
}

func (r RecycleQueryReq) pager() *app.Pager {
	return (&app.Pager{Page: r.Page, PageSize: r.PageSize}).SetDefault()
}

type CrmRecycleService struct{}

func NewCrmRecycle() *CrmRecycleService { return &CrmRecycleService{} }

func requireCRMManager(ctx context.Context, tx *gorm.DB) error {
	_, manager, err := customerActor(ctx, tx)
	if err != nil {
		return err
	}
	if !manager {
		return errors.New("此操作需要管理员或经理权限")
	}
	return nil
}
func recycleEntity(ctx context.Context, tx *gorm.DB, kind, id, name string) error {
	rec := models.CrmRecycle{Model: models.Model{ID: id_generator.StringID()}, TenantID: identity.TenantID(ctx), EntityType: kind, EntityID: id, EntityName: name, DeletedBy: identity.UserID(ctx)}
	return createCRM(tx, &rec)
}
func (s CrmRecycleService) List(ctx context.Context, entityType string, req RecycleQueryReq) ([]models.CrmRecycle, int64, error) {
	if err := requireCRMManager(ctx, models.DB); err != nil {
		return nil, 0, err
	}
	if entityType == "contract" {
		entityType = "order"
	}
	p := req.pager()
	db := models.MustFor(ctx).Model(&models.CrmRecycle{})
	if entityType != "" {
		if _, err := entityTable(entityType); err != nil {
			return nil, 0, err
		}
		db = db.Where("entity_type = ?", entityType)
	}
	if req.Keyword != "" {
		db = db.Where("entity_name LIKE ?", "%"+req.Keyword+"%")
	}
	var count int64
	if err := db.Count(&count).Error; err != nil {
		return nil, 0, err
	}
	var rows []models.CrmRecycle
	err := db.Order("created_at desc, id desc").Offset(p.GetOffset()).Limit(p.GetLimit()).Find(&rows).Error
	return rows, count, err
}
func recycledModel(kind string) (any, error) {
	switch kind {
	case "customer":
		return &models.CrmCustomer{}, nil
	case "lead":
		return &models.CrmLead{}, nil
	case "opportunity":
		return &models.CrmOpportunity{}, nil
	case "order":
		return &models.CrmOrder{}, nil
	case "contact":
		return &models.CrmContact{}, nil
	}
	return nil, errors.New("回收站类型无效")
}
func loadRecycled(ctx context.Context, tx *gorm.DB, id string) (models.CrmRecycle, any, error) {
	var rec models.CrmRecycle
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", identity.TenantID(ctx), id).First(&rec).Error; err != nil {
		return rec, nil, errors.New("回收站记录不存在")
	}
	model, err := recycledModel(rec.EntityType)
	if err != nil {
		return rec, nil, err
	}
	err = tx.Unscoped().Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ? AND deleted_at IS NOT NULL", rec.TenantID, rec.EntityID).First(model).Error
	if err != nil {
		return rec, nil, errors.New("记录已恢复或已被彻底删除，请刷新")
	}
	return rec, model, nil
}
func (s CrmRecycleService) Restore(ctx context.Context, id string) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		if err := contactWriteLock(ctx, tx); err != nil {
			return err
		}
		rec, model, err := loadRecycled(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = validateRestoredReferences(tx, rec.TenantID, model); err != nil {
			return err
		}
		changes := map[string]any{"deleted_at": nil}
		if rec.EntityType == "contact" {
			changes["is_primary"] = false
		}
		if err = tx.Unscoped().Model(model).Where("tenant_id = ? AND id = ?", rec.TenantID, rec.EntityID).Updates(changes).Error; err != nil {
			return fmt.Errorf("恢复失败，请核对手机号等唯一字段是否已被占用：%w", err)
		}
		return tx.Delete(&rec).Error
	})
}
func (s CrmRecycleService) Clear(ctx context.Context, id string) error {
	_, err := s.DeleteRecord(ctx, []string{id})
	return err
}

// Removing a recycle entry must never hide a still recoverable entity. Delete both atomically after checking references.
func (s CrmRecycleService) DeleteRecord(ctx context.Context, ids []string) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireCRMManager(ctx, tx); err != nil {
			return err
		}
		for _, id := range ids {
			rec, model, err := loadRecycled(ctx, tx, id)
			if err != nil {
				return err
			}
			if err = guardEntityReferences(tx, rec.TenantID, rec.EntityType, rec.EntityID, true); err != nil {
				return err
			}
			if err = tx.Unscoped().Delete(model).Error; err != nil {
				return err
			}
			if err = tx.Delete(&rec).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

var entityTypeTable = map[string]string{"customer": "crm_customer", "lead": "crm_lead", "opportunity": "crm_opportunity", "order": "crm_order", "contact": "crm_contact"}

func entityTable(kind string) (string, error) {
	if table, ok := entityTypeTable[kind]; ok {
		return table, nil
	}
	return "", errors.New("实体类型无效")
}

func guardEntityReferences(tx *gorm.DB, tenant, kind, id string, includeDeleted bool) error {
	type ref struct {
		model  any
		column string
	}
	refs := []ref{}
	switch kind {
	case "customer":
		var links int64
		if err := tx.Model(&models.ChannelLink{}).Where("tenant_id = ? AND entity_type = ? AND entity_id = ?", tenant, "customer", id).Count(&links).Error; err != nil {
			return err
		}
		if links > 0 {
			return errors.New("客户仍绑定渠道身份，不能删除")
		}
		for _, r := range customerMergeRelations {
			refs = append(refs, ref{r.Model, r.Column})
		}
		refs = append(refs, ref{&models.CrmActivityRecord{}, "customer_id"})
	case "lead":
		refs = []ref{{&models.CrmFollowup{}, "lead_id"}, {&models.CrmOpportunity{}, "lead_id"}}
	case "opportunity":
		refs = []ref{{&models.CrmFollowup{}, "opportunity_id"}, {&models.CrmQuote{}, "opportunity_id"}, {&models.CrmOrder{}, "opportunity_id"}}
	case "order":
		refs = []ref{{&models.CrmQuote{}, "order_id"}, {&models.CrmReceivables{}, "order_id"}, {&models.CrmReceivablesPlan{}, "order_id"}, {&models.CrmOrderProduct{}, "order_id"}, {&models.CrmOrderAttachment{}, "order_id"}, {&models.CrmOrderProgress{}, "order_id"}}
	case "contact":
		refs = []ref{{&models.CrmQuote{}, "contact_id"}, {&models.CrmFollowup{}, "contact_id"}}
	}
	for _, r := range refs {
		db := tx.Model(r.model)
		if includeDeleted {
			db = db.Unscoped()
		}
		var count int64
		if err := db.Where("tenant_id = ? AND "+r.column+" = ?", tenant, id).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			return errors.New("记录仍有关联业务数据，不能删除，请保留历史记录")
		}
	}
	return nil
}

func validateRestoredReferences(tx *gorm.DB, tenant string, model any) error {
	type reference struct {
		model     any
		id, label string
	}
	refs := []reference{}
	owner := ""
	switch row := model.(type) {
	case *models.CrmCustomer:
		owner = row.OwnerUserID
	case *models.CrmContact:
		refs = append(refs, reference{&models.CrmCustomer{}, row.CustomerID, "客户"})
	case *models.CrmLead:
		owner = row.OwnerUserID
		refs = append(refs, reference{&models.CrmCustomer{}, row.CustomerID, "客户"}, reference{&models.CrmCustomer{}, row.ConvertCustomerID, "已转化客户"})
	case *models.CrmOpportunity:
		owner = row.OwnerUserID
		refs = append(refs, reference{&models.CrmCustomer{}, row.CustomerID, "客户"}, reference{&models.CrmLead{}, row.LeadID, "线索"}, reference{&models.CrmPipelineStage{}, row.StageID, "销售阶段"})
	case *models.CrmOrder:
		owner = row.UserID
		refs = append(refs, reference{&models.CrmCustomer{}, row.CustomerID, "客户"}, reference{&models.CrmOpportunity{}, row.OpportunityID, "商机"})
	}
	for _, ref := range refs {
		if ref.id == "" || ref.id == "0" {
			continue
		}
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ? AND id = ?", tenant, ref.id).First(ref.model).Error; err != nil {
			return fmt.Errorf("关联%s不可用，请先恢复关联记录", ref.label)
		}
	}
	if owner != "" && owner != "0" {
		var user models.User
		if err := tx.Where("tenant_id = ? AND id = ? AND status = 1", tenant, owner).First(&user).Error; err != nil {
			return errors.New("原负责人已停用或不存在，请先处理负责人账号再恢复")
		}
	}
	return nil
}
