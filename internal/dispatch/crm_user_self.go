package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
	"core/common/identity"
)

// CRM 租户员工「自服务」dispatch：任何已登录 CRM 本地用户都能调用，
// 身份由 ctx 中的 user_id/tenant_id 决定，不要求 admin 角色。
// 单独成文件，避免与 admin-only 的 crm_user/* 混在一起。

func init() {
	Reg.Register("crm_user", "GetCurrent", crmUserGetCurrent)
	Reg.Register("crm_user", "UpdateProfile", crmUserUpdateProfile)
	// 头像直传：返回 OSS/COS 预签 PUT URL，前端拿到后再调 UpdateProfile 落库。
	Reg.Register("crm_storage", "GetAvatarUploadURL", crmStorageGetAvatarUploadURL)
}

// crmUserGetCurrent 返回 ctx 中本地 user 的完整 User 行，供 header/个人中心展示。
func crmUserGetCurrent(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	_ = identity.UserID(ctx) // ensure identity injection
	return services.NewUser().GetCurrent(ctx)
}

// crmUserUpdateProfile 自助改 name/mobile/email/position/avatar。
func crmUserUpdateProfile(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req services.UserUpdateProfileReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewUser().UpdateProfile(ctx, req)
}

// crmStorageGetAvatarUploadURL 返回头像直传 OSS/COS 的预签 PUT URL。
func crmStorageGetAvatarUploadURL(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		MimeType string `json:"mime_type"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewUserStorage().GetAvatarUploadURL(ctx, req.MimeType)
}