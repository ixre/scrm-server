package services

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"core/common/log"
	"core/conf"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// Each integration test uses its own schema. Never use the application's configured DB.
func regressionDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("SCRM_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SCRM_TEST_DATABASE_URL to an isolated PostgreSQL database")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config := &gorm.Config{NamingStrategy: schema.NamingStrategy{SingularTable: true}, Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("regression_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + name).Error; err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", name)
	u.RawQuery = q.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), config)
	if err != nil {
		t.Fatal(err)
	}
	previousDB, previousLogger, previousSugar := models.DB, log.Logger, log.Sugar
	models.DB = db
	log.Logger = zap.NewNop()
	log.Sugar = log.Logger.Sugar()
	id_generator.SetupIDGenerator()
	t.Cleanup(func() {
		models.DB, log.Logger, log.Sugar = previousDB, previousLogger, previousSugar
		if conn, err := db.DB(); err == nil {
			_ = conn.Close()
		}
		if err := admin.Exec("DROP SCHEMA " + name + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		if conn, err := admin.DB(); err == nil {
			_ = conn.Close()
		}
	})
	entities := []any{&models.CrmFollowupComment{}, &models.CrmSmartCard{}, &models.CrmSmartCardView{}, &models.CrmLandingPage{}, &models.Tenant{}, &models.PlatformAdmin{}, &models.CrmRecycle{}, &models.CrmBatchOp{}, &models.CrmOrderProduct{}, &models.CrmOrderAttachment{}, &models.CrmOrderProgress{}, &models.User{}, &models.CrmRole{}, &models.CrmCustomer{}, &models.ChannelLink{}, &models.CrmActivity{}, &models.CrmActivityRecord{}, &models.CrmCoupon{}, &models.CrmCouponRecord{}, &models.CrmPoolRule{}, &models.CrmSopRule{}, &models.CrmPipelineStage{}, &models.CrmSalesRule{}}
	for _, rel := range customerMergeRelations {
		entities = append(entities, rel.Model)
	}
	if err := db.AutoMigrate(entities...); err != nil {
		t.Fatal(err)
	}
	seed := []any{
		&models.CrmRole{Model: models.Model{ID: "1"}, TenantID: "1", Code: "sales"},
		&models.CrmRole{Model: models.Model{ID: "2"}, TenantID: "1", Code: "admin"},
		&models.User{Model: models.Model{ID: "10"}, TenantID: "1", Username: "sales-a", RoleID: "1", Status: 1},
		&models.User{Model: models.Model{ID: "11"}, TenantID: "1", Username: "sales-b", RoleID: "1", Status: 1},
		&models.User{Model: models.Model{ID: "12"}, TenantID: "1", Username: "admin", RoleID: "2", Status: 1},
		&models.User{Model: models.Model{ID: "20"}, TenantID: "2", Username: "foreign", Status: 1},
		&models.CrmCustomer{Model: models.Model{ID: "100"}, TenantID: "1", OwnerUserID: "10", Name: "Own"},
		&models.CrmCustomer{Model: models.Model{ID: "101"}, TenantID: "1", OwnerUserID: "11", Name: "Other"},
		&models.CrmCustomer{Model: models.Model{ID: "102"}, TenantID: "2", OwnerUserID: "20", Name: "Foreign"},
	}
	for _, item := range seed {
		mustCreate(t, db, item)
	}
	return db
}

func testIdentity(tenant, user string) context.Context {
	return identity.WithIdentity(context.Background(), identity.Identity{TenantID: tenant, UserID: user})
}

func mustCreate(t *testing.T, db *gorm.DB, item any) {
	t.Helper()
	if err := createCRM(db, item); err != nil {
		t.Fatal(err)
	}
}

func TestCustomerAccessRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	svc := NewCrmCustomer()
	if _, err := svc.Get(ctx, "100"); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"101", "102"} {
		if _, err := svc.Get(ctx, id); err == nil {
			t.Fatalf("read unauthorized customer %s", id)
		}
		if _, err := svc.Update(ctx, id, models.CrmCustomer{Name: "Changed", OwnerUserID: "10"}); err == nil {
			t.Fatalf("updated unauthorized customer %s", id)
		}
	}
	if _, err := svc.Delete(ctx, []string{"100", "101"}); err == nil {
		t.Fatal("partial unauthorized batch accepted")
	}
	if _, err := svc.Get(ctx, "100"); err != nil {
		t.Fatalf("authorized customer removed on failed batch: %v", err)
	}
	if _, err := svc.Update(testIdentity("1", "12"), "100", models.CrmCustomer{OwnerUserID: "20"}); err == nil {
		t.Fatal("cross-tenant owner accepted")
	}
	mustCreate(t, db, &models.CrmCustomer{Model: models.Model{ID: "103"}, TenantID: "1", OwnerUserID: "11", SharedStaffIDs: constants.StringArrayField{"10"}})

	if err := NewCrmCustomerExt().SetKey(ctx, []string{"101"}, true); err == nil {
		t.Fatal("set key bypassed ownership")
	}
	if err := NewCrmCustomerExt().Share(ctx, "101", []string{"10"}); err == nil {
		t.Fatal("share bypassed ownership")
	}
	if err := NewCrmCustomerExt().Share(ctx, "100", []string{"20"}); err == nil {
		t.Fatal("cross-tenant sharing accepted")
	}
	if err := NewCrmCustomerExt().SetKey(ctx, []string{"100"}, true); err != nil {
		t.Fatal(err)
	}
	if _, total, err := svc.Query(ctx, "", "__me__", "", 0, -1, "", nil); err != nil || total != 1 {
		t.Fatalf("my customers: total=%d err=%v", total, err)
	}
	if _, err := svc.Get(ctx, "103"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Delete(ctx, []string{"103"}); err == nil {
		t.Fatal("shared read access granted write access")
	}
	_, total, err := svc.Query(ctx, "", "", "", 0, -1, "", nil)
	if err != nil || total != 2 {
		t.Fatalf("scope total=%d err=%v", total, err)
	}
	for _, tenant := range []string{"1", "2"} {
		customer := models.CrmCustomer{Model: models.Model{ID: "104" + tenant}, TenantID: tenant}
		if err := createCRM(db, &customer, "OwnerUserID"); err != nil {
			t.Fatal(err)
		}
	}
	if _, total, err := svc.Query(testIdentity("1", "12"), "", "", "", 0, -1, "unreceive", nil); err != nil || total != 1 {
		t.Fatalf("unreceived customers: total=%d err=%v", total, err)
	}
	if _, total, err := svc.Query(ctx, "", "", "", 0, -1, "unreceive", nil); err != nil || total != 0 {
		t.Fatalf("sales unreceived scope: total=%d err=%v", total, err)
	}
	if _, err := svc.Get(testIdentity("1", ""), "100"); err == nil {
		t.Fatal("accepted missing user")
	}
	if err := db.Model(&models.User{}).Where("id = ?", "10").Update("status", 2).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Get(ctx, "100"); err == nil {
		t.Fatal("disabled user retained access")
	}
}

func TestCustomerMergeRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "12")
	svc := NewCrmDedup()
	if err := svc.Merge(ctx, "102", []string{"101"}); err == nil {
		t.Fatal("cross-tenant primary accepted")
	}
	if err := svc.Merge(ctx, "100", []string{"999"}); err == nil {
		t.Fatal("missing duplicate accepted")
	}
	for _, item := range []any{
		&models.ChannelLink{Model: models.Model{ID: "300"}, TenantID: "1", EntityType: "customer", EntityID: "100", Channel: "wecom", ExtID: "a", ExtCorpID: "corp"},
		&models.ChannelLink{Model: models.Model{ID: "301"}, TenantID: "1", EntityType: "customer", EntityID: "101", Channel: "wecom", ExtID: "b", ExtCorpID: "corp"},
		&models.CrmFollowup{Model: models.Model{ID: "400"}, TenantID: "1", CustomerID: "101"},
		&models.CrmActivity{Model: models.Model{ID: "500"}, TenantID: "1"},
		&models.CrmActivityRecord{Model: models.Model{ID: "501"}, TenantID: "1", ActivityID: "500", CustomerID: "100", RecordType: "reached"},
		&models.CrmActivityRecord{Model: models.Model{ID: "502"}, TenantID: "1", ActivityID: "500", CustomerID: "101", RecordType: "converted"},
	} {
		mustCreate(t, db, item)
	}
	// Simulate a database failure after related updates begin, and verify rollback.
	if err := db.Exec("ALTER TABLE crm_followup ADD CONSTRAINT reject_primary CHECK (customer_id <> 100)").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Merge(ctx, "100", []string{"101"}); err == nil {
		t.Fatal("constraint error swallowed")
	}
	var original models.CrmFollowup
	if err := db.First(&original, "id = ?", "400").Error; err != nil || original.CustomerID != "101" {
		t.Fatalf("partial merge persisted: %+v %v", original, err)
	}
	if err := db.Exec("ALTER TABLE crm_followup DROP CONSTRAINT reject_primary").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Merge(ctx, "100", []string{"101"}); err != nil {
		t.Fatal(err)
	}
	var links []models.ChannelLink
	if err := db.Where("entity_id = ?", "100").Find(&links).Error; err != nil || len(links) != 2 {
		t.Fatalf("lost channel identities: %d %v", len(links), err)
	}
	var records []models.CrmActivityRecord
	if err := db.Find(&records).Error; err != nil || len(records) != 1 || records[0].RecordType != "converted" {
		t.Fatalf("activity merge: %+v %v", records, err)
	}
	var activity models.CrmActivity
	if err := db.First(&activity, "id = ?", "500").Error; err != nil {
		t.Fatal(err)
	}
	if activity.ReachCount != 1 || activity.ConversionCount != 1 {
		t.Fatalf("stale counters: %+v", activity)
	}
	var count int64
	db.Model(&models.CrmCustomer{}).Where("id = ?", "101").Count(&count)
	if count != 0 {
		t.Fatal("duplicate customer still active")
	}
}

func TestCouponConcurrencyRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "12")
	mustCreate(t, db, &models.CrmCoupon{Model: models.Model{ID: "600"}, TenantID: "1", Status: "active", TotalCount: 5})
	svc := NewCrmCoupon()
	var issued atomic.Int64
	parallel := func(fn func()) {
		var wg sync.WaitGroup
		for i := 0; i < 24; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); fn() }()
		}
		wg.Wait()
	}
	parallel(func() {
		if _, err := svc.Issue(ctx, "600", "100"); err == nil {
			issued.Add(1)
		}
	})
	if issued.Load() != 5 {
		t.Fatalf("issued=%d, want 5", issued.Load())
	}
	var records []models.CrmCouponRecord
	if err := db.Order("id").Find(&records).Error; err != nil || len(records) != 5 {
		t.Fatalf("records=%d err=%v", len(records), err)
	}
	var verified atomic.Int64
	parallel(func() {
		if _, err := svc.Verify(ctx, records[0].Code, ""); err == nil {
			verified.Add(1)
		}
	})
	if verified.Load() != 1 {
		t.Fatalf("verified %d times", verified.Load())
	}
	mustCreate(t, db, &models.CrmOrder{Model: models.Model{ID: "700"}, TenantID: "1", CustomerID: "100", OrderNo: "test-order"})
	var retries atomic.Int64
	parallel(func() {
		if _, err := svc.Verify(ctx, records[1].Code, "700"); err == nil {
			retries.Add(1)
		}
	})
	if retries.Load() != 24 {
		t.Fatalf("idempotent requests succeeded %d/24", retries.Load())
	}
	var coupon models.CrmCoupon
	if err := db.First(&coupon, "id = ?", "600").Error; err != nil {
		t.Fatal(err)
	}
	if coupon.IssuedCount != 5 || coupon.UsedCount != 2 {
		t.Fatalf("bad counters: %+v", coupon)
	}
	if _, err := svc.Verify(testIdentity("2", "20"), records[2].Code, ""); err == nil {
		t.Fatal("cross-tenant redemption accepted")
	}
}

func TestSopExecutionRegression(t *testing.T) {
	db := regressionDB(t)
	now := time.Now()
	for _, task := range []models.CrmSopTask{
		{Model: models.Model{ID: "800"}, TenantID: "1", CustomerID: "100", AssignUserID: "10", ActionType: "create_followup", Status: "pending", ExecuteAt: &now},
		{Model: models.Model{ID: "801"}, TenantID: "1", CustomerID: "100", ActionType: "send_message", Status: "pending", ExecuteAt: &now},
	} {
		copy := task
		mustCreate(t, db, &copy)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := NewCrmSop().executeTask(models.CrmSopTask{Model: models.Model{ID: "800"}}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	NewCrmSop().ExecutePendingTasks()
	var count int64
	db.Model(&models.CrmFollowup{}).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate followups: %d", count)
	}
	var manual models.CrmSopTask
	if err := db.First(&manual, "id = ?", "801").Error; err != nil {
		t.Fatal(err)
	}
	if manual.Status != "pending" || manual.DoneAt != nil {
		t.Fatal("unsent message marked done")
	}
}

func TestForceLoginRejectsProduction(t *testing.T) {
	before := conf.Settings.App.Env
	defer func() { conf.Settings.App.Env = before }()
	for _, env := range []string{"PROD", "DEMO", ""} {
		conf.Settings.App.Env = env
		if _, err := NewDefaultLogin().ForceConsoleLogin("", ""); err == nil {
			t.Fatalf("force login accepted in %q", env)
		}
	}
}
