package migrate

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"core/app/models"
	"core/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

const legacyVersion = "0006_strict_tenant_id.sql"
const repairVersion = "0014_legacy_tenant_compatibility.sql"

var legacyTables = []string{"customer", "staff", "department", "group_chat", "mass_msg", "material", "contact_way"}

func legacyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("SCRM_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SCRM_TEST_DATABASE_URL to an isolated PostgreSQL database")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	config := &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true, NamingStrategy: schema.NamingStrategy{SingularTable: true}, Logger: logger.Default.LogMode(logger.Silent)}
	admin, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("legacy_migration_%d", time.Now().UnixNano())
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
	previous := models.DB
	models.DB = db
	t.Cleanup(func() {
		models.DB = previous
		conn, _ := db.DB()
		_ = conn.Close()
		if err := admin.Exec("DROP SCHEMA " + name + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		conn, _ = admin.DB()
		_ = conn.Close()
	})
	if err := db.AutoMigrate(&models.Customer{}, &models.Staff{}, &models.Department{}, &models.GroupChat{}, &models.MassMsg{}, &models.Material{}, &models.ContactWay{}); err != nil {
		t.Fatal(err)
	}
	// These tests model the pre-multi-corp schema explicitly; AutoMigrate now
	// creates composite enterprise keys instead of these historical indexes.
	for _, stmt := range []string{
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_customer_id ON customer(ext_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_corp_external ON customer(ext_corp_id,ext_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_corp_external ON group_chat(ext_corp_id,ext_chat_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_corp_id_ext_staff_id ON staff(ext_corp_id,ext_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_ext_corp_id_ext_dept_id ON department(ext_corp_id,ext_id)",
		"CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_ext_chat_id ON group_chat(ext_chat_id)",
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := ensureTable(db); err != nil {
		t.Fatal(err)
	}
	if err := applyFile(db, "0004_add_tenant_id_to_entity_tables.sql"); err != nil {
		t.Fatal(err)
	}
	return db
}

func seedLegacyRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range legacyTables {
		if err := db.Exec("INSERT INTO " + table + " (id,ext_corp_id,tenant_id) VALUES (1,'legacy-corp',NULL),(2,'mapped-corp',42)").Error; err != nil {
			t.Fatal(err)
		}
	}
}

func assertLegacyContents(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range legacyTables {
		var count int64
		if err := db.Table(table).Where("(id=1 AND ext_corp_id='legacy-corp' AND tenant_id IS NULL) OR (id=2 AND ext_corp_id='mapped-corp' AND tenant_id=42)").Count(&count).Error; err != nil || count != 2 {
			t.Fatalf("%s lost historical ownership: count=%d err=%v", table, count, err)
		}
	}
}

// The migration must preserve exactly the conflict targets used by the live models.
func assertLegacyWrites(t *testing.T, db *gorm.DB) {
	t.Helper()
	for n := 0; n < 2; n++ {
		id := fmt.Sprint(100 + n)
		customer := models.Customer{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: "corp-a"}, ExtID: "customer-a", Name: fmt.Sprint(n)}
		if err := customer.Upsert(customer); err != nil {
			t.Fatal(err)
		}
		staff := models.Staff{Model: models.Model{ID: id}, ExtCorpID: "corp-a", ExtID: "staff-a", RoleID: "1", Name: fmt.Sprint(n)}
		if err := staff.Upsert(staff); err != nil {
			t.Fatal(err)
		}
		dept := models.Department{Model: models.Model{ID: id}, ExtCorpID: "corp-a", ExtID: 1, Name: fmt.Sprint(n)}
		if err := dept.Upsert(dept); err != nil {
			t.Fatal(err)
		}
		chat := models.GroupChat{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: "corp-a"}, ExtChatID: "chat-a", Name: fmt.Sprint(n)}
		if err := db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "ext_corp_id"}, {Name: "ext_chat_id"}}, DoUpdates: clause.AssignmentColumns([]string{"name"})}).Omit(clause.Associations).Create(&chat).Error; err != nil {
			t.Fatal(err)
		}
		for _, row := range []any{
			&models.Material{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: "corp-a"}, FileSize: "0"},
			&models.MassMsg{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: "corp-a"}},
			&models.ContactWay{ExtCorpModel: models.ExtCorpModel{ID: id, ExtCorpID: "corp-a"}, GroupID: "1", SkipVerifyStartTime: "00:00", SkipVerifyEndTime: "23:59"},
		} {
			if err := db.Omit(clause.Associations).Create(row).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, table := range []string{"customer", "staff", "department", "group_chat"} {
		var count int64
		if err := db.Table(table).Where("ext_corp_id='corp-a' AND name='1'").Count(&count).Error; err != nil || count != 1 {
			t.Fatalf("%s upsert did not update the existing record: %d, %v", table, count, err)
		}
	}
}

func TestLegacyMigrationPending(t *testing.T) {
	db := legacyTestDB(t)
	seedLegacyRows(t, db)
	if err := applyFile(db, legacyVersion); err != nil {
		t.Fatal(err)
	}
	assertLegacyContents(t, db)
	assertLegacyWrites(t, db)
}

func markOtherVersionsApplied(t *testing.T, db *gorm.DB, pending ...string) {
	t.Helper()
	files, err := migrations.Files.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		skip := false
		for _, name := range pending {
			skip = skip || file.Name() == name
		}
		if !skip && strings.HasSuffix(file.Name(), ".sql") {
			if err := db.Exec("INSERT INTO schema_migrations(version) VALUES (?) ON CONFLICT DO NOTHING", file.Name()).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
}

func simulateStrictLegacySchema(t *testing.T, db *gorm.DB) {
	t.Helper()
	for _, table := range legacyTables {
		if err := db.Exec("ALTER TABLE " + table + " ALTER COLUMN tenant_id SET NOT NULL").Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, sql := range []string{
		"DROP INDEX idx_ext_customer_id",
		"DROP INDEX idx_ext_corp_id_ext_staff_id",
		"DROP INDEX idx_ext_corp_id_ext_dept_id",
		"DROP INDEX idx_group_chat_ext_chat_id",
		"CREATE UNIQUE INDEX idx_customer_tenant_ext_id ON customer(tenant_id,ext_id) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX idx_staff_tenant_ext_id ON staff(tenant_id,ext_id) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX idx_department_tenant_ext_id ON department(tenant_id,ext_id) WHERE deleted_at IS NULL",
		"CREATE UNIQUE INDEX idx_group_chat_tenant_ext_chat_id ON group_chat(tenant_id,ext_chat_id) WHERE deleted_at IS NULL",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestLegacyMigrationAlreadyApplied(t *testing.T) {
	db := legacyTestDB(t)
	simulateStrictLegacySchema(t, db)
	markOtherVersionsApplied(t, db, repairVersion)
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	assertLegacyWrites(t, db)
	// A later AutoMigrate must keep the repaired shape and legacy writes working.
	if err := db.AutoMigrate(&models.Department{}); err != nil {
		t.Fatal(err)
	}
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table(schemaMigrationsTable).Where("version = ?", repairVersion).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("repair version not recorded exactly once: %d %v", count, err)
	}
}

func TestLegacyRepairRejectsConflictingHistory(t *testing.T) {
	db := legacyTestDB(t)
	simulateStrictLegacySchema(t, db)
	if err := db.Exec("INSERT INTO customer(id,tenant_id,ext_corp_id,ext_id) VALUES (1,1,'corp-a','same'),(2,2,'corp-b','same')").Error; err != nil {
		t.Fatal(err)
	}
	if err := applyFile(db, repairVersion); err == nil || !strings.Contains(err.Error(), "idx_ext_customer_id") {
		t.Fatalf("expected conflicting legacy key to reject the migration, got %v", err)
	}
	var count int64
	if err := db.Table("customer").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("conflicting history was changed: %d %v", count, err)
	}
	if err := db.Table(schemaMigrationsTable).Where("version = ?", repairVersion).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed repair was marked applied: %d %v", count, err)
	}
	if err := db.Exec("INSERT INTO material(id) VALUES(999)").Error; err == nil {
		t.Fatal("failed migration did not roll back NOT NULL changes")
	}
}

func TestLegacyMigrationConcurrentRunners(t *testing.T) {
	db := legacyTestDB(t)
	seedLegacyRows(t, db)
	markOtherVersionsApplied(t, db, legacyVersion, repairVersion)
	var wg sync.WaitGroup
	errors := make(chan error, 2)
	for n := 0; n < 2; n++ {
		wg.Add(1)
		go func() { defer wg.Done(); errors <- Apply(db) }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	assertLegacyContents(t, db)
	if err := Apply(db); err != nil {
		t.Fatal(err)
	}
}

func TestLegacyDepartmentNullableTenantAndRollback(t *testing.T) {
	db := legacyTestDB(t)
	if err := applyFile(db, legacyVersion); err != nil {
		t.Fatal(err)
	}
	dept := models.Department{Model: models.Model{ID: "10"}, ExtCorpID: "corp-a", ExtID: 1, TenantID: "42", Name: "before"}
	if err := dept.Upsert(dept); err != nil {
		t.Fatal(err)
	}
	// A background sync with no local identity must not erase an existing mapping.
	dept.ID, dept.TenantID, dept.Name = "11", "", "after"
	if err := dept.Upsert(dept); err != nil {
		t.Fatal(err)
	}
	var saved models.Department
	if err := db.First(&saved, "id=10").Error; err != nil || saved.TenantID != "42" || saved.Name != "after" {
		t.Fatalf("existing tenant mapping changed: %+v %v", saved, err)
	}
	// The same external department ID can exist in a different enterprise.
	dept.ID, dept.ExtCorpID = "12", "corp-b"
	if err := dept.Upsert(dept); err != nil {
		t.Fatal(err)
	}
	var count int64
	if err := db.Table("department").Where("id=12 AND tenant_id IS NULL").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("unmapped department must use SQL NULL: %d %v", count, err)
	}
	first := models.Department{Model: models.Model{ID: "20"}, ExtCorpID: "corp-c", ExtID: 1}
	invalid := models.Department{Model: models.Model{ID: "21"}, ExtCorpID: "corp-c", ExtID: 2, TenantID: "invalid-bigint"}
	if err := first.Upsert(first, invalid); err == nil {
		t.Fatal("expected invalid tenant ID to fail")
	}
	if err := db.Table("department").Where("ext_corp_id='corp-c'").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("batch did not roll back: %d %v", count, err)
	}
}
