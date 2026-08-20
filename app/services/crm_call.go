package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"
)

// CallRecordQueryReq 通话记录查询参数。
type CallRecordQueryReq struct {
	Page       int
	PageSize   int
	UserID     string
	CustomerID string
	Direction  string
	Status     string
}

// CrmCallRecordService 通话记录服务。
type CrmCallRecordService struct{}

func NewCrmCallRecord() *CrmCallRecordService { return &CrmCallRecordService{} }

// Create 创建通话记录。
func (s CrmCallRecordService) Create(ctx context.Context, r *models.CrmCallRecord) error {
	tenantID := identity.TenantID(ctx)


	if r == nil {
		return errors.New("通话记录不能为空")
	}
	r.ID = id_generator.StringID()
	r.TenantID = tenantID
	if r.StartAt == nil {
		now := time.Now()
		r.StartAt = &now
	}
	return createCRM(models.DB, r, "UserID", "CustomerID", "ContactID")
}

// Update 更新通话记录。
func (s CrmCallRecordService) Update(ctx context.Context, id string, r *models.CrmCallRecord) error {


	if r == nil {
		return errors.New("通话记录不能为空")
	}
	m := map[string]any{
		"direction":     r.Direction,
		"phone":         r.Phone,
		"duration":      r.Duration,
		"recording_url": r.RecordingURL,
		"status":        r.Status,
		"remark":        r.Remark,
	}
	updatesCRM(m, "user_id", "customer_id", "contact_id")
	return models.MustFor(ctx).Where("id = ?",  id).Updates(m).Error
}

// Delete 删除通话记录。
func (s CrmCallRecordService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmCallRecord{})
	return res.RowsAffected, res.Error
}

// Query 分页查询通话记录。
func (s CrmCallRecordService) Query(ctx context.Context, req CallRecordQueryReq) ([]models.CrmCallRecord, int64, error) {


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if req.UserID != "" {
		db = db.Where("user_id = ?", req.UserID)
	}
	if req.CustomerID != "" {
		db = db.Where("customer_id = ?", req.CustomerID)
	}
	if d := strings.TrimSpace(req.Direction); d != "" {
		db = db.Where("direction = ?", d)
	}
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmCallRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCallRecord
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// Stats 通话统计（总通话数、总时长、已接通数）。
func (s CrmCallRecordService) Stats(ctx context.Context) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	var totalCount int64
	if err := models.DB.Model(&models.CrmCallRecord{}).Scopes(models.TenantScope(tenantID)).Count(&totalCount).Error; err != nil {
		return nil, err
	}
	var connectedCount int64
	if err := models.DB.Model(&models.CrmCallRecord{}).Scopes(models.TenantScope(tenantID)).Where("status = ?",  "connected").Count(&connectedCount).Error; err != nil {
		return nil, err
	}
	type sumRow struct {
		Total int64
	}
	var row sumRow
	models.DB.Model(&models.CrmCallRecord{}).
		Select("COALESCE(SUM(duration),0) as total").Scopes(models.TenantScope(tenantID)).Scan(&row)
	return map[string]any{
		"total_count":     totalCount,
		"connected_count": connectedCount,
		"total_duration":  row.Total,
	}, nil
}
