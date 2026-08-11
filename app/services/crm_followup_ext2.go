package services

import (
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// PageReq 通用分页请求（跟进视图等扩展列表复用）。
type PageReq struct {
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
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
func (s CrmFollowupCommentService) Create(tenantID, followupID, userID, content string) (*models.CrmFollowupComment, error) {
	if followupID == "" {
		return nil, errors.New("followup_id 不能为空")
	}
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("评论内容不能为空")
	}
	c := &models.CrmFollowupComment{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		FollowupID: followupID,
		UserID:     userID,
		Content:    strings.TrimSpace(content),
	}
	if err := createCRM(models.DB, c, "FollowupID", "UserID"); err != nil {
		log.Sugar.Errorw("followup comment create failed", "err", err)
		return nil, err
	}
	return c, nil
}

// ListByFollowup 按跟进记录列出评论。
func (s CrmFollowupCommentService) ListByFollowup(tenantID, followupID string) ([]models.CrmFollowupComment, error) {
	var items []models.CrmFollowupComment
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("followup_id = ?",  followupID).
		Order("created_at asc, id asc").Find(&items).Error
	return items, err
}

// Delete 删除跟进评论（仅评论人可删）。
func (s CrmFollowupCommentService) Delete(tenantID, userID, id string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ? AND user_id = ?",  id,  userID).
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

// Create 创建常用备注。UserID 为空表示全员共用。
func (s CrmQuickRemarkService) Create(tenantID, userID, content string) (*models.CrmQuickRemark, error) {
	if strings.TrimSpace(content) == "" {
		return nil, errors.New("备注内容不能为空")
	}
	r := &models.CrmQuickRemark{
		Model:    models.Model{ID: id_generator.StringID()},
		TenantID: tenantID,
		UserID:   userID,
		Content:  strings.TrimSpace(content),
	}
	if err := createCRM(models.DB, r, "UserID"); err != nil {
		log.Sugar.Errorw("quick remark create failed", "err", err)
		return nil, err
	}
	return r, nil
}

// Update 更新常用备注（仅本人可改自己的备注；全员共用备注需管理员，此处简化为创建人/空归属可改）。
func (s CrmQuickRemarkService) Update(tenantID, userID, id, content string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	if strings.TrimSpace(content) == "" {
		return errors.New("备注内容不能为空")
	}
	// 个人备注仅本人可改；全员共用备注（user_id 为空）任意登录用户可改。
	q := models.DB.Model(&models.CrmQuickRemark{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id)
	if userID != "" {
		q = q.Where("user_id = ? OR user_id = ''", userID)
	}
	res := q.Updates(map[string]any{"content": strings.TrimSpace(content)})
	if res.Error != nil {
		log.Sugar.Errorw("quick remark update failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// Delete 删除常用备注。
func (s CrmQuickRemarkService) Delete(tenantID, userID, id string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	q := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?",  id)
	if userID != "" {
		q = q.Where("user_id = ? OR user_id = ''", userID)
	}
	res := q.Delete(&models.CrmQuickRemark{})
	if res.Error != nil {
		log.Sugar.Errorw("quick remark delete failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// List 列出常用备注：返回个人 + 全员共用的备注（user_id = userID OR user_id = ''）。
func (s CrmQuickRemarkService) List(tenantID, userID string) ([]models.CrmQuickRemark, error) {
	var items []models.CrmQuickRemark
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("(user_id = ? OR user_id = '')",  userID).
		Order("sort desc, id desc").Find(&items).Error
	return items, err
}

// SetSort 设置常用备注排序。
func (s CrmQuickRemarkService) SetSort(tenantID, id string, sort int) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	res := models.DB.Model(&models.CrmQuickRemark{}).Scopes(models.TenantScope(tenantID)).Where("id = ?",  id).Update("sort", sort)
	if res.Error != nil {
		log.Sugar.Errorw("quick remark set sort failed", "err", res.Error)
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

// ListTodayTodo 今日待跟进客户：归属当前用户、NextFollowupAt <= today 且今日尚无跟进记录。
func (s CrmFollowupViewService) ListTodayTodo(tenantID, userID string, req PageReq) ([]models.CrmCustomer, int64, error) {
	p := req.pager()
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	// 今日已有跟进的客户 ID 集合。
	var doneIDs []string
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND created_at >= ?",  userID,  start).
		Distinct("customer_id").Pluck("customer_id", &doneIDs)
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ? AND next_followup_at <= ?",  userID,  start)
	if len(doneIDs) > 0 {
		db = db.Where("id NOT IN ?", doneIDs)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("next_followup_at asc, id desc").Find(&items).Error
	return items, total, err
}

// ListTodayDone 今日已跟进：当前用户今日创建的跟进记录。
func (s CrmFollowupViewService) ListTodayDone(tenantID, userID string, req PageReq) ([]models.CrmFollowup, int64, error) {
	p := req.pager()
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND created_at >= ?",  userID,  start)
	var total int64
	if err := db.Model(&models.CrmFollowup{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmFollowup
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("created_at desc, id desc").Find(&items).Error
	return items, total, err
}

// ListNeverFollowup 从未跟进的客户：归属当前用户且无任何跟进记录。
func (s CrmFollowupViewService) ListNeverFollowup(tenantID, userID string, req PageReq) ([]models.CrmCustomer, int64, error) {
	p := req.pager()
	// 有跟进记录的客户 ID 集合。
	var hasIDs []string
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).Distinct("customer_id").Pluck("customer_id", &hasIDs)
	db := models.DB.Scopes(models.TenantScope(tenantID)).Where("owner_user_id = ?",  userID)
	if len(hasIDs) > 0 {
		db = db.Where("id NOT IN ?", hasIDs)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomer
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).
		Order("created_at desc, id desc").Find(&items).Error
	return items, total, err
}
