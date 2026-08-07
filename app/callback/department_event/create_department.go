package department_event

import (
	"github.com/pkg/errors"
	"core/app/models"
	"core/common/id_generator"
	log2 "core/common/log"
	gowx "core/pkg/easywework"
)

// EventCreateDepartment
// Description: 新建部门事件回调
// Detail: 回调数据可以直接创建部门
func EventCreateDepartment(msg *gowx.RxMessage) error {
	if msg.MsgType != gowx.MessageTypeEvent ||
		msg.Event != gowx.EventTypeChangeContact ||
		msg.ChangeType != gowx.ChangeTypeCreateParty {
		return errors.New("wrong handler for the callback event")
	}
	eventCreateParty, ok := msg.EventCrateParty()
	if !ok {
		return errors.New("msg.EventEditExternalContact failed")
	}
	log2.Sugar.Debug(eventCreateParty)
	department := models.Department{
		Model:       models.Model{ID: id_generator.StringID()},
		ExtCorpID:   msg.ToUserID,
		ExtID:       eventCreateParty.GetID(),
		Name:        eventCreateParty.GetName(),
		ExtParentID: eventCreateParty.GetParentID(),
		Order:       uint32(eventCreateParty.GetOrder()),
	}
	return models.DB.Create(&department).Error
}
