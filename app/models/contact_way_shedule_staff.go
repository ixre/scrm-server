package models

import (
	"core/app/constants"

	"github.com/pkg/errors"
)

// ContactWayScheduleStaff 渠道码绑定的员工
type ContactWayScheduleStaff struct {
	ExtCorpModel
	// 渠道码ID
	ContactWayID          string `json:"contact_way_id" gorm:"index;type:bigint;comment:'渠道码ID'"`
	ContactWayScheduleID  string `json:"contact_way_schedule_id" gorm:"default:0;index:ContactWayScheduleIndex;type:bigint;comment:'调度设置ID'"`
	AddCustomerCount      int    `json:"add_customer_count" gorm:"->;default:0;comment:'员工累计添加客户计数'"`
	DailyAddCustomerCount int    `json:"daily_add_customer_count" gorm:"->;default:0;comment:'员工每日添加客户计数'"`
	DailyAddCustomerLimit int    `json:"daily_add_customer_limit" gorm:"comment:'员工每日添加客户上限'"`
	ExtStaffID            string `json:"ext_staff_id" gorm:"index:ContactWayScheduleIndex;comment:'外部员工ID'"`
	// 员工名称
	Name string `gorm:"type:varchar(255);comment:员工名" json:"name"`
	// 头像url
	AvatarURL string            `gorm:"type:varchar(128);comment:头像地址" json:"avatar_url"`
	Online    constants.Boolean `json:"online" gorm:"index;default:1;comment:'员工是否在线'"`
	Timestamp
}

// DailyClean 每日清空添加数统计
func (o ContactWayScheduleStaff) DailyClean() (err error) {
	err = DB.Model(&ContactWayScheduleStaff{}).Update("daily_add_customer_count", 0).Error
	if err != nil {
		err = errors.Wrap(err, "update failed")
		return
	}

	return
}
