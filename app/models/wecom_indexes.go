package models

import "gorm.io/gorm"

// EnsureWecomIndexes is also used when preparing isolated integration schemas.
// Historical indexes are replaced by the versioned migration, never at runtime.
func EnsureWecomIndexes(db *gorm.DB) error {
	for _, idx := range []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_corp_external ON customer(ext_corp_id, ext_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_staff_corp_external ON customer_staff(ext_corp_id, ext_customer_id, ext_staff_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_info_corp_external ON customer_info(ext_corp_id, ext_customer_id, ext_staff_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_corp_external ON tag(ext_corp_id, ext_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_tag_group_corp_external ON tag_group(ext_corp_id, ext_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_corp_external ON group_chat(ext_corp_id, ext_chat_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_group_chat_member_corp_external ON group_chat_member(ext_corp_id, ext_chat_id, userid)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_customer_statistic_corp_day ON customer_statistic(ext_corp_id, ext_staff_id, date)`,
	} {
		if err := db.Exec(idx).Error; err != nil {
			return err
		}
	}
	return nil
}
