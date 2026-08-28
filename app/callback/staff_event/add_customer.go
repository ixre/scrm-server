package staff_event

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"core/app/constants"
	"core/app/models"
	"core/app/services"
	"core/common/delay_queue"
	"core/common/id_generator"
	"core/common/log"
	gowx "core/pkg/easywework"
	"time"
)

// EventAddExternalContactHandler
// Description: 添加客户回调事件处理
// Detail:
// 	1. 同步客户数据
//	2. 发送欢迎语
//	3. 更新客户数
// Param: msg  wx回调参数
func EventAddExternalContactHandler(msg *gowx.RxMessage) error {
	if msg.MsgType != gowx.MessageTypeEvent ||
		msg.Event != gowx.EventTypeChangeExternalContact ||
		msg.ChangeType != gowx.ChangeTypeAddExternalContact {
		err := errors.New("wrong handler for the callback event")
		log.Sugar.Error("err", err)
		return err
	}
	eventAddExternalContact, ok := msg.EventAddExternalContact()
	if !ok {
		err := errors.New("msg.EventEditExternalContact failed")
		log.Sugar.Errorw("get event msg failed", "err", err)
		return err
	}
	extStaffID := eventAddExternalContact.GetUserID()
	extCustomerID := eventAddExternalContact.GetExternalUserID()
	welcomeCode := eventAddExternalContact.GetWelcomeCode()
	log.Sugar.Debugw("call back biz info:",
		"customerID", extCustomerID, "staffID", extStaffID, "welcomeCode", welcomeCode)

	customerSrv := services.NewCustomer()
	err := customerSrv.SyncSingleCustomerData(services.SyncTenantCtx(context.Background(), msg.ToUserID), extStaffID, extCustomerID)
	if err != nil {
		log.Sugar.Errorw("customerSrv.SyncSingleCustomerData failed",
			"extStaffID", extStaffID, "extCustomerID", extCustomerID)
		return err
	}

	// 更新员工的客户数
	err = models.CustomerStatistic{ExtCorpModel: models.ExtCorpModel{ExtCorpID: msg.ToUserID}}.Upsert(extStaffID, 1)
	if err != nil {
		log.Sugar.Errorw("update customer statistics failed",
			"extStaffID", extStaffID, "extCustomerID", extCustomerID)
		return err
	}

	err = SyncExternalContact(msg.ToUserID, extStaffID, extCustomerID)
	if err != nil {
		log.Sugar.Errorw("add sync job failed", "err", err)
		return err
	}

	shouldSendWelcomeMsg, err := (&services.ContactWay{}).DealAddCustomerEvent(services.SyncTenantCtx(context.Background(), msg.ToUserID), models.DB, eventAddExternalContact)
	if err != nil {
		log.Sugar.Errorw("ContactWay.DealAddCustomerEvent failed",
			"err", err, "eventAddExternalContact", eventAddExternalContact)
		return err
	}

	// 渠道码已发过欢迎语，这里就不再发了
	if shouldSendWelcomeMsg {
		welcomeCode := eventAddExternalContact.GetWelcomeCode()
		if welcomeCode != "" {
			log.Sugar.Infow("send default welcome msg")
			staff := &models.Staff{}
			err := models.DB.Model(&models.Staff{}).Where("ext_id = ? AND ext_corp_id = ?", extStaffID, msg.ToUserID).First(&staff).Error
			if err != nil {
				log.Sugar.Infow("get staff info failed", "err", err)
				return err
			}

			// send default welcome msg
			staffService := services.NewStaffService()
			return staffService.SendDefaultWelcomeMsg(services.SyncTenantCtx(context.Background(), staff.ExtCorpID), welcomeCode, staff.ExtCorpID, staff.ExtID)
		}
	}

	return nil
}

// SyncExternalContact
// Description: 生成同步客户数据的异步任务
// Detail: 异步处理，出错后可重试
// Param: extStaffID 员工外部ID
// Param: extCustomerID 客户外部ID
func SyncExternalContact(extCorpID, extStaffID, extCustomerID string) error {
	customerStaffRelation := models.CustomerStaffRelation{
 ExtCorpID: extCorpID,
		ExtStaffID:    extStaffID,
		ExtCustomerID: extCustomerID,
	}
	relation, err := json.Marshal(customerStaffRelation)
	if err != nil {
		err = errors.WithStack(err)
		return err
	}
	job := delay_queue.Job{
		Topic:     constants.SyncCustomerDataTopic,
		ID:        id_generator.StringID(),
		ExecuteAt: time.Now().Unix(),
		TTR:       10,
		Body:      string(relation),
	}
	return delay_queue.Add(job)
}
