package services

import (
	"core/app/constants"
	"core/app/models"
	"testing"
	"time"
)

func TestCustomerHistoryAccessBoundaries(t *testing.T) {
	db := regressionDB(t)
	ctx, other := testIdentity("1", "10"), testIdentity("1", "11")
	svc := NewCrmChannelInteraction()
	for _, r := range []models.CrmChannelInteraction{
		{Model: models.Model{ID: "800"}, TenantID: "1", CustomerID: "100", Content: "own"},
		{Model: models.Model{ID: "801"}, TenantID: "1", CustomerID: "101", Content: "other"},
		{Model: models.Model{ID: "802"}, TenantID: "2", CustomerID: "102", Content: "foreign"},
	} {
		mustCreate(t, db, &r)
	}
	mustCreate(t, db, &models.CrmTransferLog{Model: models.Model{ID: "810"}, TenantID: "1", CustomerID: "100", Reason: "private handover"})
	mustCreate(t, db, &models.CrmActivity{Model: models.Model{ID: "820"}, TenantID: "2", Name: "foreign campaign"})
	mustCreate(t, db, &models.CrmActivityRecord{Model: models.Model{ID: "821"}, TenantID: "1", CustomerID: "100", ActivityID: "820"})
	mustCreate(t, db, &models.CrmActivityRecord{Model: models.Model{ID: "822"}, TenantID: "1", CustomerID: "100"})
	requireNoError(t, db.Delete(&models.CrmActivityRecord{}, "id = 822").Error)
	rows, n, err := svc.Query(ctx, "", "", nil)
	requireNoError(t, err)
	if n != 1 || len(rows) != 1 || rows[0].ID != "800" {
		t.Fatal("unfiltered interaction list leaked another customer")
	}
	if _, _, err = svc.Query(ctx, "101", "", nil); err == nil {
		t.Fatal("unauthorized filter accepted")
	}
	for _, ids := range [][]string{{"800", "801"}, {"800", "802"}} {
		if n, err = svc.Delete(ctx, ids); err == nil || n != 0 {
			t.Fatal("mixed unauthorized deletion accepted")
		}
	}
	requireNoError(t, db.Model(&models.CrmChannelInteraction{}).Count(&n).Error)
	if n != 3 {
		t.Fatal("partial delete committed")
	}
	if _, err = svc.Create(ctx, "10", models.CrmChannelInteraction{CustomerID: "101"}); err == nil {
		t.Fatal("created other owner's interaction")
	}
	if _, _, err = NewCrmTransfer().ListTransferLog(other, "100", nil); err == nil {
		t.Fatal("handover history leaked")
	}
	if _, err = NewCrmActivity().ListByCustomer(other, "100"); err == nil {
		t.Fatal("activity history leaked")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("shared_staff_ids", constants.StringArrayField{"11"}).Error)
	if _, _, err = NewCrmTransfer().ListTransferLog(other, "100", nil); err != nil {
		t.Fatal(err)
	}
	activities, err := NewCrmActivity().ListByCustomer(other, "100")
	requireNoError(t, err)
	if len(activities) != 1 || activities[0].ActivityName != "" {
		t.Fatal("deleted record or foreign activity metadata leaked")
	}
	if _, err = svc.Create(other, "11", models.CrmChannelInteraction{CustomerID: "100"}); err == nil {
		t.Fatal("shared reader wrote interaction")
	}
	if _, err = svc.Delete(other, []string{"800"}); err == nil {
		t.Fatal("shared reader deleted interaction")
	}
	created, err := svc.Create(ctx, "10", models.CrmChannelInteraction{CustomerID: "100", TenantID: "2", Content: "manual"})
	requireNoError(t, err)
	if created.TenantID != "1" {
		t.Fatal("trusted submitted tenant")
	}
	_, err = svc.Delete(ctx, []string{created.ID, created.ID})
	requireNoError(t, err)
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Updates(map[string]any{"owner_user_id": "11", "shared_staff_ids": constants.StringArrayField{}}).Error)
	if _, _, err = svc.Query(ctx, "100", "", nil); err == nil {
		t.Fatal("former owner retained interaction access")
	}
	if _, _, err = NewCrmTransfer().ListTransferLog(ctx, "100", nil); err == nil {
		t.Fatal("former owner retained transfer history")
	}
	if _, err = NewCrmActivity().ListByCustomer(ctx, "100"); err == nil {
		t.Fatal("former owner retained campaign history")
	}
}

func TestCustomerArchiveIdentityIsolation(t *testing.T) {
	db := regressionDB(t)
	requireNoError(t, db.AutoMigrate(&models.ChatMsg{}))
	ctx := testIdentity("1", "10")
	for _, link := range []models.ChannelLink{
		{Model: models.Model{ID: "900"}, TenantID: "2", Channel: "wecom", EntityType: "tenant", EntityID: "2", ExtCorpID: "foreigncorp", ExtID: "foreigncorp"},
		{Model: models.Model{ID: "901"}, TenantID: "1", Channel: "wecom", EntityType: "customer", EntityID: "100", ExtCorpID: "foreigncorp", ExtID: "external"},
		{Model: models.Model{ID: "902"}, TenantID: "1", Channel: "wecom", EntityType: "tenant", EntityID: "1", ExtCorpID: "owncorp", ExtID: "owncorp"},
		{Model: models.Model{ID: "903"}, TenantID: "1", Channel: "wecom", EntityType: "customer", EntityID: "100", ExtCorpID: "owncorp", ExtID: "ownexternal"},
	} {
		mustCreate(t, db, &link)
	}
	for i, corp := range []string{"foreigncorp", "owncorp"} {
		external := "external"
		id := "910"
		if i == 1 {
			external = "ownexternal"
			id = "911"
		}
		mustCreate(t, db, &models.ChatMsg{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: corp}, MsgID: id, From: external, RoomID: "", MsgTime: time.Now().UnixMilli(), ContentText: "private archive"})
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Updates(map[string]any{"ext_corp_id": "foreigncorp", "ext_customer_id": "external"}).Error)
	detail, err := NewCrmCustomer().GetDetail(ctx, "100")
	requireNoError(t, err)
	if len(detail["wecom_messages"].([]models.ChatMsg)) != 0 {
		t.Fatal("forged customer snapshot leaked foreign enterprise archive")
	}
	if _, err = NewCrmCustomer().Create(ctx, "10", models.CrmCustomer{Name: "forged", ExtCorpID: "owncorp", ExtCustomerID: "other-customer"}); err == nil {
		t.Fatal("manual create established external identity")
	}
	if _, err = NewCrmCustomer().Update(ctx, "100", models.CrmCustomer{Name: "forged", OwnerUserID: "10", ExtCorpID: "owncorp", ExtCustomerID: "other-customer"}); err == nil {
		t.Fatal("manual edit rebound external identity")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Updates(map[string]any{"ext_corp_id": "owncorp", "ext_customer_id": "ownexternal"}).Error)
	detail, err = NewCrmCustomer().GetDetail(ctx, "100")
	requireNoError(t, err)
	if len(detail["wecom_messages"].([]models.ChatMsg)) != 1 {
		t.Fatal("valid mapped archive missing")
	}
	requireNoError(t, db.Delete(&models.ChannelLink{}, "id = 903").Error)
	detail, err = NewCrmCustomer().GetDetail(ctx, "100")
	requireNoError(t, err)
	if len(detail["wecom_messages"].([]models.ChatMsg)) != 0 {
		t.Fatal("deleted identity mapping still authorized archive")
	}
}

func TestAllocationConfigurationGuards(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "allocation", Status: 1})
	requireNoError(t, db.AutoMigrate(&models.CrmAutoAllocation{}))
	svc := NewCrmAutoAllocation()
	admin := testIdentity("1", "12")
	sales := testIdentity("1", "10")
	rule := models.CrmAutoAllocation{Name: "Draft", EntityType: "customer", Strategy: "round_robin", TargetIDs: constants.StringArrayField{"11", "10", "11"}, IsEnabled: false, Conditions: constants.StringArrayField{"legacy-condition"}}
	if err := svc.Create(sales, &rule); err == nil {
		t.Fatal("sales created allocation rule")
	}
	requireNoError(t, svc.Create(admin, &rule))
	var saved models.CrmAutoAllocation
	requireNoError(t, db.First(&saved, "id = ?", rule.ID).Error)
	if saved.IsEnabled || len(saved.TargetIDs) != 2 || saved.TargetIDs[0] != "11" {
		t.Fatal("disabled state or target order lost")
	}
	update := models.CrmAutoAllocation{Name: "Updated", EntityType: "lead", Strategy: "random", TargetIDs: constants.StringArrayField{"10"}}
	requireNoError(t, svc.Update(admin, rule.ID, &update))
	requireNoError(t, db.First(&saved, "id = ?", rule.ID).Error)
	if saved.Name != "Updated" || len(saved.Conditions) != 1 {
		t.Fatal("update failed or erased omitted conditions")
	}
	if err := svc.Update(admin, "99999", &update); err == nil {
		t.Fatal("missing rule reported success")
	}
	update.TargetIDs = constants.StringArrayField{"20"}
	if err := svc.Update(admin, rule.ID, &update); err == nil {
		t.Fatal("foreign target accepted")
	}
	if err := svc.ToggleStatus(sales, rule.ID, true); err == nil {
		t.Fatal("sales toggled rule")
	}
	requireNoError(t, db.Model(&models.User{}).Where("id = 10").Update("status", 2).Error)
	if err := svc.ToggleStatus(admin, rule.ID, true); err == nil {
		t.Fatal("enabled rule with disabled recipient")
	}
	requireNoError(t, svc.ToggleStatus(admin, rule.ID, false))
	if _, err := svc.Delete(admin, []string{rule.ID, "99999"}); err == nil {
		t.Fatal("partial invalid rule deletion accepted")
	}
	rows, n, err := svc.Query(admin, AutoAllocationQueryReq{})
	requireNoError(t, err)
	if len(rows) != 1 || n != 1 {
		t.Fatal("rule lost after failed delete")
	}
	if _, _, err = svc.Query(sales, AutoAllocationQueryReq{}); err == nil {
		t.Fatal("invalid sales session queried settings")
	}
}
