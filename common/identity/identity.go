// Package identity 通过 context 携带"本地身份"（租户 + 本地用户），
// 与企微身份（corp_id / staff_id，仍作为 Adapter 的入参透传）解耦。
//
// 背景：CRM 优先化要求客户/员工/销售管线都归到本地 ID，而非企微 ID。
// 为避免改 130+ 个既有 Adapter 的签名，本地身份走 context 传递：
//   - BusinessServer.Invoke 从 BusinessRequest 读取 tenant_id / user_id，注入 ctx；
//   - 新增 CRM Adapter 用 TenantID(ctx) / UserID(ctx) 取本地身份；
//   - 既有企微 Adapter 继续用 corpID / staffID 入参，不受影响。
package identity

import "context"

type identityKey struct{}

// Identity 本地身份。企微 corp_id / staff_id 仅是可选渠道上下文，不入此结构。
// IsPlatform/PlatformID 标记平台超管身份（独立于租户，用于 crm_tenant 等平台管理 API）。
type Identity struct {
	TenantID   string // 本地租户（企业/组织）主键
	UserID     string // 本地用户（销售账号）主键
	IsPlatform bool   // 是否平台超管
	PlatformID string // 平台超管主键
}

// WithIdentity 把本地身份注入 context。
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// FromContext 从 context 读取本地身份；未注入时返回零值。
func FromContext(ctx context.Context) Identity {
	id, _ := ctx.Value(identityKey{}).(Identity)
	return id
}

// TenantID 从 context 读取租户 ID；为空字符串表示未注入。
func TenantID(ctx context.Context) string { return FromContext(ctx).TenantID }

// UserID 从 context 读取本地用户 ID；为空字符串表示未注入。
func UserID(ctx context.Context) string { return FromContext(ctx).UserID }

// IsPlatform 从 context 读取是否平台超管。
func IsPlatform(ctx context.Context) bool { return FromContext(ctx).IsPlatform }

// PlatformID 从 context 读取平台超管 ID；为空字符串表示未注入。
func PlatformID(ctx context.Context) string { return FromContext(ctx).PlatformID }
