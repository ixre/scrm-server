package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/id_generator"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ---------- 平台账号管理 ----------

// PlatformAdminService 平台超管账号管理（super_admin 专属）。
type PlatformAdminService struct{}

// PlatformAdminListReq 列表查询参数。
type PlatformAdminListReq struct {
	Keyword  string `json:"keyword"`
	Role     string `json:"role"`
	Status   int    `json:"status"` // 0=全部 1=启用 2=停用
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// PlatformAdminListResult 列表结果。
type PlatformAdminListResult struct {
	Total int64                  `json:"total"`
	List  []models.PlatformAdmin `json:"list"`
}

// ListAdmins 分页列出平台超管。
func (s PlatformAdminService) ListAdmins(req PlatformAdminListReq) (PlatformAdminListResult, error) {
	var result PlatformAdminListResult
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 200 {
		req.PageSize = 200
	}
	q := models.DB.Model(&models.PlatformAdmin{})
	if req.Keyword != "" {
		q = q.Where("username LIKE ? OR name LIKE ? OR mobile LIKE ? OR email LIKE ?",
			"%"+req.Keyword+"%", "%"+req.Keyword+"%", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}
	if req.Role != "" {
		q = q.Where("role = ?", req.Role)
	}
	if req.Status > 0 {
		q = q.Where("status = ?", req.Status)
	}
	q.Count(&result.Total)
	offset := (req.Page - 1) * req.PageSize
	var admins []models.PlatformAdmin
	q.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&admins)
	result.List = admins
	return result, nil
}

// PlatformAdminCreateReq 创建参数。
type PlatformAdminCreateReq struct {
	Perms    *[]string `json:"perms"`
	Username string    `json:"username"`
	Password string    `json:"password"`
	Name     string    `json:"name"`
	Avatar   string    `json:"avatar"`
	Mobile   string    `json:"mobile"`
	Email    string    `json:"email"`
	Role     string    `json:"role"`
}

// CreateAdmin 创建平台超管账号。
func (s PlatformAdminService) CreateAdmin(ctx context.Context, req PlatformAdminCreateReq) (models.PlatformAdmin, error) {
	if _, err := RequirePlatformPermission(ctx, "platform:admin:create"); err != nil {
		return models.PlatformAdmin{}, err
	}

	if req.Username == "" {
		return models.PlatformAdmin{}, errors.New("用户名不能为空")
	}
	if err := validatePassword(req.Password); err != nil {
		return models.PlatformAdmin{}, err
	}
	if req.Role == "" {
		req.Role = "operator"
	}
	grants, err := grantablePlatformPermissions(ctx, req.Role, req.Perms)
	if err != nil {
		return models.PlatformAdmin{}, err
	}
	// 唯一性检查
	var cnt int64
	models.DB.Model(&models.PlatformAdmin{}).Where("username = ?", req.Username).Count(&cnt)
	if cnt > 0 {
		return models.PlatformAdmin{}, errors.New("用户名已存在")
	}
	h, err := hashPassword(req.Password)
	if err != nil {
		return models.PlatformAdmin{}, err
	}
	admin := models.PlatformAdmin{
		Model:    models.Model{ID: id_generator.StringID()},
		Username: req.Username,
		Password: h,
		Name:     req.Name,
		Avatar:   req.Avatar,
		Mobile:   req.Mobile,
		Email:    req.Email,
		Role:     req.Role,
		Perms:    grants,
		Status:   1,
	}
	if err := models.DB.Create(&admin).Error; err != nil {
		return models.PlatformAdmin{}, err
	}
	return admin, nil
}

// PlatformAdminUpdateReq 更新参数。
type PlatformAdminUpdateReq struct {
	Perms  *[]string `json:"perms"`
	Name   string    `json:"name"`
	Avatar string    `json:"avatar"`
	Mobile string    `json:"mobile"`
	Email  string    `json:"email"`
	Role   string    `json:"role"`
	Status int       `json:"status"`
}

// UpdateAdmin 更新平台超管（不含密码）。
func (s PlatformAdminService) UpdateAdmin(ctx context.Context, id string, req PlatformAdminUpdateReq) error {
	if _, err := RequirePlatformPermission(ctx, "platform:admin:update"); err != nil {
		return err
	}

	admin, err := manageablePlatformAdmin(ctx, id)
	if err != nil {
		return err
	}
	changes := map[string]any{"name": req.Name, "avatar": req.Avatar, "mobile": req.Mobile, "email": req.Email}
	if req.Role != "" && req.Role != admin.Role || req.Perms != nil {
		role := req.Role
		if role == "" {
			role = admin.Role
		}
		grants, err := grantablePlatformPermissions(ctx, role, req.Perms)
		if err != nil {
			return err
		}
		changes["role"], changes["perms"] = role, grants
	}
	if req.Status == 1 || req.Status == 2 {
		changes["status"] = req.Status
	}
	return mutatePlatformAdmin(ctx, id, changes, false)
}

// ResetAdminPassword 重置平台超管密码。
func (s PlatformAdminService) ResetAdminPassword(ctx context.Context, id string, newPassword string) error {
	if _, err := RequirePlatformPermission(ctx, "platform:admin:reset_password"); err != nil {
		return err
	}

	if id == "" {
		return errors.New("id 不能为空")
	}
	if err := validatePassword(newPassword); err != nil {
		return err
	}
	if _, err := manageablePlatformAdmin(ctx, id); err != nil {
		return err
	}
	var admin models.PlatformAdmin
	if err := models.DB.Where("id = ?", id).First(&admin).Error; err != nil {
		return errors.New("账号不存在")
	}
	h, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	return models.DB.Model(&admin).Update("password", h).Error
}

// SetAdminStatus 启用/停用平台超管。
func (s PlatformAdminService) SetAdminStatus(ctx context.Context, id string, status int) error {
	if _, err := RequirePlatformPermission(ctx, "platform:admin:set_status"); err != nil {
		return err
	}

	if status != 1 && status != 2 {
		return errors.New("账号状态无效")
	}
	if _, err := manageablePlatformAdmin(ctx, id); err != nil {
		return err
	}
	return mutatePlatformAdmin(ctx, id, map[string]any{"status": status}, false)
}

func (s PlatformAdminService) DeleteAdmin(ctx context.Context, id string) error {
	if _, err := RequirePlatformPermission(ctx, "platform:admin:delete"); err != nil {
		return err
	}

	if _, err := manageablePlatformAdmin(ctx, id); err != nil {
		return err
	}
	return mutatePlatformAdmin(ctx, id, nil, true)
}

// ---------- 平台操作日志 ----------

// PlatformLogService 平台操作日志服务。
type PlatformLogService struct{}

// PlatformLogListReq 日志查询参数。
type PlatformLogListReq struct {
	Keyword  string `json:"keyword"`
	Action   string `json:"action"`
	Resource string `json:"resource"`
	AdminID  string `json:"admin_id"`
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
}

// PlatformLogListResult 日志列表结果。
type PlatformLogListResult struct {
	Total int64                         `json:"total"`
	List  []models.PlatformOperationLog `json:"list"`
}

// ListLogs 分页列出操作日志。
func (s PlatformLogService) ListLogs(req PlatformLogListReq) (PlatformLogListResult, error) {
	var result PlatformLogListResult
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 200 {
		req.PageSize = 200
	}
	q := models.DB.Model(&models.PlatformOperationLog{})
	if req.Keyword != "" {
		q = q.Where("admin_name LIKE ? OR action LIKE ? OR detail LIKE ?",
			"%"+req.Keyword+"%", "%"+req.Keyword+"%", "%"+req.Keyword+"%")
	}
	if req.Action != "" {
		q = q.Where("action = ?", req.Action)
	}
	if req.Resource != "" {
		q = q.Where("resource = ?", req.Resource)
	}
	if req.AdminID != "" {
		q = q.Where("admin_id = ?", req.AdminID)
	}
	q.Count(&result.Total)
	offset := (req.Page - 1) * req.PageSize
	var logs []models.PlatformOperationLog
	q.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&logs)
	result.List = logs
	return result, nil
}

// WriteLog 写入操作日志（供 dispatch handler 调用）。
func WriteLog(adminID, adminName, adminRole, action, resource, resourceID, detail, ip string) {
	log := models.PlatformOperationLog{
		Model:      models.Model{ID: id_generator.StringID()},
		AdminID:    adminID,
		AdminName:  adminName,
		AdminRole:  adminRole,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
		IP:         ip,
	}
	models.DB.Create(&log)
}

// ---------- 平台统计 ----------

// PlatformStatsService 平台统计服务。
type PlatformStatsService struct{}

// PlatformStatsResult 平台统计概览。
type PlatformStatsResult struct {
	// 租户统计
	TotalTenants    int64 `json:"total_tenants"`
	ActiveTenants   int64 `json:"active_tenants"`
	TrialTenants    int64 `json:"trial_tenants"`
	ExpiredTenants  int64 `json:"expired_tenants"`
	ExpiringTenants int64 `json:"expiring_tenants"` // 7天内到期
	// 用户统计
	TotalUsers int64 `json:"total_users"`
	// 订单统计
	TotalOrders   int64 `json:"total_orders"`
	PaidOrders    int64 `json:"paid_orders"`
	PendingOrders int64 `json:"pending_orders"`
	TotalRevenue  int64 `json:"total_revenue"` // 已支付总金额(分)
	// 套餐分布
	PlanDistribution []PlanDistItem `json:"plan_distribution"`
	// 近30天新增租户趋势
	TenantTrend []TrendItem `json:"tenant_trend"`
	// 近30天收入趋势
	RevenueTrend []TrendItem `json:"revenue_trend"`
}

// PlanDistItem 套餐分布项。
type PlanDistItem struct {
	PlanCode string `json:"plan_code"`
	PlanName string `json:"plan_name"`
	Count    int64  `json:"count"`
}

// TrendItem 趋势项。
type TrendItem struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

// Stats 获取平台统计概览。
func (s PlatformStatsService) Stats() (PlatformStatsResult, error) {
	var result PlatformStatsResult
	// 租户统计
	models.DB.Model(&models.Tenant{}).Where("deleted_at IS NULL").Count(&result.TotalTenants)
	models.DB.Model(&models.Tenant{}).Where("deleted_at IS NULL AND status = 1").Count(&result.ActiveTenants)
	// 试用租户：plan=trial
	models.DB.Model(&models.Tenant{}).Where("deleted_at IS NULL AND plan = 'trial'").Count(&result.TrialTenants)
	// 已过期租户
	now := time.Now()
	models.DB.Model(&models.Tenant{}).Where("deleted_at IS NULL AND expire_at IS NOT NULL AND expire_at < ?", now).Count(&result.ExpiredTenants)
	// 7天内到期
	sevenDaysLater := now.AddDate(0, 0, 7)
	models.DB.Model(&models.Tenant{}).Where("deleted_at IS NULL AND expire_at IS NOT NULL AND expire_at >= ? AND expire_at <= ?", now, sevenDaysLater).Count(&result.ExpiringTenants)
	// 用户统计
	models.CrossTenant().Model(&models.User{}).Where("deleted_at IS NULL").Count(&result.TotalUsers)
	// 订单统计
	models.CrossTenant().Model(&models.TenantOrder{}).Where("deleted_at IS NULL").Count(&result.TotalOrders)
	models.CrossTenant().Model(&models.TenantOrder{}).Where("deleted_at IS NULL AND pay_status = 2").Count(&result.PaidOrders)
	models.CrossTenant().Model(&models.TenantOrder{}).Where("deleted_at IS NULL AND pay_status = 1").Count(&result.PendingOrders)
	// 已支付总金额
	type revResult struct{ Total int64 }
	var rev revResult
	models.CrossTenant().Model(&models.TenantOrder{}).Where("deleted_at IS NULL AND pay_status = 2").Select("COALESCE(SUM(amount),0) as total").Scan(&rev)
	result.TotalRevenue = rev.Total
	// 套餐分布
	var planDist []PlanDistItem
	models.DB.Raw(`
		SELECT t.plan as plan_code, COALESCE(p.name, t.plan) as plan_name, COUNT(*) as count
	 FROM tenant t
	 LEFT JOIN plan p ON p.code = t.plan AND p.deleted_at IS NULL
	 WHERE t.deleted_at IS NULL AND t.plan != ''
	 GROUP BY t.plan, p.name
	 ORDER BY count DESC
	`).Scan(&planDist)
	result.PlanDistribution = planDist
	// 近30天新增租户趋势
	var tenantTrend []TrendItem
	startDate := now.AddDate(0, 0, -29)
	models.DB.Raw(`
	 SELECT to_char(d::date, 'YYYY-MM-DD') as date, COALESCE(cnt, 0) as value
	 FROM generate_series(?::date, ?::date, '1 day') d
	 LEFT JOIN (
	   SELECT to_char(created_at, 'YYYY-MM-DD') as dt, COUNT(*) as cnt
	   FROM tenant
	   WHERE deleted_at IS NULL AND created_at >= ?
	   GROUP BY dt
	 ) t ON t.dt = to_char(d::date, 'YYYY-MM-DD')
	 ORDER BY d
	`, startDate, now, startDate).Scan(&tenantTrend)
	result.TenantTrend = tenantTrend
	// 近30天收入趋势
	var revTrend []TrendItem
	models.DB.Raw(`
	 SELECT to_char(d::date, 'YYYY-MM-DD') as date, COALESCE(SUM(o.amount), 0) as value
	 FROM generate_series(?::date, ?::date, '1 day') d
	 LEFT JOIN tenant_order o ON o.deleted_at IS NULL AND o.pay_status = 2 AND to_char(o.paid_at, 'YYYY-MM-DD') = to_char(d::date, 'YYYY-MM-DD')
	 GROUP BY d::date
	 ORDER BY d
	`, startDate, now, startDate).Scan(&revTrend)
	result.RevenueTrend = revTrend
	return result, nil
}

// TenantUsageResult 单个租户的用量统计。
type TenantUsageResult struct {
	TenantID         string     `json:"tenant_id"`
	TenantName       string     `json:"tenant_name"`
	PlanCode         string     `json:"plan_code"`
	PlanName         string     `json:"plan_name"`
	ExpireAt         *time.Time `json:"expire_at"`
	Status           int        `json:"status"`
	UserCount        int64      `json:"user_count"`
	CustomerCount    int64      `json:"customer_count"`
	LeadCount        int64      `json:"lead_count"`
	OpportunityCount int64      `json:"opportunity_count"`
	OrderCount       int64      `json:"order_count"`
	ContractAmount   int64      `json:"contract_amount"`
}

// TenantUsage 获取单个租户的详细用量。
func (s PlatformStatsService) TenantUsage(tenantID string) (TenantUsageResult, error) {
	var result TenantUsageResult
	var tenant models.Tenant
	if err := models.DB.Where("id = ?", tenantID).First(&tenant).Error; err != nil {
		return result, errors.New("租户不存在")
	}
	result.TenantID = tenant.ID
	result.TenantName = tenant.Name
	result.PlanCode = tenant.Plan
	result.ExpireAt = tenant.ExpireAt
	result.Status = tenant.Status
	// 套餐名
	var plan models.Plan
	if err := models.DB.Where("code = ?", tenant.Plan).First(&plan).Error; err == nil {
		result.PlanName = plan.Name
	}
	// 用户数
	models.DB.Model(&models.User{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&result.UserCount)
	// 客户数
	models.DB.Model(&models.Customer{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&result.CustomerCount)
	// 线索数
	models.DB.Model(&models.CrmLead{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&result.LeadCount)
	// 商机数
	models.DB.Model(&models.CrmOpportunity{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&result.OpportunityCount)
	// 订单数
	models.DB.Model(&models.CrmOrder{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Count(&result.OrderCount)
	// 合同金额
	type amtResult struct{ Total float64 }
	var amt amtResult
	models.DB.Model(&models.CrmOrder{}).Where("tenant_id = ? AND deleted_at IS NULL", tenantID).Select("COALESCE(SUM(amount),0) as total").Scan(&amt)
	result.ContractAmount = int64(amt.Total * 100) // 转为分
	return result, nil
}

// AllTenantUsage 列出所有租户的用量（分页）。
func (s PlatformStatsService) AllTenantUsage(page, pageSize int) (struct {
	Total int64               `json:"total"`
	List  []TenantUsageResult `json:"list"`
}, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	var total int64
	models.DB.Model(&models.Tenant{}).Where("deleted_at IS NULL").Count(&total)
	var tenants []models.Tenant
	models.DB.Where("deleted_at IS NULL").Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).Find(&tenants)
	var list []TenantUsageResult
	for _, t := range tenants {
		item := TenantUsageResult{
			TenantID:   t.ID,
			TenantName: t.Name,
			PlanCode:   t.Plan,
			ExpireAt:   t.ExpireAt,
			Status:     t.Status,
		}
		var plan models.Plan
		if err := models.DB.Where("code = ?", t.Plan).First(&plan).Error; err == nil {
			item.PlanName = plan.Name
		}
		models.DB.Model(&models.User{}).Where("tenant_id = ? AND deleted_at IS NULL", t.ID).Count(&item.UserCount)
		models.DB.Model(&models.Customer{}).Where("tenant_id = ? AND deleted_at IS NULL", t.ID).Count(&item.CustomerCount)
		models.DB.Model(&models.CrmLead{}).Where("tenant_id = ? AND deleted_at IS NULL", t.ID).Count(&item.LeadCount)
		models.DB.Model(&models.CrmOpportunity{}).Where("tenant_id = ? AND deleted_at IS NULL", t.ID).Count(&item.OpportunityCount)
		models.DB.Model(&models.CrmOrder{}).Where("tenant_id = ? AND deleted_at IS NULL", t.ID).Count(&item.OrderCount)
		list = append(list, item)
	}
	return struct {
		Total int64               `json:"total"`
		List  []TenantUsageResult `json:"list"`
	}{Total: total, List: list}, nil
}

// suppress unused import warnings when building without certain deps
var _ = json.Marshal
var _ = fmt.Sprintf
var _ = bcrypt.CompareHashAndPassword
var _ = gorm.ErrRecordNotFound
