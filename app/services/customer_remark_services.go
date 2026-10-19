package services

import (
	"core/app/dto"
	"core/app/models"
	"core/common/dbutil"
	"core/common/ecode"
	"core/common/id_generator"

	"github.com/pkg/errors"
)

type CustomerRemarkService struct {
	infoRepo     models.CustomerInfo
	remarkRepo   models.CustomerRemark
	optionRepo   models.RemarkOption
	displayRules models.CustomerInfoDisplayRule
}

func NewCustomerRemarkService() *CustomerRemarkService {
	return &CustomerRemarkService{
		remarkRepo:   models.CustomerRemark{},
		optionRepo:   models.RemarkOption{},
		infoRepo:     models.CustomerInfo{},
		displayRules: models.CustomerInfoDisplayRule{},
	}
}

type InfoRemark struct {
	DisplayRules *models.CustomerInfoDisplayRule `json:"display_rules"`
	Remark       []*models.CustomerRemark        `json:"remark"`
}

func (cs CustomerRemarkService) Get(extCorpID string) (InfoRemark, error) {
	remarks, err := cs.remarkRepo.Get(extCorpID)
	if err != nil {
		return InfoRemark{}, err
	}

	displayRules := &models.CustomerInfoDisplayRule{ExtCorpID: extCorpID}
	infoDisplayRules, err := cs.displayRules.Get(displayRules)
	if err != nil {
		return InfoRemark{}, err
	}

	return InfoRemark{DisplayRules: infoDisplayRules, Remark: remarks}, nil
}

func (cs CustomerRemarkService) Create(req *dto.AddCustomerRemarkReq, extCorpID string) (remark models.CustomerRemark, err error) {
	remark = models.CustomerRemark{
		ExtCorpModel: models.ExtCorpModel{ID: id_generator.StringID()},
		FieldType:    req.FieldType,
		Name:         req.FieldName,
	}
	options := make([]models.RemarkOption, 0)
	if req.FieldType == "option_text" && len(req.OptionNameList) != 0 {
		for _, optionName := range req.OptionNameList {
			options = append(options, models.RemarkOption{Model: models.Model{ID: id_generator.StringID()}, Name: optionName})
		}
	}
	remark.Options = options
	err = cs.remarkRepo.Create(remark)
	if dbutil.IsUniqueViolation(err) {
		err = ecode.DuplicateRemarkNameError
		return
	}

	return
}

func (cs CustomerRemarkService) Delete(ids []string, ExtCorpID string) error {
	return cs.remarkRepo.Delete(ids, ExtCorpID)
}

func (cs CustomerRemarkService) Update(req *dto.UpdateRemarkReq) (remark models.CustomerRemark, err error) {
	remark = models.CustomerRemark{ExtCorpModel: models.ExtCorpModel{ID: req.ID}, Name: req.Name}
	err = cs.remarkRepo.Update(remark)
	if err != nil {
		err = errors.WithStack(err)
		return
	}
	return
}

// --------------------------------------

func (cs CustomerRemarkService) AddRemarkOption(req *dto.AddRemarkOptionReq) error {
	return cs.optionRepo.Create(models.RemarkOption{
		Model:    models.Model{ID: id_generator.StringID()},
		RemarkID: req.RemarkID,
		Name:     req.Name,
	})
}

func (cs CustomerRemarkService) UpdateRemarkOption(req *dto.UpdateRemarkOptionReq) error {
	textOption := &models.RemarkOption{
		Model: models.Model{ID: req.RemarkOptionID},
		Name:  req.Name,
	}
	err := cs.optionRepo.Update(textOption)
	if err != nil {
		if dbutil.IsUniqueViolation(err) {
			return ecode.DuplicateRemarkNameError
		}
		return err
	}
	return nil
}

func (cs CustomerRemarkService) DeleteRemarkOption(ids []string) error {
	return cs.optionRepo.Delete(ids)
}

func (cs CustomerRemarkService) ExchangeOrder(req *dto.ExchangeOrderReq) error {
	return cs.remarkRepo.ExchangeOrder(req.ID, req.ExchangeOrderID)
}
