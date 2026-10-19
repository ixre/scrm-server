package services

import (
	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"core/app/dto"
	"core/app/models"
	"core/common/id_generator"
)

type CustomerInfo struct {
	customerInfoRepo models.CustomerInfo
}

func (ci CustomerInfo) Get(req dto.GetCustomerInfoReq, extCorpID string) (res models.CustomerInfo, err error) {
	info := models.CustomerInfo{
		ExtCorpModel:  models.ExtCorpModel{ExtCorpID: extCorpID},
		ExtCustomerID: req.ExtCustomerID,
		ExtStaffID:    req.ExtStaffID,
	}
	res, err = ci.customerInfoRepo.Get(info)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

func (ci CustomerInfo) Update(req *dto.UpdateCustomerInfoReq, extCorpID string) error {
	info := models.CustomerInfo{ExtCorpModel: models.ExtCorpModel{ID: id_generator.StringID(), ExtCorpID: extCorpID}}
	err := copier.CopyWithOption(&info, req, copier.Option{IgnoreEmpty: true, DeepCopy: true})
	if err != nil {
		return err
	}

	return ci.customerInfoRepo.Update(info)
}

func NewCustomerInfo() *CustomerInfo {
	return &CustomerInfo{customerInfoRepo: models.CustomerInfo{}}
}
