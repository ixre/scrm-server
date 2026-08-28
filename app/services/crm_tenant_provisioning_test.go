package services

import (
	"encoding/base64"
	"strings"
	"testing"

	"core/app/models"
	"core/conf"
	"core/internal/migrate"
	"core/migrations"
)

func TestWecomTenantProvisioningRegression(t *testing.T) {
	db := regressionDB(t)
	db.Config.DisableForeignKeyConstraintWhenMigrating = true
	if err := db.AutoMigrate(&models.CrmChannelConfig{}, &models.Customer{}, &models.Staff{}, &models.Department{}, &models.GroupChat{}, &models.MassMsg{}, &models.Material{}, &models.ContactWay{}); err != nil {
		t.Fatal(err)
	}
	content, err := migrations.Files.ReadFile("0004_add_tenant_id_to_entity_tables.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, sql := range strings.Split(string(content), ";") {
		if strings.TrimSpace(sql) != "" {
			if err := db.Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := db.Exec("CREATE TABLE schema_migrations(version varchar(255) PRIMARY KEY,applied_at timestamp DEFAULT now())").Error; err != nil {
		t.Fatal(err)
	}
	files, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".sql") && file.Name() != "0015_legacy_tenant_enforcement.sql" {
			if err := db.Exec("INSERT INTO schema_migrations(version) VALUES(?)", file.Name()).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := migrate.Apply(db); err != nil {
		t.Fatal(err)
	}
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "2"}, Code: "b", Status: 1})
	previousKey := conf.Settings.App.Key
	conf.Settings.App.Key = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", 32)))
	t.Cleanup(func() { conf.Settings.App.Key = previousKey })
	service := CrmChannelConfigService{}
	ctx := testIdentity("1", "12")
	cfg, err := service.Create(ctx, ChannelConfigReq{Channel: "wecom", CorpID: "test-provision", Name: "test", Enabled: 2})
	if err != nil {
		t.Fatal(err)
	}
	var link models.ChannelLink
	if err := db.Where("entity_type='tenant' AND ext_id='test-provision'").First(&link).Error; err != nil || link.TenantID != "1" || link.EntityID != "1" {
		t.Fatalf("missing provisioned mapping: %+v %v", link, err)
	}
	if err := db.Exec("INSERT INTO material(id,ext_corp_id) VALUES(800,'test-provision')").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, cfg.ID, ChannelConfigReq{Name: "renamed", Enabled: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, cfg.ID, ChannelConfigReq{CorpID: "changed-corp"}); err == nil {
		t.Fatal("enterprise was rebound")
	}
	if _, err := service.Create(testIdentity("2", "20"), ChannelConfigReq{Channel: "wecom", CorpID: "test-provision", Enabled: 2}); err == nil {
		t.Fatal("other tenant reused enterprise")
	}
	// Force the configuration insert to fail after mapping creation; both must roll back.
	if err := db.Exec("ALTER TABLE crm_channel_config ADD CONSTRAINT reject_test_name CHECK(name <> 'reject-test')").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, ChannelConfigReq{Channel: "wecom", CorpID: "rollback-corp", Name: "reject-test", Enabled: 2}); err == nil {
		t.Fatal("expected configuration failure")
	}
	var count int64
	if err := db.Model(&models.ChannelLink{}).Where("ext_id='rollback-corp'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("mapping leaked after rollback: %d %v", count, err)
	}
}
