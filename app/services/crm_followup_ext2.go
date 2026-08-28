package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PageReq 通用分页请求（跟进视图等扩展列表复用）。
type PageReq struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Keyword  string `json:"keyword"`
}

func (r PageReq) pager() *app.Pager {
	p := &app.Pager{Page: r.Page, PageSize: r.PageSize}
	p.SetDefault()
	return p
}

// ---------- 跟进评论 ----------

type CrmFollowupCommentService struct{}

func NewCrmFollowupComment() *CrmFollowupCommentService { return &CrmFollowupCommentService{} }

// Create 创建跟进评论。
func (s CrmFollowupCommentService) Create(ctx context.Context, followupID, userID, content string) (*models.CrmFollowupComment, error) {
	if userID == "" || userID != identity.UserID(ctx) {
		return nil, ErrCustomerAccess
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, errors.New("评论内容不能为空")
	}
	c := &models.CrmFollowupComment{Model: models.Model{ID: id_generator.StringID()}, TenantID: identity.TenantID(ctx), FollowupID: followupID, UserID: userID, Content: content}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := authorizeFollowup(ctx, tx, followupID); err != nil {
			return err
		}
		return createCRM(tx, c)
	})
	return c, err
}

// ListByFollowup 按跟进记录列出评论。
func (s CrmFollowupCommentService) ListByFollowup(ctx context.Context, followupID string) ([]models.CrmFollowupComment, error) {
	if err := authorizeFollowup(ctx, models.DB, followupID); err != nil {
		return nil, err
	}
	var rows []models.CrmFollowupComment
	err := models.MustFor(ctx).Where("followup_id = ?", followupID).Order("created_at asc, id asc").Find(&rows).Error
	return rows, err
}

// Delete 删除跟进评论（仅评论人可删）。
func (s CrmFollowupCommentService) Delete(ctx context.Context, userID, id string) error {
	if userID == "" || userID != identity.UserID(ctx) {
		return ErrCustomerAccess
	}

	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.MustFor(ctx).Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.CrmFollowupComment{})
	if res.Error != nil {
		log.Sugar.Errorw("followup comment delete failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ---------- 常用备注（话术库） ----------

type CrmQuickRemarkService struct{}

func NewCrmQuickRemark() *CrmQuickRemarkService { return &CrmQuickRemarkService{} }

// Personal snippets are private; tenant-wide snippets can only be changed by managers.
func quickRemarkAccess(ctx context.Context, userID string, write bool) (*gorm.DB, error) {
	actor, manager, err := customerActor(ctx, models.DB)
	if err != nil {
		return nil, err
	}
	if userID != actor.ID {
		return nil, ErrCustomerAccess
	}
	q := models.DB.WithContext(ctx).Model(&models.CrmQuickRemark{}).Where("tenant_id = ?", actor.TenantID)
	if !write || manager {
		q = q.Where("(user_id = ? OR user_id IS NULL)", actor.ID)
	} else {
		q = q.Where("user_id = ?", actor.ID)
	}
	return q, nil
}
func (s CrmQuickRemarkService) Create(ctx context.Context, userID, content string, sort ...int) (*models.CrmQuickRemark, error) {
	if _, err := quickRemarkAccess(ctx, userID, true); err != nil {
		return nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("备注内容不能为空")
	}
	r := &models.CrmQuickRemark{Model: models.Model{ID: id_generator.StringID()}, TenantID: identity.TenantID(ctx), UserID: userID, Content: strings.TrimSpace(content)}
	if len(sort) > 0 {
		r.Sort = sort[0]
	}
	return r, createCRM(models.DB.WithContext(ctx), r)
}
func (s CrmQuickRemarkService) Update(ctx context.Context, userID, id, content string, sort ...int) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("备注内容不能为空")
	}
	q, err := quickRemarkAccess(ctx, userID, true)
	if err != nil {
		return err
	}
	changes := map[string]any{"content": strings.TrimSpace(content)}
	if len(sort) > 0 {
		changes["sort"] = sort[0]
	}
	res := q.Where("id = ?", id).Updates(changes)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s CrmQuickRemarkService) Delete(ctx context.Context, userID, id string) error {
	q, err := quickRemarkAccess(ctx, userID, true)
	if err != nil {
		return err
	}
	res := q.Where("id = ?", id).Delete(&models.CrmQuickRemark{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
func (s CrmQuickRemarkService) List(ctx context.Context, userID string) ([]models.CrmQuickRemark, error) {
	q, err := quickRemarkAccess(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	var items []models.CrmQuickRemark
	err = q.Order("sort desc, id desc").Limit(1000).Find(&items).Error
	return items, err
}
func (s CrmQuickRemarkService) SetSort(ctx context.Context, id string, sort int) error {
	q, err := quickRemarkAccess(ctx, identity.UserID(ctx), true)
	if err != nil {
		return err
	}
	res := q.Where("id = ?", id).Update("sort", sort)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ---------- 跟进待跟进视图 ----------

type CrmFollowupViewService struct{}

func NewCrmFollowupView() *CrmFollowupViewService { return &CrmFollowupViewService{} }

// ListTodayTodo includes overdue appointments and every appointment today.
// A followup earlier today must not hide a second appointment later the same day.
func (s CrmFollowupViewService) ListTodayTodo(ctx context.Context, userID string, req PageReq) ([]models.CrmCustomer, int64, error) {
	db, err := actionableFollowupCustomers(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	_, end := followupDay(time.Now())
	return queryFollowupCustomers(db.Where("next_followup_at < ?", end), req, "next_followup_at asc, id desc")
}

type FollowupDoneItem struct {
	models.CrmFollowup
	CustomerName string `json:"customer_name"`
}

func (s CrmFollowupViewService) ListTodayDone(ctx context.Context, userID string, req PageReq) ([]FollowupDoneItem, int64, error) {
	actor, err := todoActor(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	start, end := followupDay(time.Now())
	// Current access matters even for notes authored before a customer transfer.
	access, err := customerAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	customers := access.Model(&models.CrmCustomer{}).Select("id")
	leads, err := salesAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	leadIDs := leads.Model(&models.CrmLead{}).Select("id")
	opportunities, err := salesAccessDB(ctx, models.DB, false)
	if err != nil {
		return nil, 0, err
	}
	opportunityIDs := opportunities.Model(&models.CrmOpportunity{}).Select("id")
	db := models.DB.WithContext(ctx).Table("crm_followup AS f").
		Joins("LEFT JOIN crm_customer c ON c.id = f.customer_id AND c.tenant_id = f.tenant_id AND c.deleted_at IS NULL").
		Where("f.tenant_id = ? AND f.user_id = ? AND f.deleted_at IS NULL AND f.created_at >= ? AND f.created_at < ?", actor.TenantID, userID, start, end).
		Where("(f.customer_id IN (?) OR (f.customer_id IS NULL AND (f.lead_id IN (?) OR f.opportunity_id IN (?))))", customers, leadIDs, opportunityIDs)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		db = db.Where("(c.name ILIKE ? OR f.content ILIKE ?)", "%"+kw+"%", "%"+kw+"%")
	}
	var total int64
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []FollowupDoneItem
	p := req.pager()
	err = db.Select("f.*, c.name AS customer_name").Order("f.created_at desc, f.id desc").Offset(p.GetOffset()).Limit(p.GetLimit()).Scan(&items).Error
	return items, total, err
}

func (s CrmFollowupViewService) ListNeverFollowup(ctx context.Context, userID string, req PageReq) ([]models.CrmCustomer, int64, error) {
	db, err := actionableFollowupCustomers(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	db = db.Where(`NOT EXISTS (SELECT 1 FROM crm_followup f WHERE f.tenant_id = crm_customer.tenant_id AND f.customer_id = crm_customer.id AND f.deleted_at IS NULL)`)
	return queryFollowupCustomers(db, req, "created_at desc, id desc")
}

func actionableFollowupCustomers(ctx context.Context, userID string) (*gorm.DB, error) {
	actor, err := todoActor(ctx, userID)
	if err != nil {
		return nil, err
	}
	return models.DB.WithContext(ctx).Model(&models.CrmCustomer{}).
		Where("tenant_id = ? AND owner_user_id = ? AND pool_status = 0 AND is_blacklist = false AND reassign_status = 0", actor.TenantID, actor.ID), nil
}

func queryFollowupCustomers(db *gorm.DB, req PageReq, order string) ([]models.CrmCustomer, int64, error) {
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		db = db.Where("(name ILIKE ? OR mobile ILIKE ? OR company ILIKE ?)", "%"+kw+"%", "%"+kw+"%", "%"+kw+"%")
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	p := req.pager()
	err := db.Order(order).Offset(p.GetOffset()).Limit(p.GetLimit()).Find(&items).Error
	return items, total, err
}

// Comments inherit the current parent record's visibility, including shared customer access.
func authorizeFollowup(ctx context.Context, tx *gorm.DB, id string) error {
	var f models.CrmFollowup
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ? AND id = ?", identity.TenantID(ctx), id).First(&f).Error; err != nil {
		return errors.New("跟进不存在或已删除")
	}
	var model any
	var parent string
	var db *gorm.DB
	var err error
	if f.CustomerID != "" {
		model = &models.CrmCustomer{}
		parent = f.CustomerID
		db, err = customerAccessDB(ctx, tx, false)
	} else if f.LeadID != "" {
		model = &models.CrmLead{}
		parent = f.LeadID
		db, err = salesAccessDB(ctx, tx, false)
	} else if f.OpportunityID != "" {
		model = &models.CrmOpportunity{}
		parent = f.OpportunityID
		db, err = salesAccessDB(ctx, tx, false)
	} else {
		return ErrCustomerAccess
	}
	if err != nil {
		return err
	}
	if err = db.Where("id = ?", parent).First(model).Error; err != nil {
		return ErrCustomerAccess
	}
	return nil
}
