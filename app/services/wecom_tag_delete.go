package services

import (
	"core/app/models"
	"errors"
	"gorm.io/gorm"
)

func DeleteWecomTag(corpID, externalID, kind string) error {
	if corpID == "" || externalID == "" {
		return errors.New("企微标签身份缺失")
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		ids := []string{externalID}
		if kind == "tag_group" {
			if err := tx.Model(&models.Tag{}).Where("ext_corp_id=? AND ext_group_id=?", corpID, externalID).Pluck("ext_id", &ids).Error; err != nil {
				return err
			}
			if err := tx.Where("ext_corp_id=? AND ext_id=?", corpID, externalID).Delete(&models.TagGroup{}).Error; err != nil {
				return err
			}
		}
		if len(ids) == 0 {
			return nil
		}
		if err := tx.Where("ext_corp_id=? AND ext_tag_id IN ?", corpID, ids).Delete(&models.CustomerStaffTag{}).Error; err != nil {
			return err
		}
		return tx.Where("ext_corp_id=? AND ext_id IN ?", corpID, ids).Delete(&models.Tag{}).Error
	})
}
