package models

import (
	"context"
	"errors"
	"strings"
	"testing"

	"core/common/identity"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// guardDB 返回不连接数据库的 DryRun 会话：回调照常执行，SQL 只生成不发送。
func guardDB(t *testing.T, cfg TenantGuardConfig) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(postgres.New(postgres.Config{DSN: "host=127.0.0.1 port=1 dbname=none sslmode=disable"}), &gorm.Config{
		DryRun:                 true,
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
		Logger:                 logger.Default.LogMode(logger.Silent),
		NamingStrategy:         schema.NamingStrategy{SingularTable: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := UseTenantGuard(db, cfg); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestTenantGuardReads(t *testing.T) {
	db := guardDB(t, TenantGuardConfig{LegacyMode: TenantGuardEnforce})
	cases := []struct {
		name    string
		run     func(*gorm.DB) *gorm.DB
		wantErr error
	}{
		{"no condition", func(db *gorm.DB) *gorm.DB { return db.Find(&[]CrmCustomer{}) }, ErrTenantScopeMissing},
		{"primary key only", func(db *gorm.DB) *gorm.DB { return db.First(&CrmCustomer{}, "id = ?", "1") }, ErrTenantScopeMissing},
		{"count without tenant", func(db *gorm.DB) *gorm.DB { var n int64; return db.Model(&CrmCustomer{}).Count(&n) }, ErrTenantScopeMissing},
		{"pluck without tenant", func(db *gorm.DB) *gorm.DB { var ids []string; return db.Model(&CrmCustomer{}).Pluck("id", &ids) }, ErrTenantScopeMissing},
		{"empty tenant value", func(db *gorm.DB) *gorm.DB { return db.Where("tenant_id = ?", "").Find(&[]CrmCustomer{}) }, ErrTenantScopeMissing},
		{"empty TenantScope", func(db *gorm.DB) *gorm.DB { return db.Scopes(TenantScope("")).Find(&[]CrmCustomer{}) }, ErrMissingTenant},
		{"tenant inside OR expression", func(db *gorm.DB) *gorm.DB { return db.Where("tenant_id = ? OR 1=1", "t1").Find(&[]CrmCustomer{}) }, ErrTenantScopeMissing},
		{"top-level Or", func(db *gorm.DB) *gorm.DB {
			return db.Where("tenant_id = ?", "t1").Or("name = ?", "x").Find(&[]CrmCustomer{})
		}, ErrTenantScopeMissing},
		{"aliased table without tenant", func(db *gorm.DB) *gorm.DB {
			return db.Table("crm_customer c").Where("c.name = ?", "x").Find(&[]map[string]any{})
		}, ErrTenantScopeMissing},
		{"scan into dto via Table", func(db *gorm.DB) *gorm.DB {
			var rows []struct{ ID string }
			return db.Table("crm_customer").Find(&rows)
		}, ErrTenantScopeMissing},
		{"legacy table without corp", func(db *gorm.DB) *gorm.DB { return db.Where("ext_id = ?", "x").Find(&[]Customer{}) }, ErrTenantScopeMissing},

		{"empty tenant after nested group", func(db *gorm.DB) *gorm.DB {
			return db.Where("(name = ? OR mobile = ?) AND tenant_id = ?", "a", "b", "").Find(&[]CrmCustomer{})
		}, ErrTenantScopeMissing},
		{"tenant inside parenthesized OR", func(db *gorm.DB) *gorm.DB {
			return db.Where("(tenant_id = ? OR name = ?)", "t1", "x").Find(&[]CrmCustomer{})
		}, ErrTenantScopeMissing},
		{"tenant only inside subquery", func(db *gorm.DB) *gorm.DB {
			return db.Where("id IN (SELECT customer_id FROM crm_followup WHERE tenant_id = ?)", "t1").Find(&[]CrmCustomer{})
		}, ErrTenantScopeMissing},

		{"tenant expr", func(db *gorm.DB) *gorm.DB { return db.Where("tenant_id = ?", "t1").Find(&[]CrmCustomer{}) }, nil},
		{"tenant with nested OR in same expr", func(db *gorm.DB) *gorm.DB {
			return db.Where("tenant_id = ? AND (approve_status IN ? OR approve_status IS NULL) AND note <> 'a OR b'", "t1", []string{"none"}).Find(&[]CrmCustomer{})
		}, nil},
		{"tenant placeholder after nested group", func(db *gorm.DB) *gorm.DB {
			return db.Where("(name = ? OR mobile = ?) AND tenant_id = ?", "a", "b", "t1").Find(&[]CrmCustomer{})
		}, nil},
		{"TenantScope", func(db *gorm.DB) *gorm.DB {
			return db.Scopes(TenantScope("t1")).Where("name = ?", "x").Find(&[]CrmCustomer{})
		}, nil},
		{"tenant with other OR group", func(db *gorm.DB) *gorm.DB {
			return db.Where("tenant_id = ?", "t1").Where("name = ? OR mobile = ?", "a", "b").Find(&[]CrmCustomer{})
		}, nil},
		{"struct condition", func(db *gorm.DB) *gorm.DB { return db.Where(&CrmCustomer{TenantID: "t1"}).Find(&[]CrmCustomer{}) }, nil},
		{"map condition", func(db *gorm.DB) *gorm.DB { return db.Where(map[string]any{"tenant_id": "t1"}).Find(&[]CrmCustomer{}) }, nil},
		{"tenant IN", func(db *gorm.DB) *gorm.DB { return db.Where("tenant_id IN ?", []string{"t1"}).Find(&[]CrmCustomer{}) }, nil},
		{"aliased table with tenant", func(db *gorm.DB) *gorm.DB {
			return db.Table("crm_customer c").Where("c.tenant_id = ?", "t1").Find(&[]map[string]any{})
		}, nil},
		{"quoted column", func(db *gorm.DB) *gorm.DB {
			return db.Where(`"crm_customer"."tenant_id" = ?`, "t1").Find(&[]CrmCustomer{})
		}, nil},
		{"legacy table with corp", func(db *gorm.DB) *gorm.DB { return db.Where("ext_corp_id = ?", "c1").Find(&[]Customer{}) }, nil},
		{"non-tenant table", func(db *gorm.DB) *gorm.DB { return db.Find(&[]Tenant{}) }, nil},
		{"cross tenant", func(db *gorm.DB) *gorm.DB { return CrossTenantDB(db).Find(&[]CrmCustomer{}) }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.run(db.Session(&gorm.Session{NewDB: true})).Error
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func TestTenantGuardWrites(t *testing.T) {
	db := guardDB(t, TenantGuardConfig{})

	loaded := CrmCustomer{Model: Model{ID: "1"}, TenantID: "t1"}
	tx := db.Model(&loaded).Update("name", "x")
	if tx.Error != nil {
		t.Fatalf("update loaded record: %v", tx.Error)
	}
	if sql := tx.Statement.SQL.String(); !strings.Contains(sql, `"tenant_id" =`) {
		t.Fatalf("update of loaded record must be pinned to its tenant, got %s", sql)
	}
	if err := db.Delete(&CrmCustomer{Model: Model{ID: "1"}, TenantID: "t1"}).Error; err != nil {
		t.Fatalf("delete loaded record: %v", err)
	}
	if err := db.Model(&CrmCustomer{TenantID: "t1"}).Update("name", "x").Error; err == nil {
		t.Fatal("tenant-only object without key or WHERE must not widen into a tenant-wide update")
	}

	for name, run := range map[string]func() error{
		"update by id":             func() error { return db.Model(&CrmCustomer{}).Where("id = ?", "1").Update("name", "x").Error },
		"update record w/o tenant": func() error { return db.Model(&CrmCustomer{Model: Model{ID: "1"}}).Update("name", "x").Error },
		"delete by id":             func() error { return db.Delete(&CrmCustomer{}, "id = ?", "1").Error },
		"create without tenant":    func() error { return db.Create(&CrmCustomer{Model: Model{ID: "1"}, Name: "x"}).Error },
		"batch with one missing": func() error {
			return db.Create(&[]CrmCustomer{{Model: Model{ID: "1"}, TenantID: "t1"}, {Model: Model{ID: "2"}}}).Error
		},
		"create map without tenant": func() error { return db.Model(&CrmCustomer{}).Create(map[string]any{"id": "1", "name": "x"}).Error },
	} {
		if err := run(); !errors.Is(err, ErrTenantScopeMissing) {
			t.Errorf("%s: error = %v, want ErrTenantScopeMissing", name, err)
		}
	}

	for name, run := range map[string]func() error{
		"update with tenant": func() error {
			return db.Model(&CrmCustomer{}).Where("tenant_id = ? AND id = ?", "t1", "1").Update("name", "x").Error
		},
		"create with tenant": func() error { return db.Create(&CrmCustomer{Model: Model{ID: "1"}, TenantID: "t1"}).Error },
		"batch with tenant":  func() error { return db.Create(&[]CrmCustomer{{Model: Model{ID: "1"}, TenantID: "t1"}}).Error },
		"legacy create":      func() error { return db.Create(&Customer{ExtCorpModel: ExtCorpModel{ID: "1", ExtCorpID: "c1"}}).Error },
		"cross tenant update": func() error {
			return CrossTenantDB(db).Model(&CrmCustomer{}).Where("id = ?", "1").Update("name", "x").Error
		},
	} {
		if err := run(); err != nil {
			t.Errorf("%s: unexpected error %v", name, err)
		}
	}
}

func TestTenantGuardModes(t *testing.T) {
	logOnly := guardDB(t, TenantGuardConfig{Mode: TenantGuardLog})
	if err := logOnly.Find(&[]CrmCustomer{}).Error; err != nil {
		t.Fatalf("log mode must not block: %v", err)
	}
	defaults := guardDB(t, TenantGuardConfig{})
	if err := defaults.Where("ext_id = ?", "x").Find(&[]Customer{}).Error; err != nil {
		t.Fatalf("legacy tables default to log mode: %v", err)
	}
	if err := defaults.Find(&[]CrmCustomer{}).Error; !errors.Is(err, ErrTenantScopeMissing) {
		t.Fatalf("CRM tables default to enforce mode, got %v", err)
	}
	if err := UseTenantGuard(defaults, TenantGuardConfig{Mode: "off"}); err == nil {
		t.Fatal("unknown mode must be rejected")
	}
}

func TestForBindsContextTenant(t *testing.T) {
	previous := DB
	DB = guardDB(t, TenantGuardConfig{})
	t.Cleanup(func() { DB = previous })

	if _, err := For(context.Background()); !errors.Is(err, ErrMissingTenant) {
		t.Fatalf("For without identity: %v", err)
	}
	db, err := For(identity.WithIdentity(context.Background(), identity.Identity{TenantID: "t1"}))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Find(&[]CrmCustomer{}).Error; err != nil {
		t.Fatalf("For(ctx) query must satisfy the guard: %v", err)
	}
	if err := CrossTenant().Find(&[]CrmCustomer{}).Error; err != nil {
		t.Fatalf("CrossTenant query: %v", err)
	}
}
