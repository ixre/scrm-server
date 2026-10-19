package migrate

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"core/app/dto"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"gorm.io/gorm"
)

const strictVersion = "0015_legacy_tenant_enforcement.sql"

func strictTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := legacyTestDB(t)
	id_generator.SetupIDGenerator()
	if err := db.AutoMigrate(&models.Tenant{}, &models.ChannelLink{}, &models.GroupChatMember{}, &models.CrmChannelConfig{}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS old_customer_external ON customer(ext_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_corp_external ON customer(ext_corp_id,ext_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS old_group_chat_external ON group_chat(ext_chat_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_corp_external ON group_chat(ext_corp_id,ext_chat_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_member_corp_external ON group_chat_member(ext_corp_id,ext_chat_id,userid)",
		"INSERT INTO tenant(id,code) VALUES(41,'tenant-a'),(42,'tenant-b')",
		"INSERT INTO channel_link(id,tenant_id,channel,entity_type,entity_id,ext_id,ext_corp_id) VALUES (1,41,'wecom','tenant',41,'legacy-corp',''),(2,42,'wecom','tenant',42,'mapped-corp',''),(3,41,'wecom','tenant',41,'corp-a','')",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := applyFile(db, legacyVersion); err != nil {
		t.Fatal(err)
	}
	if err := applyFile(db, repairVersion); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestStrictTenantMigrationAndWrites(t *testing.T) {
	db := strictTestDB(t)
	seedLegacyRows(t, db)
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	for _, table := range legacyTables {
		var count int64
		if err := db.Table(table).Where("(id=1 AND tenant_id=41) OR (id=2 AND tenant_id=42)").Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("%s backfill mismatch: %d %v", table, count, err)
		}
		if err := db.Exec("INSERT INTO " + table + "(id,ext_corp_id) VALUES(50,'corp-a')").Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Table(table).Where("id=50 AND tenant_id=41").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s insert did not resolve tenant: %d %v", table, count, err)
		}
		for _, sql := range []string{
			"INSERT INTO " + table + "(id,ext_corp_id) VALUES(51,'unknown')",
			"INSERT INTO " + table + "(id,ext_corp_id,tenant_id) VALUES(51,'corp-a',42)",
			"UPDATE " + table + " SET tenant_id=42,ext_corp_id='mapped-corp' WHERE id=50",
			"UPDATE " + table + " SET tenant_id=NULL WHERE id=50",
		} {
			if err := db.Exec(sql).Error; err == nil {
				t.Fatalf("accepted invalid ownership: %s", sql)
			}
		}
	}
	assertLegacyWrites(t, db)
	var customer models.Customer
	if err := db.First(&customer, "id=50").Error; err != nil || customer.TenantID != "41" {
		t.Fatalf("model failed to expose tenant: %+v %v", customer, err)
	}
	if err := db.AutoMigrate(&models.Customer{}, &models.Staff{}, &models.Department{}, &models.GroupChat{}, &models.MassMsg{}, &models.Material{}, &models.ContactWay{}); err != nil {
		t.Fatal(err)
	}
	var nonnull int64
	if err := db.Raw("SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND column_name='tenant_id' AND is_nullable='NO' AND table_name IN ?", legacyTables).Scan(&nonnull).Error; err != nil || nonnull != 7 {
		t.Fatalf("AutoMigrate weakened constraints: %d %v", nonnull, err)
	}
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"INSERT INTO staff(id,ext_corp_id,ext_id,avatar_url) VALUES(1000,'corp-a','shared-owner','own-avatar'),(1001,'mapped-corp','shared-owner','foreign-avatar')",
		"INSERT INTO group_chat(id,ext_corp_id,ext_chat_id,name,owner) VALUES(1000,'corp-a','query-a','query-only','shared-owner'),(1001,'mapped-corp','query-b','query-only','shared-owner')",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	groups, total, err := (models.GroupChat{}).Query(dto.QueryGroupChatReq{Name: "query-only"}, "corp-a", &app.Pager{Page: 1, PageSize: 20}, &app.Sorter{})
	if err != nil || total != 1 || len(groups) != 1 || groups[0].OwnerAvatarURL != "own-avatar" {
		t.Fatalf("group query crossed enterprises: count=%d groups=%+v err=%v", total, groups, err)
	}
}

func TestStrictTenantMigrationRejectsInvalidHistory(t *testing.T) {
	for _, scenario := range []string{"missing_mapping", "conflicting_tenant", "duplicate_mapping", "invalid_mapping"} {
		t.Run(scenario, func(t *testing.T) {
			db := strictTestDB(t)
			seedLegacyRows(t, db)
			queries := map[string]string{
				"missing_mapping":    "UPDATE contact_way SET ext_corp_id='unknown' WHERE id=1",
				"conflicting_tenant": "UPDATE material SET tenant_id=42 WHERE id=1",
				"duplicate_mapping":  "INSERT INTO channel_link(id,tenant_id,channel,entity_type,entity_id,ext_id,ext_corp_id) VALUES(9,42,'wecom','tenant',42,'legacy-corp','legacy-corp')",
				"invalid_mapping":    "UPDATE channel_link SET tenant_id=42 WHERE id=1",
			}
			if err := db.Exec(queries[scenario]).Error; err != nil {
				t.Fatal(err)
			}
			if err := applyFile(db, strictVersion); err == nil {
				t.Fatal("invalid history accepted")
			}
			// Earlier tables must roll back too if a later table fails preflight.
			var count int64
			if err := db.Table("customer").Where("id=1 AND tenant_id IS NULL").Count(&count).Error; err != nil || count != 1 {
				t.Fatalf("partial backfill committed: %d %v", count, err)
			}
			if err := db.Table(schemaMigrationsTable).Where("version=?", strictVersion).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("failed version recorded: %d %v", count, err)
			}
		})
	}
}

func TestStrictTenantMappingProtection(t *testing.T) {
	db := strictTestDB(t)
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"UPDATE channel_link SET tenant_id=42,entity_id=42 WHERE id=1",
		"UPDATE channel_link SET deleted_at=now() WHERE id=1",
		"DELETE FROM channel_link WHERE id=1",
		"UPDATE channel_link SET channel='other' WHERE id=1",
		"DELETE FROM tenant WHERE id=41",
		"UPDATE tenant SET id=99 WHERE id=41",
		"INSERT INTO channel_link(id,tenant_id,channel,entity_type,entity_id,ext_id) VALUES(9,99,'wecom','tenant',99,'corp-new')",
		"INSERT INTO channel_link(id,tenant_id,channel,entity_type,entity_id,ext_id) VALUES(9,41,'wecom','tenant',42,'corp-new')",
	} {
		if err := db.Exec(sql).Error; err == nil {
			t.Fatalf("mapping protection failed: %s", sql)
		}
	}
	if err := db.Exec("UPDATE tenant SET status=2 WHERE id=41").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return models.EnsureWecomTenantLink(tx, "41", "new-corp") }); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return models.EnsureWecomTenantLink(tx, "42", "new-corp") }); err == nil {
		t.Fatal("enterprise was reassigned")
	}
	if err := db.Exec("INSERT INTO material(id,ext_corp_id) VALUES(30,'new-corp')").Error; err != nil {
		t.Fatal(err)
	}
}

func TestStrictTenantConcurrentProvisioning(t *testing.T) {
	db := strictTestDB(t)
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan string, 8)
	for n := 0; n < 8; n++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			tenant := fmt.Sprint(41 + n%2)
			if err := db.Transaction(func(tx *gorm.DB) error { return models.EnsureWecomTenantLink(tx, tenant, "concurrent") }); err == nil {
				results <- tenant
			}
		}(n)
	}
	wg.Wait()
	close(results)
	winner := ""
	for tenant := range results {
		if winner != "" && winner != tenant {
			t.Fatal("two tenants claimed the same enterprise")
		}
		winner = tenant
	}
	if winner == "" {
		t.Fatal("no provisioning succeeded")
	}
	var count int64
	if err := db.Model(&models.ChannelLink{}).Where("channel='wecom' AND entity_type='tenant' AND ext_id='concurrent'").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("duplicate mapping: %d %v", count, err)
	}
}

func TestSQLFunctionStatementBoundaries(t *testing.T) {
	input := "-- migrate:statement\nDO $$ BEGIN PERFORM 1; PERFORM 2; END $$;\n-- migrate:statement\nSELECT ';';"
	statements := splitStatements(input)
	if len(statements) != 2 || !strings.Contains(statements[0], "PERFORM 1; PERFORM 2;") || statements[1] != "SELECT ';';" {
		t.Fatalf("function body split: %#v", statements)
	}
}

func TestStrictTenantConcurrentDelete(t *testing.T) {
	db := strictTestDB(t)
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO tenant(id,code) VALUES(99,'concurrent-delete')").Error; err != nil {
		t.Fatal(err)
	}
	tx := db.Begin()
	defer tx.Rollback()
	if err := models.EnsureWecomTenantLink(tx, "99", "delete-race"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- db.WithContext(ctx).Exec("DELETE FROM tenant WHERE id=99").Error }()
	select {
	case err := <-done:
		t.Fatalf("tenant delete did not wait for mapping transaction: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := tx.Commit().Error; err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil || strings.Contains(err.Error(), "deadline") {
		t.Fatalf("concurrent delete should fail ownership check: %v", err)
	}
}

func TestStrictTenantLookupUsesIndex(t *testing.T) {
	db := strictTestDB(t)
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"INSERT INTO tenant(id,code) SELECT n,'scale-'||n FROM generate_series(1000,10999) n",
		"INSERT INTO channel_link(id,tenant_id,channel,entity_type,entity_id,ext_id) SELECT n,n,'wecom','tenant',n,'scale-'||n FROM generate_series(1000,10999) n",
		"ANALYZE channel_link",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	var plan string
	if err := db.Raw("EXPLAIN (ANALYZE, FORMAT JSON) SELECT entity_id FROM channel_link WHERE channel='wecom' AND entity_type='tenant' AND deleted_at IS NULL AND ext_id='scale-10999'").Scan(&plan).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(plan, "idx_wecom_tenant_corp") || strings.Contains(plan, `"Node Type": "Seq Scan"`) {
		t.Fatalf("tenant resolution scans mappings: %s", plan)
	}
	start := time.Now()
	if err := db.Exec("INSERT INTO material(id,ext_corp_id) SELECT n,'scale-'||(1000+n%10000) FROM generate_series(1,5000) n").Error; err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("material").Where("tenant_id IS NOT NULL").Count(&count).Error; err != nil || count != 5000 {
		t.Fatalf("bulk insert missing ownership: %d %v", count, err)
	}
	t.Logf("10,000 mappings: indexed lookup; 5,000 guarded inserts: %s", time.Since(start))
}

func TestStrictTenantUpsertOwnership(t *testing.T) {
	db := strictTestDB(t)
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	customer := models.Customer{ExtCorpModel: models.ExtCorpModel{ID: "700", ExtCorpID: "corp-a"}, ExtID: "shared-external", Name: "original"}
	if err := customer.Upsert(customer); err != nil {
		t.Fatal(err)
	}
	foreign := customer
	foreign.ID, foreign.ExtCorpID, foreign.Name = "701", "mapped-corp", "foreign"
	if err := foreign.Upsert(foreign); err == nil {
		t.Fatal("customer upsert crossed enterprise")
	}
	batch := make([]models.Customer, 101)
	for i := 0; i < 100; i++ {
		batch[i] = models.Customer{ExtCorpModel: models.ExtCorpModel{ID: fmt.Sprint(1000 + i), ExtCorpID: "mapped-corp"}, ExtID: fmt.Sprintf("batch-%d", i)}
	}
	batch[100] = foreign
	if err := customer.BatchUpsert(batch); err == nil {
		t.Fatal("customer batch accepted foreign conflict")
	}
	var count int64
	if err := db.Model(&models.Customer{}).Where("ext_corp_id='mapped-corp'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("customer batch did not roll back: %d %v", count, err)
	}
	if err := db.Model(&models.Customer{}).Where("id=700 AND name='original'").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("foreign customer overwritten: %d %v", count, err)
	}
	chat := models.GroupChat{ExtCorpModel: models.ExtCorpModel{ID: "800", ExtCorpID: "corp-a"}, ExtChatID: "shared-chat", Name: "original",
		MemberList: []models.GroupChatMember{{ExtCorpModel: models.ExtCorpModel{ID: "900"}, Userid: "member-a"}, {ExtCorpModel: models.ExtCorpModel{ID: "901"}, Userid: "member-b"}},
	}
	if err := chat.Upsert(chat); err != nil {
		t.Fatal(err)
	}
	chat.ID = "801"
	chat.MemberList[0].ID, chat.MemberList[1].ID = "902", "903"
	if err := chat.Upsert(chat); err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&models.GroupChatMember{}).Where("ext_chat_id='shared-chat'").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("member upsert duplicated rows: %d %v", count, err)
	}
	chat.ID, chat.ExtCorpID, chat.Name = "802", "mapped-corp", "foreign"
	if err := chat.Upsert(chat); err == nil {
		t.Fatal("group upsert crossed enterprise")
	}
	chat.ExtCorpID = "corp-a"
	chat.MemberList[0].ExtCorpID = "mapped-corp"
	if err := chat.Upsert(chat); err == nil {
		t.Fatal("foreign member accepted")
	}
	if err := db.Model(&models.GroupChat{}).Where("id=800 AND name='original'").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("parent update did not roll back: %d %v", count, err)
	}
	if err := (models.GroupChatMember{}).Upsert([]models.GroupChatMember{{ExtCorpModel: models.ExtCorpModel{ID: "910", ExtCorpID: "mapped-corp"}, ExtChatID: "shared-chat", Userid: "foreign"}}); err == nil {
		t.Fatal("member inserted into foreign parent")
	}
}
