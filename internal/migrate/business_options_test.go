package migrate

import "testing"

func TestOpportunityReadMigration(t *testing.T) {
	db := legacyTestDB(t)
	for _, query := range []string{
		`CREATE TABLE crm_opportunity(id bigint PRIMARY KEY,tenant_id bigint,owner_user_id bigint,customer_id bigint,stage_id bigint,title varchar(255),amount numeric(16,2),probability int,expected_close_date timestamptz,status varchar(32),deleted_at timestamptz)`,
		`INSERT INTO crm_opportunity(id,tenant_id,owner_user_id,customer_id,stage_id,title,amount,probability,status) VALUES(1,1,10,100,200,'preserved',12.34,50,'open')`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := applyFile(db, "0026_opportunity_bounded_reads.sql"); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Raw(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND i.indisvalid AND c.relname IN ('idx_opportunity_customer_title_prefix','idx_opportunity_open_stage_cursor','idx_opportunity_open_owner_stage_cursor','idx_opportunity_open_calendar','idx_opportunity_open_owner_calendar')`).Scan(&count).Error; err != nil || count != 5 {
		t.Fatalf("invalid opportunity read indexes: %d %v", count, err)
	}
	if err := db.Table("crm_opportunity").Where("id=1 AND tenant_id=1 AND owner_user_id=10 AND customer_id=100 AND stage_id=200 AND title='preserved' AND amount=12.34 AND probability=50 AND status='open'").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("opportunity altered", err)
	}
}

func TestBusinessOptionsMigration(t *testing.T) {
	db := legacyTestDB(t)
	for _, query := range []string{
		`CREATE TABLE crm_customer(id bigint PRIMARY KEY,tenant_id bigint,owner_user_id bigint,name varchar(128),company varchar(255),mobile varchar(32),deleted_at timestamptz)`,
		`CREATE TABLE crm_order(id bigint PRIMARY KEY,tenant_id bigint,user_id bigint,title varchar(255),order_no varchar(64),deleted_at timestamptz)`,
		`INSERT INTO crm_customer(id,tenant_id,owner_user_id,name) VALUES(1,1,10,'unchanged')`,
		`INSERT INTO crm_order(id,tenant_id,user_id,title,order_no) VALUES(1,1,10,'contract','SO1')`,
	} {
		if err := db.Exec(query).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := applyFile(db, "0025_business_object_options.sql"); err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	if err := db.Raw(`SELECT count(*) FROM pg_index i JOIN pg_class c ON c.oid=i.indexrelid WHERE c.relnamespace=current_schema()::regnamespace AND i.indisvalid AND c.relname IN ('idx_customer_option_cursor','idx_customer_owner_option_cursor','idx_customer_name_prefix','idx_customer_company_prefix','idx_customer_mobile_prefix','idx_order_option_cursor','idx_order_user_option_cursor','idx_order_title_prefix','idx_order_number_prefix')`).Scan(&count).Error; err != nil || count != 9 {
		t.Fatalf("invalid picker indexes: %d %v", count, err)
	}
	if err := db.Table("crm_customer").Where("id=1 AND tenant_id=1 AND owner_user_id=10 AND name='unchanged'").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("customer altered", err)
	}
	if err := db.Table("crm_order").Where("id=1 AND tenant_id=1 AND user_id=10 AND title='contract' AND order_no='SO1'").Count(&count).Error; err != nil || count != 1 {
		t.Fatal("contract altered", err)
	}
}
