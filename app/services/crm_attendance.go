package services

import (
	"core/common/identity"

	"context"

	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
)

// ---------- 考勤 ----------

// AttendanceQueryReq 考勤查询参数。
type AttendanceQueryReq struct {
	Page     int
	PageSize int
	UserID   string
	Type     string
}

// CrmAttendanceService 考勤服务。
type CrmAttendanceService struct{}

func NewCrmAttendance() *CrmAttendanceService { return &CrmAttendanceService{} }

// Create 创建考勤记录。
func (s CrmAttendanceService) Create(ctx context.Context, userID string, a *models.CrmAttendance) error {
	tenantID := identity.TenantID(ctx)

	if a == nil {
		return errors.New("考勤记录不能为空")
	}
	if a.Type == "" {
		return errors.New("考勤类型不能为空")
	}
	a.ID = id_generator.StringID()
	a.TenantID = tenantID
	a.UserID = userID
	return createCRM(models.DB, a, "UserID")
}

// Query 分页查询考勤记录。
func (s CrmAttendanceService) Query(ctx context.Context, req AttendanceQueryReq) ([]models.CrmAttendance, int64, error) {

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
	if t := strings.TrimSpace(req.Type); t != "" {
		db = db.Where("type = ?", t)
	}
	var total int64
	if err := db.Model(&models.CrmAttendance{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmAttendance
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// ListByUser 按用户列出考勤记录。
func (s CrmAttendanceService) ListByUser(ctx context.Context, userID string) ([]models.CrmAttendance, error) {

	var items []models.CrmAttendance
	err := models.MustFor(ctx).Where("user_id = ?", userID).
		Order("id desc").Find(&items).Error
	return items, err
}

// ---------- 请假 ----------

// LeaveQueryReq 请假查询参数。
type LeaveQueryReq struct {
	Page     int
	PageSize int
	UserID   string
	Status   string
}

// CrmLeaveService 请假服务。
type CrmLeaveService struct{}

func NewCrmLeave() *CrmLeaveService { return &CrmLeaveService{} }

// Create 创建请假记录。
func (s CrmLeaveService) Create(ctx context.Context, userID string, l *models.CrmLeave) error {
	tenantID := identity.TenantID(ctx)

	if l == nil {
		return errors.New("请假记录不能为空")
	}
	if l.Type == "" {
		return errors.New("请假类型不能为空")
	}
	l.ID = id_generator.StringID()
	l.TenantID = tenantID
	l.UserID = userID
	if l.Status == "" {
		l.Status = "pending"
	}
	return createCRM(models.DB, l, "UserID")
}

// Update 更新请假记录。
func (s CrmLeaveService) Update(ctx context.Context, id string, l *models.CrmLeave) error {

	if l == nil {
		return errors.New("请假记录不能为空")
	}
	m := map[string]any{
		"type":     l.Type,
		"start_at": l.StartAt,
		"end_at":   l.EndAt,
		"days":     l.Days,
		"reason":   l.Reason,
		"status":   l.Status,
	}
	if err := models.MustFor(ctx).Model(&models.CrmLeave{}).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除请假记录。
func (s CrmLeaveService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmLeave{})
	return res.RowsAffected, res.Error
}

// Query 分页查询请假记录。
func (s CrmLeaveService) Query(ctx context.Context, req LeaveQueryReq) ([]models.CrmLeave, int64, error) {

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
	if st := strings.TrimSpace(req.Status); st != "" {
		db = db.Where("status = ?", st)
	}
	var total int64
	if err := db.Model(&models.CrmLeave{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmLeave
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// Approve 批准请假。
func (s CrmLeaveService) Approve(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)

	return models.DB.Model(&models.CrmLeave{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Update("status", "approved").Error
}

// Reject 驳回请假。
func (s CrmLeaveService) Reject(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)

	return models.DB.Model(&models.CrmLeave{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Update("status", "rejected").Error
}
