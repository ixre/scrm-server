package services

import (
	"core/common/identity"

	"context"

	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------- H5 落地页 ----------

type CrmLandingPageService struct{}

func NewCrmLandingPage() *CrmLandingPageService { return &CrmLandingPageService{} }

// LandingPageCreateReq 落地页创建请求。
type LandingPageCreateReq struct {
	Page models.CrmLandingPage `json:"page"`
}

// Create 创建落地页。Slug 自动生成若留空。
func (s CrmLandingPageService) Create(ctx context.Context, req models.CrmLandingPage) (models.CrmLandingPage, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Slug == "" {
		var err error
		req.Slug, err = newPublicCode()
		if err != nil {
			return req, err
		}
	}
	if strings.TrimSpace(req.Title) == "" || !publicSlug.MatchString(req.Slug) || req.Status != "active" && req.Status != "disabled" {
		return req, errors.New("请填写标题、有效链接标识与状态")
	}
	if req.ActivityID != "" {
		var activity models.CrmActivity
		if err := models.MustFor(ctx).Where("id = ?", req.ActivityID).First(&activity).Error; err != nil {
			return req, errors.New("关联活动不存在")
		}
	}
	if len(req.FormFields) == 0 {
		req.FormFields = constants.StringArrayField{"name", "mobile"}
	}
	if err := createCRM(models.DB, &req, "ActivityID"); err != nil {
		log.Sugar.Errorw("landing page create failed", "err", err)
		return req, err
	}
	return req, nil
}

// Query 列出落地页。
func (s CrmLandingPageService) Query(ctx context.Context, activityID string, pager *app.Pager) ([]models.CrmLandingPage, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if activityID != "" {
		db = db.Where("activity_id = ?", activityID)
	}
	var total int64
	if err := db.Model(&models.CrmLandingPage{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmLandingPage
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// GetBySlug 按 slug 查落地页（公开访问，不要求登录）。
func (s CrmLandingPageService) GetBySlug(slug string) (models.CrmLandingPage, error) {
	var p models.CrmLandingPage
	if !publicSlug.MatchString(slug) {
		return p, errors.New("页面标识无效")
	}
	// 公开入口按 slug 反查落地页及其租户：显式跨租户。
	if err := models.CrossTenant().Where("slug = ? AND status = ?", slug, "active").First(&p).Error; err != nil {
		return p, err
	}
	if err := validatePublicTenant(models.DB, p.TenantID, "marketing_landing"); err != nil {
		return models.CrmLandingPage{}, err
	}
	return p, nil
}

// Get 查单页。
func (s CrmLandingPageService) Get(ctx context.Context, id string) (models.CrmLandingPage, error) {

	var p models.CrmLandingPage
	err := models.MustFor(ctx).Where("id = ?", id).First(&p).Error
	return p, err
}

// Delete 删除落地页。
func (s CrmLandingPageService) Delete(ctx context.Context, ids []string) (int64, error) {

	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmLandingPage{})
	return res.RowsAffected, res.Error
}

// SubmitForm 落地页表单提交：创建/关联客户 + 写活动参与记录 + 自动建线索。
// formData 为表单字段键值对（name/mobile/company 等）。
func (s CrmLandingPageService) SubmitForm(slug string, formData map[string]string) (models.CrmCustomer, error) {
	var customer models.CrmCustomer
	name, mobile, err := validateAcquisitionContact(formData["name"], formData["mobile"])
	if err != nil {
		return customer, err
	}
	company := strings.TrimSpace(formData["company"])
	if utf8.RuneCountInString(company) > 255 {
		return customer, errors.New("公司名称过长")
	}
	page, err := s.GetBySlug(slug)
	if err != nil {
		return customer, errors.New("落地页不存在或已停用")
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: page.TenantID})
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND tenant_id = ? AND status = ?", page.ID, page.TenantID, "active").First(&page).Error; err != nil {
			return err
		}
		if err := validatePublicTenant(tx, page.TenantID, "marketing_landing", "customer", "lead"); err != nil {
			return err
		}
		if err := lockSalesKey(ctx, tx, "lead-conversion-mobile", mobile); err != nil {
			return err
		}
		var matches []models.CrmCustomer
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ? AND mobile = ?", page.TenantID, mobile).Limit(2).Find(&matches).Error; err != nil {
			return err
		}
		if len(matches) > 1 {
			return errors.New("暂时无法提交，请联系商家处理客户资料")
		}
		now := time.Now()
		if len(matches) == 1 {
			customer = matches[0]
		} else {
			customer = models.CrmCustomer{Model: models.Model{ID: id_generator.StringID()}, TenantID: page.TenantID, Name: name, Mobile: mobile, Company: company, Source: constants.SourceWebForm, PoolStatus: models.CustomerPoolIn, PoolAt: &now, Level: 1}
			if err := createCRM(tx, &customer); err != nil {
				return err
			}
			if err := allocateNewCustomer(ctx, tx, &customer); err != nil {
				return err
			}
		}
		if page.ActivityID != "" {
			var activity models.CrmActivity
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", page.TenantID, page.ActivityID).First(&activity).Error; err != nil {
				return errors.New("关联活动不可用")
			}
			// Preserve a stronger converted record on repeat participation.
			rec := models.CrmActivityRecord{Model: models.Model{ID: id_generator.StringID()}, TenantID: page.TenantID, ActivityID: page.ActivityID, CustomerID: customer.ID, Channel: "landing_page", RecordType: "participated", HappenedAt: &now}
			if err := createCRM(tx.Clauses(clause.OnConflict{DoNothing: true}), &rec); err != nil {
				return err
			}
			if err := recomputeActivityCounters(tx, page.TenantID, page.ActivityID); err != nil {
				return err
			}
		}
		var count int64
		title := fmt.Sprintf("落地页[%s]留资", page.Title)
		if err := tx.Model(&models.CrmLead{}).Where("tenant_id = ? AND customer_id = ? AND source = ? AND title = ? AND status IN ?", page.TenantID, customer.ID, constants.SourceWebForm, title, []string{"new", "contacted"}).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			lead := models.CrmLead{Model: models.Model{ID: id_generator.StringID()}, TenantID: page.TenantID, CustomerID: customer.ID, OwnerUserID: customer.OwnerUserID, Title: title, Name: name, Mobile: mobile, Company: company, Source: constants.SourceWebForm, Status: "new"}
			if lead.OwnerUserID == "" || customer.InPool() {
				lead.OwnerUserID = ""
				lead.PoolStatus = 1
				lead.PoolAt = &now
			}
			if err := createCRM(tx, &lead); err != nil {
				return err
			}
		}
		return tx.Model(&page).UpdateColumn("submit_count", gorm.Expr("submit_count + 1")).Error
	})
	return customer, err
}
