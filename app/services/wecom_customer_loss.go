package services

import (
	"context"
	"core/app/models"
	"core/common/id_generator"
	"errors"
	"gopkg.in/guregu/null.v4"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

func RemoveWecomCustomerRelation(ctx context.Context, corpID, staffID, customerID string, customerDeletedStaff bool) error {
	if corpID == "" || staffID == "" || customerID == "" {
		return errors.New("企微客户关系身份缺失")
	}
	if err := models.DB.Transaction(func(tx *gorm.DB) error {
		var relation models.CustomerStaff
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("ext_corp_id=? AND ext_staff_id=? AND ext_customer_id=?", corpID, staffID, customerID).Take(&relation).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		history := models.CustomerStaffRelationHistory{ExtCorpModel: models.ExtCorpModel{ID: id_generator.StringID(), ExtCorpID: corpID, ExtCreatorID: staffID}, ExtStaffID: staffID, ExtCustomerID: customerID, Createtime: relation.Createtime, StaffDeleteCustomerAt: null.TimeFrom(time.Now())}
		if customerDeletedStaff {
			history.StaffDeleteCustomerAt = null.Time{}
			history.CustomerDeleteStaffAt = null.TimeFrom(time.Now())
		}
		if err := tx.Create(&history).Error; err != nil {
			return err
		}
		return tx.Where("id=? AND ext_corp_id=?", relation.ID, corpID).Delete(&models.CustomerStaff{}).Error
	}); err != nil {
		return err
	}
	var remaining int64
	if err := models.DB.Model(&models.CustomerStaff{}).Where("ext_corp_id=? AND ext_customer_id=?", corpID, customerID).Count(&remaining).Error; err != nil {
		return err
	}
	if remaining == 0 {
		return NewCrmCustomer().MarkWecomCustomerLost(ctx, corpID, customerID)
	}
	return nil
}
