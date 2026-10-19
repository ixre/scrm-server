package models

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"core/app/constants"
	"core/app/dto"
	"core/common/id_generator"
	"core/migrations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

// These tests never use the application's configured database.
func aggregateTestDB(t *testing.T) *gorm.DB {
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
	name := fmt.Sprintf("aggregate_%d", time.Now().UnixNano())
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
	previous := DB
	DB = db
	id_generator.SetupIDGenerator()
	t.Cleanup(func() {
		DB = previous
		conn, _ := db.DB()
		_ = conn.Close()
		if err := admin.Exec("DROP SCHEMA " + name + " CASCADE").Error; err != nil {
			t.Error(err)
		}
		conn, _ = admin.DB()
		_ = conn.Close()
	})
	if err := db.AutoMigrate(&ContactWay{}, &ContactWayStaff{}, &ContactWayBackupStaff{}, &ContactWaySchedule{}, &ContactWayScheduleStaff{}, &QuickReplyGroup{}, &QuickReply{}, &QuickReplyDetail{}, &MassMsg{}, &MassMsgStaff{}, &WelcomeMsg{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestAggregateContactWayReplaceAndRollback(t *testing.T) {
	db := aggregateTestDB(t)
	item := ContactWay{ExtCorpModel: ExtCorpModel{ID: "300", ExtCorpID: "corp-a"}, GroupID: "1", SkipVerifyStartTime: "00:00", SkipVerifyEndTime: "23:59",
		Staffs:       []ContactWayStaff{{ExtCorpModel: ExtCorpModel{ID: "301"}, ExtStaffID: "sales", DailyAddCustomerLimit: 10}},
		BackupStaffs: []ContactWayBackupStaff{{ContactWayStaff: ContactWayStaff{ExtCorpModel: ExtCorpModel{ID: "302"}, ExtStaffID: "backup"}}},
		Schedules:    []ContactWaySchedule{{ExtCorpModel: ExtCorpModel{ID: "303"}, StartTime: "09:00", EndTime: "18:00", Staffs: []ContactWayScheduleStaff{{ExtCorpModel: ExtCorpModel{ID: "304"}, ExtStaffID: "shift"}}}},
	}
	if err := createContactWay(db, &item); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE contact_way_staff SET add_customer_count = 9, daily_add_customer_count = 3 WHERE id = 301").Error; err != nil {
		t.Fatal(err)
	}
	if err := loadContactWays(db, []*ContactWay{&item}); err != nil {
		t.Fatal(err)
	}
	got, err := (ContactWay{}).Get(item.ID, item.ExtCorpID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Staffs) != 1 || len(got.BackupStaffs) != 1 || len(got.Schedules) != 1 || len(got.Schedules[0].Staffs) != 1 {
		t.Fatalf("incomplete aggregate: %+v", got)
	}
	created := got.Staffs[0].CreatedAt
	item.Staffs[0].DailyAddCustomerLimit = 20
	item.BackupStaffs, item.Schedules = nil, nil
	if err := db.Transaction(func(tx *gorm.DB) error { return replaceContactWayChildren(tx, &item) }); err != nil {
		t.Fatal(err)
	}
	got, err = (ContactWay{}).Get(item.ID, item.ExtCorpID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Staffs[0].AddCustomerCount != 9 || got.Staffs[0].DailyAddCustomerCount != 3 || got.Staffs[0].DailyAddCustomerLimit != 20 || !got.Staffs[0].CreatedAt.Equal(created) {
		t.Fatalf("operational state lost: %+v", got.Staffs[0])
	}
	if len(got.BackupStaffs) != 0 || len(got.Schedules) != 0 {
		t.Fatal("removed children still visible")
	}
	var retired ContactWayScheduleStaff
	if err := db.Unscoped().First(&retired, "id = 304").Error; err != nil {
		t.Fatal(err)
	}
	if !retired.DeletedAt.Valid || retired.ContactWayID != item.ID || retired.ContactWayScheduleID != "303" {
		t.Fatal("child history detached")
	}
	foreign := ContactWay{ExtCorpModel: ExtCorpModel{ID: "400", ExtCorpID: "corp-b"}, GroupID: "1", SkipVerifyStartTime: "00:00", SkipVerifyEndTime: "23:59", Staffs: []ContactWayStaff{{ExtCorpModel: ExtCorpModel{ID: "401"}, ExtStaffID: "foreign"}}}
	if err := createContactWay(db, &foreign); err != nil {
		t.Fatal(err)
	}
	item.Staffs = []ContactWayStaff{{ExtCorpModel: ExtCorpModel{ID: "401"}}}
	if err := db.Transaction(func(tx *gorm.DB) error { return replaceContactWayChildren(tx, &item) }); err == nil {
		t.Fatal("accepted another enterprise's child ID")
	}
	got, err = (ContactWay{}).Get("300", "corp-a")
	if err != nil || len(got.Staffs) != 1 || got.Staffs[0].ID != "301" {
		t.Fatal("failed replacement changed original staff")
	}
	items, total, err := (ContactWay{}).Query(dto.QueryContactWayReq{ExtStaffIDs: []string{"sales"}}, "corp-a")
	if err != nil || total != 1 || len(items) != 1 || items[0].ID != "300" {
		t.Fatalf("staff EXISTS filter failed: %+v %d %v", items, total, err)
	}
	_, total, err = (ContactWay{}).Query(dto.QueryContactWayReq{ExtStaffIDs: []string{"foreign", "backup"}}, "corp-a")
	if err != nil || total != 0 {
		t.Fatalf("staff filter matched foreign or deleted rows: %d %v", total, err)
	}
	broken := ContactWay{ExtCorpModel: ExtCorpModel{ID: "500", ExtCorpID: "corp-a"}, GroupID: "1", SkipVerifyStartTime: "00:00", SkipVerifyEndTime: "23:59", Staffs: []ContactWayStaff{{ExtCorpModel: ExtCorpModel{ID: "501"}}, {ExtCorpModel: ExtCorpModel{ID: "501"}}}}
	if err := createContactWay(db, &broken); err == nil {
		t.Fatal("accepted duplicate children")
	}
	var count int64
	if err := db.Model(&ContactWay{}).Where("id = 500").Count(&count).Error; err != nil || count != 0 {
		t.Fatal("failed create did not roll back parent")
	}
}

func TestAggregateQuickReplyTransaction(t *testing.T) {
	db := aggregateTestDB(t)
	for _, base := range []int{100, 200} {
		reply := QuickReply{ExtCorpModel: ExtCorpModel{ID: fmt.Sprint(base), ExtCorpID: "corp-a"}, GroupID: "1", Name: "before", ReplyDetails: []QuickReplyDetail{{ExtCorpModel: ExtCorpModel{ID: fmt.Sprint(base + 1)}, ContentType: 2}, {ExtCorpModel: ExtCorpModel{ID: fmt.Sprint(base + 2)}, ContentType: 3}}}
		if err := (QuickReply{}).Create(&reply); err != nil {
			t.Fatal(err)
		}
	}
	update := QuickReply{ExtCorpModel: ExtCorpModel{ID: "100", ExtCorpID: "corp-a"}, Name: "after", ReplyDetails: []QuickReplyDetail{{ExtCorpModel: ExtCorpModel{ID: "201"}, ContentType: 6}}}
	if _, err := (QuickReply{}).UpdateWithDetails(update, []string{"102"}); err == nil {
		t.Fatal("accepted another reply's detail")
	}
	before, err := (QuickReply{}).Get("100")
	if err != nil || before.Name != "before" || len(before.ReplyDetails) != 2 {
		t.Fatalf("partial update escaped rollback: %+v, %v", before, err)
	}
	update.ReplyDetails[0].ID = "101"
	got, err := (QuickReply{}).UpdateWithDetails(update, []string{"102"})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "after" || len(got.ReplyDetails) != 1 || got.ReplyDetails[0].ContentType != 6 {
		t.Fatalf("details not updated: %+v", got)
	}
	update.ExtCorpID = "corp-b"
	if _, err := (QuickReply{}).UpdateWithDetails(update, nil); err == nil {
		t.Fatal("cross-enterprise update succeeded")
	}
	// A corrupt cross-enterprise row must not be included in the response.
	if err := db.Create(&QuickReplyDetail{ExtCorpModel: ExtCorpModel{ID: "300", ExtCorpID: "corp-b"}, QuickReplyID: "100"}).Error; err != nil {
		t.Fatal(err)
	}
	got, err = (QuickReply{}).Get("100")
	if err != nil || len(got.ReplyDetails) != 1 {
		t.Fatal("loader included a foreign detail")
	}
}

func TestAggregateGroupHydrationUsesBoundedQueries(t *testing.T) {
	db := aggregateTestDB(t)
	for i := 1; i <= 8; i++ {
		group := QuickReplyGroup{ExtCorpModel: ExtCorpModel{ID: fmt.Sprint(i), ExtCorpID: "corp-a"}, QuickReplies: []QuickReply{{ExtCorpModel: ExtCorpModel{ID: fmt.Sprint(100 + i)}, ReplyDetails: []QuickReplyDetail{{ExtCorpModel: ExtCorpModel{ID: fmt.Sprint(200 + i)}}}}}}
		if err := (QuickReplyGroup{}).Create(group); err != nil {
			t.Fatal(err)
		}
	}
	var groups []QuickReplyGroup
	if err := db.Find(&groups).Error; err != nil {
		t.Fatal(err)
	}
	var queries atomic.Int32
	if err := db.Callback().Query().After("gorm:query").Register("test:count", func(*gorm.DB) { queries.Add(1) }); err != nil {
		t.Fatal(err)
	}
	if err := loadQuickReplyGroups(db, groups, nil); err != nil {
		t.Fatal(err)
	}
	if queries.Load() != 2 {
		t.Fatalf("expected two batch queries, got %d", queries.Load())
	}
	for _, group := range groups {
		if len(group.QuickReplies) != 1 || len(group.QuickReplies[0].ReplyDetails) != 1 {
			t.Fatalf("incomplete group: %+v", group)
		}
	}
	if err := loadQuickReplyGroups(db, groups, []string{"101"}); err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if group.ID != "1" && len(group.QuickReplies) != 0 {
			t.Fatal("reply filter ignored")
		}
	}
}

func TestAggregateWelcomePeriods(t *testing.T) {
	db := aggregateTestDB(t)
	msg := WelcomeMsg{ExtCorpModel: ExtCorpModel{ID: "600", ExtCorpID: "corp-a"}, StartTime: "00:00", EndTime: "23:59", EnableTimePeriodMsg: constants.True, TimePeriodMsg: []WelcomeMsg{{ExtCorpModel: ExtCorpModel{ID: "601"}, StartTime: "09:00", EndTime: "12:00", Name: "morning"}}}
	if err := (WelcomeMsg{}).Create(db, msg); err != nil {
		t.Fatal(err)
	}
	got, err := (WelcomeMsg{}).Get("600", "corp-a")
	if err != nil || len(got.TimePeriodMsg) != 1 {
		t.Fatalf("welcome periods missing: %+v %v", got, err)
	}
	got.TimePeriodMsg[0].Name = "afternoon"
	if err := (WelcomeMsg{}).Update(db, got); err != nil {
		t.Fatal(err)
	}
	got, err = (WelcomeMsg{}).Get("600", "corp-a")
	if err != nil || got.TimePeriodMsg[0].Name != "afternoon" {
		t.Fatalf("welcome period edit lost: %+v, err=%v", got, err)
	}
	got.TimePeriodMsg = []WelcomeMsg{}
	if err := (WelcomeMsg{}).Update(db, got); err != nil {
		t.Fatal(err)
	}
	got, err = (WelcomeMsg{}).Get("600", "corp-a")
	if err != nil || len(got.TimePeriodMsg) != 0 {
		t.Fatal("empty collection did not clear periods")
	}
}

func TestAggregateModelsHaveNoImplicitRelations(t *testing.T) {
	for _, model := range []any{&ContactWay{}, &ContactWaySchedule{}, &QuickReply{}, &QuickReplyGroup{}, &MassMsg{}, &WelcomeMsg{}} {
		s, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{SingularTable: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(s.Relationships.Relations) != 0 {
			t.Fatalf("%T still has implicit associations", model)
		}
	}
}

func TestAggregateEnterpriseJSON(t *testing.T) {
	data, err := json.Marshal(QuickReply{ExtCorpModel: ExtCorpModel{ExtCorpID: "corp-a"}})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if fields["ext_corp_id"] != "corp-a" {
		t.Fatalf("enterprise field was shadowed: %s", data)
	}
}

func TestAggregateFieldMigrationProtectsLegacyValues(t *testing.T) {
	db := aggregateTestDB(t)
	if err := db.Exec("ALTER TABLE quick_reply ADD COLUMN corp_id text").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("ALTER TABLE welcome_msg ALTER COLUMN name TYPE char(128)").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO quick_reply (id,corp_id) VALUES (1,'historical-id')").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO welcome_msg (id,name) VALUES (1,'欢迎咨询')").Error; err != nil {
		t.Fatal(err)
	}
	content, err := migrations.Files.ReadFile("0013_model_field_cleanup.sql")
	if err != nil {
		t.Fatal(err)
	}
	apply := func(tx *gorm.DB) error {
		for _, sql := range strings.Split(string(content), ";") {
			if strings.TrimSpace(sql) != "" {
				if err := tx.Exec(sql).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := db.Transaction(apply); err == nil {
		t.Fatal("discarded nonempty legacy enterprise ID")
	}
	var legacyID string
	if err := db.Raw("SELECT corp_id FROM quick_reply WHERE id = 1").Scan(&legacyID).Error; err != nil || legacyID != "historical-id" {
		t.Fatal("failed migration lost legacy data")
	}
	if err := db.Exec("UPDATE quick_reply SET corp_id = NULL WHERE id = 1").Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Transaction(apply); err != nil {
			t.Fatal(err)
		}
	}
	if db.Migrator().HasColumn("quick_reply", "corp_id") {
		t.Fatal("unused enterprise column retained")
	}
	var title string
	if err := db.Raw("SELECT name FROM welcome_msg WHERE id = 1").Scan(&title).Error; err != nil || title != "欢迎咨询" {
		t.Fatalf("title changed: %q %v", title, err)
	}
}

func TestAggregateConcurrentChildOwnership(t *testing.T) {
	db := aggregateTestDB(t)
	start := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []string{"700", "800"} {
		go func(id string) {
			<-start
			reply := QuickReply{ExtCorpModel: ExtCorpModel{ID: id, ExtCorpID: "corp-a"}, GroupID: "1", ReplyDetails: []QuickReplyDetail{{ExtCorpModel: ExtCorpModel{ID: "900"}}}}
			results <- (QuickReply{}).Create(&reply)
		}(id)
	}
	close(start)
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected one owner, got %d", successes)
	}
	var count int64
	if err := db.Model(&QuickReply{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatal("losing transaction left a parent")
	}
}

func TestAggregateBackfillAndIndexes(t *testing.T) {
	db := aggregateTestDB(t)
	group := QuickReplyGroup{ExtCorpModel: ExtCorpModel{ID: "1", ExtCorpID: "corp-a"}}
	reply := QuickReply{ExtCorpModel: ExtCorpModel{ID: "2"}, GroupID: "1"}
	detail := QuickReplyDetail{ExtCorpModel: ExtCorpModel{ID: "3"}, QuickReplyID: "2"}
	foreign := QuickReplyDetail{ExtCorpModel: ExtCorpModel{ID: "4", ExtCorpID: "corp-b"}, QuickReplyID: "2"}
	for _, row := range []any{&group, &reply, &detail, &foreign} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	content, err := migrations.Files.ReadFile("0012_child_enterprise_backfill.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		for _, sql := range strings.Split(string(content), ";") {
			if strings.TrimSpace(sql) != "" {
				if err := db.Exec(sql).Error; err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	got, err := (QuickReply{}).Get("2")
	if err != nil || len(got.ReplyDetails) != 1 || got.ReplyDetails[0].ID != "3" {
		t.Fatalf("backfill failed: %+v, %v", got, err)
	}
	if err := db.First(&foreign, "id = 4").Error; err != nil || strings.TrimSpace(foreign.ExtCorpID) != "corp-b" {
		t.Fatal("backfill overwrote a nonempty enterprise")
	}
	if err := db.AutoMigrate(&CrmCustomer{}, &CrmLead{}, &CrmOpportunity{}, &CrmFollowup{}, &CrmOrder{}, &CrmReceivables{}, &CrmQuote{}); err != nil {
		t.Fatal(err)
	}
	indexes, err := migrations.ModelIndexStatements()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		for _, sql := range indexes {
			if err := db.Exec(sql).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	var count int64
	if err := db.Raw("SELECT count(*) FROM pg_constraint WHERE contype = 'f' AND connamespace = current_schema()::regnamespace").Scan(&count).Error; err != nil || count != 0 {
		t.Fatal("migration introduced physical foreign keys")
	}
}

func TestAggregateQuickReplyRoundTrip(t *testing.T) {
	aggregateTestDB(t)
	reply := QuickReply{ExtCorpModel: ExtCorpModel{ID: "100", ExtCorpID: "corp-a"}, GroupID: "10", Name: "售前介绍", ReplyDetails: []QuickReplyDetail{{ExtCorpModel: ExtCorpModel{ID: "101", ExtCorpID: "corp-a"}, ContentType: 2}}}
	if err := (QuickReply{}).Create(&reply); err != nil {
		t.Fatal(err)
	}
	got, err := (QuickReply{}).Get("100")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.ReplyDetails) != 1 || got.ReplyDetails[0].QuickReplyID != "100" {
		t.Fatalf("details not loaded: %+v", got)
	}
}

func TestAggregateMassMessageRoundTrip(t *testing.T) {
	aggregateTestDB(t)
	msg := MassMsg{ExtCorpModel: ExtCorpModel{ID: "200", ExtCorpID: "corp-a"}, Staffs: []MassMsgStaff{{ExtCorpModel: ExtCorpModel{ID: "201", ExtCorpID: "corp-a"}, ExtStaffID: "sales", ExtCustomerID: "customer"}}}
	if err := (MassMsg{}).Create(msg); err != nil {
		t.Fatal(err)
	}
	got, err := (MassMsg{}).Get("200")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Staffs) != 1 || got.Staffs[0].MassMsgID != "200" {
		t.Fatalf("recipients not loaded: %+v", got)
	}
	second := MassMsg{ExtCorpModel: ExtCorpModel{ID: "202", ExtCorpID: "corp-a"}}
	if err := (MassMsg{}).Create(second); err != nil {
		t.Fatal(err)
	}
	msgs, err := (MassMsg{}).GetMsgs([]string{"200", "202"})
	if err != nil || len(msgs) != 2 {
		t.Fatalf("batch read omitted messages: %+v %v", msgs, err)
	}
}
