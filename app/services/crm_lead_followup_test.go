package services

import (
	"core/app/models"
	"core/migrations"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/gorm"
)

func leadFollowupDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := reminderDB(t)
	ddl, err := migrations.Files.ReadFile("0024_lead_followup_workflow.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(ddl)).Error)
	requireNoError(t, db.Exec(string(ddl)).Error)
	return db
}

func TestLeadAppointmentReminderLifecycle(t *testing.T) {
	db := leadFollowupDB(t)
	ctx := testIdentity("1", "10")
	lead, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "预约演示"})
	requireNoError(t, err)
	due := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	req := LeadScheduleRequest{ID: lead.ID, NextFollowupAt: &due, NextAction: "演示产品"}
	requireNoError(t, NewCrmLead().SetSchedule(ctx, req))
	requireNoError(t, NewCrmLead().SetSchedule(ctx, req))
	var logs int64
	requireNoError(t, db.Model(&models.CrmFollowup{}).Where("lead_id = ?", lead.ID).Count(&logs).Error)
	if logs != 1 {
		t.Fatal("identical appointment retried with duplicate history")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := NewCrmTodo().GenerateLeadTodos(tenantReminderContext()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	rows, total, err := NewCrmTodo().Query(ctx, "10", TodoQueryReq{Type: "lead_followup", Status: "pending"})
	requireNoError(t, err)
	if total != 1 || len(rows) != 1 || rows[0].RefID != lead.ID || rows[0].Content != "演示产品" {
		t.Fatal("duplicate/missing lead reminder", total, rows)
	}
	requireNoError(t, NewCrmTodo().Handle(ctx, []string{rows[0].ID}, "done"))
	requireNoError(t, NewCrmTodo().GenerateLeadTodos(tenantReminderContext()))
	count, err := NewCrmTodo().Count(ctx, "10")
	requireNoError(t, err)
	if count["lead_followup"] != 0 {
		t.Fatal("handled reminder regenerated")
	}
	other := due.Add(time.Minute)
	req.NextFollowupAt = &other
	requireNoError(t, NewCrmLead().SetSchedule(ctx, req))
	requireNoError(t, NewCrmTodo().GenerateLeadTodos(tenantReminderContext()))
	requireNoError(t, NewCrmLeadExt().Transfer(testIdentity("1", "12"), lead.ID, "11"))
	count, err = NewCrmTodo().Count(ctx, "10")
	requireNoError(t, err)
	if count["lead_followup"] != 0 {
		t.Fatal("former owner kept reminder")
	}
	if err := NewCrmLead().SetSchedule(ctx, req); err == nil {
		t.Fatal("former owner changed appointment")
	}
	requireNoError(t, NewCrmTodo().GenerateLeadTodos(tenantReminderContext()))
	count, err = NewCrmTodo().Count(testIdentity("1", "11"), "11")
	requireNoError(t, err)
	if count["lead_followup"] != 1 {
		t.Fatal("new owner missed reminder")
	}
	requireNoError(t, NewCrmLeadExt().MarkLost(testIdentity("1", "11"), "11", lead.ID, "暂无预算"))
	count, err = NewCrmTodo().Count(testIdentity("1", "11"), "11")
	requireNoError(t, err)
	if count["lead_followup"] != 0 {
		t.Fatal("lost lead still reminds")
	}
	requireNoError(t, NewCrmLeadExt().Reopen(testIdentity("1", "11"), "11", lead.ID))
	got, err := NewCrmLead().Get(testIdentity("1", "11"), lead.ID)
	requireNoError(t, err)
	if got.NextFollowupAt != nil {
		t.Fatal("reopening revived old appointment")
	}
	if err := NewCrmLead().SetSchedule(testIdentity("2", "20"), req); err == nil {
		t.Fatal("foreign tenant changed appointment")
	}
	if err := NewCrmLead().SetSchedule(testIdentity("1", "11"), LeadScheduleRequest{ID: lead.ID, NextAction: strings.Repeat("字", 513)}); err == nil {
		t.Fatal("oversized action accepted")
	}
}

func TestLeadImportRequiresActiveActor(t *testing.T) {
	db := leadFollowupDB(t)
	ctx := testIdentity("1", "10")
	if _, _, err := NewCrmLead().BatchImport(ctx, "11", []map[string]string{{"标题": "越权导入"}}); err == nil {
		t.Fatal("forged import actor accepted")
	}
	if _, _, err := NewCrmLead().BatchImport(ctx, "10", make([]map[string]string, 1001)); err == nil {
		t.Fatal("unbounded import accepted")
	}
	requireNoError(t, db.Model(&models.User{}).Where("id=10").Update("status", 2).Error)
	if _, _, err := NewCrmLead().BatchImport(ctx, "10", []map[string]string{{"标题": "停用用户导入"}}); err == nil {
		t.Fatal("disabled user imported leads")
	}
	var count int64
	requireNoError(t, db.Model(&models.CrmLead{}).Count(&count).Error)
	if count != 0 {
		t.Fatal("rejected import partially wrote leads")
	}
}

func TestLeadAppointmentFollowupConversionAndRollback(t *testing.T) {
	db := leadFollowupDB(t)
	ctx := testIdentity("1", "10")
	due := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	later := due.Add(48 * time.Hour)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id=100").Update("next_followup_at", later).Error)
	lead, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "老客户新咨询", CustomerID: "100"})
	requireNoError(t, err)
	requireNoError(t, NewCrmLead().SetSchedule(ctx, LeadScheduleRequest{ID: lead.ID, NextFollowupAt: &due, NextAction: "讨论新需求"}))
	f, err := NewCrmFollowup().Create(ctx, "10", models.CrmFollowup{LeadID: lead.ID, Content: "完成首次联系", NextFollowupAt: &later, NextAction: "补充材料"})
	requireNoError(t, err)
	got, err := NewCrmLead().Get(ctx, lead.ID)
	requireNoError(t, err)
	if got.Status != "contacted" || !got.NextFollowupAt.Equal(later) {
		t.Fatal("followup failed to update current appointment")
	}
	_, err = NewCrmFollowup().Delete(ctx, []string{f.ID})
	requireNoError(t, err)
	got, err = NewCrmLead().Get(ctx, lead.ID)
	requireNoError(t, err)
	if got.NextFollowupAt == nil || !got.NextFollowupAt.Equal(due) {
		t.Fatal("followup deletion lost prior appointment")
	}
	var customer models.CrmCustomer
	requireNoError(t, db.First(&customer, "id=100").Error)
	if !customer.NextFollowupAt.Equal(later) {
		t.Fatal("lead-only work overwrote independent customer appointment")
	}
	callback := "test_lead_appointment_rollback"
	requireNoError(t, db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "crm_followup" {
			tx.AddError(errors.New("audit write failed"))
		}
	}))
	err = NewCrmLead().SetSchedule(ctx, LeadScheduleRequest{ID: lead.ID, NextFollowupAt: &later})
	if err == nil {
		t.Fatal("audit failure not returned")
	}
	requireNoError(t, db.Callback().Create().Remove(callback))
	got, err = NewCrmLead().Get(ctx, lead.ID)
	requireNoError(t, err)
	if !got.NextFollowupAt.Equal(due) {
		t.Fatal("failed audit partially changed appointment")
	}
	c, err := NewCrmLeadExt().ConvertToCustomer(ctx, "10", lead.ID, "", "")
	requireNoError(t, err)
	if c.ID != "100" || c.NextFollowupAt == nil || !c.NextFollowupAt.Equal(due) {
		t.Fatal("conversion lost earlier appointment")
	}
	got, err = NewCrmLead().Get(ctx, lead.ID)
	requireNoError(t, err)
	if got.NextFollowupAt != nil || got.NextAction != "" {
		t.Fatal("conversion left duplicate lead source")
	}
	requireNoError(t, NewCrmTodo().GenerateLeadTodos(tenantReminderContext()))
	requireNoError(t, NewCrmTodo().GenerateFollowupTodos(tenantReminderContext()))
	count, err := NewCrmTodo().Count(ctx, "10")
	requireNoError(t, err)
	if count["lead_followup"] != 0 || count["followup"] != 1 {
		t.Fatal("conversion produced duplicate/missing reminders", count)
	}
	second, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "另一次咨询", CustomerID: "100", NextFollowupAt: &later})
	requireNoError(t, err)
	c, err = NewCrmLeadExt().ConvertToCustomer(ctx, "10", second.ID, "", "")
	requireNoError(t, err)
	if !c.NextFollowupAt.Equal(due) {
		t.Fatal("later lead overwrote earlier customer appointment")
	}
	pool, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "待分配"})
	requireNoError(t, err)
	requireNoError(t, NewCrmLead().SetSchedule(ctx, LeadScheduleRequest{ID: pool.ID, NextFollowupAt: &due}))
	requireNoError(t, NewCrmLeadExt().MoveToPool(ctx, []string{pool.ID}))
	if err := NewCrmLead().SetSchedule(ctx, LeadScheduleRequest{ID: pool.ID, NextFollowupAt: &due}); err == nil {
		t.Fatal("pool lead scheduled before claim")
	}
	pool, err = NewCrmLead().Get(ctx, pool.ID)
	requireNoError(t, err)
	if pool.NextFollowupAt != nil {
		t.Fatal("pool release kept old sales appointment")
	}
}

func TestLeadHistoryAndReminderBeyond500(t *testing.T) {
	db := leadFollowupDB(t)
	ctx := testIdentity("1", "10")
	due := time.Now().Add(-time.Hour).Truncate(time.Microsecond)
	leads := make([]models.CrmLead, 605)
	for i := range leads {
		leads[i] = models.CrmLead{Model: models.Model{ID: fmt.Sprint(10000 + i)}, TenantID: "1", OwnerUserID: "10", Title: "预约线索", Status: "new", NextFollowupAt: &due}
	}
	requireNoError(t, db.Omit("CustomerID", "ConvertCustomerID").CreateInBatches(&leads, 200).Error)
	requireNoError(t, NewCrmTodo().GenerateLeadTodos(tenantReminderContext()))
	count, err := NewCrmTodo().Count(ctx, "10")
	requireNoError(t, err)
	if count["lead_followup"] != 605 {
		t.Fatal("lead scan stopped at first 500", count)
	}
	rows := make([]models.CrmFollowup, 1005)
	for i := range rows {
		rows[i] = models.CrmFollowup{Model: models.Model{ID: fmt.Sprint(20000 + i)}, TenantID: "1", LeadID: leads[0].ID, UserID: "10", Type: "call", Content: "跟进历史"}
	}
	requireNoError(t, db.Omit("CustomerID", "OpportunityID", "ContactID").CreateInBatches(&rows, 200).Error)
	page, err := NewCrmLead().History(ctx, LeadHistoryRequest{LeadID: leads[0].ID, PageSize: 1000})
	requireNoError(t, err)
	if len(page.Items) != 100 || !page.HasMore {
		t.Fatal("unbounded history page")
	}
	first := page.NextCursor
	seen := map[string]bool{}
	for {
		for _, row := range page.Items {
			if seen[row.ID] {
				t.Fatal("duplicate history")
			}
			seen[row.ID] = true
		}
		if !page.HasMore {
			break
		}
		page, err = NewCrmLead().History(ctx, LeadHistoryRequest{LeadID: leads[0].ID, Cursor: page.NextCursor})
		requireNoError(t, err)
	}
	if len(seen) != 1005 {
		t.Fatal("history truncation", len(seen))
	}
	if _, err := NewCrmLead().History(ctx, LeadHistoryRequest{LeadID: leads[1].ID, Cursor: first}); err == nil {
		t.Fatal("cursor crossed lead")
	}
	if _, err := NewCrmFollowup().ListByLead(ctx, leads[0].ID); err == nil {
		t.Fatal("legacy unbounded history still enabled")
	}
	requireNoError(t, NewCrmLeadExt().Transfer(testIdentity("1", "12"), leads[0].ID, "11"))
	if _, err := NewCrmLead().History(ctx, LeadHistoryRequest{LeadID: leads[0].ID, Cursor: first}); err == nil {
		t.Fatal("former owner retained cursor access")
	}
}
