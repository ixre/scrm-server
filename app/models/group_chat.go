package models

import (
	"core/app/constants"
	"core/app/requests"
	"core/common/app"
	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

type GroupChat struct {
	ExtCorpModel
	LegacyTenant
	ExtChatID          string                     `gorm:";type:char(64);comment:群聊id" json:"ext_chat_id"`
	Name               string                     `gorm:"type:varchar(255);index;comment:群名字" json:"name"`
	Owner              string                     `gorm:"type:char(64);index;comment:群主ExtID" json:"owner"`
	OwnerName          string                     `gorm:"type:char(64);comment:群主名字" json:"owner_name"`
	CreateTime         time.Time                  `gorm:"comment:创建时间" json:"create_time"`
	Notice             string                     `gorm:"type:text;comment:群公告" json:"notice"`
	MemberList         []GroupChatMember          `gorm:"foreignKey:ExtCorpID,ExtChatID;references:ExtCorpID,ExtChatID;" json:"member_list"`
	AdminList          constants.StringArrayField `gorm:"type:jsonb;comment:群管理员列表" json:"admin_list"`
	Status             constants.GroupChatStatus  `gorm:"type:smallint;default:2;comment:群状态 1-解散 2-未解散" json:"status"`
	Total              int64                      `gorm:"type:bigint;default:0;comment:群人数" json:"total"`
	TodayJoinMemberNum int64                      `gorm:"type:bigint;default:0;comment:今日进群人数" json:"today_join_member_num"`
	TodayQuitMemberNum int64                      `gorm:"type:bigint;default:0;comment:今日退群人数" json:"today_quit_member_num"`
	Tags               []GroupChatTag             `gorm:"many2many:group_chat_tags" json:"tags"`
	OwnerAvatarURL     string                     `gorm:"->" json:"owner_avatar_url"`
	OwnerRoleType      string                     `gorm:"->" json:"owner_role_type"`
	Timestamp
}

type GroupChatMainInfo struct {
	ExtChatID string `gorm:";type:char(64);comment:群聊id" json:"ext_chat_id"`
	Name      string `gorm:"type:text;comment:群名字" json:"name"`
	OwnerName string `json:"owner_name"`
}

type GroupChatWithStaffMainInfo struct {
	GroupChat
	OwnerAvatarURL string `json:"owner_avatar_url"`
	OwnerRoleType  string `json:"owner_role_type"`
}

func (g GroupChat) TableName() string {
	return "group_chat"
}

func (g GroupChat) Upsert(chat GroupChat) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ext_corp_id"}, {Name: "ext_chat_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "owner", "create_time", "notice", "admin_list", "total"}),
			Where:     clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "group_chat.ext_corp_id = excluded.ext_corp_id"}}},
		}).Omit(clause.Associations).Create(&chat)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("群聊外部 ID 已属于其他企业")
		}
		for i := range chat.MemberList {
			member := &chat.MemberList[i]
			if (member.ExtCorpID != "" && strings.TrimSpace(member.ExtCorpID) != strings.TrimSpace(chat.ExtCorpID)) || (member.ExtChatID != "" && strings.TrimSpace(member.ExtChatID) != strings.TrimSpace(chat.ExtChatID)) {
				return errors.New("群成员与所属企业或群聊不匹配")
			}
			member.ExtCorpID, member.ExtChatID = chat.ExtCorpID, chat.ExtChatID
		}
		return upsertGroupChatMembers(tx, chat.MemberList)
	})
}

func (g GroupChat) Query(req requests.QueryGroupChatReq, extCorpID string, pager *app.Pager, sorter *app.Sorter) (gc []GroupChat, total int64, err error) {
	gc = make([]GroupChat, 0)

	db := DB.Table("group_chat").
		Where("group_chat.ext_corp_id = ? AND group_chat.deleted_at IS NULL", extCorpID).
		Joins("left join staff on staff.ext_id = group_chat.owner AND staff.ext_corp_id = group_chat.ext_corp_id AND staff.deleted_at IS NULL")

	if len(req.Owners) != 0 {
		db = db.Where("owner in (?)", req.Owners)
	}

	if req.Name != "" {
		db = db.Where("group_chat.name like ? ", req.Name+"%")
	}

	if req.CreateTimeStart != "" && req.CreateTimeEnd != "" {
		db = db.Where("create_time between ? and ?", req.CreateTimeStart, req.CreateTimeEnd)
	}

	if req.Status != 0 {
		db = db.Where("group_chat.status = ?", req.Status)
	}

	if req.GroupTagIDs != nil && len(req.GroupTagIDs) > 0 {
		db = db.Joins("left join group_chat_tags on  group_chat_tags.group_chat_id = group_chat.id").
			Where("group_chat_tags.group_chat_tag_id in (?)", req.GroupTagIDs.ToInt64Array())
	}

	err = db.Count(&total).Error
	if err != nil || total == 0 {
		err = errors.Wrap(err, "Count GroupChatReq failed")
		return
	}

	sorter.SetDefault()
	db = db.Order(clause.OrderByColumn{Column: clause.Column{Name: string(sorter.SortField)}, Desc: sorter.SortType == constants.SortTypeDesc})

	pager.SetDefault()
	db = db.Offset(pager.GetOffset()).Limit(pager.GetLimit())

	err = db.Preload("Tags").Preload("MemberList").
		Select("group_chat.*, staff.avatar_url as owner_avatar_url, staff.role_type as owner_role_type").Find(&gc).Error
	if err != nil {
		err = errors.Wrap(err, "Find GroupChat failed")
		return
	}
	return
}

func (g GroupChat) GetAll(req requests.QueryGroupChatReq, extCorpID string) (gc []GroupChat, err error) {
	db := DB.Model(&GroupChat{}).Where("ext_corp_id = ?", extCorpID)
	if len(req.Owners) != 0 {
		db = db.Where("owner in (?)", len(req.Owners))
	}

	if req.Name != "" {
		db = db.Where("name = ?", req.Name)
	}

	if req.Status != 0 {
		db = db.Where("status = ?", req.Status)
	}
	var items []GroupChat
	err = db.Find(&items).Error
	if err != nil {
		err = errors.Wrap(err, "Find GroupChat failed")
		return nil, err
	}
	return items, nil

}

// UpdateMemNum
// Description: 更新群的增量数据
// Detail: 更新当日入群/退群 人数, 每日清零
func (g GroupChat) UpdateMemNum(chatID string, updateDetail string, changeCnt int64) error {

	db := DB.Model(&GroupChat{}).Where("ext_chat_id = ?", chatID)

	if updateDetail == "del_member" {
		return db.Update("today_quit_member_num", gorm.Expr("today_quit_member_num + ?", changeCnt)).Error
	} else if updateDetail == "add_member" {
		return db.Update("today_join_member_num", gorm.Expr("today_join_member_num + ?", changeCnt)).Error
	}

	return nil
}

// GetAllOwners
// Description: 查询所有群主
// Detail: 只返回群主简要信息
func (g GroupChat) GetAllOwners(extCorpId string) (staffs []StaffMainInfo, err error) {
	var owners []string
	err = DB.Model(&GroupChat{}).Where("ext_corp_id = ?", extCorpId).Distinct("owner").Pluck("owner", &owners).Error
	if err != nil {
		err = errors.WithStack(err)
		return
	}

	err = DB.Model(&Staff{}).Where("ext_corp_id = ? and ext_id  in (?)", extCorpId, owners).Find(&staffs).Error
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

// UpdateTags
// Description: 给客户群打标签
// Detail: group_chat_tags 是gorm 生成的关联表
// Param: gid 群ID
// Param: addTagIDs 新增的标签ID
// Param: removeTagIDs 删除的标签ID
func (g GroupChat) UpdateTags(gid string, addTagIDs constants.StringArrayField, removeTagIDs constants.StringArrayField) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if len(addTagIDs) > 0 {
			for _, tid := range addTagIDs {
				rowSQL := `replace into group_chat_tags ( group_chat_id, group_chat_tag_id ) values ( ?, ? )`
				err := DB.Exec(rowSQL, gid, tid).Error
				if err != nil {
					return err
				}
			}
		}
		if len(removeTagIDs) > 0 {
			rowSQL := `delete from group_chat_tags where group_chat_tag_id in (?)`
			err := DB.Exec(rowSQL, removeTagIDs.ToStringArray()).Error
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// GetAllChatsMainInfo
// Description: 查询群聊的简要信息列表
// Detail: 群名/群主名/外部群ID
func (g GroupChat) GetAllChatsMainInfo(
	pager *app.Pager, sorter *app.Sorter, extCorpID string) (items []GroupChatMainInfo, total int64, err error) {

	db := DB.Model(&GroupChat{}).Where("ext_corp_id = ?", extCorpID)

	err = db.Count(&total).Error
	if err != nil {
		err = errors.Wrap(err, "Count ChatsMainInfo failed")
		return
	}

	sorter.SetDefault()
	db = db.Order(
		clause.OrderByColumn{
			Column: clause.Column{Name: string(sorter.SortField)},
			Desc:   sorter.SortType == constants.SortTypeDesc,
		},
	)

	pager.SetDefault()
	db = db.Offset(pager.GetOffset()).Limit(pager.GetLimit())

	err = db.Select("ext_chat_id, name, owner_name").Find(&items).Error
	if err != nil {
		err = errors.Wrap(err, "Find ChatsMainInfo failed")
		return
	}
	return
}

// CleanGroupChatIncrement
// Description: 客户群增量数据清零
func (g GroupChat) CleanGroupChatIncrement(extCorpID string) error {
	return DB.Model(&GroupChat{}).
		Where("ext_corp_id= ?", extCorpID).
		Updates(map[string]interface{}{"today_join_member_num": 0, "today_quit_member_num": 0}).Error
}

func (g GroupChat) Update(chat GroupChat) (err error) {
	return DB.Model(&GroupChat{}).Where("ext_chat_id = ? AND ext_corp_id = ?", chat.ExtChatID, chat.ExtCorpID).Updates(chat).Error
}
