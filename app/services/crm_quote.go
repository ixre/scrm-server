package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"
)

// ---------- 报价单 ----------

type CrmQuoteService struct{}

func NewCrmQuote() *CrmQuoteService { return &CrmQuoteService{} }

// QuoteQueryReq 报价单查询请求。
type QuoteQueryReq struct {
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
	CustomerID string `json:"customer_id"`
	Status     string `json:"status"`
	Keyword    string `json:"keyword"`
}

// quoteStatusFlow 允许的状态流转：from -> [to...]
var quoteStatusFlow = map[string]map[string]bool{
	"draft":    {"sent": true, "void": true},
	"sent":     {"accepted": true, "rejected": true, "void": true},
	"accepted": {},
	"rejected": {},
	"void":     {},
}

// genQuoteNo 生成报价单号：Q + yyyyMMdd + 4位当日序号。
func genQuoteNo(tenantID string) string {
	day := time.Now().Format("20060102")
	prefix := "Q" + day
	var cnt int64
	models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("quote_no LIKE ?",  prefix+"%").Count(&cnt)
	return fmt.Sprintf("%s%04d", prefix, cnt+1)
}

// Create 创建报价单，自动生成 QuoteNo。
func (s CrmQuoteService) Create(ctx context.Context, q *models.CrmQuote) error {
	tenantID := identity.TenantID(ctx)


	if q == nil {
		return errors.New("报价单不能为空")
	}
	q.ID = id_generator.StringID()
	q.TenantID = tenantID
	if q.Status == "" {
		q.Status = "draft"
	}
	if q.QuoteNo == "" {
		q.QuoteNo = genQuoteNo(tenantID)
	}
	// 最终金额未传则按金额-折扣计算。
	if q.FinalAmount == 0 && q.Amount > 0 {
		q.FinalAmount = q.Amount - q.Discount
	}
	if err := createCRM(models.DB, q, "CustomerID", "ContactID", "OpportunityID", "OwnerUserID"); err != nil {
		log.Sugar.Errorw("crm quote create failed", "err", err)
		return err
	}
	return nil
}

// Update 更新报价单（仅 draft 状态可改业务字段）。
func (s CrmQuoteService) Update(ctx context.Context, id string, q *models.CrmQuote) error {
	tenantID := identity.TenantID(ctx)


	if q == nil {
		return errors.New("报价单不能为空")
	}
	var exist models.CrmQuote
	if err := models.MustFor(ctx).Where("id = ?", id).First(&exist).Error; err != nil {
		return errors.New("报价单不存在")
	}
	if exist.Status != "draft" {
		return errors.New("非草稿状态报价单不可编辑")
	}
	upd := updatesCRM(map[string]any{
		"customer_id":    q.CustomerID,
		"contact_id":     q.ContactID,
		"opportunity_id": q.OpportunityID,
		"subject":        q.Subject,
		"amount":         q.Amount,
		"discount":       q.Discount,
		"final_amount":   q.FinalAmount,
		"items":          q.Items,
		"valid_until":    q.ValidUntil,
		"owner_user_id":  q.OwnerUserID,
		"remark":         q.Remark,
	}, "customer_id", "contact_id", "opportunity_id", "owner_user_id")
	selectCols := []string{
		"customer_id", "contact_id", "opportunity_id", "subject", "amount",
		"discount", "final_amount", "items", "valid_until", "owner_user_id", "remark",
	}
	if err := models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Select(selectCols).Updates(upd).Error; err != nil {
		log.Sugar.Errorw("crm quote update failed", "err", err)
		return err
	}
	return nil
}

// Delete 删除报价单。
func (s CrmQuoteService) Delete(ctx context.Context, ids []string) (int64, error) {


	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmQuote{})
	return res.RowsAffected, res.Error
}

// Get 查单个报价单。
func (s CrmQuoteService) Get(ctx context.Context, id string) (*models.CrmQuote, error) {


	var q models.CrmQuote
	if err := models.MustFor(ctx).Where("id = ?", id).First(&q).Error; err != nil {
		return nil, err
	}
	return &q, nil
}

// Query 列出报价单，支持按 customer_id/status/keyword 筛选。
func (s CrmQuoteService) Query(ctx context.Context, req QuoteQueryReq) ([]models.CrmQuote, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 10
	}
	db := models.MustFor(ctx)
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if req.Status != "" {
		db = db.Where("status = ?", req.Status)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		db = db.Where("subject LIKE ? OR quote_no LIKE ?", like, like)
	}
	var total int64
	if err := db.Model(&models.CrmQuote{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmQuote
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("id desc").Find(&items).Error
	return items, total, err
}

// ChangeStatus 修改报价单状态，校验状态流转：
// draft→sent→accepted/rejected, draft/sent→void。
func (s CrmQuoteService) ChangeStatus(ctx context.Context, id, status string) error {
	tenantID := identity.TenantID(ctx)


	if _, ok := quoteStatusFlow[status]; !ok {
		return errors.New("无效的报价单状态")
	}
	var q models.CrmQuote
	if err := models.MustFor(ctx).Where("id = ?", id).First(&q).Error; err != nil {
		return errors.New("报价单不存在")
	}
	if q.Status == status {
		return nil
	}
	transitions, ok := quoteStatusFlow[q.Status]
	if !ok || !transitions[status] {
		return fmt.Errorf("不允许从 %s 流转到 %s", q.Status, status)
	}
	if err := models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Update("status", status).Error; err != nil {
		log.Sugar.Errorw("crm quote change status failed", "err", err)
		return err
	}
	return nil
}

// Transfer 转移报价单负责人。
func (s CrmQuoteService) Transfer(ctx context.Context, id, toUserID string) error {
	tenantID := identity.TenantID(ctx)


	if toUserID == "" {
		return errors.New("目标负责人不能为空")
	}
	res := models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Update("owner_user_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("crm quote transfer failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("报价单不存在")
	}
	return nil
}
