package models

import (
	"core/app/constants"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func loadMassMsgStaffs(db *gorm.DB, msgs []*MassMsg) error {
	parents := make([]ExtCorpModel, 0, len(msgs))
	byID := make(map[parentKey]*MassMsg, len(msgs))
	for _, msg := range msgs {
		parents = append(parents, msg.ExtCorpModel)
		byID[aggregateKey(msg.ExtCorpID, msg.ID)] = msg
		msg.Staffs = []MassMsgStaff{}
	}
	rows, err := loadCorpChildren[MassMsgStaff](db, "mass_msg_id", parents, "id")
	if err != nil {
		return err
	}
	for _, row := range rows {
		p := byID[aggregateKey(row.ExtCorpID, row.MassMsgID)]
		p.Staffs = append(p.Staffs, row)
	}
	return nil
}

func writeMassMsgStaffs(tx *gorm.DB, msg *MassMsg) error {
	ids := make([]string, 0, len(msg.Staffs))
	for i := range msg.Staffs {
		row := &msg.Staffs[i]
		row.MassMsgID, row.ExtCorpID = msg.ID, msg.ExtCorpID
		ids = append(ids, row.ID)
	}
	// Match the previous append/upsert semantics: delivery history is not deleted.
	return writeOwnedRows(tx, ownedRows{"mass_msg_id", msg.ID, msg.ExtCorpID}, msg.Staffs, ids,
		[]string{"ext_staff_id", "ext_customer_id", "ext_chat_id", "is_sent", "is_delivered", "failed_reason", "updated_at"}, false)
}

func loadWelcomePeriods(db *gorm.DB, msgs []*WelcomeMsg) error {
	parents := make([]ExtCorpModel, 0, len(msgs))
	byID := make(map[parentKey]*WelcomeMsg, len(msgs))
	for _, msg := range msgs {
		parents = append(parents, msg.ExtCorpModel)
		byID[aggregateKey(msg.ExtCorpID, msg.ID)] = msg
		msg.TimePeriodMsg = []WelcomeMsg{}
	}
	rows, err := loadCorpChildren[WelcomeMsg](db, "main_welcome_msg_id", parents, "id")
	if err != nil {
		return err
	}
	for _, row := range rows {
		p := byID[aggregateKey(row.ExtCorpID, *row.MainWelcomeMsgID)]
		p.TimePeriodMsg = append(p.TimePeriodMsg, row)
	}
	return nil
}

func writeWelcomePeriods(tx *gorm.DB, msg *WelcomeMsg) error {
	ids := make([]string, 0, len(msg.TimePeriodMsg))
	for i := range msg.TimePeriodMsg {
		row := &msg.TimePeriodMsg[i]
		row.MainWelcomeMsgID, row.ExtCorpID = &msg.ID, msg.ExtCorpID
		if row.ExtCreatorID == "" {
			row.ExtCreatorID = msg.ExtCreatorID
		}
		ids = append(ids, row.ID)
	}
	return writeOwnedRows(tx, ownedRows{"main_welcome_msg_id", msg.ID, msg.ExtCorpID}, msg.TimePeriodMsg, ids,
		[]string{"name", "welcome_msg", "effective_at", "start_time", "end_time", "updated_at"}, true)
}

func updateWelcomeMsg(db *gorm.DB, msg WelcomeMsg) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var current WelcomeMsg
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND ext_corp_id = ?", msg.ID, msg.ExtCorpID).First(&current).Error; err != nil {
			return err
		}
		if err := tx.Model(&current).Omit("ext_creator_id", "created_at").Updates(&msg).Error; err != nil {
			return err
		}
		if msg.EnableTimePeriodMsg == constants.False {
			msg.TimePeriodMsg = []WelcomeMsg{}
		}
		if msg.TimePeriodMsg == nil {
			return nil
		}
		return writeWelcomePeriods(tx, &msg)
	})
}
