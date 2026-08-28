package services

import (
	"core/app/constants"
	"core/app/models"
	"testing"
)

func TestGenericActionsCannotBypassEntityRights(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	requireNoError(t, db.AutoMigrate(&models.CrmFieldDef{}, &models.CrmFieldValue{}))
	perms := constants.StringArrayField{"crm:batch:write", "crm:field_value:read", "crm:field_value:write", "crm:customer:read", "crm:data:all", "crm:lead:write"}
	mustCreate(t, db, &models.CrmRole{Model: models.Model{ID: "900"}, TenantID: "1", Code: "reader", Perms: perms})
	requireNoError(t, db.Model(&models.User{}).Where("id=11").Update("role_id", "900").Error)
	ctx := testIdentity("1", "11")
	if _, err := NewCrmBatch().Execute(ctx, "11", BatchExecReq{EntityType: "customer", EntityIDs: []string{"100"}, OpType: "lock"}); err == nil {
		t.Fatal("batch endpoint bypassed customer write permission")
	}
	fields := NewCrmField()
	if err := fields.SetValue(ctx, "customer", "100", "private_note", "changed"); err == nil {
		t.Fatal("custom-field endpoint bypassed parent write permission")
	}
	if err := ValidateBusinessPermission(ctx, "crm_lead", "ConvertToCustomer"); err == nil {
		t.Fatal("conversion bypassed destination create permission")
	}
	perms = constants.StringArrayField{"crm:field_value:read", "crm:field_value:write", "crm:customer:read", "crm:customer:write"}
	requireNoError(t, db.Model(&models.CrmRole{}).Where("id=900").Update("perms", perms).Error)
	if err := fields.SetValue(ctx, "customer", "100", "private_note", "changed"); err == nil {
		t.Fatal("custom-field endpoint bypassed parent ownership")
	}
	requireNoError(t, fields.SetValue(ctx, "customer", "101", "private_note", "own"))
	values, err := fields.GetValues(ctx, "customer", "101")
	requireNoError(t, err)
	if len(values) != 1 || values[0].Value != "own" {
		t.Fatal("authorized field write failed", values)
	}
	if _, err := fields.GetValues(testIdentity("1", "10"), "customer", "101"); err == nil {
		t.Fatal("custom-field read bypassed parent ownership")
	}
	// The manager's recycle endpoint cannot restore a module removed from the plan.
	mustCreate(t, db, &models.CrmLead{Model: models.Model{ID: "999"}, TenantID: "1", OwnerUserID: "10", Title: "hidden lead"})
	requireNoError(t, db.Where("id=999").Delete(&models.CrmLead{}).Error)
	mustCreate(t, db, &models.CrmRecycle{Model: models.Model{ID: "998"}, TenantID: "1", EntityType: "lead", EntityID: "999", EntityName: "hidden lead"})
	requireNoError(t, db.Model(&models.Plan{}).Where("code='free'").Update("modules", `["customer"]`).Error)
	admin := testIdentity("1", "12")
	if err := NewCrmRecycle().Restore(admin, "998"); err == nil {
		t.Fatal("recycle endpoint restored unpaid module")
	}
	rows, total, err := NewCrmRecycle().List(admin, "", RecycleQueryReq{})
	requireNoError(t, err)
	if total != 0 || len(rows) != 0 {
		t.Fatal("recycle list leaked unpaid module", rows)
	}
	if _, err := NewCrmBatch().Execute(admin, "12", BatchExecReq{EntityType: "lead", EntityIDs: []string{"999"}, OpType: "seas"}); err == nil {
		t.Fatal("batch endpoint bypassed module entitlement")
	}
}
