package services

import (
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/log"

	"gorm.io/gorm"
)

// CrmQuoteExtService 报价扩展服务（单号生成 / 作废 / 转移）。
type CrmQuoteExtService struct{}

func NewCrmQuoteExt() *CrmQuoteExtService { return &CrmQuoteExtService{} }

// GenerateNo 生成报价单号：Q + yyyyMMdd + 4位当日序号（租户内当日递增）。
func (s CrmQuoteExtService) GenerateNo(tenantID string) (string, error) {
	day := time.Now().Format("20060102")
	prefix := "Q" + day
	var cnt int64
	if err := models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("quote_no LIKE ?",  prefix+"%").Count(&cnt).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%04d", prefix, cnt+1), nil
}

// Void 报价作废：Status=void, VoidReason, VoidAt=now。
func (s CrmQuoteExtService) Void(tenantID, id, reason string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	now := time.Now()
	res := models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("id = ? AND status <> ?", id,  "void").
		Updates(map[string]any{"status": "void", "void_reason": reason, "void_at": now})
	if res.Error != nil {
		log.Sugar.Errorw("quote void failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("报价单不存在或已作废")
	}
	return nil
}

// Transfer 报价转移：改 OwnerUserID。
func (s CrmQuoteExtService) Transfer(tenantID, id, toUserID string) error {
	if id == "" {
		return errors.New("id 不能为空")
	}
	if toUserID == "" {
		return errors.New("to_user_id 不能为空")
	}
	res := models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).Update("owner_user_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("quote transfer failed", "err", res.Error)
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// BatchTransfer 批量转移报价。
func (s CrmQuoteExtService) BatchTransfer(tenantID string, ids []string, toUserID string) error {
	if len(ids) == 0 {
		return errors.New("ids 不能为空")
	}
	if toUserID == "" {
		return errors.New("to_user_id 不能为空")
	}
	res := models.DB.Model(&models.CrmQuote{}).Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Update("owner_user_id", toUserID)
	if res.Error != nil {
		log.Sugar.Errorw("quote batch transfer failed", "err", res.Error)
	}
	return res.Error
}
