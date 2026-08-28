package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func newPublicCode() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func validatePublicTenant(tx *gorm.DB, tenantID string, modules ...string) error {
	if err := models.RequirePlanModules(tx, tenantID, modules...); err != nil {
		return errors.New("当前页面暂不可用，请联系商家")
	}
	var tenant models.Tenant
	if err := tx.Where("id = ? AND status = 1", tenantID).First(&tenant).Error; err != nil || tenant.ExpireAt != nil && !tenant.ExpireAt.After(time.Now()) {
		return errors.New("当前页面暂不可用，请联系商家")
	}
	return nil
}

var publicPhone = regexp.MustCompile(`^\+?[0-9]{7,15}$`)
var publicSlug = regexp.MustCompile(`^[A-Za-z0-9_-]{4,64}$`)

func validateAcquisitionContact(name, mobile string) (string, string, error) {
	name, mobile = strings.TrimSpace(name), strings.TrimSpace(mobile)
	mobile = strings.NewReplacer(" ", "", "-", "", "(", "", ")", "").Replace(mobile)
	if name == "" || utf8.RuneCountInString(name) > 128 || !publicPhone.MatchString(mobile) {
		return "", "", errors.New("请填写姓名和有效联系电话")
	}
	return name, mobile, nil
}

// SubmitLead de-duplicates retries against an open lead for this card owner and phone.
// The caller must never receive existing customer or lead details from this public action.
func (s CrmSmartCardService) SubmitLead(shareCode, name, mobile, remark string) error {
	name, mobile, err := validateAcquisitionContact(name, mobile)
	if err != nil {
		return err
	}
	if utf8.RuneCountInString(remark) > 1024 {
		return errors.New("备注不能超过 1024 字")
	}
	card, err := s.GetByShareCode(shareCode)
	if err != nil {
		return errors.New("名片不存在或暂不可用")
	}
	ctx := identity.WithIdentity(context.Background(), identity.Identity{TenantID: card.TenantID, UserID: card.UserID})
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("tenant_id = ? AND id = ? AND status = ?", card.TenantID, card.ID, "active").First(&card).Error; err != nil {
			return err
		}
		if err := validatePublicTenant(tx, card.TenantID, "workbench", "lead"); err != nil {
			return err
		}
		var user models.User
		if err := tx.Where("tenant_id = ? AND id = ? AND status = 1", card.TenantID, card.UserID).First(&user).Error; err != nil {
			return errors.New("名片暂不可用")
		}
		if err := lockSalesKey(ctx, tx, "smart-card-lead", card.UserID+":"+mobile); err != nil {
			return err
		}
		var existing models.CrmLead
		err := tx.Where("tenant_id = ? AND owner_user_id = ? AND source = ? AND mobile = ? AND status IN ?", card.TenantID, card.UserID, "smart_card", mobile, []string{"new", "contacted"}).First(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		lead := models.CrmLead{Model: models.Model{ID: id_generator.StringID()}, TenantID: card.TenantID, OwnerUserID: card.UserID, Title: "名片留资-" + name, Name: name, Mobile: mobile, Remark: strings.TrimSpace(remark), Source: "smart_card", Status: "new"}
		return createCRM(tx, &lead)
	})
}
