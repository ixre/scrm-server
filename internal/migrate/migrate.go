// Package migrate 执行手工 SQL 迁移（AutoMigrate 覆盖不了的"改列定义"类 DDL）。
//
// 幂等机制：以文件名作为版本号记录进 schema_migrations 表，
// 已记录的文件重复运行直接跳过；未记录的文件在事务内执行后写入记录。
package migrate

import (
	"fmt"
	"sort"
	"strings"

	"core/migrations"

	"gorm.io/gorm"
)

const schemaMigrationsTable = "schema_migrations"

// Apply 对 db 执行所有未应用的手工迁移。
func Apply(db *gorm.DB) error {
	if err := ensureTable(db); err != nil {
		return err
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	files, err := migrations.Files.ReadDir(".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}
	var names []string
	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}
		names = append(names, f.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		if applied[name] {
			continue
		}
		if err := applyFile(db, name); err != nil {
			return fmt.Errorf("migration %s failed: %w", name, err)
		}
		fmt.Printf("applied: %s\n", name)
	}
	if len(names) == 0 {
		fmt.Println("no migrations found")
	}
	return nil
}

func ensureTable(db *gorm.DB) error {
	return db.Exec(fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (version VARCHAR(255) PRIMARY KEY, applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP)",
		schemaMigrationsTable)).Error
}

func appliedVersions(db *gorm.DB) (map[string]bool, error) {
	var versions []string
	if err := db.Table(schemaMigrationsTable).Pluck("version", &versions).Error; err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(versions))
	for _, v := range versions {
		set[v] = true
	}
	return set, nil
}

func applyFile(db *gorm.DB, name string) error {
	content, err := migrations.Files.ReadFile(name)
	if err != nil {
		return err
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	for _, stmt := range splitStatements(string(content)) {
		if stmt == "" {
			continue
		}
		if err := tx.Exec(stmt).Error; err != nil {
			return fmt.Errorf("statement [%s]: %w", stmt, err)
		}
	}

	if err := tx.Exec(fmt.Sprintf("INSERT INTO %s (version) VALUES (?)", schemaMigrationsTable), name).Error; err != nil {
		return err
	}
	return tx.Commit().Error
}

// splitStatements 按分号切分 SQL 语句。适用于 DDL 迁移（无存储过程/触发器内的分号）。
func splitStatements(s string) []string {
	raw := strings.Split(s, ";")
	out := make([]string, 0, len(raw))
	for _, stmt := range raw {
		out = append(out, strings.TrimSpace(stmt))
	}
	return out
}
