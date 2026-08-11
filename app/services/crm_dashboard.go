package services

import (
	"fmt"
	"time"

	"core/app/models"
)

// CrmDashboardService 仪表盘服务：关键指标卡、汇总、趋势、排行榜、业绩达成。
type CrmDashboardService struct{}

func NewCrmDashboard() *CrmDashboardService { return &CrmDashboardService{} }

// monthBounds 返回本月 [start, end) 与上月 [prevStart, prevEnd)。
func monthBounds(now time.Time) (time.Time, time.Time, time.Time, time.Time) {
	y, m, _ := now.Date()
	start := time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
	end := start.AddDate(0, 1, 0)
	prevStart := start.AddDate(0, -1, 0)
	prevEnd := start
	return start, end, prevStart, prevEnd
}

// ratio 环比：(cur-prev)/prev*100。prev<=0 时：cur>0 返回 100，否则 0。
func ratio(cur, prev float64) float64 {
	if prev <= 0 {
		if cur > 0 {
			return 100
		}
		return 0
	}
	return (cur - prev) / prev * 100
}

// countRange 统计某模型在 [start,end) 内的行数。
func countRange(m any, tenantID string, start, end time.Time) int64 {
	var n int64
	models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Where("created_at >= ? AND created_at < ?", start, end).
		Count(&n)
	return n
}

// sumRange 统计某模型某字段在 [start,end) 内的合计。
func sumRange(m any, tenantID, field string, start, end time.Time, extra string) float64 {
	var row struct {
		Total float64 `gorm:"column:total"`
	}
	q := models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Where("created_at >= ? AND created_at < ?", start, end).
		Select(fmt.Sprintf("COALESCE(SUM(%s),0) AS total", field))
	if extra != "" {
		q = q.Where(extra)
	}
	q.Scan(&row)
	return row.Total
}

// Briefing 本月关键指标卡 + 环比。
func (s CrmDashboardService) Briefing(tenantID string) (map[string]any, error) {
	now := time.Now()
	start, end, prevStart, prevEnd := monthBounds(now)

	newCustomers := countRange(&models.CrmCustomer{}, tenantID, start, end)
	prevCustomers := countRange(&models.CrmCustomer{}, tenantID, prevStart, prevEnd)

	newContacts := countRange(&models.CrmContact{}, tenantID, start, end)
	prevContacts := countRange(&models.CrmContact{}, tenantID, prevStart, prevEnd)

	newOpportunities := countRange(&models.CrmOpportunity{}, tenantID, start, end)
	prevOpportunities := countRange(&models.CrmOpportunity{}, tenantID, prevStart, prevEnd)

	newOrders := countRange(&models.CrmOrder{}, tenantID, start, end)
	prevOrders := countRange(&models.CrmOrder{}, tenantID, prevStart, prevEnd)

	contractAmount := sumRange(&models.CrmOrder{}, tenantID, "amount", start, end, "status IN ('signed','completed')")
	prevContractAmount := sumRange(&models.CrmOrder{}, tenantID, "amount", prevStart, prevEnd, "status IN ('signed','completed')")

	receivablesAmount := sumRange(&models.CrmReceivables{}, tenantID, "amount", start, end, "status = 'confirmed'")
	prevReceivablesAmount := sumRange(&models.CrmReceivables{}, tenantID, "amount", prevStart, prevEnd, "status = 'confirmed'")

	followupCount := countRange(&models.CrmFollowup{}, tenantID, start, end)
	prevFollowupCount := countRange(&models.CrmFollowup{}, tenantID, prevStart, prevEnd)

	return map[string]any{
		"new_customers":            newCustomers,
		"new_customers_ratio":      ratio(float64(newCustomers), float64(prevCustomers)),
		"new_contacts":             newContacts,
		"new_contacts_ratio":       ratio(float64(newContacts), float64(prevContacts)),
		"new_opportunities":        newOpportunities,
		"new_opportunities_ratio":  ratio(float64(newOpportunities), float64(prevOpportunities)),
		"new_orders":               newOrders,
		"new_orders_ratio":         ratio(float64(newOrders), float64(prevOrders)),
		"contract_amount":          contractAmount,
		"contract_amount_ratio":    ratio(contractAmount, prevContractAmount),
		"receivables_amount":       receivablesAmount,
		"receivables_amount_ratio": ratio(receivablesAmount, prevReceivablesAmount),
		"followup_count":           followupCount,
		"followup_count_ratio":     ratio(float64(followupCount), float64(prevFollowupCount)),
	}, nil
}

// Summary 汇总Tab：客户/商机/合同/跟进/回款等汇总计数。
func (s CrmDashboardService) Summary(tenantID string) (map[string]any, error) {
	now := time.Now()
	start, end, _, _ := monthBounds(now)
	monthEnd := end.AddDate(0, 0, -1)

	count := func(m any) int64 {
		var n int64
		models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Count(&n)
		return n
	}
	countMonth := func(m any) int64 {
		var n int64
		models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Where("created_at >= ? AND created_at < ?", start, end).
			Count(&n)
		return n
	}
	countWhere := func(m any, where string, args ...any) int64 {
		var n int64
		models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Where(where, args...).Count(&n)
		return n
	}
	sumAll := func(m any, field, extra string) float64 {
		var row struct {
			Total float64 `gorm:"column:total"`
		}
		q := models.DB.Model(m).Scopes(models.TenantScope(tenantID)).
			Select(fmt.Sprintf("COALESCE(SUM(%s),0) AS total", field))
		if extra != "" {
			q = q.Where(extra)
		}
		q.Scan(&row)
		return row.Total
	}

	return map[string]any{
		"add_customer":      countMonth(&models.CrmCustomer{}),
		"contract_customer": countWhere(&models.CrmCustomer{}, "id IN (SELECT DISTINCT customer_id FROM crm_order WHERE tenant_id = ? AND status IN ('signed','completed'))", tenantID),
		"seas_customer":     countWhere(&models.CrmCustomer{}, "pool_status = 1"),
		"add_business":      countMonth(&models.CrmOpportunity{}),
		"contract_business": countWhere(&models.CrmOpportunity{}, "status = 'won'"),
		"contract":          count(&models.CrmOrder{}),
		"contract_expire":   countWhere(&models.CrmOrder{}, "expire_date IS NOT NULL AND expire_date >= ? AND expire_date <= ?", start, monthEnd),
		"record":            count(&models.CrmFollowup{}),
		"norecord":          countWhere(&models.CrmCustomer{}, "id NOT IN (SELECT DISTINCT customer_id FROM crm_followup WHERE tenant_id = ?)", tenantID),
		"receivables":       sumAll(&models.CrmReceivables{}, "amount", "status = 'confirmed'"),
		"receivables_plan":  sumAll(&models.CrmReceivablesPlan{}, "amount", "status IN ('pending','overdue')"),
	}, nil
}

// Trend 近6个月趋势：客户/订单/商机/跟进/回款按月计数。
func (s CrmDashboardService) Trend(tenantID string) (map[string]any, error) {
	now := time.Now()
	months := make([]string, 6)
	labels := make([]time.Time, 6)
	for i := 0; i < 6; i++ {
		t := now.AddDate(0, -5+i, 0)
		y, m, _ := t.Date()
		labels[i] = time.Date(y, m, 1, 0, 0, 0, 0, now.Location())
		months[i] = fmt.Sprintf("%04d-%02d", y, m)
	}

	monthCount := func(m any, idx int) int64 {
		var n int64
		models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Where("created_at >= ? AND created_at < ?", labels[idx], labels[idx].AddDate(0, 1, 0)).
			Count(&n)
		return n
	}
	monthSum := func(m any, field string, idx int, extra string) float64 {
		var row struct {
			Total float64 `gorm:"column:total"`
		}
		q := models.DB.Model(m).Scopes(models.TenantScope(tenantID)).Where("created_at >= ? AND created_at < ?", labels[idx], labels[idx].AddDate(0, 1, 0)).
			Select(fmt.Sprintf("COALESCE(SUM(%s),0) AS total", field))
		if extra != "" {
			q = q.Where(extra)
		}
		q.Scan(&row)
		return row.Total
	}

	customers := make([]int64, 6)
	orders := make([]int64, 6)
	opportunities := make([]int64, 6)
	followups := make([]int64, 6)
	receivables := make([]float64, 6)
	for i := 0; i < 6; i++ {
		customers[i] = monthCount(&models.CrmCustomer{}, i)
		orders[i] = monthCount(&models.CrmOrder{}, i)
		opportunities[i] = monthCount(&models.CrmOpportunity{}, i)
		followups[i] = monthCount(&models.CrmFollowup{}, i)
		receivables[i] = monthSum(&models.CrmReceivables{}, "amount", i, "status = 'confirmed'")
	}
	return map[string]any{
		"months":        months,
		"customers":     customers,
		"orders":        orders,
		"opportunities": opportunities,
		"followups":     followups,
		"receivables":   receivables,
	}, nil
}

// rankRow 排行榜行。
type rankRow struct {
	ID     string  `gorm:"column:id" json:"user_id"`
	Name   string  `gorm:"column:name" json:"name"`
	Amount float64 `gorm:"column:amount" json:"amount"`
	Count  int64   `gorm:"column:cnt" json:"count"`
}

// Ranking 排行榜：合同金额/回款金额/新增客户/跟进次数，取前10。
func (s CrmDashboardService) Ranking(tenantID string) (map[string]any, error) {
	now := time.Now()
	start, end, _, _ := monthBounds(now)

	// 合同金额排行
	var byContract []rankRow
	models.DB.Table("crm_order AS o").
		Select("o.user_id AS id, u.name AS name, COALESCE(SUM(o.amount),0) AS amount").
		Joins("LEFT JOIN \"user\" u ON u.id = o.user_id").Scopes(models.TenantScope(tenantID)).Where("o.status IN ? AND o.signed_at >= ? AND o.signed_at < ?", []string{"signed", "completed"}, start, end).
		Group("o.user_id, u.name").Order("amount DESC").Limit(10).Scan(&byContract)

	// 回款金额排行
	var byReceivables []rankRow
	models.DB.Table("crm_receivables AS r").
		Select("o.user_id AS id, u.name AS name, COALESCE(SUM(r.amount),0) AS amount").
		Joins("LEFT JOIN crm_order o ON o.id = r.order_id").
		Joins("LEFT JOIN \"user\" u ON u.id = o.user_id").Scopes(models.TenantScope(tenantID)).Where("r.status = ? AND r.received_at >= ? AND r.received_at < ?", "confirmed", start, end).
		Group("o.user_id, u.name").Order("amount DESC").Limit(10).Scan(&byReceivables)

	// 新增客户排行
	var byNewCustomers []rankRow
	models.DB.Table("crm_customer AS c").
		Select("c.owner_user_id AS id, u.name AS name, COUNT(*) AS cnt").
		Joins("LEFT JOIN \"user\" u ON u.id = c.owner_user_id").Scopes(models.TenantScope(tenantID)).Where("c.created_at >= ? AND c.created_at < ?", start, end).
		Group("c.owner_user_id, u.name").Order("cnt DESC").Limit(10).Scan(&byNewCustomers)

	// 跟进次数排行
	var byFollowups []rankRow
	models.DB.Table("crm_followup AS f").
		Select("f.user_id AS id, u.name AS name, COUNT(*) AS cnt").
		Joins("LEFT JOIN \"user\" u ON u.id = f.user_id").Scopes(models.TenantScope(tenantID)).Where("f.created_at >= ? AND f.created_at < ?", start, end).
		Group("f.user_id, u.name").Order("cnt DESC").Limit(10).Scan(&byFollowups)

	return map[string]any{
		"by_contract":      byContract,
		"by_receivables":   byReceivables,
		"by_new_customers": byNewCustomers,
		"by_followups":     byFollowups,
	}, nil
}

// Achievement 业绩达成：本月各销售目标 vs 实际回款。
func (s CrmDashboardService) Achievement(tenantID string) (map[string]any, error) {
	now := time.Now()
	yearMonth := fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
	start, end, _, _ := monthBounds(now)

	type achRow struct {
		UserID       string  `gorm:"column:user_id" json:"user_id"`
		Name         string  `gorm:"column:name" json:"name"`
		TargetAmount float64 `gorm:"column:target_amount" json:"target_amount"`
		ActualAmount float64 `gorm:"column:actual_amount" json:"actual_amount"`
	}
	var rows []achRow
	// 以本月有目标的销售为基准，左联实际回款。
	models.DB.Table("crm_quota AS q").
		Select("q.user_id, u.name AS name, q.target_amount, COALESCE(a.actual_amount,0) AS actual_amount").
		Joins("LEFT JOIN \"user\" u ON u.id = q.user_id").
		Joins("LEFT JOIN (SELECT o.user_id, COALESCE(SUM(r.amount),0) AS actual_amount FROM crm_receivables r JOIN crm_order o ON o.id = r.order_id WHERE r.tenant_id = ? AND r.status = ? AND r.received_at >= ? AND r.received_at < ? GROUP BY o.user_id) a ON a.user_id = q.user_id", tenantID, "confirmed", start, end).Scopes(models.TenantScope(tenantID)).Where("q.year_month = ?", yearMonth).
		Scan(&rows)

	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		rate := 0.0
		if r.TargetAmount > 0 {
			rate = r.ActualAmount / r.TargetAmount * 100
		}
		out = append(out, map[string]any{
			"user_id":       r.UserID,
			"name":          r.Name,
			"target_amount": r.TargetAmount,
			"actual_amount": r.ActualAmount,
			"rate":          rate,
		})
	}
	return map[string]any{"items": out}, nil
}
