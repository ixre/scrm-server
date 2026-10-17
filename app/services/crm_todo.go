package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CrmTodoService struct{}

func NewCrmTodo() *CrmTodoService { return &CrmTodoService{} }

type TodoQueryReq struct {
	Page     int    `json:"page"`
	PageSize int    `json:"page_size"`
	Type     string `json:"type"`
	Status   string `json:"status"`
}

func (s CrmTodoService) Create(ctx context.Context, t *models.CrmTodo) error {
	if t == nil || identity.TenantID(ctx) == "" {
		return errors.New("待办及租户不能为空")
	}
	if t.ID == "" {
		t.ID = id_generator.StringID()
	}
	t.TenantID = identity.TenantID(ctx)
	if t.Status == "" {
		t.Status = "pending"
	}
	return createCRM(models.DB.WithContext(ctx), t)
}

func todoActor(ctx context.Context, userID string) (models.User, error) {
	actor, _, err := customerActor(ctx, models.DB)
	if err != nil {
		return actor, err
	}
	if actor.ID != userID {
		return actor, ErrCustomerAccess
	}
	return actor, nil
}

func (s CrmTodoService) Query(ctx context.Context, userID string, req TodoQueryReq) ([]models.CrmTodo, int64, error) {
	actor, err := todoActor(ctx, userID)
	if err != nil {
		return nil, 0, err
	}
	if err = reconcileReminders(ctx, models.DB, actor.TenantID, userID); err != nil {
		return nil, 0, err
	}
	p := (&app.Pager{Page: req.Page, PageSize: req.PageSize}).SetDefault()
	db := models.DB.WithContext(ctx).Model(&models.CrmTodo{}).Where("tenant_id = ? AND user_id = ?", actor.TenantID, userID)
	if req.Type != "" {
		db = db.Where("type = ?", req.Type)
	}
	if req.Status != "" {
		db = db.Where("status = ?", req.Status)
	}
	var total int64
	if err = db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmTodo
	err = db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("due_at asc, id desc").Find(&items).Error
	return items, total, err
}

// Handle only resolves the actor's reminders. It never writes a followup or receipt.
func (s CrmTodoService) Handle(ctx context.Context, ids []string, status string) error {
	if status != "done" && status != "ignored" {
		return errors.New("待办处理状态无效")
	}
	ids, err := workflowIDs(ids)
	if err != nil {
		return err
	}
	actor, err := todoActor(ctx, identity.UserID(ctx))
	if err != nil {
		return err
	}
	// Commit stale-state reconciliation even if the requested transition is rejected.
	if err = reconcileReminders(ctx, models.DB, actor.TenantID, actor.ID); err != nil {
		return err
	}
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []models.CrmTodo
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND user_id = ? AND id IN ?", actor.TenantID, actor.ID, ids).Order("id").Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != len(ids) {
			return errors.New("待办不存在或不属于当前用户")
		}
		for _, row := range rows {
			if row.Status != "pending" && row.Status != status {
				return errors.New("待办已处理或已失效，请刷新后重试")
			}
		}
		return tx.Model(&models.CrmTodo{}).Where("tenant_id = ? AND user_id = ? AND id IN ? AND status = 'pending'", actor.TenantID, actor.ID, ids).Updates(map[string]any{"status": status, "handled_at": time.Now()}).Error
	})
}

func (s CrmTodoService) Count(ctx context.Context, userID string) (map[string]int64, error) {
	actor, err := todoActor(ctx, userID)
	if err != nil {
		return nil, err
	}
	if err = reconcileReminders(ctx, models.DB, actor.TenantID, userID); err != nil {
		return nil, err
	}
	var rows []struct {
		Type  string
		Count int64
	}
	err = models.DB.WithContext(ctx).Model(&models.CrmTodo{}).Select("type, count(*) as count").Where("tenant_id = ? AND user_id = ? AND status = 'pending'", actor.TenantID, userID).Group("type").Scan(&rows).Error
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.Type] = r.Count
	}
	return out, err
}

// Reconcile pending occurrences against their current source. History remains intact.
// This also runs before lists/counts so transferred customers disappear immediately.
func reconcileReminders(ctx context.Context, db *gorm.DB, tenantID, userID string) error {
	if tenantID == "" {
		return errors.New("租户不能为空")
	}
	q := db.WithContext(ctx).Model(&models.CrmTodo{}).Where("tenant_id = ? AND status = 'pending'", tenantID)
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	stale := `((type = 'followup' AND (event_key = '' OR NOT EXISTS (
  SELECT 1 FROM crm_customer c JOIN "user" u ON u.id = c.owner_user_id AND u.tenant_id = c.tenant_id AND u.status = 1 AND u.deleted_at IS NULL
  WHERE c.tenant_id = crm_todo.tenant_id AND c.id = crm_todo.source_id AND c.owner_user_id = crm_todo.user_id
  AND c.deleted_at IS NULL AND c.pool_status = 0 AND c.is_blacklist = false AND c.reassign_status = 0 AND c.next_followup_at = crm_todo.due_at
 ))) OR (type = 'receivables_expire' AND (event_key = '' OR NOT EXISTS (
  SELECT 1 FROM crm_receivables_plan p JOIN crm_order o ON o.id = p.order_id AND o.tenant_id = p.tenant_id
  JOIN "user" u ON u.id = o.user_id AND u.tenant_id = o.tenant_id AND u.status = 1 AND u.deleted_at IS NULL
  WHERE p.tenant_id = crm_todo.tenant_id AND p.id = crm_todo.source_id AND p.order_id = crm_todo.ref_id
  AND p.deleted_at IS NULL AND p.status IN ('pending', 'overdue') AND p.plan_date = crm_todo.due_at
  AND o.deleted_at IS NULL AND o.status IN ('signed', 'completed') AND o.user_id = crm_todo.user_id
 ))))`
	return q.Where(stale).Updates(map[string]any{"status": "cancelled", "handled_at": time.Now()}).Error
}

const reminderBatchSize = 200

// A durable occurrence is identified by source + recipient + due time, not pending status.
// PostgreSQL timestamps have microsecond precision; use the same representation in Go/SQL.
func reminderEventKey(kind, source, user string, due time.Time) string {
	return fmt.Sprintf("%s:%s:%s:%d", kind, source, user, due.UnixMicro())
}
func reminderEventSQL(kind, source, user, due string) string {
	return fmt.Sprintf("concat('%s:', %s, ':', %s, ':', floor(extract(epoch from %s) * 1000000)::bigint)", kind, source, user, due)
}
func missingReminderSQL(event string) string {
	return `NOT EXISTS (SELECT 1 FROM crm_todo t WHERE t.tenant_id = s.tenant_id AND t.event_key = ` + event + ` AND (t.status <> 'cancelled' OR t.deleted_at IS NOT NULL))`
}
func insertReminders(tx *gorm.DB, rows []models.CrmTodo) error {
	if len(rows) == 0 {
		return nil
	}
	return tx.Clauses(clause.OnConflict{
		Columns:     []clause.Column{{Name: "tenant_id"}, {Name: "event_key"}},
		TargetWhere: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "event_key <> ''"}}},
		DoUpdates:   clause.Assignments(map[string]any{"status": "pending", "handled_at": nil, "title": gorm.Expr("excluded.title"), "content": gorm.Expr("excluded.content"), "updated_at": time.Now()}),
		Where:       clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "crm_todo.status = 'cancelled' AND crm_todo.deleted_at IS NULL"}}},
	}).Create(&rows).Error
}

func (s CrmTodoService) GenerateFollowupTodos(ctx context.Context) error {
	tid := identity.TenantID(ctx)
	if err := reconcileReminders(ctx, models.DB, tid, ""); err != nil {
		return err
	}
	_, tomorrow := followupDay(time.Now())
	var cursor time.Time
	lastID := "0"
	event := reminderEventSQL("followup", "s.id", "s.owner_user_id", "s.next_followup_at")
	for {
		var rows []models.CrmCustomer
		q := models.DB.WithContext(ctx).Table("crm_customer AS s").Select("s.*").Where("s.tenant_id = ? AND s.deleted_at IS NULL AND s.pool_status = 0 AND s.is_blacklist = false AND s.reassign_status = 0 AND s.next_followup_at < ?", tid, tomorrow).
			Where(`EXISTS (SELECT 1 FROM "user" u WHERE u.id = s.owner_user_id AND u.tenant_id = s.tenant_id AND u.status = 1 AND u.deleted_at IS NULL)`).Where(missingReminderSQL(event))
		if !cursor.IsZero() {
			q = q.Where("(s.next_followup_at, s.id) > (?, ?)", cursor, lastID)
		}
		if err := q.Order("s.next_followup_at, s.id").Limit(reminderBatchSize).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		todos := make([]models.CrmTodo, 0, len(rows))
		for _, c := range rows {
			todos = append(todos, models.CrmTodo{Model: models.Model{ID: id_generator.StringID()}, TenantID: tid, UserID: c.OwnerUserID, Type: "followup", Title: fmt.Sprintf("跟进客户：%s", c.Name), Content: "请按预约时间联系客户并记录跟进结果。", RefType: "customer", RefID: c.ID, SourceID: c.ID, DueAt: c.NextFollowupAt, Status: "pending", EventKey: reminderEventKey("followup", c.ID, c.OwnerUserID, *c.NextFollowupAt)})
		}
		if err := insertReminders(models.DB.WithContext(ctx), todos); err != nil {
			return err
		}
		cursor, lastID = *rows[len(rows)-1].NextFollowupAt, rows[len(rows)-1].ID
	}
}

func (s CrmTodoService) GenerateExpireTodos(ctx context.Context) error {
	tid := identity.TenantID(ctx)
	if err := reconcileReminders(ctx, models.DB, tid, ""); err != nil {
		return err
	}
	deadline := time.Now().AddDate(0, 0, 7)
	var cursor time.Time
	lastID := "0"
	event := reminderEventSQL("receivables_expire", "s.id", "o.user_id", "s.plan_date")
	for {
		var rows []struct {
			models.CrmReceivablesPlan
			UserID  string
			OrderNo string
		}
		q := models.DB.WithContext(ctx).Table("crm_receivables_plan AS s").Select("s.*, o.user_id, o.order_no").
			Joins("JOIN crm_order o ON o.id = s.order_id AND o.tenant_id = s.tenant_id AND o.deleted_at IS NULL AND o.status IN ('signed','completed')").
			Joins(`JOIN "user" u ON u.id = o.user_id AND u.tenant_id = o.tenant_id AND u.status = 1 AND u.deleted_at IS NULL`).
			Where("s.tenant_id = ? AND s.deleted_at IS NULL AND s.status IN ('pending','overdue') AND s.plan_date <= ?", tid, deadline).Where(missingReminderSQL(event))
		if !cursor.IsZero() {
			q = q.Where("(s.plan_date, s.id) > (?, ?)", cursor, lastID)
		}
		if err := q.Order("s.plan_date, s.id").Limit(reminderBatchSize).Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		todos := make([]models.CrmTodo, 0, len(rows))
		for _, p := range rows {
			todos = append(todos, models.CrmTodo{Model: models.Model{ID: id_generator.StringID()}, TenantID: tid, UserID: p.UserID, Type: "receivables_expire", Title: fmt.Sprintf("回款到期：第%d期 %.2f", p.PeriodNo, p.Amount), Content: fmt.Sprintf("合同 %s 第 %d 期计划回款 %.2f，请核对回款记录。", p.OrderNo, p.PeriodNo, p.Amount), RefType: "order", RefID: p.OrderID, SourceID: p.ID, DueAt: p.PlanDate, Status: "pending", EventKey: reminderEventKey("receivables_expire", p.ID, p.UserID, *p.PlanDate)})
		}
		if err := insertReminders(models.DB.WithContext(ctx), todos); err != nil {
			return err
		}
		cursor, lastID = *rows[len(rows)-1].PlanDate, rows[len(rows)-1].ID
	}
}
