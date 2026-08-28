package services

import (
	"context"
	"core/app/constants"
	"core/app/models"
	"core/common/identity"
	"encoding/json"
)

var wecomBusinessPermissions = func() map[string]string {
	result := map[string]string{}
	add := func(biz string, names ...string) {
		for _, name := range names {
			result[name] = biz
		}
	}
	add("BizCustomerInfo", "customer", "wecom_customer", "customer_event", "customer_info", "customer_info_display_rules", "clue_manual", "remainder")
	add("BizCustomerRemark", "customer_remark")
	add("BizCustomerTag", "tag", "tag_group", "internal_tag")
	add("BizStaffInfo", "staff", "wecom_staff")
	add("BizDepartment", "department")
	add("BizContactWay", "contact_way", "contact_way_group")
	add("BizRiskCenter", "corporation_risk_mgr")
	add("BizRole", "role")
	add("BizMediaMgr", "material_lib", "material_lib_tag", "material_lib_tag_frontend", "temp_material")
	add("BizMassMsg", "mass_msg")
	add("BizWelcomeMsg", "welcome_msg")
	add("BizQuickReply", "quick_reply", "quick_reply_frontend")
	add("BizQuickReplyGroup", "quick_reply_group", "quick_reply_group_frontend")
	add("BizCustomerGroupChat", "group_chat", "group_chat_group", "group_chat_auto_join", "group_chat_mass_msg", "group_chat_stats", "group_chat_tag", "group_chat_tag_group", "group_chat_welcome_msg")
	add("BizMsgArch", "msg_arch")
	add("BizCustomerInfo", "data_export")
	return result
}()

// Legacy channel sessions use actual PermissionIDs, never Role.Type or RoleType.
func ValidateWecomBusinessPermission(ctx context.Context, service, action, corpID, staffID string, params []byte) error {
	if identity.IsPlatform(ctx) {
		return nil
	}
	if identity.UserID(ctx) != "" {
		if service != "role" || readBusinessAction(action) {
			return nil
		}
		perms, err := crmActorPermissions(ctx, models.DB)
		if err != nil {
			return err
		}
		if HasCRMPermission(perms, "crm:*:full") {
			return nil
		}
		// Delegated channel-role operators also need the grants of their bound staff account.
	}
	biz, ok := wecomBusinessPermissions[service]
	if !ok {
		return ErrPermissionDenied
	}
	staff, err := (&models.Staff{}).Get(staffID, corpID, false)
	if err != nil {
		return ErrPermissionDenied
	}
	var role models.Role
	if err := models.DB.Where("id=? AND (ext_corp_id=? OR (rtrim(ext_corp_id::text)='' AND is_default=?))", staff.RoleID, corpID, constants.True).First(&role).Error; err != nil {
		return ErrPermissionDenied
	}
	has := func(point string) bool {
		for _, p := range role.PermissionIDs {
			if p == point {
				return true
			}
		}
		return false
	}
	if !has(biz+"_Full") && !(readBusinessAction(action) && has(biz+"_Read")) {
		return ErrPermissionDenied
	}
	if service != "role" || readBusinessAction(action) {
		return nil
	}
	var req struct {
		PermissionIDs []string `json:"permission_ids"`
		Req           struct {
			PermissionIDs []string `json:"permission_ids"`
		} `json:"req"`
		RoleID string `json:"role_id"`
		ID     string `json:"id"`
	}
	if json.Unmarshal(params, &req) != nil {
		return ErrPermissionDenied
	}
	grants := append(req.PermissionIDs, req.Req.PermissionIDs...)
	if action == "AssignToStaffs" || action == "Update" {
		id := req.RoleID
		if action == "Update" {
			id = req.ID
		}
		var target models.Role
		if err := models.DB.Where("id=? AND (ext_corp_id=? OR (rtrim(ext_corp_id::text)='' AND is_default=?))", id, corpID, constants.True).First(&target).Error; err != nil {
			return ErrPermissionDenied
		}
		grants = append(grants, target.PermissionIDs...)
	}
	for _, grant := range grants {
		if !has(grant) {
			return ErrPermissionDenied
		}
	}
	return nil
}
