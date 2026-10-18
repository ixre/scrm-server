package migrate

import "testing"

func TestReceiptRequestMigration(t *testing.T) {
	db := legacyTestDB(t)
	for _, statement := range []string{
		`CREATE TABLE crm_receivables (id bigint PRIMARY KEY, tenant_id bigint, order_id bigint, amount numeric(16,2), receivables_no varchar(32), deleted_at timestamptz)`,
		`CREATE TABLE crm_receivables_plan (id bigint PRIMARY KEY, tenant_id bigint, order_id bigint, period_no integer, plan_date timestamptz, deleted_at timestamptz)`,
		`INSERT INTO crm_receivables(id,tenant_id,order_id,amount) VALUES (1,1,10,12.34),(2,1,10,12.34)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatal(err)
		}
	}
	const version = "0022_receipt_requests.sql"
	for i := 0; i < 2; i++ {
		if err := applyFile(db, version); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Table("crm_receivables").Where("amount=12.34 AND client_request_id IS NULL AND request_digest IS NULL").Count(&count).Error; err != nil || count != 2 {
		t.Fatalf("historical receipts changed: %d %v", count, err)
	}
	if err := db.Exec(`INSERT INTO crm_receivables(id,tenant_id,order_id,client_request_id,deleted_at) VALUES (3,1,10,'reserved-key',now())`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO crm_receivables(id,tenant_id,order_id,client_request_id) VALUES (4,1,10,'reserved-key')`).Error; err == nil {
		t.Fatal("soft-deleted receipt no longer reserves the request key")
	}
	if err := db.Exec(`INSERT INTO crm_receivables(id,tenant_id,order_id,client_request_id) VALUES (4,1,11,'reserved-key'),(5,2,10,'reserved-key')`).Error; err != nil {
		t.Fatalf("request key was incorrectly scoped globally: %v", err)
	}
	// Exercise IF NOT EXISTS as well as the recorded-version short circuit.
	if err := db.Exec("DELETE FROM schema_migrations WHERE version=?", version).Error; err != nil {
		t.Fatal(err)
	}
	if err := applyFile(db, version); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := applyFile(db, "0023_finance_workbench_reads.sql"); err != nil {
			t.Fatal(err)
		}
	}

}
