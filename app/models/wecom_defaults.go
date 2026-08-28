package models

import (
	"core/app/constants"
	"core/common/id_generator"
	"errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureWecomDefaults provisions enterprise-local defaults without resetting
// existing settings or reusing a process-wide primary key.
func EnsureWecomDefaults(db *gorm.DB, corpID string) error {
	if corpID == "" {
		return errors.New("missing enterprise for WeCom defaults")
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "wecom-defaults:"+corpID).Error; err != nil {
			return err
		}
		group := ContactWayGroup{ExtCorpModel: ExtCorpModel{ID: id_generator.StringID(), ExtCorpID: corpID}, Name: "默认分组", IsDefault: constants.True}
		if err := tx.Where("ext_corp_id = ? AND is_default = ?", corpID, constants.True).Attrs(group).FirstOrCreate(&ContactWayGroup{}).Error; err != nil {
			return err
		}
		rule := CustomerInfoDisplayRule{Model: Model{ID: id_generator.StringID()}, ExtCorpID: corpID,
			Age: constants.True, Description: constants.True, Email: constants.True, PhoneNumber: constants.True,
			QQ: constants.True, Address: constants.True, Birthday: constants.True, Weibo: constants.True}
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "ext_corp_id"}}, DoNothing: true}).Create(&rule).Error
	})
}
