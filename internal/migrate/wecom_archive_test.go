package migrate

import (
	"core/app/models"
	"testing"
)

func TestWecomArchiveTenantMigration(t *testing.T) {
	db := strictTestDB(t)
	if err := applyFile(db, strictVersion); err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.ChatMsg{}, &models.ChatMsgContent{}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"CREATE UNIQUE INDEX old_archive_message ON chat_msg(msg_id)",
		"INSERT INTO chat_msg(id,ext_corp_id,msg_id,seq) VALUES(800,'corp-a','shared-message',7)",
		"INSERT INTO chat_msg_content(id,ext_corp_id,chat_msg_id) VALUES(801,'corp-a',800)",
	} {
		if err := db.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := applyFile(db, "0028_wecom_archive_tenants.sql"); err != nil {
		t.Fatal(err)
	}
	var count int64
	for _, sql := range []string{"SELECT count(*) FROM chat_msg WHERE id=800 AND tenant_id=41", "SELECT count(*) FROM chat_msg_content WHERE id=801 AND tenant_id=41", "SELECT count(*) FROM wecom_archive_cursor WHERE corp_id='corp-a' AND tenant_id=41 AND seq=7"} {
		if err := db.Raw(sql).Scan(&count).Error; err != nil || count != 1 {
			t.Fatal("archive ownership or checkpoint missing", count, err)
		}
	}
	if err := db.Exec("INSERT INTO chat_msg(id,ext_corp_id,msg_id,seq) VALUES(802,'mapped-corp','shared-message',7)").Error; err != nil {
		t.Fatal("same external message in other enterprise rejected", err)
	}
	if err := db.Exec("INSERT INTO chat_msg(id,ext_corp_id,msg_id,tenant_id) VALUES(803,'corp-a','other-message',42)").Error; err == nil {
		t.Fatal("foreign archive tenant accepted")
	}
	if err := db.Exec("UPDATE chat_msg SET ext_corp_id='mapped-corp',tenant_id=42 WHERE id=800").Error; err == nil {
		t.Fatal("archive ownership reassigned")
	}
}
