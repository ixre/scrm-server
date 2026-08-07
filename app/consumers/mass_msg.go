package consumers

import (
	"core/app/constants"
	"core/app/models"
	"core/app/services"
	"core/common/delay_queue"
	"core/common/log"
)

func SendMassMsg(job delay_queue.Job) error {
	log.Sugar.Info("job info:", job)
	if job.Topic == constants.MassMsgTopic {
		customerService := services.NewDefaultMassMsgService()
		msgID, err := customerService.SendMassMsgToWx(job.Body, job.ID)
		if err != nil {
			log.Sugar.Error(err)
			return err
		}
		msg := models.MassMsg{
			ExtCorpModel:  models.ExtCorpModel{ID: job.ID},
			ExtMsgID:      msgID,
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
