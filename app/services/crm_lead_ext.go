package services

import (
	"bytes"
	"core/common/app"
	"core/common/identity"
	"strconv"

	"context"
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// LeadPoolQueryReq 线索池查询参数。
type LeadPoolQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
}

func (r LeadPoolQueryReq) pager() *app.Pager { return pagerOf(r.Page, r.PageSize) }

type CrmLeadExtService struct{}

func NewCrmLeadExt() *CrmLeadExtService { return &CrmLeadExtService{} }

// MoveToPool 批量入线索池：PoolStatus=1, PoolAt=now。
func (s CrmLeadExtService) MoveToPool(ctx context.Context, ids []string) error {
	_, err := mutateLeads(ctx, ids, func(tx *gorm.DB, lead *models.CrmLead) error {
		if err := mutableLead(*lead); err != nil {
			return err
		}
		return tx.Model(lead).Updates(map[string]any{"pool_status": models.LeadPoolIn, "pool_at": time.Now(), "owner_user_id": nil, "next_followup_at": nil, "next_action": ""}).Error
	})
	return err
}

// ReceiveFromPool 从线索池领取：PoolStatus=0, OwnerUserID=userID。
func (s CrmLeadExtService) ReceiveFromPool(ctx context.Context, userID, id string) error {
	if userID == "" || userID != identity.UserID(ctx) {
		return ErrCustomerAccess
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lead, err := lockSalesRecord[models.CrmLead](ctx, tx, id, true)
		if err != nil {
			return err
		}
		if err = mutableLead(lead); err != nil {
			return err
		}
		if !lead.InPool() {
			return errors.New("线索已被领取，请刷新列表")
		}
		return tx.Model(&lead).Updates(map[string]any{"pool_status": models.LeadPoolNone, "pool_at": nil, "owner_user_id": userID}).Error
	})
}

// ListPool 线索池列表（PoolStatus=1）。
func (s CrmLeadExtService) ListPool(ctx context.Context, req LeadPoolQueryReq) ([]models.CrmLead, int64, error) {

	p := req.pager()
	db, err := salesAccessDB(ctx, models.DB, true)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where("pool_status = ? AND status IN ? AND convert_customer_id IS NULL", models.LeadPoolIn, []string{models.LeadStatusNew, models.LeadStatusContacted, ""})
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("title LIKE ? OR name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmLead{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmLead
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("pool_at desc, id desc").Find(&items).Error
	return items, total, err
}

// Transfer 线索转移：改 OwnerUserID。
func (s CrmLeadExtService) Transfer(ctx context.Context, id, toUserID string) error {
	_, err := s.BatchTransfer(ctx, []string{id}, toUserID)
	return err
}

// ConvertToCustomer 线索转客户：创建 CrmCustomer，更新 CrmLead 的 ConvertCustomerID/ConvertAt/Status。
// ownerUserID 为空时保留原线索归属，池中线索默认归操作人。
// 幂等：线索已转化（ConvertCustomerID 非空）则直接返回已有客户。
func (s CrmLeadExtService) ConvertToCustomer(ctx context.Context, userID, id, ownerUserID, remark string) (*models.CrmCustomer, error) {
	if userID != identity.UserID(ctx) {
		return nil, ErrCustomerAccess
	}
	var customer models.CrmCustomer
	created, claimed := false, false
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		lead, err := lockSalesRecord[models.CrmLead](ctx, tx, id, true)
		if err != nil {
			return err
		}
		if lead.ConvertCustomerID != "" {
			db, err := customerAccessDB(ctx, tx, false)
			if err != nil {
				return err
			}
			return db.Where("id = ?", lead.ConvertCustomerID).First(&customer).Error
		}
		if !lead.Followable() {
			return errors.New("仅待跟进或已联系线索可转客户，流失线索请先恢复")
		}
		owner := ownerUserID
		if owner == "" {
			owner = lead.OwnerUserID
		}
		if owner == "" {
			owner = userID
		}
		if err = validateCustomerOwner(ctx, tx, owner); err != nil {
			return err
		}
		mobile := strings.TrimSpace(lead.Mobile)
		if mobile != "" {
			if err = lockSalesKey(ctx, tx, "lead-conversion-mobile", mobile); err != nil {
				return err
			}
		}
		if lead.CustomerID != "" {
			customer, err = activeSalesCustomerLock(ctx, tx, lead.CustomerID, "UPDATE")
			if err != nil {
				return err
			}
			if customer.OwnerUserID != owner {
				return errors.New("关联客户负责人不同，请先完成客户交接")
			}
		}
		if mobile != "" {
			var matches []models.CrmCustomer
			if err = tx.Where("tenant_id = ? AND mobile = ?", lead.TenantID, mobile).Limit(2).Find(&matches).Error; err != nil {
				return err
			}
			if len(matches) > 1 {
				return errors.New("该手机号存在多条客户记录，请先合并重复客户")
			}
			if len(matches) == 1 {
				if customer.ID != "" && customer.ID != matches[0].ID {
					return errors.New("关联客户与手机号匹配客户不同，请核对线索")
				}
				customer, err = activeSalesCustomerLock(ctx, tx, matches[0].ID, "UPDATE")
				if err != nil {
					return errors.New("该手机号已有客户，请联系负责人或先领取公海客户")
				}
				if customer.OwnerUserID != owner {
					return errors.New("已有客户的负责人不同，请先完成客户交接")
				}
			}
		}
		now := time.Now()
		if customer.ID == "" {
			name := strings.TrimSpace(lead.Name)
			if name == "" {
				name = strings.TrimSpace(lead.Company)
			}
			if name == "" {
				name = strings.TrimSpace(lead.Title)
			}
			if name == "" && mobile == "" {
				return errors.New("线索需要联系人、公司、标题或手机号才能转客户")
			}
			customer = models.CrmCustomer{Model: models.Model{ID: id_generator.StringID()}, TenantID: lead.TenantID, Name: name, Company: lead.Company, Mobile: mobile, Email: lead.Email, Position: lead.Position, Address: lead.Address, Province: lead.Province, City: lead.City, District: lead.District, Source: lead.Source, OwnerUserID: owner, Level: 1, ClaimedAt: &now, NextFollowupAt: lead.NextFollowupAt}
			if err = createCRM(tx, &customer); err != nil {
				return err
			}
			created = true
		}
		if !created && lead.NextFollowupAt != nil && (customer.NextFollowupAt == nil || lead.NextFollowupAt.Before(*customer.NextFollowupAt)) {
			if err = tx.Model(&customer).Update("next_followup_at", lead.NextFollowupAt).Error; err != nil {
				return err
			}
			customer.NextFollowupAt = lead.NextFollowupAt
		}

		claimed = lead.InPool()
		if err = tx.Model(&lead).Updates(map[string]any{"customer_id": customer.ID, "convert_customer_id": customer.ID, "convert_at": now, "status": models.LeadStatusQualified, "convert_remark": remark, "pool_status": models.LeadPoolNone, "pool_at": nil, "owner_user_id": owner, "next_followup_at": nil, "next_action": ""}).Error; err != nil {
			return err
		}
		if lead.NextFollowupAt != nil || lead.NextAction != "" {
			action := "按客户原预约继续跟进"
			if lead.NextFollowupAt != nil && customer.NextFollowupAt != nil && lead.NextFollowupAt.Equal(*customer.NextFollowupAt) {
				action = lead.NextAction
			}
			entry := models.CrmFollowup{Model: models.Model{ID: id_generator.StringID()}, TenantID: lead.TenantID, LeadID: lead.ID, CustomerID: customer.ID, UserID: userID, Type: "other", Content: "线索转客户，延续较早预约；线索原下一步：" + lead.NextAction, NextFollowupAt: customer.NextFollowupAt, NextAction: action}
			if err = createCRM(tx, &entry); err != nil {
				return err
			}
		}
		// Keep the lead's followup history visible on the new customer's timeline.
		return tx.Model(&models.CrmFollowup{}).Where("tenant_id = ? AND lead_id = ? AND customer_id IS NULL", lead.TenantID, lead.ID).Update("customer_id", customer.ID).Error
	})
	if err != nil {
		return nil, err
	}
	if created {
		NewCrmSop().Trigger(ctx, "new_customer", map[string]string{"customer_id": customer.ID, "source": customer.Source})
	}
	if claimed {
		NewCrmSop().Trigger(ctx, "pool_claim", map[string]string{"pool_type": "lead", "customer_id": customer.ID})
	}
	return &customer, nil
}

// MarkContacted 标记线索为已联系：Status=contacted。自动写入跟进记录。
func (s CrmLeadExtService) MarkContacted(ctx context.Context, userID, id, content string) error {
	return changeLeadStatus(ctx, userID, id, "contacted", "标记已联系："+content, false)
}

// MarkLost 标记线索为已流失：Status=lost。自动写入跟进记录。
func (s CrmLeadExtService) MarkLost(ctx context.Context, userID, id, reason string) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("请填写流失原因")
	}
	return changeLeadStatus(ctx, userID, id, "lost", "标记流失："+reason, false)
}

// Reopen 恢复已流失线索为可跟进：Status=contacted。自动写入跟进记录。
// 不允许恢复已转化（qualified）线索。
func (s CrmLeadExtService) Reopen(ctx context.Context, userID, id string) error {
	return changeLeadStatus(ctx, userID, id, "contacted", "恢复跟进", true)
}

// Export 导出线索为 Excel。
func (s CrmLeadExtService) Export(ctx context.Context, status, ownerUserID string) (*bytes.Buffer, string, error) {
	db, err := salesAccessDB(ctx, models.DB, true)
	if err != nil {
		return nil, "", err
	}
	db = db.Limit(10000).Order("id desc")
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if ownerUserID != "" {
		db = db.Where("owner_user_id = ?", ownerUserID)
	}
	var leads []models.CrmLead
	if err := db.Find(&leads).Error; err != nil {
		return nil, "", err
	}
	f := excelize.NewFile()
	sheet := "线索列表"
	f.SetSheetName(f.GetSheetName(0), sheet)
	headers := []string{"ID", "标题", "公司", "联系人", "电话", "邮箱", "职位", "来源", "状态", "归属销售ID", "备注", "创建时间"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	for i, l := range leads {
		row := i + 2
		f.SetCellValue(sheet, "A"+strconv.Itoa(row), l.ID)
		f.SetCellValue(sheet, "B"+strconv.Itoa(row), l.Title)
		f.SetCellValue(sheet, "C"+strconv.Itoa(row), l.Company)
		f.SetCellValue(sheet, "D"+strconv.Itoa(row), l.Name)
		f.SetCellValue(sheet, "E"+strconv.Itoa(row), l.Mobile)
		f.SetCellValue(sheet, "F"+strconv.Itoa(row), l.Email)
		f.SetCellValue(sheet, "G"+strconv.Itoa(row), l.Position)
		f.SetCellValue(sheet, "H"+strconv.Itoa(row), l.Source)
		f.SetCellValue(sheet, "I"+strconv.Itoa(row), l.Status)
		f.SetCellValue(sheet, "J"+strconv.Itoa(row), l.OwnerUserID)
		f.SetCellValue(sheet, "K"+strconv.Itoa(row), l.Remark)
		f.SetCellValue(sheet, "L"+strconv.Itoa(row), l.CreatedAt.Format("2006-01-02 15:04:05"))
	}
	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, "", err
	}
	return buf, "leads_" + time.Now().Format("20060102_150405") + ".xlsx", nil
}

// BatchTransfer 批量转移线索归属人。
func (s CrmLeadExtService) BatchTransfer(ctx context.Context, ids []string, toUserID string) (int64, error) {
	if toUserID == "" {
		return 0, errors.New("请选择接收销售")
	}
	return mutateLeads(ctx, ids, func(tx *gorm.DB, lead *models.CrmLead) error {
		if err := mutableLead(*lead); err != nil {
			return err
		}
		if err := validateCustomerOwner(ctx, tx, toUserID); err != nil {
			return err
		}
		return tx.Model(lead).Updates(map[string]any{"owner_user_id": toUserID, "pool_status": models.LeadPoolNone, "pool_at": nil}).Error
	})
}
