package services

import (
	"errors"
	"fmt"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- H5 落地页 ----------

type CrmLandingPageService struct{}

func NewCrmLandingPage() *CrmLandingPageService { return &CrmLandingPageService{} }

// LandingPageCreateReq 落地页创建请求。
type LandingPageCreateReq struct {
	Page models.CrmLandingPage `json:"page"`
}

// Create 创建落地页。Slug 自动生成若留空。
func (s CrmLandingPageService) Create(tenantID string, req models.CrmLandingPage) (models.CrmLandingPage, error) {
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.Slug == "" {
		req.Slug = genLandingSlug()
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
func (s CrmLandingPageService) Query(tenantID, activityID string, pager *app.Pager) ([]models.CrmLandingPage, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Scopes(models.TenantScope(tenantID))
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
	err := models.DB.Where("slug = ? AND status = ?", slug, "active").First(&p).Error
	return p, err
}

// Get 查单页。
func (s CrmLandingPageService) Get(tenantID, id string) (models.CrmLandingPage, error) {
	var p models.CrmLandingPage
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&p).Error
	return p, err
}

// Delete 删除落地页。
func (s CrmLandingPageService) Delete(tenantID string, ids []string) (int64, error) {
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?", ids).Delete(&models.CrmLandingPage{})
	return res.RowsAffected, res.Error
}

// SubmitForm 落地页表单提交：创建/关联客户 + 写活动参与记录 + 自动建线索。
// formData 为表单字段键值对（name/mobile/company 等）。
func (s CrmLandingPageService) SubmitForm(slug string, formData map[string]string) (models.CrmCustomer, error) {
	page, err := s.GetBySlug(slug)
	if err != nil {
		return models.CrmCustomer{}, errors.New("落地页不存在或已停用")
	}
	name := formData["name"]
	mobile := formData["mobile"]
	company := formData["company"]

	// 去重：同手机号已有客户则关联
	var cust models.CrmCustomer
	if mobile != "" {
		models.DB.Scopes(models.TenantScope(page.TenantID)).Where("mobile = ?", mobile).First(&cust)
	}
	var isNew bool
	if cust.ID == "" {
		isNew = true
		cust = models.CrmCustomer{
			Model:      models.Model{ID: id_generator.StringID()},
			TenantID:   page.TenantID,
			Name:       name,
			Mobile:     mobile,
			Company:    company,
			Source:     constants.SourceWebForm, // 落地页表单留资归入 web_form 来源
			PoolStatus: 1,
		}
		now := time.Now()
		cust.PoolAt = &now
		if err := createCRM(models.DB, &cust, "OwnerUserID", "ReassignFrom"); err != nil {
			log.Sugar.Errorw("landing page form: create customer failed", "err", err)
			return cust, err
		}
	} else {
		// 更新非空字段
		updates := map[string]any{}
		if name != "" && cust.Name == "" {
			updates["name"] = name
		}
		if company != "" && cust.Company == "" {
			updates["company"] = company
		}
		if len(updates) > 0 {
			models.DB.Model(&models.CrmCustomer{}).Where("id = ?", cust.ID).Updates(updates)
		}
	}

	// 写活动参与记录（participated）
	if page.ActivityID != "" {
		now := time.Now()
		rec := models.CrmActivityRecord{
			Model:      models.Model{ID: id_generator.StringID()},
			TenantID:   page.TenantID,
			ActivityID: page.ActivityID,
			CustomerID: cust.ID,
			Channel:    "landing_page",
			RecordType: "participated",
			HappenedAt: &now,
		}
		models.DB.Create(&rec)
		// 重算活动计数
		NewCrmActivity().recomputeActivityCounters(page.TenantID, page.ActivityID)
	}

	// 自动建线索
	if isNew {
		lead := models.CrmLead{
			Model:    models.Model{ID: id_generator.StringID()},
			TenantID: page.TenantID,
			Title:    fmt.Sprintf("落地页[%s]留资", page.Title),
			Name:     name,
			Mobile:   mobile,
			Company:  company,
			Source:   "landing_page",
			Status:   "new",
		}
		if err := createCRM(models.DB, &lead, "OwnerUserID", "CustomerID"); err != nil {
			log.Sugar.Errorw("landing page form: create lead failed", "err", err)
		} else {
			// 回写客户ID到线索
			models.DB.Model(&models.CrmLead{}).Where("id = ?", lead.ID).Update("customer_id", cust.ID)
		}
	}

	// 落地页提交计数 +1
	models.DB.Model(&models.CrmLandingPage{}).Where("id = ?", page.ID).
		UpdateColumn("submit_count", gorm.Expr("submit_count + 1"))

	return cust, nil
}

// genLandingSlug 生成 8 位 slug。
func genLandingSlug() string {
	return id_generator.StringID()[:8]
}
