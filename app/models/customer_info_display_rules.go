package models

import (
	"core/app/constants"
)

// CustomerInfoDisplayRule 是否显示信息
type CustomerInfoDisplayRule struct {
	Model
	ExtCorpID   string            `gorm:"type:varchar;size:100;uniqueIndex;comment:企业ID" json:"ext_corp_id"`
	Age         constants.Boolean `gorm:"type:smallint;comment:是否展示年龄" json:"age"`
	Description constants.Boolean `gorm:"type:smallint;comment:是否展示描述" json:"description"`
	Email       constants.Boolean `gorm:"type:smallint;comment:是否展示邮箱" json:"email"`
	PhoneNumber constants.Boolean `gorm:"type:smallint;comment:是否展示电话" json:"phone_number"`
	QQ          constants.Boolean `gorm:"type:smallint;comment:是否展示mqq" json:"qq"`
	Address     constants.Boolean `gorm:"type:smallint;comment:是否展示地址" json:"address"`
	Birthday    constants.Boolean `gorm:"type:smallint;comment:是否展示生日" json:"birthday"`
	Weibo       constants.Boolean `gorm:"type:smallint;comment:是否展示微博" json:"weibo" `
	Timestamp
}

func (r CustomerInfoDisplayRule) Get(rules *CustomerInfoDisplayRule) (*CustomerInfoDisplayRule, error) {
	infoRules := &CustomerInfoDisplayRule{}
	if err := DB.Model(&CustomerInfoDisplayRule{}).
		Where(" ext_corp_id = ?", rules.ExtCorpID).
		First(&infoRules).Error; err != nil {
		return nil, err
	}
	return infoRules, nil
}

func (r CustomerInfoDisplayRule) Updates(id string, rule map[string]interface{}) error {
	return DB.Model(&CustomerInfoDisplayRule{}).Where("ext_corp_id = ?", id).Updates(&rule).Error
}
