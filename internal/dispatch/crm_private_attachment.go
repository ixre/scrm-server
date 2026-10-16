package dispatch

import (
	"context"
	"core/app/services"
	"encoding/json"
)

func init() {
	// The gateway denies the entire internal_ namespace in its generic HTTP dispatcher.
	Reg.Register("internal_crm_attachment", "AuthorizeUpload", privateAttachmentAuthorize)
	Reg.Register("internal_crm_attachment", "RegisterUpload", privateAttachmentRegister)
	Reg.Register("internal_crm_attachment", "ResolveDownload", privateAttachmentDownload)
}
func privateAttachmentAuthorize(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		OrderID  string `json:"order_id"`
		UploadID string `json:"upload_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderAttachment().AuthorizeUpload(ctx, req.OrderID, req.UploadID)
}
func privateAttachmentRegister(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req services.PrivateAttachmentUpload
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOrderAttachment().RegisterUpload(ctx, req)
}
func privateAttachmentDownload(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	id, err := bindActionID(params)
	if err != nil {
		return nil, err
	}
	return services.NewCrmOrderAttachment().ResolveDownload(ctx, id)
}
