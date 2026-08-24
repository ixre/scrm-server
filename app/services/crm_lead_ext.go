package services

import (
	"bytes"
	"core/common/identity"
	"strconv"

	"context"
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// LeadPoolQueryReq 线索池查询参数。
type LeadPoolQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
}

func (r LeadPoolQueryReq) pager() *app.Pager {
	p := &app.Pager{Page: r.Page, PageSize: r.PageSize}
	p.SetDefault()
	return p
}

type CrmLeadExtService struct{}

func NewCrmLeadExt() *CrmLeadExtService { return &CrmLeadExtService{} }

// MoveToPool 批量入线索池：PoolStatus=1, PoolAt=now。
func (s CrmLeadExtService) MoveToPool(ctx context.Context, ids []string) error {
	tenantID := identity.TenantID(ctx)

	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?", ids).
		Updates(map[string]any{"pool_status": 1, "pool_at": now, "owner_user_id": ""})
	if res.Error != nil {
		log.Sugar.Errorw("crm lead move to pool failed", "err", res.Error)
	}
	return res.Error
}

// ReceiveFromPool 从线索池领取：PoolStatus=0, OwnerUserID=userID。
func (s CrmLeadExtService) ReceiveFromPool(ctx context.Context, userID, id string) error {
	tenantID := identity.TenantID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND pool_status = ?", id, 1).
		Updates(map[string]any{"pool_status": 0, "pool_at": nil, "owner_user_id": userID})
	if res.Error != nil {
		log.Sugar.Errorw("crm lead receive from pool failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListPool 线索池列表（PoolStatus=1）。
func (s CrmLeadExtService) ListPool(ctx context.Context, req LeadPoolQueryReq) ([]models.CrmLead, int64, error) {

	p := req.pager()
	db := models.MustFor(ctx).Where("pool_status = ?", 1)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		like := "%" + kw + "%"
		db = db.Where("title LIKE ? OR name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmLead{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmLead
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("pool_at desc, id desc").Find(&items).Error
	return items, total, err
}

// Transfer 线索转移：改 OwnerUserID。
func (s CrmLeadExtService) Transfer(ctx context.Context, id, toUserID string) error {
	tenantID := identity.TenantID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	if toUserID == "" {
		return errors.New("to_user_id 不能为空")
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Update("owner_user_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("crm lead transfer failed", "err", res.Error)
	}
	return res.Error
}

// ConvertToCustomer 线索转客户：创建 CrmCustomer，更新 CrmLead 的 ConvertCustomerID/ConvertAt/Status。
// ownerUserID 指定客户归属人（为空则默认为当前操作人 userID）。
// 幂等：线索已转化（ConvertCustomerID 非空）则直接返回已有客户。
func (s CrmLeadExtService) ConvertToCustomer(ctx context.Context, userID, id, ownerUserID, remark string) (*models.CrmCustomer, error) {
	tenantID := identity.TenantID(ctx)

	// 归属人默认为操作人
	custOwner := ownerUserID
	if custOwner == "" {
		custOwner = userID
	}

	if id == "" {
		return nil, errors.New("id 不能为空")
	}
	var l models.CrmLead
	if err := models.MustFor(ctx).Where("id = ?", id).First(&l).Error; err != nil {
		return nil, err
	}
	// 已转化则直接返回已有客户。
	if l.ConvertCustomerID != "" {
		var c models.CrmCustomer
		if err := models.MustFor(ctx).Where("id = ?", l.ConvertCustomerID).First(&c).Error; err != nil {
			return nil, err
		}
		return &c, nil
	}
	// 去重：同手机号已有客户则直接关联。
	if l.Mobile != "" {
		var exist models.CrmCustomer
		if err := models.MustFor(ctx).Where("mobile = ?", l.Mobile).First(&exist).Error; err == nil {
			now := time.Now()
			models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
				Updates(map[string]any{"convert_customer_id": exist.ID, "convert_at": now, "status": "qualified", "convert_remark": remark})
			return &exist, nil
		}
	}
	cust := models.CrmCustomer{
		Model:       models.Model{ID: id_generator.StringID()},
		TenantID:    tenantID,
		Name:        l.Name,
		Company:     l.Company,
		Mobile:      l.Mobile,
		Email:       l.Email,
		Position:    l.Position,
		Address:     l.Address,
		Source:      l.Source,
		OwnerUserID: custOwner,
		Level:       1,
	}
	if err := createCRM(models.DB, &cust, "OwnerUserID", "ReassignFrom", "OpportunityID", "LeadID"); err != nil {
		log.Sugar.Errorw("crm lead convert: create customer failed", "err", err)
		return nil, err
	}
	now := time.Now()
	// 线索在池中：自动领取，保持线索与客户归属一致。
	updates := map[string]any{"convert_customer_id": cust.ID, "convert_at": now, "status": "qualified", "convert_remark": remark}
	if l.PoolStatus == 1 {
		updates["pool_status"] = 0
		updates["pool_at"] = nil
		updates["owner_user_id"] = custOwner
	}
	if err := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return &cust, err
	}
	// SOP 触发：新客户（与 CrmCustomerService.Create 保持一致）
	NewCrmSop().Trigger(ctx, "new_customer", map[string]string{
		"customer_id": cust.ID,
		"source":      cust.Source,
	})
	// SOP 触发：公海认领（线索从池中直接转化）
	if l.PoolStatus == 1 {
		NewCrmSop().Trigger(ctx, "pool_claim", map[string]string{
			"pool_type":   "lead",
			"customer_id": cust.ID,
		})
	}
	return &cust, nil
}

// MarkContacted 标记线索为已联系：Status=contacted。自动写入跟进记录。
func (s CrmLeadExtService) MarkContacted(ctx context.Context, userID, id, content string) error {
	tenantID := identity.TenantID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	var lead models.CrmLead
	if err := models.MustFor(ctx).Where("id = ?", id).First(&lead).Error; err != nil {
		return err
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).
		Where("id = ? AND status = ?", id, "new").
		Update("status", "contacted")
	if res.Error != nil {
		log.Sugar.Errorw("crm lead mark contacted failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("线索不存在或非新线索状态")
	}
	// 自动写入跟进记录
	s.recordFollowup(ctx, userID, tenantID, lead, "other", "标记已联系", "标记已联系："+content)
	return nil
}

// MarkLost 标记线索为已流失：Status=lost。自动写入跟进记录。
func (s CrmLeadExtService) MarkLost(ctx context.Context, userID, id, reason string) error {
	tenantID := identity.TenantID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	var lead models.CrmLead
	if err := models.MustFor(ctx).Where("id = ?", id).First(&lead).Error; err != nil {
		return err
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).
		Where("id = ? AND status IN ?", id, []string{"new", "contacted"}).
		Update("status", "lost")
	if res.Error != nil {
		log.Sugar.Errorw("crm lead mark lost failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("线索不存在或已转化/已流失")
	}
	s.recordFollowup(ctx, userID, tenantID, lead, "other", "标记流失", "标记流失："+reason)
	return nil
}

// recordFollowup 写入一条自动跟进记录。
func (s CrmLeadExtService) recordFollowup(ctx context.Context, userID, tenantID string, lead models.CrmLead, fType, result, content string) {
	f := models.CrmFollowup{
		Model:    models.Model{ID: id_generator.StringID()},
		TenantID: tenantID,
		LeadID:   lead.ID,
		UserID:   userID,
		Type:     fType,
		Content:  content,
		Result:   result,
	}
	if err := createCRM(models.DB, &f, "CustomerID", "LeadID", "OpportunityID", "UserID", "ContactID"); err != nil {
		log.Sugar.Errorw("crm lead status change followup create failed", "err", err)
	}
}

// Reopen 恢复已流失线索为可跟进：Status=contacted。自动写入跟进记录。
// 不允许恢复已转化（qualified）线索。
func (s CrmLeadExtService) Reopen(ctx context.Context, userID, id string) error {
	tenantID := identity.TenantID(ctx)

	if id == "" {
		return errors.New("id 不能为空")
	}
	var lead models.CrmLead
	if err := models.MustFor(ctx).Where("id = ?", id).First(&lead).Error; err != nil {
		return err
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).
		Where("id = ? AND status = ?", id, "lost").
		Update("status", "contacted")
	if res.Error != nil {
		log.Sugar.Errorw("crm lead reopen failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("线索不存在或非流失状态")
	}
	s.recordFollowup(ctx, userID, tenantID, lead, "other", "恢复跟进", "恢复为可跟进状态")
	return nil
}

// Export 导出线索为 Excel。
func (s CrmLeadExtService) Export(ctx context.Context, status, ownerUserID string) (*bytes.Buffer, string, error) {
	db := models.MustFor(ctx).Limit(10000).Order("id desc")
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
	tenantID := identity.TenantID(ctx)
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	if toUserID == "" {
		return 0, errors.New("to_user_id 不能为空")
	}
	res := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?", ids).
		Update("owner_user_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("crm lead batch transfer failed", "err", res.Error)
	}
	return res.RowsAffected, res.Error
}
