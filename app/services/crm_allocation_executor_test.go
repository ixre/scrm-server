package services

import (
	"context"
	"core/app/constants"
	"core/app/models"
	"core/migrations"
	"fmt"
	"gorm.io/gorm"
	"sync"
	"testing"
)

func allocationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := channelWorkflowDB(t)
	sql, err := migrations.Files.ReadFile("0020_acquisition_allocation.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(sql)).Error)
	requireNoError(t, db.Exec(string(sql)).Error)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Status: 1})
	mustCreate(t, db, &models.CrmChannelConfig{Model: models.Model{ID: "900"}, TenantID: "1", Channel: "xiaohongshu", CorpID: "allocation-app", Enabled: 1})
	return db
}
func allocationRule(t *testing.T, targets ...string) models.CrmAutoAllocation {
	t.Helper()
	r := models.CrmAutoAllocation{Name: "新获客", EntityType: "customer", Strategy: "round_robin", IsEnabled: true, ExecutionEnabled: true, TargetIDs: constants.StringArrayField(targets)}
	requireNoError(t, NewCrmAutoAllocation().Create(testIdentity("1", "12"), &r))
	return r
}
func xhsAllocation(ctx context.Context, id string) error {
	return NewCrmCustomer().SyncFromXhs(ctx, "allocation-app", XhsLeadEvent{EventID: id, ExtUserID: id, Nickname: id, Mobile: "13900000000"})
}
func TestAllocationOptInAndGuards(t *testing.T) {
	db := allocationDB(t)
	svc := NewCrmAutoAllocation()
	admin := testIdentity("1", "12")
	legacy := models.CrmAutoAllocation{Name: "旧配置", EntityType: "customer", Strategy: "round_robin", IsEnabled: true, Conditions: constants.StringArrayField{"legacy-condition"}, TargetIDs: constants.StringArrayField{"10"}}
	requireNoError(t, svc.Create(admin, &legacy))
	requireNoError(t, xhsAllocation(testIdentity("1", ""), "legacy"))
	var c models.CrmCustomer
	requireNoError(t, db.Where("name='legacy'").First(&c).Error)
	if c.OwnerUserID != "" || c.PoolStatus != 1 {
		t.Fatal("legacy rule executed without opt-in")
	}
	update := legacy
	update.ExecutionEnabled = true
	update.Conditions = nil
	if err := svc.Update(admin, legacy.ID, &update); err == nil {
		t.Fatal("omitted legacy conditions bypassed execution guard")
	}
	for _, change := range []func(*models.CrmAutoAllocation){func(r *models.CrmAutoAllocation) { r.EntityType = "lead" }, func(r *models.CrmAutoAllocation) { r.Strategy = "weighted" }, func(r *models.CrmAutoAllocation) { r.Sources = constants.StringArrayField{"unimplemented"} }, func(r *models.CrmAutoAllocation) { r.TargetIDs = constants.StringArrayField{"20"} }} {
		r := models.CrmAutoAllocation{Name: "invalid", EntityType: "customer", Strategy: "round_robin", IsEnabled: true, ExecutionEnabled: true, TargetIDs: constants.StringArrayField{"10"}}
		change(&r)
		if err := svc.Create(admin, &r); err == nil {
			t.Fatal("invalid executable rule accepted")
		}
	}
	if _, err := svc.Logs(testIdentity("1", "10"), "", "", 20); err == nil {
		t.Fatal("sales read allocation audit")
	}
}
func TestAllocationConcurrentRoundRobinAndDedup(t *testing.T) {
	db := allocationDB(t)
	rule := allocationRule(t, "10", "11")
	ctx := testIdentity("1", "")
	var wg sync.WaitGroup
	errs := make(chan error, 24)
	for i := 0; i < 12; i++ {
		for j := 0; j < 2; j++ {
			wg.Add(1)
			go func(i int) { defer wg.Done(); errs <- xhsAllocation(ctx, fmt.Sprintf("event-%02d", i)) }(i)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		requireNoError(t, err)
	}
	for _, id := range []string{"10", "11"} {
		var n int64
		requireNoError(t, db.Model(&models.CrmCustomer{}).Where("source = 'xiaohongshu' AND owner_user_id = ?", id).Count(&n).Error)
		if n != 6 {
			t.Fatalf("unfair round robin: %s=%d", id, n)
		}
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmAllocationLog{}).Count(&n).Error)
	if n != 12 {
		t.Fatal("duplicate/missing allocation logs", n)
	}
	requireNoError(t, db.Model(&models.CrmChannelTask{}).Where("assign_user_id IS NOT NULL").Count(&n).Error)
	if n != 12 {
		t.Fatal("channel tasks did not inherit allocated owner")
	}
	first, err := NewCrmAutoAllocation().Logs(testIdentity("1", "12"), rule.ID, "", 5)
	requireNoError(t, err)
	second, err := NewCrmAutoAllocation().Logs(testIdentity("1", "12"), rule.ID, first.NextCursor, 5)
	requireNoError(t, err)
	if len(first.Items) != 5 || !first.HasMore || len(second.Items) != 5 || first.Items[4].ID == second.Items[0].ID {
		t.Fatal("audit cursor pagination failed")
	}
	// A later event for an existing identity preserves current ownership and cursor history.
	requireNoError(t, NewCrmCustomer().SyncFromXhs(ctx, "allocation-app", XhsLeadEvent{EventID: "later", ExtUserID: "event-00", Nickname: "existing"}))
	requireNoError(t, db.Model(&models.CrmAllocationLog{}).Count(&n).Error)
	if n != 12 {
		t.Fatal("existing customer was allocated again")
	}
}
func TestAllocationQuotaFallbackAndIsolation(t *testing.T) {
	db := allocationDB(t)
	rule := allocationRule(t, "10", "11")
	mustCreate(t, db, &models.CrmPoolRule{Model: models.Model{ID: "901"}, TenantID: "1", ClaimLimit: 2})
	requireNoError(t, db.Model(&models.User{}).Where("id=11").Update("status", 2).Error)
	requireNoError(t, xhsAllocation(testIdentity("1", ""), "first"))
	requireNoError(t, xhsAllocation(testIdentity("1", ""), "full"))
	var first, full models.CrmCustomer
	requireNoError(t, db.Where("name='first'").First(&first).Error)
	requireNoError(t, db.Where("name='full'").First(&full).Error)
	if first.OwnerUserID != "10" || full.OwnerUserID != "" || full.PoolStatus != 1 {
		t.Fatal("quota/disabled fallback failed")
	}
	var entry models.CrmAllocationLog
	requireNoError(t, db.Where("customer_id = ?", full.ID).First(&entry).Error)
	if entry.Outcome != "pending" {
		t.Fatal("fallback not auditable")
	}
	// Runtime tenant filtering protects against stale/corrupt foreign target IDs.
	requireNoError(t, db.Model(&models.CrmAutoAllocation{}).Where("id = ?", rule.ID).Update("target_ids", constants.StringArrayField{"20"}).Error)
	requireNoError(t, xhsAllocation(testIdentity("1", ""), "foreign-target"))
	var c models.CrmCustomer
	requireNoError(t, db.Where("name='foreign-target'").First(&c).Error)
	if c.OwnerUserID != "" {
		t.Fatal("cross-tenant allocation")
	}
}
func TestAllocationRollbackAndLandingInheritance(t *testing.T) {
	db := allocationDB(t)
	rule := allocationRule(t, "10", "11")
	page, err := NewCrmLandingPage().Create(testIdentity("1", "12"), models.CrmLandingPage{Title: "分配获客"})
	requireNoError(t, err)
	requireNoError(t, db.Exec(`CREATE FUNCTION fail_allocation_log() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit failed'; END $$; CREATE TRIGGER fail_allocation BEFORE INSERT ON crm_allocation_log FOR EACH ROW EXECUTE FUNCTION fail_allocation_log()`).Error)
	if _, err = NewCrmLandingPage().SubmitForm(page.Slug, map[string]string{"name": "新客户", "mobile": "13900009001"}); err == nil {
		t.Fatal("audit failure committed acquisition")
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("mobile='13900009001'").Count(&n).Error)
	if n != 0 {
		t.Fatal("partial acquisition committed")
	}
	var saved models.CrmAutoAllocation
	requireNoError(t, db.First(&saved, "id = ?", rule.ID).Error)
	if saved.LastTargetID != "" {
		t.Fatal("cursor advanced after rollback")
	}
	requireNoError(t, db.Exec("DROP TRIGGER fail_allocation ON crm_allocation_log").Error)
	c, err := NewCrmLandingPage().SubmitForm(page.Slug, map[string]string{"name": "新客户", "mobile": "13900009001"})
	requireNoError(t, err)
	if c.OwnerUserID != "10" {
		t.Fatal("rollback changed next recipient")
	}
	var lead models.CrmLead
	requireNoError(t, db.Where("customer_id = ?", c.ID).First(&lead).Error)
	if lead.OwnerUserID != c.OwnerUserID || lead.PoolStatus != 0 {
		t.Fatal("landing lead did not inherit owner")
	}
	_, err = NewCrmLandingPage().SubmitForm(page.Slug, map[string]string{"name": "新客户", "mobile": "13900009001"})
	requireNoError(t, err)
	requireNoError(t, db.Model(&models.CrmAllocationLog{}).Count(&n).Error)
	if n != 1 {
		t.Fatal("repeat landing submission reallocated customer")
	}
}
func TestAllocationSourcePriorityAndRandom(t *testing.T) {
	db := allocationDB(t)
	low := allocationRule(t, "10")
	high := allocationRule(t, "11")
	requireNoError(t, db.Model(&models.CrmAutoAllocation{}).Where("id = ?", high.ID).Updates(map[string]any{"priority": 10, "strategy": "random", "sources": constants.StringArrayField{"web_form"}}).Error)
	requireNoError(t, xhsAllocation(testIdentity("1", ""), "source"))
	var c models.CrmCustomer
	requireNoError(t, db.Where("name='source'").First(&c).Error)
	if c.OwnerUserID != "10" {
		t.Fatal("source matching ignored")
	}
	page, err := NewCrmLandingPage().Create(testIdentity("1", "12"), models.CrmLandingPage{Title: "优先级"})
	requireNoError(t, err)
	c, err = NewCrmLandingPage().SubmitForm(page.Slug, map[string]string{"name": "高优先级", "mobile": "13900009002"})
	requireNoError(t, err)
	if c.OwnerUserID != "11" {
		t.Fatal("priority/random candidate failed")
	}
	requireNoError(t, NewCrmAutoAllocation().ToggleStatus(testIdentity("1", "12"), low.ID, false))
	requireNoError(t, xhsAllocation(testIdentity("1", ""), "no-match"))
	requireNoError(t, db.Where("name='no-match'").First(&models.CrmCustomer{}).Error)
	var n int64
	requireNoError(t, db.Model(&models.CrmAllocationLog{}).Where("customer_name='no-match'").Count(&n).Error)
	if n != 0 {
		t.Fatal("unmatched rule executed")
	}
}

func TestAllocationConcurrentManualQuota(t *testing.T) {
	db := allocationDB(t)
	allocationRule(t, "10")
	mustCreate(t, db, &models.CrmPoolRule{Model: models.Model{ID: "902"}, TenantID: "1", ClaimLimit: 2})
	mustCreate(t, db, &models.CrmCustomer{Model: models.Model{ID: "903"}, TenantID: "1", PoolStatus: 1, Name: "manual pool"})
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); errs <- xhsAllocation(testIdentity("1", ""), fmt.Sprint("quota-", i)) }(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _ = NewCrmPool().Assign(testIdentity("1", "12"), "12", "10", []string{"903"})
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		requireNoError(t, err)
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("tenant_id = 1 AND owner_user_id = 10 AND pool_status = 0").Count(&n).Error)
	if n != 2 {
		t.Fatal("concurrent acquisition/manual claim exceeded quota", n)
	}
	requireNoError(t, db.Model(&models.CrmAllocationLog{}).Count(&n).Error)
	if n != 8 {
		t.Fatal("quota fallback lost acquisition logs")
	}
}
