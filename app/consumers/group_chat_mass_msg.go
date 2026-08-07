package consumers

import (
	"core/app/constants"
	"core/app/models"
	"core/app/services"
	"core/common/delay_queue"
	"core/common/log"
)

func SendGroupChatMassMsg(job delay_queue.Job) error {
	log.Sugar.Info("job info:", job)
	if job.Topic == constants.GroupChatMassMsgTopic {
		groupChatMassMsg := services.NewGroupChatMassMsg()
		extMsgID, err := groupChatMassMsg.DoSendGroupChatMassMsg(job.Body, job.ID)
		if err != nil {
			log.Sugar.Error(err)
			return err
		}
		msg := models.GroupChatMassMsg{
			ExtCorpModel:  models.ExtCorpModel{ID: job.ID},
			ExtMsgID:      extMsgID,
			MissionStatus: constants.Sending,
		}
		// job body 是req
		err = models.DB.Where("id = ?", job.ID).Updates(&msg).Error
		if err != nil {
			return err
		}
		err = delay_queue.Remove(job.ID)
		if err != nil {
			return err
		}
	}
	return nil
}
