package services

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"core/app/models"
	"core/common/app"
	"core/migrations"
	"gorm.io/gorm"
)

func channelWorkflowDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := regressionDB(t)
	requireNoError(t, db.AutoMigrate(&models.CrmChannelTaskLog{}, &models.CrmChannelConfig{}))
	sql, err := migrations.Files.ReadFile("0017_channel_task_workflow.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(sql)).Error)
	return db
}
func readChannelTask(t *testing.T, db *gorm.DB, id string) models.CrmChannelTask {
	t.Helper()
	var task models.CrmChannelTask
	requireNoError(t, db.First(&task, "id = ?", id).Error)
	return task
}
func TestChannelTaskPermissionsAndLifecycle(t *testing.T) {
	db := channelWorkflowDB(t)
	svc := NewCrmChannelTask()
	ctx := testIdentity("1", "10")
	admin := testIdentity("1", "12")
	requireNoError(t, svc.CreateLeadToWecomTask(testIdentity("1", ""), "100"))
	requireNoError(t, svc.CreateLeadToWecomTask(testIdentity("1", ""), "101"))
	rows, total, err := svc.List(ctx, "", &app.Pager{Page: 1, PageSize: 1})
	requireNoError(t, err)
	if total != 1 || len(rows) != 1 || rows[0].CustomerName != "Own" || !rows[0].CanResolve {
		t.Fatalf("visibility/capabilities: %+v total=%d", rows, total)
	}
	task := rows[0].CrmChannelTask
	if task.AssignUserID != "10" || task.DueAt == nil {
		t.Fatal("owner/SLA not initialized")
	}
	if err = svc.Resolve(testIdentity("1", "11"), task.ID, "done", "越权", task.Version); err == nil {
		t.Fatal("other user resolved task")
	}
	if err = svc.Assign(ctx, task.ID, "11", task.Version); err == nil {
		t.Fatal("sales assigned task")
	}
	if err = svc.Assign(admin, task.ID, "20", task.Version); err == nil {
		t.Fatal("foreign user assigned")
	}
	if err = svc.Assign(admin, task.ID, "11", task.Version); err == nil {
		t.Fatal("assignment granted implicit customer access")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("shared_staff_ids", `["11"]`).Error)
	requireNoError(t, svc.Assign(admin, task.ID, "11", task.Version))
	task = readChannelTask(t, db, task.ID)
	if task.AssignUserID != "11" || task.Version != 2 {
		t.Fatal("assignment failed")
	}
	if err = svc.Resolve(ctx, task.ID, "done", "owner but not assignee", task.Version); err == nil {
		t.Fatal("owner resolved another assignee's task")
	}
	if err = svc.Resolve(testIdentity("1", "11"), task.ID, "done", "", task.Version); err == nil {
		t.Fatal("empty result accepted")
	}
	if err = svc.Resolve(testIdentity("1", "11"), task.ID, "done", "stale", 1); err == nil {
		t.Fatal("stale update accepted")
	}
	requireNoError(t, svc.Resolve(testIdentity("1", "11"), task.ID, "done", "已添加企微", task.Version))
	requireNoError(t, svc.Resolve(testIdentity("1", "11"), task.ID, "done", "已添加企微", task.Version))
	requireNoError(t, svc.CreateLeadToWecomTask(testIdentity("1", ""), "100"))
	var count int64
	requireNoError(t, db.Model(&models.CrmChannelTask{}).Where("customer_id = 100").Count(&count).Error)
	if count != 1 {
		t.Fatal("retry reopened or duplicated completed task")
	}
	task = readChannelTask(t, db, task.ID)
	if err = svc.Resolve(admin, task.ID, "skipped", "覆盖终态", task.Version); err == nil {
		t.Fatal("terminal result overwritten")
	}
	requireNoError(t, svc.Reopen(admin, task.ID, "客户要求再次联系", task.Version))
	task = readChannelTask(t, db, task.ID)
	requireNoError(t, svc.Claim(ctx, task.ID, task.Version))
	task = readChannelTask(t, db, task.ID)
	past := time.Now().Add(-time.Hour)
	requireNoError(t, db.Model(&task).Update("due_at", past).Error)
	requireNoError(t, svc.Expire(context.Background()))
	task = readChannelTask(t, db, task.ID)
	if task.Status != "timeout" {
		t.Fatal("task not marked overdue")
	}
	requireNoError(t, svc.Resolve(ctx, task.ID, "skipped", "客户暂不需要", task.Version))
	logs, n, err := svc.Logs(ctx, task.ID, &app.Pager{})
	requireNoError(t, err)
	if n != 7 {
		t.Fatalf("expected one log per transition, got %d: %+v", n, logs)
	}
	if _, _, err = svc.Logs(testIdentity("2", "20"), task.ID, nil); err == nil {
		t.Fatal("foreign task history disclosed")
	}
	// A handover revokes access immediately, including history.
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Updates(map[string]any{"owner_user_id": "11", "shared_staff_ids": "[]"}).Error)
	_, total, err = svc.List(ctx, "", nil)
	requireNoError(t, err)
	if total != 0 {
		t.Fatal("transferred customer visible to former owner")
	}
}

func TestChannelTaskConcurrencyAndAuditRollback(t *testing.T) {
	db := channelWorkflowDB(t)
	svc := NewCrmChannelTask()
	ctx := testIdentity("1", "")
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- svc.CreateLeadToWecomTask(ctx, "100") }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		requireNoError(t, err)
	}
	var tasks []models.CrmChannelTask
	requireNoError(t, db.Find(&tasks).Error)
	if len(tasks) != 1 {
		t.Fatal("duplicate concurrent source task")
	}
	task := tasks[0]
	// Audit failure must roll back the business state as well.
	requireNoError(t, db.Exec("ALTER TABLE crm_channel_task_log ADD CONSTRAINT reject_done CHECK (action <> 'done')").Error)
	if err := svc.Resolve(testIdentity("1", "10"), task.ID, "done", "实际结果", task.Version); err == nil {
		t.Fatal("ignored audit failure")
	}
	task = readChannelTask(t, db, task.ID)
	if task.Status != "pending" || task.Version != 1 {
		t.Fatal("partial transition committed")
	}
	requireNoError(t, db.Exec("ALTER TABLE crm_channel_task_log DROP CONSTRAINT reject_done").Error)
	results := make(chan error, 2)
	for _, status := range []string{"done", "skipped"} {
		wg.Add(1)
		go func(status string) {
			defer wg.Done()
			results <- svc.Resolve(testIdentity("1", "10"), task.ID, status, "concurrent decision", 1)
		}(status)
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("lost update protection failed: %d successes", success)
	}
}

func TestXhsLeadAtomicAndIdempotent(t *testing.T) {
	db := channelWorkflowDB(t)
	ctx := testIdentity("1", "")
	cfg := models.CrmChannelConfig{Model: models.Model{ID: "900"}, TenantID: "1", Channel: "xiaohongshu", CorpID: "xhs-app", Enabled: 1}
	mustCreate(t, db, &cfg)
	ev := XhsLeadEvent{EventID: "event-1", ExtUserID: "external-1", Nickname: "留资客户", Mobile: "13912345678", Content: "咨询方案", FormID: "f-1", HappenedAt: time.Now().Unix()}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- NewCrmCustomer().SyncFromXhs(ctx, "xhs-app", ev) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		requireNoError(t, err)
	}
	for _, table := range []string{"crm_customer", "crm_channel_interaction", "channel_link", "crm_channel_task", "crm_channel_task_log"} {
		q := db.Table(table).Where("tenant_id = 1")
		if table == "crm_customer" {
			q = q.Where("ext_customer_id = 'external-1'")
		}
		var n int64
		requireNoError(t, q.Count(&n).Error)
		if n != 1 {
			t.Fatalf("%s duplicated: %d", table, n)
		}
	}
	var customer models.CrmCustomer
	requireNoError(t, db.Where("ext_customer_id = 'external-1'").First(&customer).Error)
	if customer.PoolStatus != 1 {
		t.Fatal("unassigned customer not in pool")
	}
	// Rollback every part if the final task write fails.
	requireNoError(t, db.Exec("ALTER TABLE crm_channel_task ADD CONSTRAINT reject_new_source CHECK (customer_id = "+customer.ID+")").Error)
	bad := ev
	bad.EventID = "event-2"
	bad.ExtUserID = "external-2"
	bad.Mobile = "13987654321"
	if err := NewCrmCustomer().SyncFromXhs(ctx, "xhs-app", bad); err == nil {
		t.Fatal("task failure swallowed")
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("ext_customer_id = 'external-2'").Count(&n).Error)
	if n != 0 {
		t.Fatal("partial customer committed")
	}
	requireNoError(t, db.Exec("ALTER TABLE crm_channel_task DROP CONSTRAINT reject_new_source").Error)
	requireNoError(t, NewCrmCustomer().SyncFromXhs(ctx, "xhs-app", bad))
	// Merged source identities must follow their ChannelLink instead of resurrecting a customer.
	requireNoError(t, NewCrmDedup().Merge(testIdentity("1", "12"), "100", []string{customer.ID}))
	ev.EventID = "event-3"
	requireNoError(t, NewCrmCustomer().SyncFromXhs(ctx, "xhs-app", ev))
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("ext_customer_id = 'external-1'").Count(&n).Error)
	if n != 0 {
		t.Fatal("merged customer resurrected")
	}
	var rec models.CrmChannelInteraction
	requireNoError(t, db.Order("id desc").First(&rec).Error)
	if rec.CustomerID != "100" {
		t.Fatal("interaction ignored merged identity")
	}
	requireNoError(t, db.Model(&cfg).Update("enabled", 2).Error)
	ev.EventID = "disabled"
	if err := NewCrmCustomer().SyncFromXhs(ctx, "xhs-app", ev); err == nil {
		t.Fatal("disabled channel accepted event")
	}
}

func TestChannelTaskMigrationAndPagination(t *testing.T) {
	db := channelWorkflowDB(t)
	now := time.Now()
	for i := 0; i < 25; i++ {
		task := models.CrmChannelTask{Model: models.Model{ID: fmt.Sprint(10000 + i)}, TenantID: "1", CustomerID: "100", AssignUserID: "10", Action: "call_back", Status: "pending", DueAt: &now}
		mustCreate(t, db, &task)
	}
	rows, total, err := NewCrmChannelTask().List(testIdentity("1", "10"), "pending", &app.Pager{Page: 2, PageSize: 10})
	requireNoError(t, err)
	if total != 25 || len(rows) != 10 || rows[0].ID == "10024" {
		t.Fatal("pagination ignored")
	}
	sql, err := migrations.Files.ReadFile("0017_channel_task_workflow.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(sql)).Error)
	var n int64
	requireNoError(t, db.Model(&models.CrmChannelTask{}).Count(&n).Error)
	if n != 25 {
		t.Fatal("migration lost history")
	}
}
