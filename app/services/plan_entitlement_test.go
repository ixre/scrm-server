package services

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"core/app/models"
	"core/migrations"
)

func TestPlanEntitlementsUseCurrentTenantPlan(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Plan: "free", Status: 1})
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "2"}, Code: "b", Plan: "missing", Status: 1})
	ctx := testIdentity("1", "12")
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("modules", `["customer","workbench","channel_wecom"]`).Error)
	requireNoError(t, ValidateBusinessEntitlement(ctx, "crm_customer", "Create"))
	for _, service := range []string{"crm_order", "crm_coupon", "msg_arch", "crm_user", "unregistered_feature"} {
		if ValidateBusinessEntitlement(ctx, service, "Create") == nil {
			t.Fatalf("unpaid feature allowed: %s", service)
		}
	}
	if ValidateBusinessEntitlement(testIdentity("2", "20"), "crm_customer", "Query") == nil {
		t.Fatal("missing plan failed open")
	}
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("modules", `["customer","order","msg_archive","channel_wecom"]`).Error)
	requireNoError(t, ValidateBusinessEntitlement(ctx, "crm_order", "Create"))
	requireNoError(t, ValidateBusinessEntitlement(ctx, "msg_arch", "QueryMsgs"))
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("status", 2).Error)
	requireNoError(t, ValidateBusinessEntitlement(ctx, "crm_order", "Query")) // delisting preserves paid usage
	requireNoError(t, db.Model(&models.Tenant{}).Where("id=1").Update("expire_at", time.Now().Add(-time.Hour)).Error)
	if ValidateBusinessEntitlement(ctx, "crm_customer", "Query") == nil {
		t.Fatal("expired tenant allowed")
	}
	for _, tt := range [][2]string{{"crm_user", "GetCurrent"}, {"crm_tenant", "GetCurrentInfo"}, {"crm_tenant_order", "Create"}, {"crm_auth", "ChangePassword"}} {
		requireNoError(t, ValidateBusinessEntitlement(ctx, tt[0], tt[1]))
	}
}

func TestPlanQuotaAllWritesAndConcurrency(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Plan: "free", Status: 1, MaxUsers: 100, MaxCustomers: 100})
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "2"}, Code: "b", Plan: "free", Status: 1})
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Updates(map[string]any{"max_users": 4, "max_customers": 3}).Error)
	sql, err := migrations.Files.ReadFile("0031_plan_entitlement_quotas.sql")
	requireNoError(t, err)
	for _, stmt := range strings.Split(string(sql), "-- migrate:statement") {
		requireNoError(t, db.Exec(stmt).Error)
	}
	for _, table := range []string{"crm_customer", `"user"`} {
		var successes atomic.Int32
		var wg sync.WaitGroup
		for n := 0; n < 10; n++ {
			wg.Add(1)
			go func(n int) {
				defer wg.Done()
				query := fmt.Sprintf("INSERT INTO %s(id,tenant_id%s) VALUES(?,1%s)", table, "", "")
				if table == `"user"` {
					query = `INSERT INTO "user"(id,tenant_id,username,status) VALUES(?,1,?,1)`
					if db.Exec(query, 200+n, fmt.Sprintf("seat-%d", n)).Error == nil {
						successes.Add(1)
					}
					return
				}
				if err := db.Exec(query, 200+n).Error; err == nil {
					successes.Add(1)
				}
			}(n)
		}
		wg.Wait()
		if successes.Load() != 1 {
			t.Fatalf("%s exceeded quota under concurrency: %d", table, successes.Load())
		}
	}
	// Updates/upserts of existing rows continue at the cap.
	requireNoError(t, db.Exec("INSERT INTO crm_customer(id,tenant_id,name) VALUES(100,1,'same') ON CONFLICT(id) DO UPDATE SET name=excluded.name").Error)
	// A soft deletion frees a slot, but restoration consumes it again.
	requireNoError(t, db.Exec("UPDATE crm_customer SET deleted_at=CURRENT_TIMESTAMP WHERE id=101").Error)
	requireNoError(t, db.Exec("INSERT INTO crm_customer(id,tenant_id) VALUES(300,1)").Error)
	if db.Exec("UPDATE crm_customer SET deleted_at=NULL WHERE id=101").Error == nil {
		t.Fatal("restore bypassed customer quota")
	}
	requireNoError(t, db.Exec(`UPDATE "user" SET status=2 WHERE id=10`).Error)
	requireNoError(t, db.Exec(`INSERT INTO "user"(id,tenant_id,username,status) VALUES(300,1,'replacement',1)`).Error)
	if db.Exec(`UPDATE "user" SET status=1 WHERE id=10`).Error == nil {
		t.Fatal("reactivation bypassed seat quota")
	}
	// Other tenants have independent usage; a failed multi-row insert is atomic.
	if db.Exec("INSERT INTO crm_customer(id,tenant_id) VALUES(400,2),(401,2),(402,2)").Error == nil {
		t.Fatal("batch overflow allowed")
	}
	var count int64
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("tenant_id=2").Count(&count).Error)
	if count != 1 {
		t.Fatal("partial batch committed", count)
	}
	requireNoError(t, db.Exec("INSERT INTO crm_customer(id,tenant_id) VALUES(400,2)").Error)
	// Changes to the plan take effect without rewriting tenant copies.
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("max_customers", 4).Error)
	requireNoError(t, db.Exec("UPDATE crm_customer SET deleted_at=NULL WHERE id=101").Error)
}
