package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/identity"
	"core/common/log"
)

// CrmCareService 客户关怀服务。
type CrmCareService struct{}

func NewCrmCare() *CrmCareService { return &CrmCareService{} }

// List 查询关怀模板列表。
func (s CrmCareService) List(ctx context.Context, careType string) ([]models.CrmCareTemplate, error) {
	var items []models.CrmCareTemplate
	db := models.MustFor(ctx).Model(&models.CrmCareTemplate{})
	if careType != "" {
		db = db.Where("type = ?", careType)
	}
	err := db.Order("created_at desc").Find(&items).Error
	return items, err
}

// Create 新建关怀模板。
func (s CrmCareService) Create(ctx context.Context, item *models.CrmCareTemplate) error {
	if item == nil || strings.TrimSpace(item.Name) == "" {
		return fmt.Errorf("模板名称不能为空")
	}
	item.ID = id_generator.StringID()
	item.TenantID = identity.TenantID(ctx)
	return createCRM(models.DB, item)
}

// Update 更新关怀模板。
func (s CrmCareService) Update(ctx context.Context, id string, item *models.CrmCareTemplate) error {
	if item == nil {
		return fmt.Errorf("模板不能为空")
	}
	m := map[string]any{
		"name":         item.Name,
		"type":         item.Type,
		"content":      item.Content,
		"advance_days": item.AdvanceDays,
		"enabled":      item.Enabled,
	}
	return models.MustFor(ctx).Model(&models.CrmCareTemplate{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除关怀模板。
func (s CrmCareService) Delete(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmCareTemplate{})
	return res.RowsAffected, res.Error
}

// GenerateBirthdayCareTodos 扫描生日客户生成关怀待办（定时任务调用）。
// 对每个租户，查找生日为今天（或提前N天）的客户，生成 birthday 类型的 CrmTodo。
func (s CrmCareService) GenerateBirthdayCareTodos() {
	var tenantIDs []string
	models.DB.Model(&models.Tenant{}).Pluck("id", &tenantIDs)
	today := time.Now().Format("01-02")
	for _, tid := range tenantIDs {
		// 查启用的生日关怀模板
		var templates []models.CrmCareTemplate
		models.DB.Scopes(models.TenantScope(tid)).Where("type = ? AND enabled = ?", "birthday", true).Find(&templates)
		if len(templates) == 0 {
			continue
		}
		// 查生日匹配的客户（birthday 格式可能是 MM-DD 或 YYYY-MM-DD）
		var customers []models.CrmCustomer
		models.DB.Scopes(models.TenantScope(tid)).
			Where("pool_status = 0 AND owner_user_id != '' AND birthday != ''").
			Where("RIGHT(birthday, 5) = ?", today).Find(&customers)
		for _, c := range customers {
			// 幂等：同客户今天已有 birthday 待办则跳过
			var cnt int64
			models.DB.Model(&models.CrmTodo{}).Scopes(models.TenantScope(tid)).
				Where("user_id = ? AND type = ? AND ref_id = ? AND status = ? AND created_at >= ?",
					c.OwnerUserID, "birthday_care", c.ID, "pending", time.Now().Format("2006-01-02")).Count(&cnt)
			if cnt > 0 {
				continue
			}
			tpl := templates[0]
			todo := models.CrmTodo{
				Model:    models.Model{ID: id_generator.StringID()},
				TenantID: tid,
				UserID:   c.OwnerUserID,
				Type:     "birthday_care",
				Title:    fmt.Sprintf("生日关怀：%s", c.Name),
				Content:  fmt.Sprintf("客户 %s 今天生日。关怀话术：%s", c.Name, tpl.Content),
				RefType:  "customer",
				RefID:    c.ID,
				DueAt:    &time.Time{},
				Status:   "pending",
			}
			now := time.Now()
			todo.DueAt = &now
			if err := createCRM(models.DB, &todo, "RefID", "UserID"); err != nil {
				log.Sugar.Errorw("generate birthday care todo failed", "err", err, "customer", c.ID)
			}
		}
	}
	log.Sugar.Infow("birthday care todos generated")
}

// ---------- 客户分群 ----------

// CrmSegmentService 客户分群服务。
type CrmSegmentService struct{}

func NewCrmSegment() *CrmSegmentService { return &CrmSegmentService{} }

// List 查询分群列表。
func (s CrmSegmentService) List(ctx context.Context, pager *app.Pager) ([]models.CrmCustomerSegment, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx).Model(&models.CrmCustomerSegment{})
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCustomerSegment
	err := db.Order("created_at desc").Offset(pager.GetOffset()).Limit(pager.GetLimit()).Find(&items).Error
	return items, total, err
}

// Create 新建分群。
func (s CrmSegmentService) Create(ctx context.Context, item *models.CrmCustomerSegment) error {
	if item == nil || strings.TrimSpace(item.Name) == "" {
		return fmt.Errorf("分群名称不能为空")
	}
	item.ID = id_generator.StringID()
	item.TenantID = identity.TenantID(ctx)
	item.CreatorID = identity.UserID(ctx)
	return createCRM(models.DB, item, "CreatorID")
}

// Update 更新分群。
func (s CrmSegmentService) Update(ctx context.Context, id string, item *models.CrmCustomerSegment) error {
	if item == nil {
		return fmt.Errorf("分群不能为空")
	}
	m := map[string]any{
		"name":        item.Name,
		"description": item.Description,
		"conditions":  item.Conditions,
		"is_dynamic":  item.IsDynamic,
	}
	return models.MustFor(ctx).Model(&models.CrmCustomerSegment{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除分群。
func (s CrmSegmentService) Delete(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, fmt.Errorf("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmCustomerSegment{})
	return res.RowsAffected, res.Error
}

// Preview 预览分群结果：根据条件JSON查询匹配的客户数量。
func (s CrmSegmentService) Preview(ctx context.Context, conditions string) (int64, error) {
	db := models.MustFor(ctx).Model(&models.CrmCustomer{}).Where("pool_status = 0")
	// 解析条件JSON并应用筛选
	if conditions != "" {
		// 简单条件格式：{"keyword":"xxx","source":"wecom","level":3,"tag":"tag_id"}
		// 这里做基础筛选，复杂条件可后续扩展
		var cond struct {
			Keyword string `json:"keyword"`
			Source  string `json:"source"`
			Level   int    `json:"level"`
			Tag     string `json:"tag"`
		}
		if err := json.Unmarshal([]byte(conditions), &cond); err != nil {
			return 0, fmt.Errorf("条件格式错误")
		}
		if cond.Keyword != "" {
			db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", "%"+cond.Keyword+"%", "%"+cond.Keyword+"%", "%"+cond.Keyword+"%")
		}
		if cond.Source != "" {
			db = db.Where("source = ?", cond.Source)
		}
		if cond.Level > 0 {
			db = db.Where("level = ?", cond.Level)
		}
		if cond.Tag != "" {
			jsonbVal, _ := json.Marshal([]string{cond.Tag})
			db = db.Where("tags @> ?::jsonb", string(jsonbVal))
		}
	}
	var total int64
	err := db.Count(&total).Error
	return total, err
}

// ---------- 跟进统计 ----------

// CrmFollowupStatsService 跟进统计服务。
type CrmFollowupStatsService struct{}

func NewCrmFollowupStats() *CrmFollowupStatsService { return &CrmFollowupStatsService{} }

// FollowupStatsResult 跟进统计结果。
type FollowupStatsResult struct {
	Total      int64              `json:"total"`       // 总跟进数
	TodayCount int64              `json:"today_count"` // 今日跟进数
	WeekCount  int64              `json:"week_count"`  // 本周跟进数
	MonthCount int64              `json:"month_count"` // 本月跟进数
	ByType     map[string]int64   `json:"by_type"`     // 按类型统计
	ByResult   map[string]int64   `json:"by_result"`   // 按结果统计
	ByUser     []FollowupUserStat `json:"by_user"`     // 按销售统计
}

// FollowupUserStat 按销售的跟进统计。
type FollowupUserStat struct {
	UserID   string `json:"user_id"`
	Count    int64  `json:"count"`
	LastTime string `json:"last_time"`
}

// Stats 获取跟进统计概览。
func (s CrmFollowupStatsService) Stats(ctx context.Context) (FollowupStatsResult, error) {
	tenantID := identity.TenantID(ctx)
	var result FollowupStatsResult
	result.ByType = make(map[string]int64)
	result.ByResult = make(map[string]int64)

	db := models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID))
	db.Count(&result.Total)

	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).Where("created_at >= ?", todayStart).Count(&result.TodayCount)
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).Where("created_at >= ?", weekStart).Count(&result.WeekCount)
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).Where("created_at >= ?", monthStart).Count(&result.MonthCount)

	// 按类型统计
	type typeRow struct {
		Type  string
		Count int64
	}
	var typeRows []typeRow
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).
		Select("type, count(*) as count").Group("type").Scan(&typeRows)
	for _, r := range typeRows {
		result.ByType[r.Type] = r.Count
	}

	// 按结果统计
	type resultRow struct {
		Result string
		Count  int64
	}
	var resultRows []resultRow
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).
		Select("result, count(*) as count").Where("result != ''").Group("result").Scan(&resultRows)
	for _, r := range resultRows {
		result.ByResult[r.Result] = r.Count
	}

	// 按销售统计（Top 10）
	var userRows []FollowupUserStat
	models.DB.Model(&models.CrmFollowup{}).Scopes(models.TenantScope(tenantID)).
		Select("user_id, count(*) as count, MAX(created_at) as last_time").
		Where("user_id != ''").Group("user_id").Order("count desc").Limit(10).Scan(&userRows)
	result.ByUser = userRows

	return result, nil
}
