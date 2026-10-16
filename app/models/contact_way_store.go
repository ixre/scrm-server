package models

import "gorm.io/gorm"

func filterContactWayStaffs(db *gorm.DB, staffIDs []string) *gorm.DB {
	// Keep the candidate IDs in PostgreSQL. The old three Pluck calls loaded
	// every matching relation into Go before applying the parent page limit.
	return db.Where(`(EXISTS (SELECT 1 FROM contact_way_staff s
		WHERE s.ext_corp_id = contact_way.ext_corp_id AND s.contact_way_id = contact_way.id AND s.deleted_at IS NULL AND s.ext_staff_id IN ?)
		OR EXISTS (SELECT 1 FROM contact_way_backup_staff s
		WHERE s.ext_corp_id = contact_way.ext_corp_id AND s.contact_way_id = contact_way.id AND s.deleted_at IS NULL AND s.ext_staff_id IN ?)
		OR EXISTS (SELECT 1 FROM contact_way_schedule_staff s
		WHERE s.ext_corp_id = contact_way.ext_corp_id AND s.contact_way_id = contact_way.id AND s.contact_way_schedule_id IS NOT NULL AND s.deleted_at IS NULL AND s.ext_staff_id IN ?))`, staffIDs, staffIDs, staffIDs)
}

func loadContactWays(db *gorm.DB, items []*ContactWay) error {
	parents := make([]ExtCorpModel, 0, len(items))
	byID := make(map[parentKey]*ContactWay, len(items))
	for _, item := range items {
		parents = append(parents, item.ExtCorpModel)
		byID[aggregateKey(item.ExtCorpID, item.ID)] = item
		item.Staffs, item.BackupStaffs, item.Schedules = []ContactWayStaff{}, []ContactWayBackupStaff{}, []ContactWaySchedule{}
	}
	staffs, err := loadCorpChildren[ContactWayStaff](db, "contact_way_id", parents, "id")
	if err != nil {
		return err
	}
	for _, row := range staffs {
		p := byID[aggregateKey(row.ExtCorpID, row.ContactWayID)]
		p.Staffs = append(p.Staffs, row)
	}
	backups, err := loadCorpChildren[ContactWayBackupStaff](db, "contact_way_id", parents, "id")
	if err != nil {
		return err
	}
	for _, row := range backups {
		p := byID[aggregateKey(row.ExtCorpID, row.ContactWayID)]
		p.BackupStaffs = append(p.BackupStaffs, row)
	}
	schedules, err := loadCorpChildren[ContactWaySchedule](db, "contact_way_id", parents, "id")
	if err != nil {
		return err
	}
	scheduleParents := make([]ExtCorpModel, 0, len(schedules))
	for i := range schedules {
		scheduleParents = append(scheduleParents, schedules[i].ExtCorpModel)
		schedules[i].Staffs = []ContactWayScheduleStaff{}
	}
	scheduledStaffs, err := loadCorpChildren[ContactWayScheduleStaff](db, "contact_way_schedule_id", scheduleParents, "id")
	if err != nil {
		return err
	}
	bySchedule := make(map[parentKey]*ContactWaySchedule, len(schedules))
	for i := range schedules {
		bySchedule[aggregateKey(schedules[i].ExtCorpID, schedules[i].ID)] = &schedules[i]
	}
	for _, row := range scheduledStaffs {
		p := bySchedule[aggregateKey(row.ExtCorpID, row.ContactWayScheduleID)]
		if p.ContactWayID == row.ContactWayID {
			p.Staffs = append(p.Staffs, row)
		}
	}
	for _, row := range schedules {
		p := byID[aggregateKey(row.ExtCorpID, row.ContactWayID)]
		p.Schedules = append(p.Schedules, row)
	}
	return nil
}

func createContactWay(db *gorm.DB, item *ContactWay) error {
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(item).Error; err != nil {
			return err
		}
		return replaceContactWayChildren(tx, item)
	})
}

func replaceContactWayChildren(tx *gorm.DB, item *ContactWay) error {
	owner := ownedRows{"contact_way_id", item.ID, item.ExtCorpID}
	columns := []string{"daily_add_customer_limit", "ext_staff_id", "name", "avatar_url", "online", "updated_at"}
	staffIDs := make([]string, 0, len(item.Staffs))
	for i := range item.Staffs {
		row := &item.Staffs[i]
		row.ContactWayID, row.ExtCorpID = item.ID, item.ExtCorpID
		staffIDs = append(staffIDs, row.ID)
	}
	if err := writeOwnedRows(tx, owner, item.Staffs, staffIDs, columns, true); err != nil {
		return err
	}
	backupIDs := make([]string, 0, len(item.BackupStaffs))
	for i := range item.BackupStaffs {
		row := &item.BackupStaffs[i]
		row.ContactWayID, row.ExtCorpID = item.ID, item.ExtCorpID
		backupIDs = append(backupIDs, row.ID)
	}
	if err := writeOwnedRows(tx, owner, item.BackupStaffs, backupIDs, columns, true); err != nil {
		return err
	}
	scheduleIDs := make([]string, 0, len(item.Schedules))
	for i := range item.Schedules {
		row := &item.Schedules[i]
		row.ContactWayID, row.ExtCorpID = item.ID, item.ExtCorpID
		scheduleIDs = append(scheduleIDs, row.ID)
	}
	if err := writeOwnedRows(tx, owner, item.Schedules, scheduleIDs, []string{"daily_add_customer_limit", "weekdays", "start_time", "end_time", "updated_at"}, true); err != nil {
		return err
	}
	for i := range item.Schedules {
		schedule := &item.Schedules[i]
		ids := make([]string, 0, len(schedule.Staffs))
		for j := range schedule.Staffs {
			row := &schedule.Staffs[j]
			row.ContactWayID, row.ContactWayScheduleID, row.ExtCorpID = item.ID, schedule.ID, item.ExtCorpID
			ids = append(ids, row.ID)
		}
		if err := writeOwnedRows(tx, ownedRows{"contact_way_schedule_id", schedule.ID, item.ExtCorpID}, schedule.Staffs, ids, columns, true); err != nil {
			return err
		}
	}
	// Retired schedules retain their IDs for history, while their staff stop participating.
	query := owner.scope(tx)
	if len(scheduleIDs) > 0 {
		query = query.Where("contact_way_schedule_id NOT IN ?", scheduleIDs)
	}
	return query.Delete(&ContactWayScheduleStaff{}).Error
}
