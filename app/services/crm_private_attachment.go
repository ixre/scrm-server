package services

import (
	"context"
	"errors"
	"fmt"
	"path"
	"regexp"
	"strings"
	"unicode/utf8"

	"core/app/models"
	"core/common/id_generator"
	"core/common/identity"
	"gorm.io/gorm"
)

const MaxContractAttachmentSize int64 = 32 << 20

var attachmentUploadID = regexp.MustCompile(`^[a-f0-9]{32}$`)
var attachmentDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)

// PrivateAttachmentUpload is constructed by the trusted file gateway, never by a public business route.
type PrivateAttachmentUpload struct {
	OrderID  string `json:"order_id"`
	UploadID string `json:"upload_id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	SHA256   string `json:"sha256"`
}

func (s CrmOrderAttachmentService) AuthorizeUpload(ctx context.Context, orderID string, uploadIDs ...string) error {
	return models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockPaymentOrder(ctx, tx, orderID)
		if err != nil {
			return err
		}
		if len(uploadIDs) > 0 && attachmentUploadID.MatchString(uploadIDs[0]) {
			var existing models.CrmOrderAttachment
			err = tx.Unscoped().Where("tenant_id = ? AND storage_key = ?", order.TenantID, fmt.Sprintf("%s/%s/%s", order.TenantID, order.ID, uploadIDs[0])).First(&existing).Error
			if err == nil {
				if existing.DeletedAt.Valid || existing.UploadedBy != identity.UserID(ctx) {
					return errors.New("上传标识不可复用，请重新选择文件")
				}
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if order.Status != "draft" && order.Status != "signed" {
			return errors.New("已完成或作废的合同不可新增附件")
		}
		return nil
	})
}

// RegisterUpload is idempotent for an immutable storage key, including retries after a lost response.
func (s CrmOrderAttachmentService) RegisterUpload(ctx context.Context, req PrivateAttachmentUpload) (models.CrmOrderAttachment, error) {
	var attachment models.CrmOrderAttachment
	req.FileName = strings.TrimSpace(req.FileName)
	if !attachmentUploadID.MatchString(req.UploadID) || !attachmentDigest.MatchString(req.SHA256) || req.FileSize <= 0 || req.FileSize > MaxContractAttachmentSize || req.FileName == "" || utf8.RuneCountInString(req.FileName) > 255 || strings.ContainsAny(req.FileName, "/\\\r\n\x00") {
		return attachment, errors.New("附件信息无效，请重新选择文件上传")
	}
	key := fmt.Sprintf("%s/%s/%s", identity.TenantID(ctx), req.OrderID, req.UploadID)
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		order, err := lockPaymentOrder(ctx, tx, req.OrderID)
		if err != nil {
			return err
		}
		// Check an existing upload before state restrictions: a completed retry does not mutate a signed or closed contract.
		err = tx.Unscoped().Where("tenant_id = ? AND storage_key = ?", order.TenantID, key).First(&attachment).Error
		if err == nil {
			if attachment.DeletedAt.Valid {
				return errors.New("附件已删除，请重新选择文件")
			}
			if attachment.OrderID != order.ID || attachment.UploadedBy != identity.UserID(ctx) || attachment.SHA256 != req.SHA256 || attachment.FileSize != req.FileSize || attachment.FileName != req.FileName {
				return errors.New("上传标识已被使用，请重新选择文件")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if order.Status != "draft" && order.Status != "signed" {
			return errors.New("已完成或作废的合同不可新增附件")
		}
		attachment = models.CrmOrderAttachment{Model: models.Model{ID: id_generator.StringID()}, TenantID: order.TenantID, OrderID: order.ID, FileName: req.FileName, FileSize: req.FileSize, FileType: strings.TrimPrefix(strings.ToLower(path.Ext(req.FileName)), "."), UploadedBy: identity.UserID(ctx), StorageKey: key, SHA256: req.SHA256}
		return createCRM(tx, &attachment)
	})
	return attachment, err
}

// ResolveDownload checks current membership and order ownership on every request. No expiring public URL is issued.
func (s CrmOrderAttachmentService) ResolveDownload(ctx context.Context, id string) (models.CrmOrderAttachment, error) {
	var attachment models.CrmOrderAttachment
	if err := models.MustFor(ctx).Where("id = ?", id).First(&attachment).Error; err != nil {
		return attachment, errors.New("附件不存在或已删除")
	}
	if _, err := NewCrmOrder().Get(ctx, attachment.OrderID); err != nil {
		return models.CrmOrderAttachment{}, ErrCustomerAccess
	}
	if attachment.StorageKey == "" {
		return models.CrmOrderAttachment{}, errors.New("此附件为历史外部链接，请使用原链接或重新上传")
	}
	return attachment, nil
}
