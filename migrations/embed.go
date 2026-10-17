// Package migrations 嵌入手工迁移 SQL（*.sql），供 cmd/migrate 与 internal/migrate 使用。
package migrations

import (
	"embed"
	"fmt"
	"strings"
)

// Files 嵌入 migrations 目录下所有 .sql，运行时无需依赖 cwd 位置。
//
//go:embed *.sql
var Files embed.FS

// ModelIndexStatements shares the same index definitions between new databases
// created by AutoMigrate and installations upgraded with manual migrations.
func ModelIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0011_explicit_model_indexes.sql")
	if err != nil {
		return nil, err
	}
	var statements []string
	for _, statement := range strings.Split(string(content), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements, nil
}

// ReminderIndexStatements shares DDL with AutoMigrate without replaying data changes.
func ReminderIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0016_actionable_reminders.sql")
	if err != nil {
		return nil, err
	}
	_, ddl, ok := strings.Cut(string(content), "-- Reminder indexes.\n")
	if !ok {
		return nil, fmt.Errorf("reminder index marker missing")
	}
	var statements []string
	for _, statement := range strings.Split(ddl, ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements, nil
}
