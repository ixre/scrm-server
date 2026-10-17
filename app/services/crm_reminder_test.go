package services

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"core/app/models"
	"core/common/identity"
	"core/migrations"
	"gorm.io/gorm"
)

func reminderDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := regressionDB(t)
	if err := db.AutoMigrate(&models.CrmTodo{}); err != nil {
		t.Fatal(err)
	}
	sql, err := migrations.Files.ReadFile("0016_actionable_reminders.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.Exec(string(sql)).Error; err != nil {
		t.Fatal(err)
	}
	return db
}
func requireNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func tenantReminderContext() context.Context {
	return identity.WithIdentity(context.Background(), identity.Identity{TenantID: "1"})
}

func TestReminderLifecycleAndOwnership(t *testing.T) {
	db := reminderDB(t)
	svc := NewCrmTodo()
	ctx := testIdentity("1", "10")
	due := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("next_followup_at", due).Error)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- svc.GenerateFollowupTodos(tenantReminderContext()) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		requireNoError(t, err)
	}
	rows, total, err := svc.Query(ctx, "10", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 1 {
		t.Fatalf("concurrent generation produced %d rows", total)
	}
	original := rows[0]
	if err := svc.Handle(testIdentity("1", "11"), []string{original.ID}, "done"); err == nil {
		t.Fatal("another user's todo handled")
	}
	if _, _, err := svc.Query(ctx, "11", TodoQueryReq{}); err == nil {
		t.Fatal("queried another user")
	}
	requireNoError(t, svc.Handle(ctx, []string{original.ID}, "done"))
	requireNoError(t, svc.Handle(ctx, []string{original.ID}, "done"))
	requireNoError(t, svc.GenerateFollowupTodos(tenantReminderContext()))
	_, total, err = svc.Query(ctx, "10", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 0 {
		t.Fatal("resolved occurrence regenerated")
	}
	var resolved models.CrmTodo
	requireNoError(t, db.First(&resolved, "id = ?", original.ID).Error)
	if resolved.HandledAt == nil {
		t.Fatal("missing resolution timestamp")
	}
	// A second appointment today is a new occurrence.
	next := due.Add(30 * time.Minute)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("next_followup_at", next).Error)
	requireNoError(t, svc.GenerateFollowupTodos(tenantReminderContext()))
	rows, total, err = svc.Query(ctx, "10", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 1 || rows[0].ID == original.ID {
		t.Fatal("new appointment missing")
	}
	second := rows[0]
	// Ownership changes invalidate the old user's pending reminder on read.
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("owner_user_id", "11").Error)
	counts, err := svc.Count(ctx, "10")
	requireNoError(t, err)
	if counts["followup"] != 0 {
		t.Fatal("old owner retains pending reminder")
	}
	resolved = models.CrmTodo{}
	requireNoError(t, db.First(&resolved, "id = ?", second.ID).Error)
	if resolved.Status != "cancelled" {
		t.Fatal("stale reminder not retired")
	}
	requireNoError(t, svc.GenerateFollowupTodos(tenantReminderContext()))
	_, total, err = svc.Query(testIdentity("1", "11"), "11", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 1 {
		t.Fatal("new owner missing reminder")
	}
	// A foreign ID in a batch must roll back the whole operation.
	own := models.CrmTodo{Model: models.Model{ID: "9001"}, TenantID: "1", UserID: "10", Type: "task", Status: "pending"}
	foreign := models.CrmTodo{Model: models.Model{ID: "9002"}, TenantID: "2", UserID: "20", Type: "task", Status: "pending"}
	mustCreate(t, db, &own)
	mustCreate(t, db, &foreign)
	if err := svc.Handle(ctx, []string{own.ID, foreign.ID}, "done"); err == nil {
		t.Fatal("mixed tenant batch accepted")
	}
	requireNoError(t, db.First(&own, "id = ?", own.ID).Error)
	if own.Status != "pending" {
		t.Fatal("partial batch committed")
	}
}

func TestReminderInstallmentsAndSourceEligibility(t *testing.T) {
	db := reminderDB(t)
	svc := NewCrmTodo()
	ctx := testIdentity("1", "10")
	due := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	order := models.CrmOrder{Model: models.Model{ID: "300"}, TenantID: "1", CustomerID: "100", UserID: "10", OrderNo: "REM-1", Status: "signed", Amount: 300}
	mustCreate(t, db, &order)
	for i, status := range []string{"pending", "overdue", "received"} {
		p := models.CrmReceivablesPlan{Model: models.Model{ID: fmt.Sprint(400 + i)}, TenantID: "1", OrderID: "300", CustomerID: "100", PeriodNo: i + 1, Amount: 100, PlanDate: &due, Status: status}
		mustCreate(t, db, &p)
	}
	// Invalid cross-tenant logical reference must not leak another tenant's contract.
	mustCreate(t, db, &models.CrmReceivablesPlan{Model: models.Model{ID: "500"}, TenantID: "2", OrderID: "300", PlanDate: &due, Status: "pending"})
	requireNoError(t, svc.GenerateExpireTodos(tenantReminderContext()))
	rows, total, err := svc.Query(ctx, "10", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 2 || rows[0].SourceID == rows[1].SourceID {
		t.Fatalf("installments collapsed: %+v", rows)
	}
	requireNoError(t, svc.GenerateExpireTodos(testIdentity("2", "20")))
	_, total, err = svc.Query(testIdentity("2", "20"), "20", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 0 {
		t.Fatal("cross-tenant order joined")
	}
	requireNoError(t, db.Model(&models.CrmReceivablesPlan{}).Where("id = 400").Update("status", "received").Error)
	_, total, err = svc.Query(ctx, "10", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 1 {
		t.Fatal("paid installment still pending")
	}
	requireNoError(t, db.Model(&order).Update("status", "cancelled").Error)
	counts, err := svc.Count(ctx, "10")
	requireNoError(t, err)
	if counts["receivables_expire"] != 0 {
		t.Fatal("cancelled order still pending")
	}
	// Reinstating an unhandled source revives the same cancelled occurrence.
	requireNoError(t, db.Model(&order).Update("status", "signed").Error)
	requireNoError(t, svc.GenerateExpireTodos(tenantReminderContext()))
	_, total, err = svc.Query(ctx, "10", TodoQueryReq{Status: "pending"})
	requireNoError(t, err)
	if total != 1 {
		t.Fatal("restored source not reactivated")
	}
}

func TestReminderGenerationBeyondFiveHundred(t *testing.T) {
	db := reminderDB(t)
	due := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	customers := make([]models.CrmCustomer, 0, 601)
	for i := 0; i < 601; i++ {
		customers = append(customers, models.CrmCustomer{Model: models.Model{ID: fmt.Sprint(10000 + i)}, TenantID: "1", OwnerUserID: "10", Name: "Batch", NextFollowupAt: &due})
	}
	requireNoError(t, db.Omit("ReassignFrom").CreateInBatches(&customers, 100).Error)
	requireNoError(t, NewCrmTodo().GenerateFollowupTodos(tenantReminderContext()))
	counts, err := NewCrmTodo().Count(testIdentity("1", "10"), "10")
	requireNoError(t, err)
	if counts["followup"] != 601 {
		t.Fatalf("starved later pages: %v", counts)
	}
	requireNoError(t, NewCrmTodo().GenerateFollowupTodos(tenantReminderContext()))
	var total int64
	requireNoError(t, db.Model(&models.CrmTodo{}).Count(&total).Error)
	if total != 601 {
		t.Fatal("duplicate occurrences")
	}
}

func TestFollowupWorkdayAndViews(t *testing.T) {
	db := reminderDB(t)
	ctx := testIdentity("1", "10")
	start, end := followupDay(time.Now())
	due := start.Add(15 * time.Hour)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("next_followup_at", due).Error)
	f := models.CrmFollowup{Model: models.Model{ID: "800"}, TenantID: "1", CustomerID: "100", UserID: "10", Content: "morning call", Type: "call"}
	mustCreate(t, db, &f)
	// Explicit morning creation: tests are independent of the runner's timezone/time of day.
	requireNoError(t, db.Model(&f).Update("created_at", start.Add(time.Hour)).Error)
	view := NewCrmFollowupView()
	rows, total, err := view.ListTodayTodo(ctx, "10", PageReq{})
	requireNoError(t, err)
	if total != 1 || rows[0].ID != "100" {
		t.Fatal("afternoon appointment hidden by morning followup")
	}
	done, total, err := view.ListTodayDone(ctx, "10", PageReq{Keyword: "morning"})
	requireNoError(t, err)
	if total != 1 || done[0].CustomerName != "Own" {
		t.Fatalf("done customer missing: %+v", done)
	}
	_, total, err = view.ListNeverFollowup(ctx, "10", PageReq{})
	requireNoError(t, err)
	if total != 0 {
		t.Fatal("customer with history marked never followed")
	}
	requireNoError(t, db.Delete(&f).Error)
	_, total, err = view.ListNeverFollowup(ctx, "10", PageReq{Keyword: "Own"})
	requireNoError(t, err)
	if total != 1 {
		t.Fatal("deleted history incorrectly suppresses customer")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("next_followup_at", end).Error)
	_, total, err = view.ListTodayTodo(ctx, "10", PageReq{})
	requireNoError(t, err)
	if total != 0 {
		t.Fatal("tomorrow midnight included")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Updates(map[string]any{"next_followup_at": due, "pool_status": 1}).Error)
	_, total, err = view.ListTodayTodo(ctx, "10", PageReq{})
	requireNoError(t, err)
	if total != 0 {
		t.Fatal("pool customer included")
	}
	if _, _, err = view.ListTodayTodo(ctx, "11", PageReq{}); err == nil {
		t.Fatal("spoofed view user accepted")
	}
}

func TestReminderMigrationRetainsHistory(t *testing.T) {
	db := regressionDB(t)
	requireNoError(t, db.AutoMigrate(&models.CrmTodo{}))
	for i, status := range []string{"pending", "done", "ignored"} {
		mustCreate(t, db, &models.CrmTodo{Model: models.Model{ID: fmt.Sprint(700 + i)}, TenantID: "1", UserID: "10", Type: "followup", Status: status})
	}
	sql, err := migrations.Files.ReadFile("0016_actionable_reminders.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(sql)).Error)
	requireNoError(t, db.Exec(string(sql)).Error)
	var rows []models.CrmTodo
	requireNoError(t, db.Order("id").Find(&rows).Error)
	if len(rows) != 3 || rows[0].Status != "cancelled" || rows[1].Status != "done" || rows[2].Status != "ignored" {
		t.Fatalf("history lost: %+v", rows)
	}
	// Unkeyed custom todos remain repeatable; generated events are unique across statuses.
	mustCreate(t, db, &models.CrmTodo{Model: models.Model{ID: "710"}, TenantID: "1", UserID: "10", Type: "task"})
	a := models.CrmTodo{Model: models.Model{ID: "720"}, TenantID: "1", UserID: "10", EventKey: "event", Status: "done"}
	mustCreate(t, db, &a)
	a.ID = "721"
	if err := createCRM(db, &a); err == nil {
		t.Fatal("duplicate handled event accepted")
	}
	a.TenantID = "2"
	a.UserID = "20"
	mustCreate(t, db, &a)
}

func TestQuickRemarkWorkflow(t *testing.T) {
	db := regressionDB(t)
	requireNoError(t, db.AutoMigrate(&models.CrmQuickRemark{}))
	ctx := testIdentity("1", "10")
	svc := NewCrmQuickRemark()
	own, err := svc.Create(ctx, "10", "发送报价", 7)
	requireNoError(t, err)
	shared := models.CrmQuickRemark{Model: models.Model{ID: "850"}, TenantID: "1", Content: "公共备注"}
	mustCreate(t, db, &shared)
	rows, err := svc.List(ctx, "10")
	requireNoError(t, err)
	if len(rows) != 2 || rows[0].Sort != 7 {
		t.Fatalf("snippet list invalid: %+v", rows)
	}
	if err = svc.Update(ctx, "10", shared.ID, "sales edit"); err == nil {
		t.Fatal("sales edited shared snippet")
	}
	if err = svc.SetSort(testIdentity("1", "11"), own.ID, 9); err == nil {
		t.Fatal("other user sorted private snippet")
	}
	requireNoError(t, svc.Update(ctx, "10", own.ID, "已发报价", 3))
	requireNoError(t, svc.Update(testIdentity("1", "12"), "12", shared.ID, "共享新内容"))
	requireNoError(t, svc.Delete(ctx, "10", own.ID))
	rows, err = svc.List(ctx, "10")
	requireNoError(t, err)
	if len(rows) != 1 || rows[0].Content != "共享新内容" {
		t.Fatal("snippet CRUD incomplete")
	}
}
