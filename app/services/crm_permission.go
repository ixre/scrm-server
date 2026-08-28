package services

import (
	"context"
	"errors"
	"strings"
	"unicode"

	"core/app/models"
	"core/common/identity"
	"gorm.io/gorm"
)

var ErrPermissionDenied = errors.New("没有此操作的权限")

// HasCRMPermission grants only persisted permission points. A role's name/code
// never grants privileges. Resource/full and explicit wildcard grants are supported.
func HasCRMPermission(perms []string, required string) bool {
	parts := strings.Split(required, ":")
	if len(parts) != 3 || parts[0] != "crm" {
		return false
	}
	for _, p := range perms {
		if p == required || p == "crm:*:full" || p == "crm:"+parts[1]+":full" || p == "crm:*:"+parts[2] {
			return true
		}
	}
	return false
}

func crmActorPermissions(ctx context.Context, db *gorm.DB) ([]string, error) {
	if identity.TenantID(ctx) == "" || identity.UserID(ctx) == "" {
		return nil, ErrPermissionDenied
	}
	var user models.User
	if err := db.WithContext(ctx).Where("tenant_id=? AND id=? AND status=1", identity.TenantID(ctx), identity.UserID(ctx)).First(&user).Error; err != nil {
		return nil, ErrPermissionDenied
	}
	var role models.CrmRole
	if user.RoleID == "" {
		return nil, ErrPermissionDenied
	}
	if err := db.WithContext(ctx).Where("tenant_id=? AND id=?", identity.TenantID(ctx), user.RoleID).First(&role).Error; err != nil {
		return nil, ErrPermissionDenied
	}
	return role.Perms, nil
}

func RequireCRMPermission(ctx context.Context, required string) error {
	perms, err := crmActorPermissions(ctx, models.DB)
	if err != nil {
		return err
	}
	if !HasCRMPermission(perms, required) {
		return ErrPermissionDenied
	}
	return nil
}

// Creating an account or changing its role must never grant powers that the
// operator does not have, even when the operator can manage accounts.
func requireAssignableRole(ctx context.Context, roleID string) error {
	perms, err := crmActorPermissions(ctx, models.DB)
	if err != nil {
		return err
	}
	var role models.CrmRole
	if err := models.MustFor(ctx).Where("id=?", roleID).First(&role).Error; err != nil {
		return errors.New("角色不存在")
	}
	for _, granted := range role.Perms {
		if !HasCRMPermission(perms, granted) {
			return errors.New("不能分配超出自身权限的角色")
		}
	}
	return nil
}

func HasCRMDataAccess(ctx context.Context) bool {
	perms, err := crmActorPermissions(ctx, models.DB)
	return err == nil && HasCRMPermission(perms, "crm:data:all")
}

type businessPermissionKey struct{}

func WithBusinessPermission(ctx context.Context, service, action string) context.Context {
	return context.WithValue(ctx, businessPermissionKey{}, businessPermissionResource(service))
}

func RequireBusinessManagement(ctx context.Context) error {
	resource, _ := ctx.Value(businessPermissionKey{}).(string)
	perms, err := crmActorPermissions(ctx, models.DB)
	if err != nil {
		return err
	}
	if HasCRMPermission(perms, "crm:manage:full") || resource != "" && HasCRMPermission(perms, "crm:"+resource+":manage") {
		return nil
	}
	return ErrPermissionDenied
}

func permissionAction(action string) string {
	var b strings.Builder
	for i, r := range action {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func readBusinessAction(action string) bool {
	if action == "RoleOptions" || action == "Logs" {
		return true
	}
	for _, prefix := range []string{"Get", "List", "Query", "Options", "Stats", "Stat", "Summary", "History", "Export", "Preview", "Search", "Today", "Never", "Top", "Pending", "GenerateNo", "Sales", "Pipeline", "Channel", "Briefing", "Trend", "Ranking", "Achievement"} {
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	return false
}

// Module entitlement and operation permission are independent checks.
func ValidateBusinessPermission(ctx context.Context, service, action string) error {
	if identity.IsPlatform(ctx) {
		return nil
	} // platform APIs retain their separate platform-account policy
	if service == "crm_auth" || service == "crm_tenant_order" || service == "crm_plan" && action == "List" || service == "crm_tenant" && action == "GetCurrentInfo" || service == "crm_user" && (action == "GetCurrent" || action == "UpdateProfile" || action == "Options") || service == "crm_storage" {
		return nil
	}
	if service == "login" || service == "util" || service == "permission" || service == "js_api" {
		return nil
	}
	if !strings.HasPrefix(service, "crm_") && service != "internal_crm_attachment" && identity.UserID(ctx) == "" {
		return nil
	}
	if service == "role" && !readBusinessAction(action) {
		return RequireCRMPermission(ctx, "crm:role:manage")
	}
	resource := businessPermissionResource(service)
	if service == "internal_crm_attachment" {
		resource = "order_attachment"
	}
	perms, err := crmActorPermissions(ctx, models.DB)
	if err != nil {
		return err
	}
	if (service == "crm_lead" || service == "crm_contact") && action == "ConvertToCustomer" {
		if !HasCRMPermission(perms, "crm:customer:create") && !HasCRMPermission(perms, "crm:customer:write") {
			return ErrPermissionDenied
		}
	}
	if service == "crm_quote" && action == "ConvertToOrder" {
		if !HasCRMPermission(perms, "crm:order:create") && !HasCRMPermission(perms, "crm:order:write") {
			return ErrPermissionDenied
		}
	}
	point := "crm:" + resource + ":" + permissionAction(action)
	mode := "write"
	if readBusinessAction(action) {
		mode = "read"
	}
	if HasCRMPermission(perms, point) || HasCRMPermission(perms, "crm:"+resource+":"+mode) {
		return nil
	}
	return ErrPermissionDenied
}

func requireManageableUser(ctx context.Context, userID string) error {
	var user models.User
	if err := models.MustFor(ctx).Where("id=?", userID).First(&user).Error; err != nil {
		return ErrPermissionDenied
	}
	if user.RoleID == "" {
		return nil
	}
	return requireAssignableRole(ctx, user.RoleID)
}

func businessPermissionResource(service string) string {
	switch service {
	case "crm_sensitive_alert", "crm_sensitive_word", "crm_reply_timeout":
		return "risk"
	case "internal_crm_attachment":
		return "order_attachment"
	}
	return strings.TrimPrefix(service, "crm_")
}
