package services

import (
	"core/app/constants"
	"core/app/dto"
	"core/app/models"
	"core/common/ecode"
)

type CustomerInfoDisplayRules struct {
	model models.CustomerInfoDisplayRule
}

func NewCustomerInfoDisplayRules() *CustomerInfoDisplayRules {
	return &CustomerInfoDisplayRules{model: models.CustomerInfoDisplayRule{}}
}

func (cs CustomerInfoDisplayRules) Get(extCorpID string) (*models.CustomerInfoDisplayRule, error) {
	rules := models.CustomerInfoDisplayRule{ExtCorpID: extCorpID}
	return cs.model.Get(&rules)
}

func (cs CustomerInfoDisplayRules) Update(extCorpID string, req *dto.UpdateCustomerInfoDisplayRulesReq) error {
	m := map[string]interface{}{}
	for _, filed := range req.DisplayFieldList {
		m[filed] = constants.True
	}
	for _, filed := range req.CancelDisplayFieldList {
		// 重复包含,参数校验
		if _, ok := m[filed]; ok {
			return ecode.InfoFieldDuplicateError
		}
		m[filed] = constants.False
	}
	return cs.model.Updates(extCorpID, m)
}
