package services

import (
	"core/app/constants"
	"core/app/models"
	"core/migrations"
	"fmt"
	"gorm.io/gorm"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func installPoolIndexes(t *testing.T, db *gorm.DB) {
	t.Helper()
	sql, err := migrations.Files.ReadFile("0019_pool_recycle_indexes.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(sql)).Error)
	requireNoError(t, db.Exec(string(sql)).Error)
}
func TestPoolRuleServiceGuards(t *testing.T) {
	db := regressionDB(t)
	svc := NewCrmPool()
	admin := testIdentity("1", "12")
	input := models.CrmPoolRule{TenantID: "2", Enabled: true, RecycleDays: 30, ClaimLimit: 5}
	if _, err := svc.SetRule(testIdentity("1", "10"), input); err == nil {
		t.Fatal("sales changed pool policy")
	}
	if _, err := svc.SetRule(testIdentity("2", "12"), input); err == nil {
		t.Fatal("foreign actor changed policy")
	}
	for _, r := range []models.CrmPoolRule{{RecycleDays: -1}, {NoDealDays: 36501}, {ClaimLimit: -1}, {ProtectDays: -1}, {ClaimCooldownDays: -1}, {Enabled: true}} {
		if _, err := svc.SetRule(admin, r); err == nil {
			t.Fatalf("invalid policy accepted: %+v", r)
		}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := svc.SetRule(admin, input); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		requireNoError(t, err)
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmPoolRule{}).Count(&n).Error)
	if n != 1 {
		t.Fatal("concurrent first-save duplicated policy")
	}
	saved, err := svc.SetRule(admin, models.CrmPoolRule{})
	requireNoError(t, err)
	if saved.TenantID != "1" || saved.Enabled || saved.ClaimLimit != 0 || saved.RecycleDays != 0 {
		t.Fatal("zero-valued policy not persisted")
	}
	requireNoError(t, db.Model(&models.User{}).Where("id = 12").Update("status", 2).Error)
	if _, err := svc.SetRule(admin, input); err == nil {
		t.Fatal("disabled admin changed policy")
	}
}

func TestPoolHistoryScopeAndStats(t *testing.T) {
	db := regressionDB(t)
	installPoolIndexes(t, db)
	svc := NewCrmPool()
	sales := testIdentity("1", "10")
	admin := testIdentity("1", "12")
	mustCreate(t, db, &models.CrmCustomer{Model: models.Model{ID: "103"}, TenantID: "1", Name: "Pool", PoolStatus: 1})
	if _, err := svc.Get(sales, "103"); err != nil {
		t.Fatal("sales cannot inspect pool customer", err)
	}
	for _, id := range []string{"100", "101", "102"} {
		if _, err := svc.Get(sales, id); err == nil {
			t.Fatal("pool detail exposed private customer", id)
		}
	}
	for i, cid := range []string{"100", "101", "102", "103"} {
		tenant := "1"
		if cid == "102" {
			tenant = "2"
		}
		mustCreate(t, db, &models.CrmPoolLog{Model: models.Model{ID: fmt.Sprint(800 + i)}, TenantID: tenant, CustomerID: cid, Action: "claim", CustomerName: cid})
	}
	rows, n, err := svc.ListLogs(sales, "", nil)
	requireNoError(t, err)
	if n != 2 || len(rows) != 2 {
		t.Fatalf("private/foreign logs leaked: %d", n)
	}
	_, n, err = svc.ListLogs(sales, "101", nil)
	requireNoError(t, err)
	if n != 0 {
		t.Fatal("private filter leaked history")
	}
	stats, err := svc.GetStats(sales)
	requireNoError(t, err)
	if stats.ClaimedToday != 2 || stats.TotalInPool != 1 {
		t.Fatalf("incorrect scoped stats: %+v", stats)
	}
	_, n, err = svc.ListLogs(admin, "", nil)
	requireNoError(t, err)
	if n != 3 {
		t.Fatal("manager cannot read tenant logs")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 101").Update("shared_staff_ids", constants.StringArrayField{"10"}).Error)
	_, n, err = svc.ListLogs(sales, "101", nil)
	requireNoError(t, err)
	if n != 1 {
		t.Fatal("shared customer history unavailable")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("owner_user_id", "11").Error)
	_, n, err = svc.ListLogs(sales, "100", nil)
	requireNoError(t, err)
	if n != 0 {
		t.Fatal("former owner retained logs")
	}
	requireNoError(t, db.Delete(&models.CrmCustomer{}, "id = 101").Error)
	_, n, err = svc.ListLogs(sales, "101", nil)
	requireNoError(t, err)
	if n != 0 {
		t.Fatal("deleted customer history leaked")
	}
	requireNoError(t, db.Exec("ALTER TABLE crm_pool_log RENAME TO crm_pool_log_unavailable").Error)
	if _, err = svc.GetStats(sales); err == nil {
		t.Fatal("stats swallowed database failure")
	}
}

func seedRecycleBatch(t *testing.T, db *gorm.DB, n int) {
	t.Helper()
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "active", Status: 1})
	installPoolIndexes(t, db)
	mustCreate(t, db, &models.CrmPoolRule{Model: models.Model{ID: "900"}, TenantID: "1", Enabled: true, RecycleDays: 7, ProtectDays: 3})
	old := time.Now().AddDate(0, 0, -30)
	rows := make([]models.CrmCustomer, 0, n)
	for i := 0; i < n; i++ {
		rows = append(rows, models.CrmCustomer{Model: models.Model{ID: fmt.Sprint(1000 + i)}, Timestamp: models.Timestamp{CreatedAt: old}, TenantID: "1", OwnerUserID: "10", Name: fmt.Sprint(i)})
	}
	requireNoError(t, db.Omit("ReassignFrom").Create(&rows).Error)
}
func TestPoolRecycleBatchConcurrent(t *testing.T) {
	db := regressionDB(t)
	seedRecycleBatch(t, db, 450)
	now := time.Now()
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 1000").Update("is_locked", true).Error)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 1001").Update("claimed_at", now).Error)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 1002").Update("reassign_status", 1).Error)
	mustCreate(t, db, &models.CrmFollowup{Model: models.Model{ID: "9000"}, TenantID: "1", CustomerID: "1003", Content: "recent"})
	// A foreign tenant's erroneous relation must not protect this customer's lifecycle.
	mustCreate(t, db, &models.CrmFollowup{Model: models.Model{ID: "9001"}, TenantID: "2", CustomerID: "1004", Content: "foreign"})
	mustCreate(t, db, &models.CrmFollowup{Model: models.Model{ID: "9002"}, TenantID: "1", CustomerID: "1005", Content: "deleted"})
	requireNoError(t, db.Delete(&models.CrmFollowup{}, "id = 9002").Error)
	var count atomic.Int64
	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := NewCrmPool().AutoRecycle(testIdentity("1", ""))
			count.Add(n)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		requireNoError(t, err)
	}
	if count.Load() != 446 {
		t.Fatalf("expected 446 recycled, got %d", count.Load())
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmPoolLog{}).Where("action='recycle'").Count(&n).Error)
	if n != 446 {
		t.Fatalf("duplicate/missing audit: %d", n)
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("pool_status = 1 AND (owner_user_id IS NOT NULL OR prev_owner_user_id <> '10')").Count(&n).Error)
	if n != 0 {
		t.Fatal("invalid recycled owner")
	}
	n, err := NewCrmPool().AutoRecycle(testIdentity("1", ""))
	requireNoError(t, err)
	if n != 0 {
		t.Fatal("retry duplicated recycling")
	}
	if _, err := NewCrmPool().AutoRecycle(testIdentity("1", "10")); err == nil {
		t.Fatal("sales invoked automatic recycling")
	}
}
func TestPoolRecycleBatchRollback(t *testing.T) {
	db := regressionDB(t)
	seedRecycleBatch(t, db, 450)
	requireNoError(t, db.Exec(`CREATE FUNCTION fail_pool_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN IF NEW.customer_id >= 1200 THEN RAISE EXCEPTION 'audit unavailable'; END IF; RETURN NEW; END $$; CREATE TRIGGER fail_pool_audit BEFORE INSERT ON crm_pool_log FOR EACH ROW EXECUTE FUNCTION fail_pool_audit()`).Error)
	n, err := NewCrmPool().AutoRecycle(testIdentity("1", ""))
	if err == nil || n != 200 {
		t.Fatalf("expected failed second batch and 200 committed, got %d/%v", n, err)
	}
	var actual int64
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("pool_status = 1").Count(&actual).Error)
	if actual != 200 {
		t.Fatal("failed batch partially recycled")
	}
	requireNoError(t, db.Exec("DROP TRIGGER fail_pool_audit ON crm_pool_log").Error)
	n, err = NewCrmPool().AutoRecycle(testIdentity("1", ""))
	requireNoError(t, err)
	if n != 250 {
		t.Fatal("retry failed to complete remaining records")
	}
}

func TestPoolRecycleNoDealAndTenantLifecycle(t *testing.T) {
	db := regressionDB(t)
	seedRecycleBatch(t, db, 8)
	requireNoError(t, db.Model(&models.CrmPoolRule{}).Where("id = 900").Updates(map[string]any{"recycle_days": 0, "no_deal_days": 7}).Error)
	mustCreate(t, db, &models.CrmOpportunity{Model: models.Model{ID: "7000"}, TenantID: "1", CustomerID: "1000", Status: "open"})
	mustCreate(t, db, &models.CrmOpportunity{Model: models.Model{ID: "7001"}, TenantID: "1", CustomerID: "1001", Status: "lost"})
	mustCreate(t, db, &models.CrmOrder{Model: models.Model{ID: "7010"}, TenantID: "1", CustomerID: "1002", Status: "signed", OrderNo: "signed"})
	mustCreate(t, db, &models.CrmOrder{Model: models.Model{ID: "7011"}, TenantID: "1", CustomerID: "1003", Status: "completed", OrderNo: "completed"})
	mustCreate(t, db, &models.CrmOrder{Model: models.Model{ID: "7012"}, TenantID: "1", CustomerID: "1004", Status: "signed", OrderNo: "deleted"})
	requireNoError(t, db.Delete(&models.CrmOrder{}, "id = 7012").Error)
	mustCreate(t, db, &models.CrmOrder{Model: models.Model{ID: "7013"}, TenantID: "2", CustomerID: "1005", Status: "signed", OrderNo: "foreign"})
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 1006").Update("claimed_at", time.Now().AddDate(0, 0, -5)).Error)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 1007").Update("created_at", nil).Error)
	// Active, disabled and expired tenants all have eligible old customers.
	expired := time.Now().Add(-time.Hour)
	for _, tenant := range []models.Tenant{
		{Model: models.Model{ID: "2"}, Code: "disabled", Status: 2},
		{Model: models.Model{ID: "3"}, Code: "expired", Status: 1, ExpireAt: &expired},
	} {
		mustCreate(t, db, &tenant)
	}
	for _, tid := range []string{"2", "3"} {
		mustCreate(t, db, &models.CrmPoolRule{Model: models.Model{ID: "90" + tid}, TenantID: tid, Enabled: true, RecycleDays: 1})
		mustCreate(t, db, &models.CrmCustomer{Model: models.Model{ID: "200" + tid}, TenantID: tid, OwnerUserID: "20", Timestamp: models.Timestamp{CreatedAt: time.Now().AddDate(0, 0, -30)}})
	}
	NewCrmPool().AutoRecycleAll()
	var ids []string
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("pool_status = 1").Order("id").Pluck("id", &ids).Error)
	if fmt.Sprint(ids) != "[1001 1004 1005]" {
		t.Fatalf("incorrect no-deal or tenant isolation: %v", ids)
	}
}
