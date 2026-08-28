package services

import (
	"core/app/constants"
	"core/app/models"
	"errors"
	"testing"
)

func TestAllocationRequiresTargetRights(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "allocation", Status: 1})
	admin := testIdentity("1", "12")
	svc := NewCrmAutoAllocation()
	rule := models.CrmAutoAllocation{Name: "lead", EntityType: "lead", Strategy: "round_robin", TargetIDs: constants.StringArrayField{"10"}}
	requireNoError(t, svc.Create(admin, &rule))
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("modules", `["customer"]`).Error)
	if err := svc.Create(admin, &models.CrmAutoAllocation{Name: "unpaid", EntityType: "lead", Strategy: "round_robin", TargetIDs: constants.StringArrayField{"10"}}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("created unpaid lead rule", err)
	}
	if err := svc.ToggleStatus(admin, rule.ID, true); !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("enabled unpaid lead rule", err)
	}
	if err := svc.Update(admin, rule.ID, &models.CrmAutoAllocation{Name: "rebind", EntityType: "customer", Strategy: "round_robin", TargetIDs: constants.StringArrayField{"10"}}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("rebound inaccessible rule", err)
	}
	rows, n, err := svc.Query(admin, AutoAllocationQueryReq{})
	requireNoError(t, err)
	if n != 0 || len(rows) != 0 {
		t.Fatal("list exposed unpaid lead rule", rows)
	}
	if _, _, err := svc.Query(admin, AutoAllocationQueryReq{EntityType: "lead"}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("explicit query bypassed unpaid target", err)
	}
	if _, err := svc.Delete(admin, []string{rule.ID}); !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("deleted unpaid lead rule", err)
	}
	// Configuration management and data visibility do not grant lead assignment.
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("modules", `["customer","lead"]`).Error)
	perms := constants.StringArrayField{"crm:auto_allocation:read", "crm:auto_allocation:toggle_status", "crm:data:all", "crm:lead:read"}
	mustCreate(t, db, &models.CrmRole{Model: models.Model{ID: "910"}, TenantID: "1", Code: "allocation_reader", Perms: perms})
	requireNoError(t, db.Model(&models.User{}).Where("id=11").Update("role_id", "910").Error)
	actor := testIdentity("1", "11")
	requireNoError(t, ValidateBusinessPermission(actor, "crm_auto_allocation", "Logs"))
	if _, err := svc.Logs(actor, "", "", 20); !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("allocation log exposed customers without read permission", err)
	}
	if err := svc.ToggleStatus(actor, rule.ID, true); !errors.Is(err, ErrPermissionDenied) {
		t.Fatal("configuration grant bypassed lead assignment permission", err)
	}
	rows, n, err = svc.Query(actor, AutoAllocationQueryReq{})
	requireNoError(t, err)
	if n != 1 || len(rows) != 1 {
		t.Fatal("authorized read denied", rows)
	}
	perms = append(perms, "crm:lead:assign", "crm:customer:read")
	requireNoError(t, db.Model(&models.CrmRole{}).Where("id=910").Update("perms", perms).Error)
	requireNoError(t, svc.ToggleStatus(actor, rule.ID, true))
	_, err = svc.Logs(actor, "", "", 20)
	requireNoError(t, err)
}

func TestAutoRecycleStopsAfterEntitlementRevocation(t *testing.T) {
	db := regressionDB(t)
	seedRecycleBatch(t, db, 450)
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("modules", `[]`).Error)
	worker := testIdentity("1", "")
	if n, err := NewCrmPool().AutoRecycle(worker); n != 0 || !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("unpaid worker changed records: %d/%v", n, err)
	}
	NewCrmPool().AutoRecycleAll()
	var n int64
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("pool_status=1").Count(&n).Error)
	if n != 0 {
		t.Fatal("scheduled worker bypassed unpaid module", n)
	}
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("modules", `["customer"]`).Error)
	// Revoke the module as the first batch commits. The next batch must not run.
	requireNoError(t, db.Exec(`CREATE FUNCTION revoke_customer_module() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN UPDATE plan SET modules='[]'::jsonb WHERE code='free'; RETURN NEW; END $$; CREATE TRIGGER revoke_customer_module AFTER INSERT ON crm_pool_log FOR EACH ROW EXECUTE FUNCTION revoke_customer_module()`).Error)
	committed, err := NewCrmPool().AutoRecycle(worker)
	if committed != 200 || !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("expected entitlement rejection after first batch: %d/%v", committed, err)
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("pool_status=1").Count(&n).Error)
	if n != 200 {
		t.Fatal("revoked module continued recycling", n)
	}
	requireNoError(t, db.Model(&models.CrmPoolLog{}).Count(&n).Error)
	if n != 200 {
		t.Fatal("recycle data/audit mismatch", n)
	}
}
