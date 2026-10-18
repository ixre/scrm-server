package migrate

import "testing"

func TestLeadFollowupMigration(t *testing.T) {
	db := legacyTestDB(t)
	for _, query := range []string{
		`CREATE TABLE crm_lead(id bigint PRIMARY KEY,tenant_id bigint,status varchar(32),pool_status smallint,convert_customer_id bigint,deleted_at timestamptz)`,
		`CREATE TABLE crm_followup(id bigint PRIMARY KEY,tenant_id bigint,lead_id bigint,deleted_at timestamptz)`,
		`INSERT INTO crm_lead(id,tenant_id,status,pool_status) VALUES(1,1,'contacted',0)`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := applyFile(db, "0024_lead_followup_workflow.sql"); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Table("crm_lead").Where("id=1 AND tenant_id=1 AND status='contacted' AND pool_status=0 AND next_followup_at IS NULL AND next_action IS NULL").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("historical lead appointment fabricated: %d %v", count, err)
	}
}
