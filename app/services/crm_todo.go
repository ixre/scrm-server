package services

import (
	"core/common/identity"
	
	"context"
	
	"fmt"
	"time"

	"core/app/models"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 待办/消息聚合 ----------

// CrmTodoService 待办聚合服务：把跟进到期/回款到期/客户流失/超时未回等聚合为统一待办。
type CrmTodoService struct{}

// NewCrmTodo 构造待办服务。
func NewCrmTodo() *CrmTodoService { return &CrmTodoService{} }

// TodoQueryReq 待办查询请求。
type TodoQueryReq struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Type     string `json:"type"`   // 可选：按类型筛选
	Status   string `json:"status"` // 可选：按状态筛选
}

// Create 创建一条待办。
func (s CrmTodoService) Create(ctx context.Context, t *models.CrmTodo) error {
	tenantID := identity.TenantID(ctx)


	if t.ID == "" {
		t.ID = id_generator.StringID()
	}
	t.TenantID = tenantID
	if t.Status == "" {
		t.Status = "pending"
	}
	if err := createCRM(models.DB, t, "RefID", "UserID"); err != nil {
		log.Sugar.Errorw("todo create failed", "err", err)
		return err
	}
	return nil
}

// Query 分页查询待办（按用户+类型+状态筛选）。
func (s CrmTodoService) Query(ctx context.Context, userID string, req TodoQueryReq) ([]models.CrmTodo, int64, error) {
	tenantID := identity.TenantID(ctx)


	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.DB.Model(&models.CrmTodo{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ?",  userID)
	if req.Type != "" {
		db = db.Where("type = ?", req.Type)
	}
	if req.Status != "" {
		db = db.Where("status = ?", req.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmTodo
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("due_at asc, id desc").Find(&items).Error
	return items, total, err
}

// Handle 批量处理待办（done/ignored）。
func (s CrmTodoService) Handle(ctx context.Context, ids []string, status string) error {
	tenantID := identity.TenantID(ctx)


	if status != "done" && status != "ignored" {
		return fmt.Errorf("invalid status: %s", status)
	}
	if len(ids) == 0 {
		return nil
	}
	return models.DB.Model(&models.CrmTodo{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).
		Update("status", status).Error
}

// Count 按类型统计某用户的 pending 待办数量，返回 map[type]count。
func (s CrmTodoService) Count(ctx context.Context, userID string) (map[string]int64, error) {
	tenantID := identity.TenantID(ctx)


	type row struct {
		Type  string
		Count int64
	}
	var rows []row
	err := models.DB.Model(&models.CrmTodo{}).
		Select("type, count(*) as count").Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND status = ?",  userID,  "pending").
		Group("type").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.Type] = r.Count
	}
	return m, nil
}

// GenerateFollowupTodos 扫描需要跟进的客户生成待办。
// 条件：CrmCustomer.next_followup_at 已过期且非公海，且该客户尚无 pending 的 followup 待办。
func (s CrmTodoService) GenerateFollowupTodos(ctx context.Context) error {
	tenantID := identity.TenantID(ctx)


	now := time.Now()
	var customers []models.CrmCustomer
	err := models.MustFor(ctx).Where("next_followup_at IS NOT NULL AND next_followup_at < ? AND pool_status = 0",  now).Limit(500).Find(&customers).Error
	if err != nil {
		return err
	}
	for _, c := range customers {
		// 幂等：同客户已有 pending followup 待办则跳过
		var cnt int64
		models.DB.Model(&models.CrmTodo{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND type = ? AND ref_type = ? AND ref_id = ? AND status = ?",  c.OwnerUserID,  "followup",  "customer",  c.ID,  "pending").Count(&cnt)
		if cnt > 0 {
			continue
		}
		todo := models.CrmTodo{
			Model:    models.Model{ID: id_generator.StringID()},
			TenantID: tenantID,
			UserID:   c.OwnerUserID,
			Type:     "followup",
			Title:    fmt.Sprintf("跟进客户：%s", c.Name),
			Content:  fmt.Sprintf("客户 %s 的计划跟进时间已到期，请及时跟进。", c.Name),
			RefType:  "customer",
			RefID:    c.ID,
			DueAt:    c.NextFollowupAt,
			Status:   "pending",
		}
		if err := createCRM(models.DB, &todo, "RefID", "UserID"); err != nil {
			log.Sugar.Errorw("generate followup todo failed", "err", err, "customer", c.ID)
		}
	}
	return nil
}

// GenerateExpireTodos 扫描即将到期/已逾期的回款计划生成待办。
// 条件：CrmReceivablesPlan.plan_date 在未来 7 天内或已逾期，且状态为 pending。
func (s CrmTodoService) GenerateExpireTodos(ctx context.Context) error {
	tenantID := identity.TenantID(ctx)


	now := time.Now()
	deadline := now.Add(7 * 24 * time.Hour)
	var plans []models.CrmReceivablesPlan
	err := models.MustFor(ctx).Where("status = ? AND plan_date IS NOT NULL AND plan_date <= ?",  "pending",  deadline).Limit(500).Find(&plans).Error
	if err != nil {
		return err
	}
	for _, p := range plans {
		// 取合同负责人作为待办归属人
		var order models.CrmOrder
		models.DB.Where("id = ?", p.OrderID).First(&order)
		if order.ID == "" {
			continue
		}
		// 幂等：同计划已有 pending receivables_expire 待办则跳过
		var cnt int64
		models.DB.Model(&models.CrmTodo{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND type = ? AND ref_type = ? AND ref_id = ? AND status = ?",  order.UserID,  "receivables_expire",  "order",  order.ID,  "pending").Count(&cnt)
		if cnt > 0 {
			continue
		}
		title := fmt.Sprintf("回款到期：第%d期 %.2f", p.PeriodNo, p.Amount)
		content := fmt.Sprintf("合同 %s 第 %d 期回款计划金额 %.2f 即将/已到期，请跟进收款。", order.OrderNo, p.PeriodNo, p.Amount)
		todo := models.CrmTodo{
			Model:    models.Model{ID: id_generator.StringID()},
			TenantID: tenantID,
			UserID:   order.UserID,
			Type:     "receivables_expire",
			Title:    title,
			Content:  content,
			RefType:  "order",
			RefID:    order.ID,
			DueAt:    p.PlanDate,
			Status:   "pending",
		}
		if err := createCRM(models.DB, &todo, "RefID", "UserID"); err != nil {
			log.Sugar.Errorw("generate expire todo failed", "err", err, "plan", p.ID)
		}
	}
	return nil
}

// _ 确保 gorm import 被使用
var _ = gorm.ErrRecordNotFound
