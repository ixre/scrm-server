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

func BusinessOptionIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0025_business_object_options.sql")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, statement := range strings.Split(string(content), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}

func LeadFollowupIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0024_lead_followup_workflow.sql")
	if err != nil {
		return nil, err
	}
	_, ddl, ok := strings.Cut(string(content), "-- Lead followup indexes.\n")
	if !ok {
		return nil, fmt.Errorf("lead followup index marker missing")
	}
	var result []string
	for _, statement := range strings.Split(ddl, ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}

// FinanceReadIndexStatements shares the finance workbench indexes with AutoMigrate.
func FinanceReadIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0023_finance_workbench_reads.sql")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, statement := range strings.Split(string(content), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}

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

// ChannelTaskIndexStatements shares pure index DDL with fresh installations.
func ChannelTaskIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0017_channel_task_workflow.sql")
	if err != nil {
		return nil, err
	}
	_, ddl, ok := strings.Cut(string(content), "-- Channel task indexes.\n")
	if !ok {
		return nil, fmt.Errorf("channel task index marker missing")
	}
	var statements []string
	for _, statement := range strings.Split(ddl, ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements, nil
}

// BoundedReadIndexStatements shares the pure DDL used for history and employee cursors.
func BoundedReadIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0018_bounded_crm_reads.sql")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, statement := range strings.Split(string(content), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}

// PoolRecycleIndexStatements shares index-only DDL with AutoMigrate.
func PoolRecycleIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0019_pool_recycle_indexes.sql")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, statement := range strings.Split(string(content), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}

// AllocationIndexStatements shares pure index DDL with fresh installations.
func AllocationIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0020_acquisition_allocation.sql")
	if err != nil {
		return nil, err
	}
	_, ddl, ok := strings.Cut(string(content), "-- Allocation indexes.\n")
	if !ok {
		return nil, fmt.Errorf("allocation index marker missing")
	}
	var result []string
	for _, statement := range strings.Split(ddl, ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}

// CustomerBusinessIndexStatements shares index definitions with AutoMigrate.
func CustomerBusinessIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0021_customer_business_reads.sql")
	if err != nil {
		return nil, err
	}
	var result []string
	for _, statement := range strings.Split(string(content), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}

// ReceiptRequestIndexStatements shares only index DDL with AutoMigrate.
func ReceiptRequestIndexStatements() ([]string, error) {
	content, err := Files.ReadFile("0022_receipt_requests.sql")
	if err != nil {
		return nil, err
	}
	_, ddl, ok := strings.Cut(string(content), "-- Receipt request indexes.\n")
	if !ok {
		return nil, fmt.Errorf("receipt request index marker missing")
	}
	var result []string
	for _, statement := range strings.Split(ddl, ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			result = append(result, statement)
		}
	}
	return result, nil
}
