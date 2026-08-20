package services

import (
	"core/common/identity"
	
	"context"
	
	"time"

	"core/app/models"
)

// StatsReq 统计报表通用入参（可选时间范围）。
type StatsReq struct {
	StartDate string `json:"start_date"` // YYYY-MM-DD 可空
	EndDate   string `json:"end_date"`   // YYYY-MM-DD 可空
}

// dateRange 解析 StatsReq 为 [start, end)。end 取结束日期的次日。
// 为空时返回零值（不过滤）。
func (r StatsReq) dateRange() (time.Time, time.Time, bool) {
	if r.StartDate == "" && r.EndDate == "" {
		return time.Time{}, time.Time{}, false
	}
	var start, end time.Time
	if r.StartDate != "" {
		if t, err := time.Parse("2006-01-02", r.StartDate); err == nil {
			start = t
		}
	}
	if r.EndDate != "" {
		if t, err := time.Parse("2006-01-02", r.EndDate); err == nil {
			end = t.AddDate(0, 0, 1)
		}
	}
	return start, end, true
}

// CrmStatisticsService 统计报表服务。
type CrmStatisticsService struct{}

func NewCrmStatistics() *CrmStatisticsService { return &CrmStatisticsService{} }

// CustomerAnalysis 客户分析：按来源/员工分布统计。
func (s CrmStatisticsService) CustomerAnalysis(ctx context.Context, req StatsReq) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	start, end, has := req.dateRange()

	// 按来源分布
	type srcRow struct {
		Source string `gorm:"column:source" json:"source"`
		Count  int64  `gorm:"column:cnt" json:"count"`
	}
	var bySource []srcRow
	q := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID))
	if has {
		if !start.IsZero() {
			q = q.Where("created_at >= ?", start)
		}
		if !end.IsZero() {
			q = q.Where("created_at < ?", end)
		}
	}
	q.Select("COALESCE(NULLIF(source,''),'unknown') AS source, COUNT(*) AS cnt").
		Group("source").Order("cnt DESC").Scan(&bySource)

	// 按员工分布
	type ownerRow struct {
		OwnerUserID string `gorm:"column:owner_user_id" json:"owner_user_id"`
		Name        string `gorm:"column:name" json:"name"`
		Count       int64  `gorm:"column:cnt" json:"count"`
	}
	var byOwner []ownerRow
	models.DB.Table("crm_customer AS c").
		Select("c.owner_user_id, COALESCE(u.name,'未分配') AS name, COUNT(*) AS cnt").
		Joins("LEFT JOIN \"user\" u ON u.id = c.owner_user_id").Scopes(models.TenantScope(tenantID)).
		Group("c.owner_user_id, u.name").Order("cnt DESC").Scan(&byOwner)

	return map[string]any{
		"by_source": bySource,
		"by_owner":  byOwner,
	}, nil
}

// LeadAnalysis 线索转化漏斗：各状态计数 + 转化率。
func (s CrmStatisticsService) LeadAnalysis(ctx context.Context, req StatsReq) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	start, end, has := req.dateRange()

	type stRow struct {
		Status string `gorm:"column:status" json:"status"`
		Count  int64  `gorm:"column:cnt" json:"count"`
	}
	var byStatus []stRow
	q := models.DB.Model(&models.CrmLead{}).Scopes(models.TenantScope(tenantID))
	if has {
		if !start.IsZero() {
			q = q.Where("created_at >= ?", start)
		}
		if !end.IsZero() {
			q = q.Where("created_at < ?", end)
		}
	}
	q.Select("COALESCE(NULLIF(status,''),'new') AS status, COUNT(*) AS cnt").
		Group("status").Scan(&byStatus)

	var total int64
	var converted int64
	for _, r := range byStatus {
		total += r.Count
		if r.Status == "qualified" || r.Status == "won" {
			converted += r.Count
		}
	}
	convRate := 0.0
	if total > 0 {
		convRate = float64(converted) / float64(total) * 100
	}
	return map[string]any{
		"by_status":    byStatus,
		"total":        total,
		"converted":    converted,
		"convert_rate": convRate,
	}, nil
}

// ContractStats 合同统计：数量/金额/回款按月统计。
func (s CrmStatisticsService) ContractStats(ctx context.Context, req StatsReq) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	start, end, has := req.dateRange()

	type monthRow struct {
		Month    string  `gorm:"column:month" json:"month"`
		Count    int64   `gorm:"column:cnt" json:"count"`
		Amount   float64 `gorm:"column:amount" json:"amount"`
		Received float64 `gorm:"column:received" json:"received"`
	}
	var rows []monthRow
	q := models.DB.Table("crm_order AS o").
		Select("to_char(o.signed_at, 'YYYY-MM') AS month, COUNT(*) AS cnt, COALESCE(SUM(o.amount),0) AS amount, COALESCE(SUM(o.paid_amount),0) AS received").Scopes(models.TenantScope(tenantID)).Where("o.status IN ? AND o.signed_at IS NOT NULL",  []string{"signed", "completed"})
	if has {
		if !start.IsZero() {
			q = q.Where("o.signed_at >= ?", start)
		}
		if !end.IsZero() {
			q = q.Where("o.signed_at < ?", end)
		}
	}
	q.Group("month").Order("month DESC").Scan(&rows)

	// 汇总
	var totalCount int64
	var totalAmount float64
	var totalReceived float64
	for _, r := range rows {
		totalCount += r.Count
		totalAmount += r.Amount
		totalReceived += r.Received
	}
	return map[string]any{
		"by_month":       rows,
		"total_count":    totalCount,
		"total_amount":   totalAmount,
		"total_received": totalReceived,
	}, nil
}

// WorkorderStats 工单统计：按状态/类型统计。
func (s CrmStatisticsService) WorkorderStats(ctx context.Context, req StatsReq) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	start, end, has := req.dateRange()

	type dimRow struct {
		Key   string `gorm:"column:key" json:"key"`
		Count int64  `gorm:"column:cnt" json:"count"`
	}

	var byStatus []dimRow
	qs := models.DB.Model(&models.CrmWorkorder{}).Scopes(models.TenantScope(tenantID))
	if has {
		if !start.IsZero() {
			qs = qs.Where("created_at >= ?", start)
		}
		if !end.IsZero() {
			qs = qs.Where("created_at < ?", end)
		}
	}
	qs.Select("status AS key, COUNT(*) AS cnt").Group("status").Scan(&byStatus)

	var byType []dimRow
	qt := models.DB.Model(&models.CrmWorkorder{}).Scopes(models.TenantScope(tenantID))
	if has {
		if !start.IsZero() {
			qt = qt.Where("created_at >= ?", start)
		}
		if !end.IsZero() {
			qt = qt.Where("created_at < ?", end)
		}
	}
	qt.Select("COALESCE(NULLIF(type,''),'unknown') AS key, COUNT(*) AS cnt").Group("type").Scan(&byType)

	return map[string]any{
		"by_status": byStatus,
		"by_type":   byType,
	}, nil
}

// ProductStats 产品统计：产品销量/金额统计（基于订单产品明细）。
func (s CrmStatisticsService) ProductStats(ctx context.Context, req StatsReq) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	start, end, has := req.dateRange()

	type prodRow struct {
		ProductName string  `gorm:"column:product_name" json:"product_name"`
		Qty         int64   `gorm:"column:qty" json:"qty"`
		Amount      float64 `gorm:"column:amount" json:"amount"`
	}
	var rows []prodRow
	q := models.DB.Table("crm_order_product AS p").
		Select("p.product_name, COALESCE(SUM(p.qty),0) AS qty, COALESCE(SUM(p.amount),0) AS amount").
		Joins("JOIN crm_order o ON o.id = p.order_id").Scopes(models.TenantScope(tenantID)).Where("o.status IN ?",  []string{"signed", "completed"})
	if has {
		if !start.IsZero() {
			q = q.Where("o.signed_at >= ?", start)
		}
		if !end.IsZero() {
			q = q.Where("o.signed_at < ?", end)
		}
	}
	q.Group("p.product_name").Order("amount DESC").Scan(&rows)

	return map[string]any{"items": rows}, nil
}

// CallStats 话务统计：通话次数/时长按员工排行。
func (s CrmStatisticsService) CallStats(ctx context.Context, req StatsReq) (map[string]any, error) {
	tenantID := identity.TenantID(ctx)


	start, end, has := req.dateRange()

	type callRow struct {
		UserID    string `gorm:"column:user_id" json:"user_id"`
		Name      string `gorm:"column:name" json:"name"`
		Calls     int64  `gorm:"column:calls" json:"calls"`
		Duration  int64  `gorm:"column:duration" json:"duration"`
		Connected int64  `gorm:"column:connected" json:"connected"`
	}
	var rows []callRow
	q := models.DB.Table("crm_call_record AS c").
		Select("c.user_id, COALESCE(u.name,'') AS name, COUNT(*) AS calls, COALESCE(SUM(c.duration),0) AS duration, COUNT(*) FILTER (WHERE c.status = 'connected') AS connected").
		Joins("LEFT JOIN \"user\" u ON u.id = c.user_id").Scopes(models.TenantScope(tenantID))
	if has {
		if !start.IsZero() {
			q = q.Where("c.start_at >= ?", start)
		}
		if !end.IsZero() {
			q = q.Where("c.start_at < ?", end)
		}
	}
	q.Group("c.user_id, u.name").Order("calls DESC").Scan(&rows)

	// 汇总
	var totalCalls int64
	var totalDuration int64
	for _, r := range rows {
		totalCalls += r.Calls
		totalDuration += r.Duration
	}
	return map[string]any{
		"items":          rows,
		"total_calls":    totalCalls,
		"total_duration": totalDuration,
	}, nil
}
