package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strings"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"

	"gorm.io/gorm"
)

type CrmSmartCardService struct{}

func NewCrmSmartCard() *CrmSmartCardService { return &CrmSmartCardService{} }

func (s CrmSmartCardService) Query(ctx context.Context, userID string, p *app.Pager) ([]models.CrmSmartCard, int64, error) {

	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if userID != "" {
		db = db.Where("user_id = ?", userID)
	}
	var total int64
	if err := db.Model(&models.CrmSmartCard{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSmartCard
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmSmartCardService) Get(ctx context.Context, id string) (models.CrmSmartCard, error) {
	tenantID := identity.TenantID(ctx)

	var item models.CrmSmartCard
	db := models.DB.Where("id = ?", id)
	if tenantID != "" {
		db = db.Scopes(models.TenantScope(tenantID))
	}
	err := db.First(&item).Error
	return item, err
}

// GetByShareCode 通过分享码查看名片（公开访问）。
func (s CrmSmartCardService) GetByShareCode(shareCode string) (models.CrmSmartCard, error) {
	if shareCode == "" {
		return models.CrmSmartCard{}, errors.New("share_code 不能为空")
	}
	var item models.CrmSmartCard
	err := models.DB.Where("share_code = ? AND status = ?", shareCode, "active").First(&item).Error
	return item, err
}

// GetByID 按 ID 公开访问名片（无租户过滤，用于分享短链外的直接 ID 访问）。
func (s CrmSmartCardService) GetByID(id string) (models.CrmSmartCard, error) {
	if id == "" {
		return models.CrmSmartCard{}, errors.New("id 不能为空")
	}
	var item models.CrmSmartCard
	err := models.DB.Where("id = ? AND status = ?", id, "active").First(&item).Error
	return item, err
}

func (s CrmSmartCardService) Create(ctx context.Context, req models.CrmSmartCard) (models.CrmSmartCard, error) {
	tenantID := identity.TenantID(ctx)

	if strings.TrimSpace(req.Name) == "" {
		return req, errors.New("姓名不能为空")
	}
	if req.UserID == "" {
		return req, errors.New("user_id 不能为空")
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	req.Name = strings.TrimSpace(req.Name)
	// 生成分享码（8位短码）
	req.ShareCode = id_generator.StringID()[:8]
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmSmartCardService) Update(ctx context.Context, id string, req models.CrmSmartCard) (models.CrmSmartCard, error) {

	if err := models.MustFor(ctx).Where("id = ?", id).Updates(map[string]any{
		"name":          req.Name,
		"title":         req.Title,
		"company":       req.Company,
		"company_intro": req.CompanyIntro,
		"address":       req.Address,
		"phone":         req.Phone,
		"email":         req.Email,
		"avatar":        req.Avatar,
		"intro":         req.Intro,
		"tags":          req.Tags,
		"products":      req.Products,
		"cases":         req.Cases,
		"wechat_qr":     req.WechatQR,
		"status":        req.Status,
	}).Error; err != nil {
		return req, err
	}
	return s.Get(ctx, id)
}

func (s CrmSmartCardService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmSmartCard{})
	return res.RowsAffected, res.Error
}

// RecordView 记录名片浏览（访客轨迹），事务保证浏览数+1和记录写入一致。
func (s CrmSmartCardService) RecordView(ctx context.Context, cardID, visitorID, ip, ua, source string, duration int) error {
	tenantID := identity.TenantID(ctx)

	rec := models.CrmSmartCardView{
		TenantID:  tenantID,
		CardID:    cardID,
		VisitorID: visitorID,
		VisitorIP: ip,
		VisitorUA: ua,
		Source:    source,
		Duration:  duration,
	}
	rec.ID = id_generator.StringID()
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.CrmSmartCard{}).Where("id = ?", cardID).UpdateColumn("view_count", gorm.Expr("view_count + 1")).Error; err != nil {
			return err
		}
		return createCRM(tx, &rec, "VisitorID")
	})
}

func (s CrmSmartCardService) ListViews(ctx context.Context, cardID string, p *app.Pager) ([]models.CrmSmartCardView, int64, error) {

	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if cardID != "" {
		db = db.Where("card_id = ?", cardID)
	}
	var total int64
	if err := db.Model(&models.CrmSmartCardView{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSmartCardView
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// IncrementShare 分享数+1。
func (s CrmSmartCardService) IncrementShare(cardID string) error {
	return models.DB.Model(&models.CrmSmartCard{}).Where("id = ?", cardID).UpdateColumn("share_count", gorm.Expr("share_count + 1")).Error
}

// SmartCardStats 智能名片统计概览。
type SmartCardStats struct {
	TotalCards    int64 `json:"total_cards"`
	TotalViews    int64 `json:"total_views"`
	TotalShares   int64 `json:"total_shares"`
	TodayVisitors int64 `json:"today_visitors"`
}

// Stats 统计概览（按当前用户过滤，管理员可看全部）。
func (s CrmSmartCardService) Stats(ctx context.Context, userID string) (SmartCardStats, error) {
	tenantID := identity.TenantID(ctx)
	var stats SmartCardStats
	cardDB := models.DB.Model(&models.CrmSmartCard{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		cardDB = cardDB.Where("user_id = ?", userID)
	}
	if err := cardDB.
		Select("COUNT(*) as total_cards, COALESCE(SUM(view_count),0) as total_views, COALESCE(SUM(share_count),0) as total_shares").
		Scan(&stats).Error; err != nil {
		return stats, err
	}
	// 今日访客
	viewDB := models.DB.Model(&models.CrmSmartCardView{}).Where("tenant_id = ?", tenantID)
	if userID != "" {
		viewDB = viewDB.Where("card_id IN (SELECT id FROM crm_smart_card WHERE user_id = ? AND tenant_id = ?)", userID, tenantID)
	}
	if err := viewDB.Where("created_at >= CURRENT_DATE").Count(&stats.TodayVisitors).Error; err != nil {
		return stats, err
	}
	return stats, nil
}
