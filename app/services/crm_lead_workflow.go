package services

import (
	"context"
	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
)

func mutableLead(lead models.CrmLead) error {
	if lead.ConvertCustomerID != "" || lead.Status == "qualified" {
		return errors.New("线索已转客户，请在客户中继续跟进")
	}
	if lead.Status == "lost" {
		return errors.New("请先恢复流失线索")
	}
	return nil
}
func validateLead(ctx context.Context, tx *gorm.DB, lead *models.CrmLead) error {
	lead.NextAction = strings.TrimSpace(lead.NextAction)
	if err := validateLeadAppointment(lead.NextFollowupAt, lead.NextAction); err != nil {
		return err
	}
	lead.Title, lead.Name, lead.Mobile = strings.TrimSpace(lead.Title), strings.TrimSpace(lead.Name), strings.TrimSpace(lead.Mobile)
	if lead.Title == "" {
		lead.Title = lead.Name
	}
	if lead.Title == "" {
		lead.Title = lead.Company
	}
	if lead.Title == "" && lead.Mobile == "" {
		return errors.New("请填写线索标题、联系人或手机号")
	}
	if err := validateCustomerOwner(ctx, tx, lead.OwnerUserID); err != nil {
		return err
	}
	if lead.CustomerID != "" {
		_, err := activeSalesCustomer(ctx, tx, lead.CustomerID)
		return err
	}
	return nil
}
func mutateLeads(ctx context.Context, ids []string, fn func(*gorm.DB, *models.CrmLead) error) (int64, error) {
	ids, err := workflowIDs(ids)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			lead, err := lockSalesRecord[models.CrmLead](ctx, tx, id, false)
			if err != nil {
				return err
			}
			if err = fn(tx, &lead); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}
func changeLeadStatus(ctx context.Context, userID, id, status, content string, reopen bool) error {
	if userID != identity.UserID(ctx) {
		return ErrCustomerAccess
	}
	_, err := mutateLeads(ctx, []string{id}, func(tx *gorm.DB, lead *models.CrmLead) error {
		if lead.PoolStatus == 1 {
			return errors.New("请先领取线索再跟进")
		}
		if lead.ConvertCustomerID != "" || lead.Status == "qualified" {
			return errors.New("线索已转客户")
		}
		if reopen {
			if lead.Status != "lost" {
				return errors.New("仅流失线索可恢复")
			}
		} else {
			if err := mutableLead(*lead); err != nil {
				return err
			}
			if lead.Status == status {
				return nil
			}
		}
		changes := map[string]any{"status": status}
		if status == "lost" {
			changes["next_followup_at"], changes["next_action"] = nil, ""
			lead.NextFollowupAt, lead.NextAction = nil, ""
		}
		if err := tx.Model(lead).Updates(changes).Error; err != nil {
			return err
		}
		f := models.CrmFollowup{Model: models.Model{ID: id_generator.StringID()}, TenantID: lead.TenantID, LeadID: lead.ID, CustomerID: lead.CustomerID, UserID: userID, Type: "other", Content: content, NextFollowupAt: lead.NextFollowupAt, NextAction: lead.NextAction}
		return createCRM(tx, &f)
	})
	return err
}
