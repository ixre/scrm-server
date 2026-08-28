package services

import (
	"context"
	"errors"
	"strings"

	"core/app/models"
	"core/common/identity"
)

// Every business service has an explicit policy. A new service must be assigned
// an entitlement before it can be exposed; aliases share the same policy.
var businessPlanModules = func() map[string][]string {
	policies := map[string][]string{}
	add := func(module, names string) {
		for _, name := range strings.Fields(names) {
			if module == "" {
				policies[name] = nil
			} else {
				policies[name] = []string{module}
			}
		}
	}
	add("", "crm_auth crm_platform crm_platform_admin crm_platform_stats crm_platform_log crm_tenant crm_plan crm_payment crm_tenant_order crm_storage crm_dict crm_reason_dict crm_industry_dict login util permission")
	add("workbench", "crm_dashboard crm_todo crm_notice crm_smart_card")
	add("customer", "crm_customer crm_contact crm_customer_tag crm_customer_setting crm_customer_loss crm_pool crm_seas_type crm_inheritance crm_dedup crm_auto_allocation crm_auto_tag crm_score crm_segment crm_care crm_scene")
	add("lead", "crm_lead crm_lead_source")
	add("opportunity", "crm_opportunity crm_pipeline crm_sales_rule crm_quota crm_quote")
	add("followup", "crm_followup crm_followup_comment crm_quick_followup crm_quick_remark crm_followup_view crm_script crm_sop crm_call_list crm_call_record")
	add("order", "crm_order crm_order_product crm_order_attachment crm_order_progress internal_crm_attachment crm_product crm_project crm_project_task crm_project_event crm_project_file crm_aftersales crm_aftersales_type crm_aftersales_comment crm_workorder crm_workorder_type crm_workorder_pool crm_workorder_log crm_supplier crm_purchase_order crm_warehouse crm_stock_in crm_stock_out crm_stock_check crm_stock_transfer crm_stock_record")
	add("receivable", "crm_receivables crm_receivables_plan crm_finance_record crm_invoice crm_account crm_fund_transfer crm_other_income crm_other_expense crm_income_category")
	add("report", "crm_report crm_stats crm_followup_stats crm_ai_analysis")
	add("company_mgmt", "crm_attendance crm_signin crm_sign_in crm_leave crm_work_report crm_work_report_comment crm_approval crm_approval_flow crm_knowledge crm_knowledge_type")
	add("staff_mgmt", "crm_user")
	add("channel_wecom", "crm_wecom crm_wecom_event_log js_api wecom_customer wecom_staff customer customer_event customer_info customer_info_display_rules customer_remark clue_manual remainder staff department contact_way contact_way_group internal_tag tag tag_group material_lib material_lib_tag material_lib_tag_frontend temp_material mass_msg welcome_msg quick_reply quick_reply_frontend quick_reply_group quick_reply_group_frontend group_chat group_chat_group group_chat_tag group_chat_tag_group group_chat_mass_msg group_chat_welcome_msg group_chat_auto_join group_chat_stats data_export")
	add("role_perm", "role crm_role")
	add("risk_control", "corporation_risk_mgr crm_sensitive_alert crm_reply_timeout")
	add("sensitive_word", "crm_sensitive_word")
	add("msg_archive", "msg_arch")
	add("marketing_coupon", "crm_coupon")
	add("marketing_fission", "crm_fission")
	add("marketing_landing", "crm_landing_page")
	add("marketing_lottery", "crm_lottery")
	add("marketing_bargain", "crm_bargain")
	add("marketing_groupbuy", "crm_group_buy")
	add("marketing_groupsop", "crm_group_sop")
	add("marketing_ext", "crm_activity crm_red_packet crm_seckill crm_full_reduction crm_consume")
	add("", "crm_batch crm_recycle crm_field_def crm_field_value")
	add("customer", "crm_channel_tag crm_channel_interaction")
	add("channel_wecom", "crm_channel_task")
	add("", "crm_channel_config") // selected channel checked in the service
	return policies
}()

func BusinessPlanModules(service, action string) ([]string, bool) {
	if service == "crm_user" && (action == "GetCurrent" || action == "UpdateProfile" || action == "Options") {
		return nil, true
	}
	if service == "crm_order" && (action == "ListAll" || action == "ManualConfirmPay") {
		return nil, true
	}
	modules, ok := businessPlanModules[service]
	if service == "crm_lead" && action == "ConvertToCustomer" {
		modules = []string{"lead", "customer"}
	}
	if service == "crm_quote" && action == "ConvertToOrder" {
		modules = []string{"opportunity", "order"}
	}
	if ok && (service == "msg_arch" || service == "role" || service == "corporation_risk_mgr") {
		modules = append(append([]string{}, modules...), "channel_wecom")
	}
	if service == "crm_user" && (action == "BindWecom" || action == "UnbindWecom") {
		modules = append(append([]string{}, modules...), "channel_wecom")
	}
	return modules, ok
}

func ValidateBusinessEntitlement(ctx context.Context, service, action string) error {
	if identity.IsPlatform(ctx) {
		return nil
	} // adapters still enforce platform roles
	if service == "crm_batch" || service == "crm_recycle" || service == "crm_field_def" || service == "crm_field_value" {
		_, err := models.TenantEntitlement(models.DB.WithContext(ctx), identity.TenantID(ctx))
		return err
	}
	if service == "crm_channel_config" {
		rights, err := models.TenantEntitlement(models.DB.WithContext(ctx), identity.TenantID(ctx))
		if err != nil {
			return err
		}
		for _, module := range rights.Modules {
			if strings.HasPrefix(module, "channel_") {
				return nil
			}
		}
		return errors.New("当前套餐未包含渠道接入，请升级套餐")
	}
	modules, ok := BusinessPlanModules(service, action)
	if !ok {
		return errors.New("业务未配置套餐权益")
	}
	if len(modules) == 0 {
		return nil
	}
	return models.RequirePlanModules(models.DB.WithContext(ctx), identity.TenantID(ctx), modules...)
}

func requireChannelEntitlement(ctx context.Context, channel string, archive bool) error {
	if channel == "" {
		channel = "wecom"
	}
	if channel == "xiaohongshu" {
		channel = "xhs"
	}
	modules := []string{"channel_" + channel}
	if archive {
		modules = append(modules, "msg_archive")
	}
	return models.RequirePlanModules(models.DB.WithContext(ctx), identity.TenantID(ctx), modules...)
}
