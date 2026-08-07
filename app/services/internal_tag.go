package services

import (
	"core/app/models"
	"core/app/requests"
	"core/common/app"
	"core/common/id_generator"
	"core/conf"
)

type InternalTag struct {
	internalTagRepo models.InternalTag
}

func NewInternalTag() *InternalTag {
	return &InternalTag{internalTagRepo: models.InternalTag{}}
}

func (o InternalTag) Create(req requests.CreateInternalTagReq, extCorpID string) ([]models.InternalTag, error) {
	tags := make([]models.InternalTag, 0)
	for _, name := range req.Names {
		tag := models.InternalTag{
			ExtCorpModel: models.ExtCorpModel{ID: id_generator.StringID(), ExtCorpID: conf.Settings.WeWork.ExtCorpID},
			Name:         name,
		}
		tags = append(tags, tag)
	}
	err := o.internalTagRepo.CreateInBatches(tags)
	return tags, err
}
func (o InternalTag) Delete(tagIDs []string, extCorpID string) (int64, error) {
	return o.internalTagRepo.Delete(tagIDs, extCorpID)
}
func (o InternalTag) Query(req requests.QueryInternalTagReq, extCorpID string, sorter *app.Sorter, pager *app.Pager) ([]models.InternalTag, int64, error) {
	tag := models.InternalTag{
		ExtCorpModel: models.ExtCorpModel{ExtCorpID: conf.Settings.WeWork.ExtCorpID},
		ExtStaffID:   req.ExtStaffID,
	}
	return o.internalTagRepo.Query(tag, sorter, pager)
}
