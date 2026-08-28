package migrate

import (
	"core/app/models"
	"testing"
)

func TestAccountAndEntitlementMigrations(t *testing.T) {
	db := legacyTestDB(t)
	if VerifyBackendPolicies(db) == nil {
		t.Fatal("server accepted missing policy migrations")
	}
	if err := db.AutoMigrate(&models.Tenant{}, &models.Plan{}, &models.CrmRole{}, &models.User{}, &models.CrmCustomer{}, &models.PlatformAdmin{}); err != nil {
		t.Fatal(err)
	}
	for _, stmt := range []string{
		`INSERT INTO plan(id,code,name,max_users,max_customers,modules) VALUES(1,'free','Free',2,2,'["customer"]')`,
		`INSERT INTO tenant(id,code,plan,status) VALUES(1,'tenant','free',1)`,
		`INSERT INTO crm_role(id,tenant_id,code,perms) VALUES(1,1,'admin','[]')`,
		`INSERT INTO platform_admin(id,username,role,perms) VALUES(1,'root','super_admin','[]')`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range []string{"0031_plan_entitlement_quotas.sql", "0032_crm_permission_points.sql", "0033_account_session_versions.sql", "0034_platform_permission_points.sql"} {
		if err := applyFile(db, file); err != nil {
			t.Fatalf("migration %s: %v", file, err)
		}
		if err := applyFile(db, file); err != nil {
			t.Fatalf("migration %s not idempotent: %v", file, err)
		}
	}
	if err := VerifyBackendPolicies(db); err != nil {
		t.Fatal(err)
	}
	var platform models.PlatformAdmin
	if err := db.First(&platform, "id=1").Error; err != nil {
		t.Fatal(err)
	}
	if len(platform.Perms) != 1 || platform.Perms[0] != "platform:*:full" {
		t.Fatal("platform permissions not migrated", platform.Perms)
	}
	var role models.CrmRole
	if err := db.First(&role, "id=1").Error; err != nil {
		t.Fatal(err)
	}
	found := false
	for _, point := range role.Perms {
		if point == "crm:*:full" {
			found = true
		}
	}
	if !found {
		t.Fatal("built-in role capabilities not migrated", role.Perms)
	}
	for _, stmt := range []string{`INSERT INTO "user"(id,tenant_id,username,status) VALUES(1,1,'first',1)`, `INSERT INTO "user"(id,tenant_id,username,status) VALUES(2,1,'second',1)`} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if db.Exec(`INSERT INTO "user"(id,tenant_id,username,status) VALUES(3,1,'overflow',1)`).Error == nil {
		t.Fatal("quota migration inactive")
	}
	if err := db.Exec(`UPDATE "user" SET password='changed' WHERE id=1`).Error; err != nil {
		t.Fatal(err)
	}
	var version int64
	if err := db.Raw(`SELECT auth_version FROM "user" WHERE id=1`).Scan(&version).Error; err != nil || version != 2 {
		t.Fatal("session invalidation migration inactive", version, err)
	}
}
