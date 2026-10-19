package services

import (
	"context"
	"errors"
	"strings"
	"testing"

	"core/app/constants"
	"core/app/dto"
	"core/app/models"
	"core/common/we_work"
	"core/migrations"
)

func TestWecomMultiCorpRouting(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "2"}, Code: "b", Status: 1})
	requireNoError(t, db.AutoMigrate(&models.CrmChannelConfig{}))
	for _, cfg := range []models.CrmChannelConfig{
		{Model: models.Model{ID: "800"}, TenantID: "1", Channel: "wecom", CorpID: "corp-a", MainAgentID: 10, Enabled: 1},
		{Model: models.Model{ID: "801"}, TenantID: "2", Channel: "wecom", CorpID: "corp-b", MainAgentID: 20, Enabled: 1},
	} {
		mustCreate(t, db, &cfg)
	}
	for _, tt := range []struct {
		tenant, corp string
		agent        int64
	}{{"1", "corp-a", 10}, {"2", "corp-b", 20}} {
		cfg, err := ResolveWecomConfig(testIdentity(tt.tenant, "10"), "")
		requireNoError(t, err)
		if cfg.CorpID != tt.corp || cfg.MainAgentID != tt.agent {
			t.Fatalf("wrong enterprise: %+v", cfg)
		}
		client, err := we_work.Clients.Get(tt.corp)
		requireNoError(t, err)
		if client.MainApp.CorpID != tt.corp || client.MainApp.AgentID != tt.agent {
			t.Fatal("wrong enterprise credentials")
		}
	}
	if _, err := ResolveWecomConfig(testIdentity("1", "10"), "corp-b"); err == nil {
		t.Fatal("foreign enterprise accepted")
	}
	if _, err := ResolveWecomConfig(context.Background(), ""); err == nil {
		t.Fatal("fell back to global enterprise")
	}
	seen := map[string]bool{}
	err := EachWecomCorp(context.Background(), func(ctx context.Context, corpID string) error {
		seen[corpID] = true
		if we_work.CorpID(ctx) != corpID {
			t.Fatal("lost enterprise context")
		}
		return errors.New("simulated sync failure")
	})
	if err == nil || !seen["corp-a"] || !seen["corp-b"] {
		t.Fatalf("enterprise failure stopped other sync: %v %v", seen, err)
	}
	mustCreate(t, db, &models.CrmChannelConfig{Model: models.Model{ID: "802"}, TenantID: "1", Channel: "wecom", CorpID: "corp-c", Enabled: 1})
	if _, err := ResolveWecomConfig(testIdentity("1", "10"), ""); err == nil {
		t.Fatal("silently chose one of multiple enterprises")
	}
	_, err = ResolveWecomConfig(testIdentity("1", "10"), "corp-a")
	requireNoError(t, err)
	requireNoError(t, db.Model(&models.CrmChannelConfig{}).Where("id = '800'").Update("enabled", 2).Error)
	if _, err := ResolveWecomConfig(testIdentity("1", "10"), "corp-a"); err == nil {
		t.Fatal("disabled enterprise accepted")
	}
}

func TestWecomRestoreFailureIsEnterpriseLocal(t *testing.T) {
	db := regressionDB(t)
	requireNoError(t, db.AutoMigrate(&models.CrmChannelConfig{}, &models.ContactWayGroup{}, &models.CustomerInfoDisplayRule{}))
	mustCreate(t, db, &models.CrmChannelConfig{Model: models.Model{ID: "800"}, TenantID: "1", Channel: "wecom", CorpID: "corp-a", ContactSecret: "invalid-ciphertext", Enabled: 1})
	mustCreate(t, db, &models.CrmChannelConfig{Model: models.Model{ID: "801"}, TenantID: "2", Channel: "wecom", CorpID: "corp-b", MainAgentID: 20, Enabled: 1})
	requireNoError(t, RestoreChannelClients())
	if _, err := we_work.Clients.Get("corp-a"); err == nil {
		t.Fatal("invalid enterprise retained stale credentials")
	}
	client, err := we_work.Clients.Get("corp-b")
	requireNoError(t, err)
	if client.MainApp.AgentID != 20 {
		t.Fatal("other enterprise failed to restore")
	}
}

func TestWecomRolesStayWithinEnterprise(t *testing.T) {
	db := regressionDB(t)
	db.Config.DisableForeignKeyConstraintWhenMigrating = true
	requireNoError(t, db.AutoMigrate(&models.Role{}, &models.Permission{}, &models.Staff{}))
	for _, role := range []models.Role{
		{ExtCorpModel: models.ExtCorpModel{ID: "800", ExtCorpID: "corp-a"}, Name: "A", Type: "staff"},
		{ExtCorpModel: models.ExtCorpModel{ID: "801", ExtCorpID: "corp-b"}, Name: "B", Type: "staff"},
		{ExtCorpModel: models.ExtCorpModel{ID: "802"}, Name: "Shared default", Type: "admin", IsDefault: constants.True},
	} {
		mustCreate(t, db, &role)
	}
	for i, corp := range []string{"corp-a", "corp-b"} {
		mustCreate(t, db, &models.Staff{Model: models.Model{ID: []string{"810", "811"}[i]}, ExtCorpID: corp, ExtID: "same-staff", RoleID: []string{"800", "801"}[i]})
	}
	if _, err := NewRole().Get("800", "corp-b"); err == nil {
		t.Fatal("foreign role read")
	}
	if _, err := NewRole().Update("800", dto.UpdateRoleReq{}, "corp-b"); err == nil {
		t.Fatal("foreign role updated")
	}
	if _, err := NewRole().AssignToStaffs([]string{"same-staff"}, "800", "corp-b"); err == nil {
		t.Fatal("foreign role assigned")
	}
	count, err := NewRole().AssignToStaffs([]string{"same-staff"}, "802", "corp-b")
	requireNoError(t, err)
	if count != 1 {
		t.Fatal("shared default was not assigned within selected enterprise")
	}
	var a, b models.Staff
	requireNoError(t, db.Where("ext_corp_id='corp-a'").Take(&a).Error)
	requireNoError(t, db.Where("ext_corp_id='corp-b'").Take(&b).Error)
	if a.RoleID != "800" || b.RoleID != "802" {
		t.Fatal("role assignment crossed enterprises")
	}
}

func TestWecomCorpLocalIdentifiersAndMigration(t *testing.T) {
	db := regressionDB(t)
	db.Config.DisableForeignKeyConstraintWhenMigrating = true
	requireNoError(t, db.AutoMigrate(&models.Customer{}, &models.CustomerStaff{}, &models.CustomerStaffTag{}, &models.CustomerInfo{}, &models.Tag{}, &models.TagGroup{}, &models.GroupChat{}, &models.GroupChatMember{}, &models.CustomerStatistic{}, &models.CrmChannelConfig{}, &models.StaffDepartment{}))
	// A previously deployed global uniqueness rule must be replaced, not retained.
	requireNoError(t, db.Exec("CREATE UNIQUE INDEX old_customer_external ON customer(ext_id)").Error)
	content, err := migrations.Files.ReadFile("0027_wecom_multi_corp_indexes.sql")
	requireNoError(t, err)
	for _, stmt := range strings.Split(string(content), "-- migrate:statement") {
		if strings.TrimSpace(stmt) != "" {
			requireNoError(t, db.Exec(stmt).Error)
		}
	}
	for i, corpID := range []string{"corp-a", "corp-b"} {
		id := []string{"810", "820"}[i]
		customer := models.Customer{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: corpID}, ExtID: "same-customer", Name: corpID}
		requireNoError(t, customer.Upsert(customer))
		relation := models.CustomerStaff{ExtCorpModel: models.ExtCorpModel{ID: id + "1", ExtCorpID: corpID}, ExtStaffID: "same-staff", ExtCustomerID: "same-customer", Remark: corpID}
		requireNoError(t, relation.Upsert(relation))
		info := models.CustomerInfo{ExtCorpModel: models.ExtCorpModel{ID: id + "2", ExtCorpID: corpID}, ExtStaffID: "same-staff", ExtCustomerID: "same-customer", Description: corpID}
		requireNoError(t, info.Upsert(info))
		tag := models.Tag{ExtCorpModel: models.ExtCorpModel{ID: id + "3", ExtCorpID: corpID}, ExtID: "same-tag", Name: corpID}
		requireNoError(t, tag.Upsert(tag))
		chat := models.GroupChat{ExtCorpModel: models.ExtCorpModel{ID: id + "4", ExtCorpID: corpID}, ExtChatID: "same-chat", Name: corpID,
			MemberList: []models.GroupChatMember{{ExtCorpModel: models.ExtCorpModel{ID: id + "5", ExtCorpID: corpID}, Userid: "same-staff"}}}
		requireNoError(t, chat.Upsert(chat))
	}
	for _, corpID := range []string{"corp-a", "corp-b"} {
		c, err := (models.Customer{}).GetByExtID("same-customer", corpID, nil, true)
		requireNoError(t, err)
		if c.Name != corpID || len(c.Staffs) != 1 || c.Staffs[0].Remark != corpID {
			t.Fatalf("mixed enterprise relations: %+v", c)
		}
	}
	info := models.CustomerInfo{ExtCorpModel: models.ExtCorpModel{ExtCorpID: "corp-b"}, ExtStaffID: "same-staff", ExtCustomerID: "same-customer", Description: "changed"}
	requireNoError(t, info.Update(info))
	info.ExtCorpID = "corp-a"
	got, err := info.Get(info)
	requireNoError(t, err)
	if got.Description != "corp-a" {
		t.Fatal("portrait update crossed enterprise")
	}
	if err := (models.CustomerStaff{}).Update(&models.CustomerStaff{InternalTagIDs: constants.StringArrayField{"901"}}, "same-staff", "same-customer", "corp-b"); err != nil {
		t.Fatal(err)
	}
	var relationA, relationB models.CustomerStaff
	requireNoError(t, db.Where("ext_corp_id='corp-a'").Take(&relationA).Error)
	requireNoError(t, db.Where("ext_corp_id='corp-b'").Take(&relationB).Error)
	if len(relationA.InternalTagIDs) != 0 || len(relationB.InternalTagIDs) != 1 {
		t.Fatal("internal tag update crossed enterprises")
	}
	var count int64
	requireNoError(t, db.Model(&models.GroupChatMember{}).Count(&count).Error)
	if count != 2 {
		t.Fatalf("members overwritten across enterprises: %d", count)
	}
}

func TestWecomEventRetriesAndDuplicates(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "2"}, Code: "b", Status: 1})
	requireNoError(t, db.AutoMigrate(&models.WecomEventReceipt{}))
	ctx := testIdentity("1", "12")
	calls := 0
	run := func() error {
		calls++
		if calls == 1 {
			return errors.New("retryable failure")
		}
		return nil
	}
	if RunWecomEvent(ctx, "corp-a:customer", "event-a", run) == nil {
		t.Fatal("failed callback acknowledged")
	}
	requireNoError(t, RunWecomEvent(ctx, "corp-a:customer", "event-a", run))
	requireNoError(t, RunWecomEvent(ctx, "corp-a:customer", "event-a", run))
	if calls != 2 {
		t.Fatalf("successful callback executed repeatedly: %d", calls)
	}
	requireNoError(t, RunWecomEvent(testIdentity("2", "20"), "corp-b:customer", "event-b", run))
	if calls != 3 {
		t.Fatal("other enterprise event was deduplicated incorrectly")
	}
}
