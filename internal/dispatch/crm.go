package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"core/app/constants"
	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// errNeedIdentity 本地身份未注入时的错误（CRM 域要求必须带 tenant_id）。
var errNeedIdentity = errors.New("missing local identity: tenant_id required (CRM is channel-independent, inject local identity via gateway)")

// CRM 原生域适配器 —— 渠道无关，走本地身份（tenant_id/user_id，来自 context）。
// 服务 key 前缀 crm_，避免与既有企微域（customer/staff/...）冲突。

func init() {
	// tenant（平台超管管理租户，需 requirePlatformAdmin）
	Reg.Register("crm_tenant", "Create", crmTenantCreate)
	Reg.Register("crm_tenant", "List", crmTenantList)
	Reg.Register("crm_tenant", "ListPaged", crmTenantListPaged)
	Reg.Register("crm_tenant", "GetDetail", crmTenantGetDetail)
	Reg.Register("crm_tenant", "GetCurrentInfo", crmTenantGetCurrentInfo)
	Reg.Register("crm_tenant", "ListUsers", crmTenantListUsers)
	Reg.Register("crm_tenant", "ResetAdminPassword", crmTenantResetAdminPassword)
	Reg.Register("crm_tenant", "Renew", crmTenantRenew)
	Reg.Register("crm_tenant", "Update", crmTenantUpdate)
	Reg.Register("crm_tenant", "SetStatus", crmTenantSetStatus)
	Reg.Register("crm_tenant", "Delete", crmTenantDelete)
	// 套餐管理
	Reg.Register("crm_plan", "Create", crmPlanCreate)
	Reg.Register("crm_plan", "List", crmPlanList)
	Reg.Register("crm_plan", "Update", crmPlanUpdate)
	Reg.Register("crm_plan", "SetStatus", crmPlanSetStatus)
	Reg.Register("crm_plan", "Delete", crmPlanDelete)
	Reg.Register("crm_plan", "ListModules", crmPlanListModules)
	Reg.Register("crm_plan", "Get", crmPlanGet)
	// 支付配置管理（平台端）
	Reg.Register("crm_payment", "Create", crmPaymentCreate)
	Reg.Register("crm_payment", "List", crmPaymentList)
	Reg.Register("crm_payment", "Update", crmPaymentUpdate)
	Reg.Register("crm_payment", "SetStatus", crmPaymentSetStatus)
	Reg.Register("crm_payment", "Delete", crmPaymentDelete)
	// 订单管理（平台端）
	Reg.Register("crm_order", "ListAll", crmOrderListAll)
	Reg.Register("crm_order", "ManualConfirmPay", crmOrderManualConfirmPay)
	// 订单（租户端）
	Reg.Register("crm_tenant_order", "Create", crmTenantOrderCreate)
	Reg.Register("crm_tenant_order", "List", crmTenantOrderList)
	Reg.Register("crm_tenant_order", "Get", crmTenantOrderGet)
	Reg.Register("crm_tenant_order", "QueryPayStatus", crmTenantOrderQueryPayStatus)
	Reg.Register("crm_tenant_order", "GetQrCode", crmTenantOrderGetQrCode)
	Reg.Register("crm_tenant_order", "Cancel", crmTenantOrderCancel)
	Reg.Register("crm_tenant_order", "ListPaymentMethods", crmTenantOrderListPaymentMethods)
	Reg.Register("crm_user", "Create", crmUserCreate)
	Reg.Register("crm_user", "Query", crmUserQuery)
	Reg.Register("crm_user", "SetPassword", crmUserSetPassword)
	Reg.Register("crm_user", "Update", crmUserUpdate)
	Reg.Register("crm_user", "BindWecom", crmUserBindWecom)
	Reg.Register("crm_user", "UnbindWecom", crmUserUnbindWecom)

	// auth（本地账号登录，SaaS 多租户）
	Reg.Register("crm_auth", "Login", crmAuthLogin)
	Reg.Register("crm_auth", "ChangePassword", crmAuthChangePassword)
	// platform auth（平台超管登录，独立于租户）
	Reg.Register("crm_platform", "Login", crmPlatformLogin)
	// 平台账号管理（super_admin 专属）
	Reg.Register("crm_platform_admin", "List", crmPlatformAdminList)
	Reg.Register("crm_platform_admin", "Create", crmPlatformAdminCreate)
	Reg.Register("crm_platform_admin", "Update", crmPlatformAdminUpdate)
	Reg.Register("crm_platform_admin", "ResetPassword", crmPlatformAdminResetPassword)
	Reg.Register("crm_platform_admin", "SetStatus", crmPlatformAdminSetStatus)
	Reg.Register("crm_platform_admin", "Delete", crmPlatformAdminDelete)
	// 平台统计
	Reg.Register("crm_platform_stats", "Overview", crmPlatformStatsOverview)
	Reg.Register("crm_platform_stats", "TenantUsage", crmPlatformStatsTenantUsage)
	Reg.Register("crm_platform_stats", "AllTenantUsage", crmPlatformStatsAllTenantUsage)
	// 平台操作日志
	Reg.Register("crm_platform_log", "List", crmPlatformLogList)

	// customer（本地客户）
	Reg.Register("crm_customer", "Create", crmCustomerCreate)
	Reg.Register("crm_customer", "Update", crmCustomerUpdate)
	Reg.Register("crm_customer", "Delete", crmCustomerDelete)
	Reg.Register("crm_customer", "Get", crmCustomerGet)
	Reg.Register("crm_customer", "GetDetail", crmCustomerGetDetail)
	Reg.Register("crm_customer", "Query", crmCustomerQuery)

	// lead
	Reg.Register("crm_lead", "Create", crmLeadCreate)
	Reg.Register("crm_lead", "Update", crmLeadUpdate)
	Reg.Register("crm_lead", "Delete", crmLeadDelete)
	Reg.Register("crm_lead", "Get", crmLeadGet)
	Reg.Register("crm_lead", "Query", crmLeadQuery)
	Reg.Register("crm_lead", "BatchImport", crmLeadBatchImport)
	Reg.Register("crm_lead", "Export", crmLeadExport)
	Reg.Register("crm_lead", "BatchTransfer", crmLeadBatchTransfer)

	// pipeline stage
	Reg.Register("crm_pipeline", "Create", crmStageCreate)
	Reg.Register("crm_pipeline", "Update", crmStageUpdate)
	Reg.Register("crm_pipeline", "Delete", crmStageDelete)
	Reg.Register("crm_pipeline", "List", crmStageList)

	// opportunity
	Reg.Register("crm_opportunity", "Create", crmOpportunityCreate)
	Reg.Register("crm_opportunity", "Update", crmOpportunityUpdate)
	Reg.Register("crm_opportunity", "Delete", crmOpportunityDelete)
	Reg.Register("crm_opportunity", "Get", crmOpportunityGet)
	Reg.Register("crm_opportunity", "Query", crmOpportunityQuery)
	Reg.Register("crm_opportunity", "MoveStage", crmOpportunityMoveStage)

	// followup
	Reg.Register("crm_followup", "Create", crmFollowupCreate)
	Reg.Register("crm_followup", "Delete", crmFollowupDelete)
	Reg.Register("crm_followup", "ListByCustomer", crmFollowupListByCustomer)
	Reg.Register("crm_followup", "ListByOpportunity", crmFollowupListByOpportunity)
	Reg.Register("crm_followup", "ListByLead", crmFollowupListByLead)

	// order
	Reg.Register("crm_order", "Create", crmOrderCreate)
	Reg.Register("crm_order", "Update", crmOrderUpdate)
	Reg.Register("crm_order", "Delete", crmOrderDelete)
	Reg.Register("crm_order", "Query", crmOrderQuery)

	// report
	Reg.Register("crm_report", "SalesSummary", crmReportSalesSummary)
	Reg.Register("crm_report", "PipelineFunnel", crmReportPipelineFunnel)
	Reg.Register("crm_report", "PipelineInspection", crmReportPipelineInspection)
	Reg.Register("crm_report", "ChannelSummary", crmReportChannelSummary)

	// activity（营销活动）
	Reg.Register("crm_activity", "Create", crmActivityCreate)
	Reg.Register("crm_activity", "Update", crmActivityUpdate)
	Reg.Register("crm_activity", "Delete", crmActivityDelete)
	Reg.Register("crm_activity", "Get", crmActivityGet)
	Reg.Register("crm_activity", "Query", crmActivityQuery)
	Reg.Register("crm_activity", "SetStatus", crmActivitySetStatus)
	Reg.Register("crm_activity", "Record", crmActivityRecord)
	Reg.Register("crm_activity", "DeleteRecord", crmActivityDeleteRecord)
	Reg.Register("crm_activity", "ListRecords", crmActivityListRecords)
	Reg.Register("crm_activity", "ListByCustomer", crmActivityListByCustomer)

	// channel interaction（渠道互动，非企微渠道行为数据）
	Reg.Register("crm_channel_interaction", "Create", crmChannelInteractionCreate)
	Reg.Register("crm_channel_interaction", "Delete", crmChannelInteractionDelete)
	Reg.Register("crm_channel_interaction", "Query", crmChannelInteractionQuery)

	// channel tag（渠道标签目录）
	Reg.Register("crm_channel_tag", "List", crmChannelTagList)
	Reg.Register("crm_channel_tag", "Create", crmChannelTagCreate)
	Reg.Register("crm_channel_tag", "Delete", crmChannelTagDelete)

	// channel config（渠道接入配置，一租户一企微）
	Reg.Register("crm_channel_config", "Create", crmChannelConfigCreate)
	Reg.Register("crm_channel_config", "Update", crmChannelConfigUpdate)
	Reg.Register("crm_channel_config", "Query", crmChannelConfigQuery)
	Reg.Register("crm_channel_config", "Delete", crmChannelConfigDelete)

	// 企微回调事件日志（排障）
	Reg.Register("crm_wecom_event_log", "Query", crmWecomEventLogQuery)

	// 字典（渠道枚举/来源定义等，后台下发，前端不硬编码）
	Reg.Register("crm_dict", "AddWay", crmDictAddWay)

	// ---- Phase 1 销售域深化 ----
	// 公海池
	Reg.Register("crm_pool", "List", crmPoolList)
	Reg.Register("crm_pool", "Claim", crmPoolClaim)
	Reg.Register("crm_pool", "Recycle", crmPoolRecycle)
	Reg.Register("crm_pool", "GetRule", crmPoolGetRule)
	Reg.Register("crm_pool", "SetRule", crmPoolSetRule)

	// 客户转移
	Reg.Register("crm_customer", "Transfer", crmCustomerTransfer)
	Reg.Register("crm_customer", "ListTransferLog", crmCustomerListTransferLog)

	// 跟进提醒
	Reg.Register("crm_followup", "ListReminders", crmFollowupListReminders)
	Reg.Register("crm_followup", "ListOverdue", crmFollowupListOverdue)

	// 销售规则（阶段流转校验）
	Reg.Register("crm_sales_rule", "Get", crmSalesRuleGet)
	Reg.Register("crm_sales_rule", "Set", crmSalesRuleSet)

	// 业绩目标
	Reg.Register("crm_quota", "Set", crmQuotaSet)
	Reg.Register("crm_quota", "Get", crmQuotaGet)
	Reg.Register("crm_quota", "List", crmQuotaList)
	Reg.Register("crm_quota", "Achievement", crmQuotaAchievement)
	Reg.Register("crm_quota", "TeamAchievement", crmQuotaTeamAchievement)

	// 客户去重
	Reg.Register("crm_dedup", "FindDuplicates", crmDedupFindDuplicates)
	Reg.Register("crm_dedup", "Merge", crmDedupMerge)

	// ---- Phase 2 小红书渠道接入 ----
	// 渠道对比看板
	Reg.Register("crm_report", "ChannelCompare", crmReportChannelCompare)
	// 引流任务
	Reg.Register("crm_channel_task", "List", crmChannelTaskList)
	Reg.Register("crm_channel_task", "MarkDone", crmChannelTaskMarkDone)
	Reg.Register("crm_channel_task", "MarkSkipped", crmChannelTaskMarkSkipped)
	Reg.Register("crm_channel_task", "Assign", crmChannelTaskAssign)

	// ---- Phase 3 营销活动落地 ----
	// 活动效果漏斗
	Reg.Register("crm_report", "ActivityFunnel", crmReportActivityFunnel)
	// 优惠券
	Reg.Register("crm_coupon", "Create", crmCouponCreate)
	Reg.Register("crm_coupon", "Update", crmCouponUpdate)
	Reg.Register("crm_coupon", "Query", crmCouponQuery)
	Reg.Register("crm_coupon", "Get", crmCouponGet)
	Reg.Register("crm_coupon", "Delete", crmCouponDelete)
	Reg.Register("crm_coupon", "Issue", crmCouponIssue)
	Reg.Register("crm_coupon", "Verify", crmCouponVerify)
	Reg.Register("crm_coupon", "ListRecords", crmCouponListRecords)
	// 裂变活动
	Reg.Register("crm_fission", "Create", crmFissionCreate)
	Reg.Register("crm_fission", "Query", crmFissionQuery)
	Reg.Register("crm_fission", "Get", crmFissionGet)
	Reg.Register("crm_fission", "Delete", crmFissionDelete)
	Reg.Register("crm_fission", "RecordInvite", crmFissionRecordInvite)
	Reg.Register("crm_fission", "ListInvites", crmFissionListInvites)
	Reg.Register("crm_fission", "StatsByInviter", crmFissionStatsByInviter)
	// 落地页
	Reg.Register("crm_landing_page", "Create", crmLandingPageCreate)
	Reg.Register("crm_landing_page", "Query", crmLandingPageQuery)
	Reg.Register("crm_landing_page", "Get", crmLandingPageGet)
	Reg.Register("crm_landing_page", "Delete", crmLandingPageDelete)

	// ---- Phase 4 自动化与智能化 ----
	// SOP 自动化
	Reg.Register("crm_sop", "CreateRule", crmSopCreateRule)
	Reg.Register("crm_sop", "UpdateRule", crmSopUpdateRule)
	Reg.Register("crm_sop", "QueryRules", crmSopQueryRules)
	Reg.Register("crm_sop", "DeleteRules", crmSopDeleteRules)
	Reg.Register("crm_sop", "ListTasks", crmSopListTasks)
	Reg.Register("crm_sop", "MarkTaskDone", crmSopMarkTaskDone)
	Reg.Register("crm_sop", "MarkTaskSkipped", crmSopMarkTaskSkipped)
	// 客户评分
	Reg.Register("crm_score", "TopByScore", crmScoreTopByScore)
	Reg.Register("crm_score", "Recalculate", crmScoreRecalculate)
	// 自动打标签
	Reg.Register("crm_auto_tag", "CreateRule", crmAutoTagCreateRule)
	Reg.Register("crm_auto_tag", "QueryRules", crmAutoTagQueryRules)
	Reg.Register("crm_auto_tag", "DeleteRules", crmAutoTagDeleteRules)
	Reg.Register("crm_auto_tag", "ApplyToCustomer", crmAutoTagApplyToCustomer)
	// 销售话术
	Reg.Register("crm_script", "Create", crmScriptCreate)
	Reg.Register("crm_script", "Update", crmScriptUpdate)
	Reg.Register("crm_script", "Query", crmScriptQuery)
	Reg.Register("crm_script", "Delete", crmScriptDelete)
	Reg.Register("crm_script", "Recommend", crmScriptRecommend)

	// Phase 6 营销活动深化
	Reg.Register("crm_seckill", "Create", crmSeckillCreate)
	Reg.Register("crm_seckill", "Query", crmSeckillQuery)
	Reg.Register("crm_seckill", "Get", crmSeckillGet)
	Reg.Register("crm_seckill", "Delete", crmSeckillDelete)
	Reg.Register("crm_seckill", "Grab", crmSeckillGrab)
	Reg.Register("crm_seckill", "ListRecords", crmSeckillListRecords)

	Reg.Register("crm_lottery", "Create", crmLotteryCreate)
	Reg.Register("crm_lottery", "Query", crmLotteryQuery)
	Reg.Register("crm_lottery", "Get", crmLotteryGet)
	Reg.Register("crm_lottery", "Delete", crmLotteryDelete)
	Reg.Register("crm_lottery", "Draw", crmLotteryDraw)
	Reg.Register("crm_lottery", "ListRecords", crmLotteryListRecords)

	// Phase 6 报表扩展
	Reg.Register("crm_report", "SalesRanking", crmReportSalesRanking)
	Reg.Register("crm_report", "CustomerLifecycle", crmReportCustomerLifecycle)
	Reg.Register("crm_report", "ChannelCAC", crmReportChannelCAC)

	// Phase 6 客户导入导出
	Reg.Register("crm_customer", "Export", crmCustomerExport)
	Reg.Register("crm_customer", "BatchImport", crmCustomerBatchImport)

	// Phase 7 A组：SCRM 风控（仅 admin/manager 可管理）
	Reg.Register("crm_sensitive_word", "List", managerOnly(crmSensitiveWordList))
	Reg.Register("crm_sensitive_word", "Create", managerOnly(crmSensitiveWordCreate))
	Reg.Register("crm_sensitive_word", "Delete", managerOnly(crmSensitiveWordDelete))
	Reg.Register("crm_sensitive_alert", "List", managerOnly(crmSensitiveAlertList))
	Reg.Register("crm_sensitive_alert", "Handle", managerOnly(crmSensitiveAlertHandle))
	Reg.Register("crm_inheritance", "Execute", managerOnly(crmInheritanceExecute))
	Reg.Register("crm_inheritance", "List", managerOnly(crmInheritanceList))
	Reg.Register("crm_customer_loss", "List", managerOnly(crmCustomerLossList))
	Reg.Register("crm_customer_loss", "Handle", managerOnly(crmCustomerLossHandle))
	Reg.Register("crm_reply_timeout", "List", managerOnly(crmReplyTimeoutList))
	Reg.Register("crm_reply_timeout", "Handle", managerOnly(crmReplyTimeoutHandle))
	Reg.Register("crm_group_sop", "List", managerOnly(crmGroupSopList))
	Reg.Register("crm_group_sop", "Create", managerOnly(crmGroupSopCreate))
	Reg.Register("crm_group_sop", "Delete", managerOnly(crmGroupSopDelete))
	Reg.Register("crm_group_sop", "ListTasks", managerOnly(crmGroupSopListTasks))
	Reg.Register("crm_group_sop", "DoneTask", managerOnly(crmGroupSopDoneTask))

	// Phase 7 B组：营销活动补充（管理操作仅 admin/manager，参与操作不限角色）
	Reg.Register("crm_group_buy", "List", managerOnly(crmGroupBuyList))
	Reg.Register("crm_group_buy", "Create", managerOnly(crmGroupBuyCreate))
	Reg.Register("crm_group_buy", "Delete", managerOnly(crmGroupBuyDelete))
	Reg.Register("crm_group_buy", "OpenGroup", crmGroupBuyOpenGroup)
	Reg.Register("crm_group_buy", "JoinGroup", crmGroupBuyJoinGroup)
	Reg.Register("crm_group_buy", "ListRecords", managerOnly(crmGroupBuyListRecords))
	Reg.Register("crm_bargain", "List", managerOnly(crmBargainList))
	Reg.Register("crm_bargain", "Create", managerOnly(crmBargainCreate))
	Reg.Register("crm_bargain", "Delete", managerOnly(crmBargainDelete))
	Reg.Register("crm_bargain", "Start", crmBargainStart)
	Reg.Register("crm_bargain", "Help", crmBargainHelp)
	Reg.Register("crm_bargain", "ListRecords", managerOnly(crmBargainListRecords))
	Reg.Register("crm_full_reduction", "List", managerOnly(crmFullReductionList))
	Reg.Register("crm_full_reduction", "Create", managerOnly(crmFullReductionCreate))
	Reg.Register("crm_full_reduction", "Delete", managerOnly(crmFullReductionDelete))
	Reg.Register("crm_full_reduction", "CalculateDiscount", managerOnly(crmFullReductionCalculate))
	Reg.Register("crm_red_packet", "List", managerOnly(crmRedPacketList))
	Reg.Register("crm_red_packet", "Create", managerOnly(crmRedPacketCreate))
	Reg.Register("crm_red_packet", "Delete", managerOnly(crmRedPacketDelete))
	Reg.Register("crm_red_packet", "Claim", crmRedPacketClaim)
	Reg.Register("crm_red_packet", "ListRecords", managerOnly(crmRedPacketListRecords))
	Reg.Register("crm_sign_in", "List", managerOnly(crmSignInList))
	Reg.Register("crm_sign_in", "Create", managerOnly(crmSignInCreate))
	Reg.Register("crm_sign_in", "Delete", managerOnly(crmSignInDelete))
	Reg.Register("crm_sign_in", "CheckIn", crmSignInCheckIn)
	Reg.Register("crm_sign_in", "ListRecords", managerOnly(crmSignInListRecords))

	// Phase 7 C4：智能名片
	Reg.Register("crm_smart_card", "List", crmSmartCardList)
	Reg.Register("crm_smart_card", "Create", crmSmartCardCreate)
	Reg.Register("crm_smart_card", "Update", crmSmartCardUpdate)
	Reg.Register("crm_smart_card", "Delete", crmSmartCardDelete)
	Reg.Register("crm_smart_card", "ListViews", crmSmartCardListViews)
	Reg.Register("crm_smart_card", "Stats", crmSmartCardStats)
	Reg.Register("crm_smart_card", "PublicGet", crmSmartCardPublicGet)
	Reg.Register("crm_smart_card", "PublicSubmitLead", crmSmartCardPublicSubmitLead)
	Reg.Register("crm_smart_card", "PublicLike", crmSmartCardPublicLike)

	// P1: CRM 核心补全 — 联系人
	Reg.Register("crm_contact", "List", crmContactList)
	Reg.Register("crm_contact", "Create", crmContactCreate)
	Reg.Register("crm_contact", "Update", crmContactUpdate)
	Reg.Register("crm_contact", "Delete", crmContactDelete)

	// P1: 回款计划
	Reg.Register("crm_receivables_plan", "List", crmReceivablesPlanList)
	Reg.Register("crm_receivables_plan", "Create", crmReceivablesPlanCreate)
	Reg.Register("crm_receivables_plan", "Update", crmReceivablesPlanUpdate)
	Reg.Register("crm_receivables_plan", "Delete", crmReceivablesPlanDelete)
	Reg.Register("crm_receivables_plan", "BatchCreate", crmReceivablesPlanBatchCreate)

	// P1: 回款登记
	Reg.Register("crm_receivables", "List", crmReceivablesList)
	Reg.Register("crm_receivables", "Create", crmReceivablesCreate)
	Reg.Register("crm_receivables", "Update", crmReceivablesUpdate)
	Reg.Register("crm_receivables", "Delete", crmReceivablesDelete)
	Reg.Register("crm_receivables", "Void", crmReceivablesVoid)

	// P1: 报价单
	Reg.Register("crm_quote", "List", crmQuoteList)
	Reg.Register("crm_quote", "Create", crmQuoteCreate)
	Reg.Register("crm_quote", "Update", crmQuoteUpdate)
	Reg.Register("crm_quote", "Delete", crmQuoteDelete)
	Reg.Register("crm_quote", "ChangeStatus", crmQuoteChangeStatus)
	Reg.Register("crm_quote", "Transfer", crmQuoteTransfer)

	// P1: 待办
	Reg.Register("crm_todo", "List", crmTodoList)
	Reg.Register("crm_todo", "Handle", crmTodoHandle)
	Reg.Register("crm_todo", "Count", crmTodoCount)

	// P1: 合同订单产品明细
	Reg.Register("crm_order_product", "List", crmOrderProductList)
	Reg.Register("crm_order_product", "Create", crmOrderProductCreate)
	Reg.Register("crm_order_product", "Delete", crmOrderProductDelete)

	// P1: 合同订单附件
	Reg.Register("crm_order_attachment", "List", crmOrderAttachmentList)
	Reg.Register("crm_order_attachment", "Create", crmOrderAttachmentCreate)
	Reg.Register("crm_order_attachment", "Delete", crmOrderAttachmentDelete)

	// P1: 合同订单进度
	Reg.Register("crm_order_progress", "List", crmOrderProgressList)
	Reg.Register("crm_order_progress", "Create", crmOrderProgressCreate)
	Reg.Register("crm_order_progress", "Update", crmOrderProgressUpdate)
	Reg.Register("crm_order_progress", "Delete", crmOrderProgressDelete)
	Reg.Register("crm_order_progress", "UpdateStatus", crmOrderProgressUpdateStatus)

	// P1: 快捷跟进
	Reg.Register("crm_quick_followup", "List", crmQuickFollowupList)
	Reg.Register("crm_quick_followup", "Create", crmQuickFollowupCreate)
	Reg.Register("crm_quick_followup", "Delete", crmQuickFollowupDelete)

	// P1: 拜访签到
	Reg.Register("crm_signin", "List", crmSigninList)
	Reg.Register("crm_signin", "Create", crmSigninCreate)
	Reg.Register("crm_signin", "Delete", crmSigninDelete)

	// P2: 动态字段
	Reg.Register("crm_field_def", "List", crmFieldDefList)
	Reg.Register("crm_field_def", "Create", crmFieldDefCreate)
	Reg.Register("crm_field_def", "Update", crmFieldDefUpdate)
	Reg.Register("crm_field_def", "Delete", crmFieldDefDelete)
	Reg.Register("crm_field_value", "Get", crmFieldValueGet)
	Reg.Register("crm_field_value", "Set", crmFieldValueSet)
	Reg.Register("crm_field_value", "BatchSet", crmFieldValueBatchSet)

	// P2: 场景
	Reg.Register("crm_scene", "List", crmSceneList)
	Reg.Register("crm_scene", "Create", crmSceneCreate)
	Reg.Register("crm_scene", "Update", crmSceneUpdate)
	Reg.Register("crm_scene", "Delete", crmSceneDelete)
	Reg.Register("crm_scene", "SetDefault", crmSceneSetDefault)
	Reg.Register("crm_scene", "DragSort", crmSceneDragSort)

	// P2: 客户消费
	Reg.Register("crm_consume", "List", crmConsumeList)
	Reg.Register("crm_consume", "Create", crmConsumeCreate)
	Reg.Register("crm_consume", "Update", crmConsumeUpdate)
	Reg.Register("crm_consume", "Delete", crmConsumeDelete)
	Reg.Register("crm_consume", "Revoke", crmConsumeRevoke)
	Reg.Register("crm_consume", "Summary", crmConsumeSummary)

	// P2: 审批引擎
	Reg.Register("crm_approval_flow", "List", crmApprovalFlowList)
	Reg.Register("crm_approval_flow", "Create", crmApprovalFlowCreate)
	Reg.Register("crm_approval_flow", "Update", crmApprovalFlowUpdate)
	Reg.Register("crm_approval_flow", "Delete", crmApprovalFlowDelete)
	Reg.Register("crm_approval", "Submit", crmApprovalSubmit)
	Reg.Register("crm_approval", "ListInstances", crmApprovalInstanceList)
	Reg.Register("crm_approval", "Approve", crmApprovalApprove)
	Reg.Register("crm_approval", "Reject", crmApprovalReject)
	Reg.Register("crm_approval", "Void", crmApprovalVoid)
	Reg.Register("crm_approval", "ListRecords", crmApprovalRecordList)

	// P3: 项目管理
	Reg.Register("crm_project", "List", crmProjectList)
	Reg.Register("crm_project", "Create", crmProjectCreate)
	Reg.Register("crm_project", "Update", crmProjectUpdate)
	Reg.Register("crm_project", "Delete", crmProjectDelete)
	Reg.Register("crm_project", "ChangeStatus", crmProjectChangeStatus)

	// P3: 项目任务
	Reg.Register("crm_project_task", "List", crmProjectTaskList)
	Reg.Register("crm_project_task", "Create", crmProjectTaskCreate)
	Reg.Register("crm_project_task", "Update", crmProjectTaskUpdate)
	Reg.Register("crm_project_task", "Delete", crmProjectTaskDelete)
	Reg.Register("crm_project_task", "UpdateStatus", crmProjectTaskUpdateStatus)

	// P3: 财务账户
	Reg.Register("crm_account", "List", crmAccountList)
	Reg.Register("crm_account", "Create", crmAccountCreate)
	Reg.Register("crm_account", "Update", crmAccountUpdate)
	Reg.Register("crm_account", "Delete", crmAccountDelete)

	// P3: 财务收支记录
	Reg.Register("crm_finance_record", "List", crmFinanceRecordList)
	Reg.Register("crm_finance_record", "Create", crmFinanceRecordCreate)
	Reg.Register("crm_finance_record", "Update", crmFinanceRecordUpdate)
	Reg.Register("crm_finance_record", "Delete", crmFinanceRecordDelete)
	Reg.Register("crm_finance_record", "Void", crmFinanceRecordVoid)

	// P3: 发票管理
	Reg.Register("crm_invoice", "List", crmInvoiceList)
	Reg.Register("crm_invoice", "Create", crmInvoiceCreate)
	Reg.Register("crm_invoice", "Update", crmInvoiceUpdate)
	Reg.Register("crm_invoice", "Delete", crmInvoiceDelete)
	Reg.Register("crm_invoice", "Void", crmInvoiceVoid)

	// P3: 产品档案
	Reg.Register("crm_product", "List", crmProductList)
	Reg.Register("crm_product", "Create", crmProductCreate)
	Reg.Register("crm_product", "Update", crmProductUpdate)
	Reg.Register("crm_product", "Delete", crmProductDelete)

	// P3: 仓库
	Reg.Register("crm_warehouse", "List", crmWarehouseList)
	Reg.Register("crm_warehouse", "Create", crmWarehouseCreate)
	Reg.Register("crm_warehouse", "Update", crmWarehouseUpdate)
	Reg.Register("crm_warehouse", "Delete", crmWarehouseDelete)

	// P3: 库存流水
	Reg.Register("crm_stock_record", "List", crmStockRecordList)
	Reg.Register("crm_stock_record", "Create", crmStockRecordCreate)
	Reg.Register("crm_stock_record", "Delete", crmStockRecordDelete)
	Reg.Register("crm_product", "StockBalance", crmProductStockBalance)
	Reg.Register("crm_product", "StockWarning", crmProductStockWarning)

	// P3: 工单管理
	Reg.Register("crm_workorder", "List", crmWorkorderList)
	Reg.Register("crm_workorder", "Create", crmWorkorderCreate)
	Reg.Register("crm_workorder", "Update", crmWorkorderUpdate)
	Reg.Register("crm_workorder", "Delete", crmWorkorderDelete)
	Reg.Register("crm_workorder", "ChangeStatus", crmWorkorderChangeStatus)
	Reg.Register("crm_workorder", "Transfer", crmWorkorderTransfer)

	// P3: 通话记录
	Reg.Register("crm_call_record", "List", crmCallRecordList)
	Reg.Register("crm_call_record", "Create", crmCallRecordCreate)
	Reg.Register("crm_call_record", "Update", crmCallRecordUpdate)
	Reg.Register("crm_call_record", "Delete", crmCallRecordDelete)
	Reg.Register("crm_call_record", "Stats", crmCallRecordStats)

	// P3: 知识库
	Reg.Register("crm_knowledge", "List", crmKnowledgeList)
	Reg.Register("crm_knowledge", "Create", crmKnowledgeCreate)
	Reg.Register("crm_knowledge", "Update", crmKnowledgeUpdate)
	Reg.Register("crm_knowledge", "Delete", crmKnowledgeDelete)

	// P3: 公告
	Reg.Register("crm_notice", "List", crmNoticeList)
	Reg.Register("crm_notice", "Create", crmNoticeCreate)
	Reg.Register("crm_notice", "Update", crmNoticeUpdate)
	Reg.Register("crm_notice", "Delete", crmNoticeDelete)

	// P3: 工作报告
	Reg.Register("crm_work_report", "List", crmWorkReportList)
	Reg.Register("crm_work_report", "Create", crmWorkReportCreate)
	Reg.Register("crm_work_report", "Update", crmWorkReportUpdate)
	Reg.Register("crm_work_report", "Delete", crmWorkReportDelete)
	Reg.Register("crm_work_report", "Review", crmWorkReportReview)

	// P3: AI分析
	Reg.Register("crm_ai_analysis", "List", crmAiAnalysisList)
	Reg.Register("crm_ai_analysis", "Create", crmAiAnalysisCreate)
	Reg.Register("crm_ai_analysis", "Delete", crmAiAnalysisDelete)
	Reg.Register("crm_ai_analysis", "GetByCustomer", crmAiAnalysisGetByCustomer)
	Reg.Register("crm_ai_analysis", "AnalyzeCustomer", crmAiAnalysisAnalyzeCustomer)
	Reg.Register("crm_ai_analysis", "Stats", crmAiAnalysisStats)

	// P3: 售后管理
	Reg.Register("crm_aftersales", "List", crmAftersalesList)
	Reg.Register("crm_aftersales", "Create", crmAftersalesCreate)
	Reg.Register("crm_aftersales", "Update", crmAftersalesUpdate)
	Reg.Register("crm_aftersales", "Delete", crmAftersalesDelete)
	Reg.Register("crm_aftersales", "ChangeStatus", crmAftersalesChangeStatus)

	// P4: 项目扩展（事件/文档）
	Reg.Register("crm_project_event", "List", crmProjectEventList)
	Reg.Register("crm_project_event", "Create", crmProjectEventCreate)
	Reg.Register("crm_project_event", "Update", crmProjectEventUpdate)
	Reg.Register("crm_project_event", "Delete", crmProjectEventDelete)
	Reg.Register("crm_project_event", "CompleteEvent", crmProjectEventCompleteEvent)
	Reg.Register("crm_project_file", "List", crmProjectFileList)
	Reg.Register("crm_project_file", "Create", crmProjectFileCreate)
	Reg.Register("crm_project_file", "Delete", crmProjectFileDelete)

	// P4: 供应商 + 采购
	Reg.Register("crm_supplier", "List", crmSupplierList)
	Reg.Register("crm_supplier", "Create", crmSupplierCreate)
	Reg.Register("crm_supplier", "Update", crmSupplierUpdate)
	Reg.Register("crm_supplier", "Delete", crmSupplierDelete)
	Reg.Register("crm_purchase_order", "List", crmPurchaseOrderList)
	Reg.Register("crm_purchase_order", "Create", crmPurchaseOrderCreate)
	Reg.Register("crm_purchase_order", "Update", crmPurchaseOrderUpdate)
	Reg.Register("crm_purchase_order", "Delete", crmPurchaseOrderDelete)
	Reg.Register("crm_purchase_order", "ChangeStatus", crmPurchaseOrderChangeStatus)

	// P4: 考勤 + 请假
	Reg.Register("crm_attendance", "List", crmAttendanceList)
	Reg.Register("crm_attendance", "Create", crmAttendanceCreate)
	Reg.Register("crm_leave", "List", crmLeaveList)
	Reg.Register("crm_leave", "Create", crmLeaveCreate)
	Reg.Register("crm_leave", "Update", crmLeaveUpdate)
	Reg.Register("crm_leave", "Delete", crmLeaveDelete)
	Reg.Register("crm_leave", "Approve", crmLeaveApprove)
	Reg.Register("crm_leave", "Reject", crmLeaveReject)

	// P4: 系统设置
	Reg.Register("crm_auto_allocation", "List", crmAutoAllocationList)
	Reg.Register("crm_auto_allocation", "Create", crmAutoAllocationCreate)
	Reg.Register("crm_auto_allocation", "Update", crmAutoAllocationUpdate)
	Reg.Register("crm_auto_allocation", "Delete", crmAutoAllocationDelete)
	Reg.Register("crm_auto_allocation", "ToggleStatus", crmAutoAllocationToggleStatus)
	Reg.Register("crm_seas_type", "List", crmSeasTypeList)
	Reg.Register("crm_seas_type", "Create", crmSeasTypeCreate)
	Reg.Register("crm_seas_type", "Update", crmSeasTypeUpdate)
	Reg.Register("crm_seas_type", "Delete", crmSeasTypeDelete)
	Reg.Register("crm_seas_type", "SetDefault", crmSeasTypeSetDefault)
	Reg.Register("crm_customer_setting", "Get", crmCustomerSettingGet)
	Reg.Register("crm_customer_setting", "Save", crmCustomerSettingSave)
	Reg.Register("crm_lead_source", "List", crmLeadSourceList)
	Reg.Register("crm_lead_source", "Create", crmLeadSourceCreate)
	Reg.Register("crm_lead_source", "Update", crmLeadSourceUpdate)
	Reg.Register("crm_lead_source", "Delete", crmLeadSourceDelete)
	Reg.Register("crm_reason_dict", "List", crmReasonDictList)
	Reg.Register("crm_reason_dict", "Create", crmReasonDictCreate)
	Reg.Register("crm_reason_dict", "Update", crmReasonDictUpdate)
	Reg.Register("crm_reason_dict", "Delete", crmReasonDictDelete)

	// 客户池扩展
	Reg.Register("crm_customer", "Lock", crmCustomerLock)
	Reg.Register("crm_customer", "Unlock", crmCustomerUnlock)
	Reg.Register("crm_customer", "Blacklist", crmCustomerBlacklist)
	Reg.Register("crm_customer", "RemoveBlacklist", crmCustomerRemoveBlacklist)
	Reg.Register("crm_customer", "SetKey", crmCustomerSetKey)
	Reg.Register("crm_customer", "Share", crmCustomerShare)
	Reg.Register("crm_customer", "ListBlacklist", crmCustomerListBlacklist)
	Reg.Register("crm_customer", "ListKey", crmCustomerListKey)
	Reg.Register("crm_customer", "ListShared", crmCustomerListShared)
	Reg.Register("crm_customer", "ListUnreceive", crmCustomerListUnreceive)
	Reg.Register("crm_customer", "ListReassign", crmCustomerListReassign)
	Reg.Register("crm_customer", "AcceptReassign", crmCustomerAcceptReassign)
	Reg.Register("crm_customer", "RejectReassign", crmCustomerRejectReassign)
	// 线索池
	Reg.Register("crm_lead", "MoveToPool", crmLeadMoveToPool)
	Reg.Register("crm_lead", "ReceiveFromPool", crmLeadReceiveFromPool)
	Reg.Register("crm_lead", "ListPool", crmLeadListPool)
	Reg.Register("crm_lead", "Transfer", crmLeadTransfer)
	Reg.Register("crm_lead", "ConvertToCustomer", crmLeadConvertToCustomer)
	Reg.Register("crm_lead", "MarkContacted", crmLeadMarkContacted)
	Reg.Register("crm_lead", "MarkLost", crmLeadMarkLost)
	Reg.Register("crm_lead", "Reopen", crmLeadReopen)
	// 回收站
	Reg.Register("crm_recycle", "List", crmRecycleList)
	Reg.Register("crm_recycle", "Restore", crmRecycleRestore)
	Reg.Register("crm_recycle", "Clear", crmRecycleClear)
	Reg.Register("crm_recycle", "DeleteRecord", crmRecycleDeleteRecord)
	// 批量操作
	Reg.Register("crm_batch", "Execute", crmBatchExecute)

	// 仪表盘
	Reg.Register("crm_dashboard", "Briefing", crmDashboardBriefing)
	Reg.Register("crm_dashboard", "Summary", crmDashboardSummary)
	Reg.Register("crm_dashboard", "Trend", crmDashboardTrend)
	Reg.Register("crm_dashboard", "Ranking", crmDashboardRanking)
	Reg.Register("crm_dashboard", "Achievement", crmDashboardAchievement)
	// 统计报表
	Reg.Register("crm_stats", "Customer", crmStatsCustomer)
	Reg.Register("crm_stats", "Lead", crmStatsLead)
	Reg.Register("crm_stats", "Contract", crmStatsContract)
	Reg.Register("crm_stats", "Workorder", crmStatsWorkorder)
	Reg.Register("crm_stats", "Product", crmStatsProduct)
	Reg.Register("crm_stats", "Call", crmStatsCall)

	// ---- 财务扩展 ----
	Reg.Register("crm_income_category", "List", crmIncomeCategoryList)
	Reg.Register("crm_income_category", "Create", crmIncomeCategoryCreate)
	Reg.Register("crm_income_category", "Update", crmIncomeCategoryUpdate)
	Reg.Register("crm_income_category", "Delete", crmIncomeCategoryDelete)
	Reg.Register("crm_other_income", "List", crmOtherIncomeList)
	Reg.Register("crm_other_income", "Create", crmOtherIncomeCreate)
	Reg.Register("crm_other_income", "Update", crmOtherIncomeUpdate)
	Reg.Register("crm_other_income", "Delete", crmOtherIncomeDelete)
	Reg.Register("crm_other_income", "ChangeStatus", crmOtherIncomeChangeStatus)
	Reg.Register("crm_other_expense", "List", crmOtherExpenseList)
	Reg.Register("crm_other_expense", "Create", crmOtherExpenseCreate)
	Reg.Register("crm_other_expense", "Update", crmOtherExpenseUpdate)
	Reg.Register("crm_other_expense", "Delete", crmOtherExpenseDelete)
	Reg.Register("crm_other_expense", "ChangeStatus", crmOtherExpenseChangeStatus)
	Reg.Register("crm_fund_transfer", "List", crmFundTransferList)
	Reg.Register("crm_fund_transfer", "Create", crmFundTransferCreate)
	Reg.Register("crm_fund_transfer", "Update", crmFundTransferUpdate)
	Reg.Register("crm_fund_transfer", "Delete", crmFundTransferDelete)
	Reg.Register("crm_fund_transfer", "ChangeStatus", crmFundTransferChangeStatus)

	// ---- 进销存单据 ----
	Reg.Register("crm_stock_in", "List", crmStockInList)
	Reg.Register("crm_stock_in", "Create", crmStockInCreate)
	Reg.Register("crm_stock_in", "Update", crmStockInUpdate)
	Reg.Register("crm_stock_in", "Delete", crmStockInDelete)
	Reg.Register("crm_stock_in", "ChangeStatus", crmStockInChangeStatus)
	Reg.Register("crm_stock_out", "List", crmStockOutList)
	Reg.Register("crm_stock_out", "Create", crmStockOutCreate)
	Reg.Register("crm_stock_out", "Update", crmStockOutUpdate)
	Reg.Register("crm_stock_out", "Delete", crmStockOutDelete)
	Reg.Register("crm_stock_out", "ChangeStatus", crmStockOutChangeStatus)
	Reg.Register("crm_stock_check", "List", crmStockCheckList)
	Reg.Register("crm_stock_check", "Create", crmStockCheckCreate)
	Reg.Register("crm_stock_check", "Update", crmStockCheckUpdate)
	Reg.Register("crm_stock_check", "Delete", crmStockCheckDelete)
	Reg.Register("crm_stock_check", "ChangeStatus", crmStockCheckChangeStatus)
	Reg.Register("crm_stock_transfer", "List", crmStockTransferList)
	Reg.Register("crm_stock_transfer", "Create", crmStockTransferCreate)
	Reg.Register("crm_stock_transfer", "Update", crmStockTransferUpdate)
	Reg.Register("crm_stock_transfer", "Delete", crmStockTransferDelete)
	Reg.Register("crm_stock_transfer", "ChangeStatus", crmStockTransferChangeStatus)

	// ---- 工单扩展 ----
	Reg.Register("crm_workorder_type", "List", crmWorkorderTypeList)
	Reg.Register("crm_workorder_type", "Create", crmWorkorderTypeCreate)
	Reg.Register("crm_workorder_type", "Update", crmWorkorderTypeUpdate)
	Reg.Register("crm_workorder_type", "Delete", crmWorkorderTypeDelete)
	Reg.Register("crm_workorder_pool", "List", crmWorkorderPoolList)
	Reg.Register("crm_workorder_pool", "MoveToPool", crmWorkorderPoolMoveToPool)
	Reg.Register("crm_workorder_pool", "Receive", crmWorkorderPoolReceive)
	Reg.Register("crm_workorder_pool", "BatchReceive", crmWorkorderPoolBatchReceive)
	Reg.Register("crm_workorder_log", "List", crmWorkorderLogList)
	Reg.Register("crm_workorder_log", "Add", crmWorkorderLogAdd)

	// ---- 呼叫中心扩展 ----
	Reg.Register("crm_call_list", "List", crmCallListList)
	Reg.Register("crm_call_list", "Create", crmCallListCreate)
	Reg.Register("crm_call_list", "Update", crmCallListUpdate)
	Reg.Register("crm_call_list", "Delete", crmCallListDelete)
	Reg.Register("crm_call_list", "BatchAssign", crmCallListBatchAssign)
	Reg.Register("crm_call_list", "MarkDialed", crmCallListMarkDialed)
	Reg.Register("crm_call_list", "MarkSkipped", crmCallListMarkSkipped)

	// ---- 办公扩展 ----
	Reg.Register("crm_knowledge_type", "List", crmKnowledgeTypeList)
	Reg.Register("crm_knowledge_type", "Create", crmKnowledgeTypeCreate)
	Reg.Register("crm_knowledge_type", "Update", crmKnowledgeTypeUpdate)
	Reg.Register("crm_knowledge_type", "Delete", crmKnowledgeTypeDelete)
	Reg.Register("crm_work_report_comment", "List", crmWorkReportCommentList)
	Reg.Register("crm_work_report_comment", "Create", crmWorkReportCommentCreate)
	Reg.Register("crm_work_report_comment", "Delete", crmWorkReportCommentDelete)
	Reg.Register("crm_work_report", "Stats", crmWorkReportStats)

	// ---- 售后扩展 ----
	Reg.Register("crm_aftersales_type", "List", crmAftersalesTypeList)
	Reg.Register("crm_aftersales_type", "Create", crmAftersalesTypeCreate)
	Reg.Register("crm_aftersales_type", "Update", crmAftersalesTypeUpdate)
	Reg.Register("crm_aftersales_type", "Delete", crmAftersalesTypeDelete)
	Reg.Register("crm_aftersales_comment", "List", crmAftersalesCommentList)
	Reg.Register("crm_aftersales_comment", "Create", crmAftersalesCommentCreate)
	Reg.Register("crm_aftersales_comment", "Delete", crmAftersalesCommentDelete)

	// ---- 跟进扩展（评论 / 常用备注 / 待跟进视图）----
	Reg.Register("crm_followup_comment", "List", crmFollowupCommentList)
	Reg.Register("crm_followup_comment", "Create", crmFollowupCommentCreate)
	Reg.Register("crm_followup_comment", "Delete", crmFollowupCommentDelete)
	Reg.Register("crm_quick_remark", "List", crmQuickRemarkList)
	Reg.Register("crm_quick_remark", "Create", crmQuickRemarkCreate)
	Reg.Register("crm_quick_remark", "Update", crmQuickRemarkUpdate)
	Reg.Register("crm_quick_remark", "Delete", crmQuickRemarkDelete)
	Reg.Register("crm_quick_remark", "SetSort", crmQuickRemarkSetSort)
	Reg.Register("crm_followup_view", "TodayTodo", crmFollowupViewTodayTodo)
	Reg.Register("crm_followup_view", "TodayDone", crmFollowupViewTodayDone)
	Reg.Register("crm_followup_view", "NeverFollowup", crmFollowupViewNeverFollowup)

	// ---- 商机扩展（阶段流转 / 赢单输单）----
	Reg.Register("crm_opportunity", "ChangeStage", crmOpportunityChangeStage)
	Reg.Register("crm_opportunity", "Win", crmOpportunityWin)
	Reg.Register("crm_opportunity", "Lose", crmOpportunityLose)
	Reg.Register("crm_opportunity", "BatchChangeStage", crmOpportunityBatchChangeStage)
	Reg.Register("crm_opportunity", "Reopen", crmOpportunityReopen)
	Reg.Register("crm_opportunity", "Export", crmOpportunityExport)
	Reg.Register("crm_opportunity", "BatchTransfer", crmOpportunityBatchTransfer)
	Reg.Register("crm_opportunity", "BatchImport", crmOpportunityBatchImport)

	// ---- 合同扩展（作废 / 变更 / 关联项目工单）----
	Reg.Register("crm_order", "Revoke", crmOrderRevoke)
	Reg.Register("crm_order", "Change", crmOrderChange)
	Reg.Register("crm_order", "LinkProject", crmOrderLinkProject)
	Reg.Register("crm_order", "LinkWorkorder", crmOrderLinkWorkorder)

	// ---- 回款扩展（计划生成 / 到期逾期 / 审批）----
	Reg.Register("crm_receivables", "GeneratePlan", crmReceivablesGeneratePlan)
	Reg.Register("crm_receivables", "ListExpiring", crmReceivablesListExpiring)
	Reg.Register("crm_receivables", "ListOverdue", crmReceivablesListOverdue)
	Reg.Register("crm_receivables", "Approve", crmReceivablesApprove)
	Reg.Register("crm_receivables", "Reject", crmReceivablesReject)

	// ---- 报价扩展（单号生成 / 作废 / 转移）----
	Reg.Register("crm_quote", "GenerateNo", crmQuoteGenerateNo)
	Reg.Register("crm_quote", "Void", crmQuoteVoid)
	Reg.Register("crm_quote", "Transfer", crmQuoteExtTransfer)
	Reg.Register("crm_quote", "BatchTransfer", crmQuoteBatchTransfer)

	// ---- 联系人扩展（转客户 / 联系人池）----
	Reg.Register("crm_contact", "ConvertToCustomer", crmContactConvertToCustomer)
	Reg.Register("crm_contact", "ListPool", crmContactListPool)
}

// requireIdentity 断言本地身份已注入（CRM 渠道无关，必须带 tenant_id）。
func requireIdentity(ctx context.Context) (string, error) {
	t := identity.TenantID(ctx)
	if t == "" {
		return "", errNeedIdentity
	}
	return t, nil
}

// errForbidden 权限不足。
var errForbidden = errors.New("无权限")

// errNeedPlatform 非平台超管身份时返回。
var errNeedPlatform = errors.New("missing platform identity: is_platform required (call from platform admin session)")

// requirePlatformAdmin 校验当前请求来自平台超管（is_platform=true 且 platform_id 非空）。
// 用于 crm_tenant/* 等平台管理 API，杜绝租户用户或匿名调用方访问。
func requirePlatformAdmin(ctx context.Context) (string, error) {
	if !identity.IsPlatform(ctx) {
		return "", errNeedPlatform
	}
	id := identity.PlatformID(ctx)
	if id == "" {
		return "", errNeedPlatform
	}
	return id, nil
}

// requirePlatformRole 校验当前请求来自平台超管，且其角色在白名单中。
// super_admin 始终通过。用于按角色细分平台管理 API 的访问权限。
func requirePlatformRole(ctx context.Context, roles ...string) (string, error) {
	id, err := requirePlatformAdmin(ctx)
	if err != nil {
		return "", err
	}
	// super_admin 始终通过
	for _, r := range roles {
		if r == "super_admin" {
			// 需要查实际角色
		}
	}
	// 查平台超管的角色
	var admin models.PlatformAdmin
	if err := models.DB.Where("id = ?", id).First(&admin).Error; err != nil {
		return "", errForbidden
	}
	if admin.Status != 1 {
		return "", errForbidden
	}
	// super_admin 始终通过
	if admin.Role == "super_admin" {
		return id, nil
	}
	for _, r := range roles {
		if admin.Role == r {
			return id, nil
		}
	}
	return "", errForbidden
}

// requireRole 校验当前用户是否拥有指定角色编码（fail closed：无角色/角色不存在一律拒绝）。
func requireRole(ctx context.Context, code string) error {
	return requireAnyRole(ctx, code)
}

// managerOnly 装饰器：仅允许 admin/manager 角色调用被包裹的 dispatch handler。
func managerOnly(fn func(context.Context, string, string, json.RawMessage) (any, error)) func(context.Context, string, string, json.RawMessage) (any, error) {
	return func(ctx context.Context, a, b string, params json.RawMessage) (any, error) {
		if err := requireAnyRole(ctx, "admin", "manager"); err != nil {
			return nil, err
		}
		return fn(ctx, a, b, params)
	}
}

// requireAnyRole 校验当前用户是否拥有任一指定角色编码（满足其一即可）。
func requireAnyRole(ctx context.Context, codes ...string) error {
	t, err := requireIdentity(ctx)
	if err != nil {
		return err
	}
	u := identity.UserID(ctx)
	if u == "" {
		return errForbidden
	}
	var usr models.User
	if err := models.DB.Scopes(models.TenantScope(t)).Where("id = ?", u).First(&usr).Error; err != nil {
		return errForbidden
	}
	if usr.RoleID == "" {
		return errForbidden
	}
	var r models.CrmRole
	if err := models.DB.Scopes(models.TenantScope(t)).Where("id = ?", usr.RoleID).First(&r).Error; err != nil {
		return errForbidden
	}
	for _, code := range codes {
		if r.Code == code {
			return nil
		}
	}
	return errForbidden
}

// currentUserRoleCode 返回当前用户的角色编码（admin/manager/sales），查询失败返回空串。
func currentUserRoleCode(ctx context.Context) string {
	t, err := requireIdentity(ctx)
	if err != nil {
		return ""
	}
	u := identity.UserID(ctx)
	if u == "" {
		return ""
	}
	var usr models.User
	if err := models.DB.Scopes(models.TenantScope(t)).Where("id = ?", u).First(&usr).Error; err != nil {
		return ""
	}
	if usr.RoleID == "" {
		return ""
	}
	var r models.CrmRole
	if err := models.DB.Scopes(models.TenantScope(t)).Where("id = ?", usr.RoleID).First(&r).Error; err != nil {
		return ""
	}
	return r.Code
}

// EffectiveOwnerFilter 数据权限分层：根据角色返回有效的 owner_user_id 过滤值。
//   - admin/manager：返回 requestedOwner（允许查看全部，也可按指定归属人筛选）
//   - sales：强制返回当前用户 ID（只能看自己的数据）
//   - 角色未知：强制返回当前用户 ID（fail-closed，最小权限）
func EffectiveOwnerFilter(ctx context.Context, requestedOwner string) string {
	// __me__ 表示当前用户自己的数据
	if requestedOwner == "__me__" {
		return identity.UserID(ctx)
	}
	role := currentUserRoleCode(ctx)
	switch role {
	case "admin", "manager":
		return requestedOwner
	default:
		// sales 或未知角色：只能看自己的数据
		return identity.UserID(ctx)
	}
}

// ---------- tenant / user ----------

// 平台角色常量
const (
	platformRoleSuperAdmin = "super_admin"
	platformRoleOperator   = "operator"
	platformRoleFinance    = "finance"
	platformRoleSupport    = "support"
)

func crmTenantCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 创建租户：super_admin / operator
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator); err != nil {
		return nil, err
	}
	var req services.TenantCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTenant().Create(req)
}

func crmTenantList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	// 列出租户：所有平台角色均可
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	return services.NewTenant().List()
}

func crmTenantUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 编辑租户：super_admin / operator
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator); err != nil {
		return nil, err
	}
	var req struct {
		ID   string                   `json:"id"`
		Data services.TenantUpdateReq `json:"data"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewTenant().Update(req.ID, req.Data); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmTenantSetStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 停用/启用租户：super_admin / operator
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator); err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if req.Status != 1 && req.Status != 2 {
		return nil, errors.New("status 只能是 1(启用) 或 2(停用)")
	}
	if err := services.NewTenant().SetStatus(req.ID, req.Status); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmTenantDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 删除租户：仅 super_admin
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewTenant().Delete(req.ID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmTenantListPaged(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	var req services.TenantListReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTenant().ListPaged(req)
}

func crmTenantGetDetail(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return services.NewTenant().GetDetail(req.ID)
}

// crmTenantGetCurrentInfo 租户端：当前登录用户查询本租户的套餐/到期信息 + 自己的角色。
// 任何租户用户均可调用（前端按 role_code 控制是否展示套餐管理入口）。
func crmTenantGetCurrentInfo(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	tenantID, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	userID := identity.UserID(ctx)
	return services.NewTenant().GetCurrentInfo(tenantID, userID)
}

func crmTenantListUsers(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleSupport); err != nil {
		return nil, err
	}
	var req struct {
		TenantID string `json:"tenant_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.TenantID == "" {
		return nil, errors.New("tenant_id 不能为空")
	}
	return services.NewTenant().ListUsers(req.TenantID)
}

func crmTenantResetAdminPassword(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleSupport); err != nil {
		return nil, err
	}
	var req struct {
		TenantID string `json:"tenant_id"`
		UserID   string `json:"user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.TenantID == "" || req.UserID == "" {
		return nil, errors.New("tenant_id 和 user_id 不能为空")
	}
	pw, err := services.NewTenant().ResetAdminPassword(req.TenantID, req.UserID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"new_password": pw}, nil
}

func crmTenantRenew(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance); err != nil {
		return nil, err
	}
	var req services.RenewReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.TenantID == "" {
		return nil, errors.New("tenant_id 不能为空")
	}
	return services.NewTenant().Renew(req)
}

// ---------- plan（套餐管理）----------

func crmPlanCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 创建套餐：super_admin / operator / finance
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance); err != nil {
		return nil, err
	}
	var req services.PlanCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewPlanService().Create(req)
}

func crmPlanList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	// 列出套餐：平台角色 或 租户身份（租户端查看可购买套餐）
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		// 非平台角色，检查是否有租户身份
		if _, err2 := requireIdentity(ctx); err2 != nil {
			return nil, err
		}
	}
	return services.NewPlanService().List()
}

func crmPlanUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 编辑套餐：super_admin / operator / finance
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance); err != nil {
		return nil, err
	}
	var req struct {
		ID   string                 `json:"id"`
		Data services.PlanUpdateReq `json:"data"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewPlanService().Update(req.ID, req.Data); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmPlanSetStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 启用/停用套餐：super_admin / operator
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator); err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewPlanService().SetStatus(req.ID, req.Status); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmPlanDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 删除套餐：仅 super_admin
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewPlanService().Delete(req.ID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmPlanListModules(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	// 列出系统模块定义：所有平台角色
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	return services.NewPlanService().ListModules(), nil
}

func crmPlanGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	// 获取套餐详情：所有平台角色 + 租户端（租户端通过 identity 访问）
	// 先尝试平台角色，如果不是平台角色则检查租户身份
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		// 非平台角色，检查是否有租户身份（租户端查看套餐详情）
		if _, err2 := requireIdentity(ctx); err2 != nil {
			return nil, err
		}
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return services.NewPlanService().Get(req.ID)
}

// ---------- payment（支付配置管理，平台端）----------

func crmPaymentCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleFinance); err != nil {
		return nil, err
	}
	var req services.PaymentConfigCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewPaymentConfigService().Create(req)
}

func crmPaymentList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	return services.NewPaymentConfigService().List()
}

func crmPaymentUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleFinance); err != nil {
		return nil, err
	}
	var req struct {
		ID   string                          `json:"id"`
		Data services.PaymentConfigUpdateReq `json:"data"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewPaymentConfigService().Update(req.ID, req.Data); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmPaymentSetStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleFinance); err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewPaymentConfigService().SetStatus(req.ID, req.Status); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmPaymentDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewPaymentConfigService().Delete(req.ID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ---------- order（订单管理，平台端）----------

func crmOrderListAll(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	var req services.OrderListReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTenantOrderService().ListAllOrders(req)
}

func crmOrderManualConfirmPay(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleFinance); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	if err := services.NewTenantOrderService().ManualConfirmPay(req.ID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ---------- tenant_order（租户端订单）----------

func crmTenantOrderCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	tenantID, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.OrderCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTenantOrderService().Create(tenantID, req)
}

func crmTenantOrderList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	tenantID, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewTenantOrderService().ListOrders(tenantID)
}

func crmTenantOrderGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	tenantID, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTenantOrderService().GetOrder(tenantID, req.ID)
}

func crmTenantOrderQueryPayStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	tenantID, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewTenantOrderService().QueryPayStatus(tenantID, req.ID)
}

func crmTenantOrderGetQrCode(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	tenantID, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	qr, err := services.NewTenantOrderService().GetQrCode(tenantID, req.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"qr_code_base64": qr}, nil
}

func crmTenantOrderCancel(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	tenantID, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if err := services.NewTenantOrderService().CancelOrder(tenantID, req.ID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

func crmTenantOrderListPaymentMethods(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	// 租户端列出启用的支付方式（不需要平台身份，但需要租户身份）
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	return services.NewPaymentConfigService().ListEnabled()
}

func crmPlatformLogin(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.Username == "" || req.Password == "" {
		return nil, errors.New("username/password 不能为空")
	}
	return services.NewPlatformAuth().Login(req.Username, req.Password)
}

// ---------- 平台账号管理（super_admin 专属）----------

func crmPlatformAdminList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin); err != nil {
		return nil, err
	}
	var req services.PlatformAdminListReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.PlatformAdminService{}.ListAdmins(req)
}

func crmPlatformAdminCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	adminID, err := requirePlatformRole(ctx, platformRoleSuperAdmin)
	if err != nil {
		return nil, err
	}
	var req services.PlatformAdminCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	admin, err := services.PlatformAdminService{}.CreateAdmin(req)
	if err != nil {
		return nil, err
	}
	platformLog(ctx, adminID, "admin.create", "admin", admin.ID, fmt.Sprintf("创建平台账号 %s (%s)", admin.Username, admin.Role))
	return admin, nil
}

func crmPlatformAdminUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	adminID, err := requirePlatformRole(ctx, platformRoleSuperAdmin)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID   string                          `json:"id"`
		Data services.PlatformAdminUpdateReq `json:"data"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	svc := services.PlatformAdminService{}
	if err := svc.UpdateAdmin(req.ID, req.Data); err != nil {
		return nil, err
	}
	platformLog(ctx, adminID, "admin.update", "admin", req.ID, "更新平台账号")
	return map[string]any{"ok": true}, nil
}

func crmPlatformAdminResetPassword(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	adminID, err := requirePlatformRole(ctx, platformRoleSuperAdmin)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID          string `json:"id"`
		NewPassword string `json:"new_password"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	svc2 := services.PlatformAdminService{}
	if err := svc2.ResetAdminPassword(req.ID, req.NewPassword); err != nil {
		return nil, err
	}
	platformLog(ctx, adminID, "admin.reset_password", "admin", req.ID, "重置平台账号密码")
	return map[string]any{"ok": true}, nil
}

func crmPlatformAdminSetStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	adminID, err := requirePlatformRole(ctx, platformRoleSuperAdmin)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		Status int    `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	svc3 := services.PlatformAdminService{}
	if err := svc3.SetAdminStatus(req.ID, req.Status); err != nil {
		return nil, err
	}
	platformLog(ctx, adminID, "admin.set_status", "admin", req.ID, fmt.Sprintf("设置账号状态 %d", req.Status))
	return map[string]any{"ok": true}, nil
}

func crmPlatformAdminDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	adminID, err := requirePlatformRole(ctx, platformRoleSuperAdmin)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	svc4 := services.PlatformAdminService{}
	if err := svc4.DeleteAdmin(req.ID); err != nil {
		return nil, err
	}
	platformLog(ctx, adminID, "admin.delete", "admin", req.ID, "删除平台账号")
	return map[string]any{"ok": true}, nil
}

// ---------- 平台统计 ----------

func crmPlatformStatsOverview(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	return services.PlatformStatsService{}.Stats()
}

func crmPlatformStatsTenantUsage(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	var req struct {
		TenantID string `json:"tenant_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.TenantID == "" {
		return nil, errors.New("tenant_id 不能为空")
	}
	return services.PlatformStatsService{}.TenantUsage(req.TenantID)
}

func crmPlatformStatsAllTenantUsage(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	var req struct {
		Page     int `json:"page"`
		PageSize int `json:"page_size"`
	}
	_ = Bind(params, &req)
	return services.PlatformStatsService{}.AllTenantUsage(req.Page, req.PageSize)
}

// ---------- 平台操作日志 ----------

func crmPlatformLogList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requirePlatformRole(ctx, platformRoleSuperAdmin, platformRoleOperator, platformRoleFinance, platformRoleSupport); err != nil {
		return nil, err
	}
	var req services.PlatformLogListReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.PlatformLogService{}.ListLogs(req)
}

// platformLog 写入平台操作日志（从 context 提取 admin 信息）。
func platformLog(ctx context.Context, adminID, action, resource, resourceID, detail string) {
	var admin models.PlatformAdmin
	if err := models.DB.Where("id = ?", adminID).First(&admin).Error; err != nil {
		services.WriteLog(adminID, "", "", action, resource, resourceID, detail, "")
		return
	}
	services.WriteLog(adminID, admin.Name, admin.Role, action, resource, resourceID, detail, "")
}

func crmUserCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req services.UserCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	u, initial, err := services.NewUser().Create(ctx, req)
	if err != nil {
		return nil, err
	}
	// initial_password 仅在创建时返回一次：用户密码为空时系统生成的初始密码。
	return map[string]any{"user": u, "initial_password": initial}, nil
}

func crmUserQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var p QueryParams[struct{}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewUser().Query(ctx, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmUserSetPassword(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	// 管理员才能重置他人密码（避免销售越权改管理员密码）。
	if err := requireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	var req struct {
		ID       string `json:"id"`
		Password string `json:"password"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return nil, services.NewCrmAuth().SetPassword(ctx, req.ID, req.Password)
}

func crmUserUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	if err := requireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
		services.UserUpdateReq
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ID == "" {
		return nil, errors.New("id 不能为空")
	}
	return nil, services.NewUser().Update(ctx, req.ID, req.UserUpdateReq)
}

func crmUserBindWecom(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	if err := requireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	var req struct {
		UserID     string `json:"user_id"`
		StaffExtID string `json:"staff_ext_id"`
		CorpID     string `json:"corp_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	// corp_id 为空时从租户级 ChannelLink 查找
	if req.CorpID == "" {
		var link models.ChannelLink
		if models.DB.Scopes(models.TenantScope(identity.TenantID(ctx))).Where("channel = ? AND entity_type = ?", "wecom", "tenant").First(&link).Error == nil {
			req.CorpID = link.ExtCorpID
		}
	}
	if req.CorpID == "" {
		return nil, errors.New("corp_id 不能为空（租户未配置企微渠道）")
	}
	return nil, services.NewUser().BindWecom(ctx, req.UserID, req.CorpID, req.StaffExtID)
}

func crmUserUnbindWecom(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	if err := requireRole(ctx, "admin"); err != nil {
		return nil, err
	}
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.UserID == "" {
		return nil, errors.New("user_id 不能为空")
	}
	return nil, services.NewUser().UnbindWecom(ctx, req.UserID)
}

// crmAuthLogin 公开登录：凭证校验在 core，返回本地身份，JWT 由网关签发。
func crmAuthLogin(_ context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		TenantCode string `json:"tenant_code"`
		Username   string `json:"username"`
		Password   string `json:"password"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.TenantCode == "" || req.Username == "" || req.Password == "" {
		return nil, errors.New("tenant_code/username/password 不能为空")
	}
	return services.NewCrmAuth().Login(req.TenantCode, req.Username, req.Password)
}

func crmAuthChangePassword(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	if u == "" {
		return nil, errors.New("missing user identity")
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCrmAuth().ChangePassword(ctx, u, req.OldPassword, req.NewPassword)
}

// ---------- crm_customer ----------

func crmCustomerCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmCustomer
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().Create(ctx, identity.UserID(ctx), req)
}

func crmCustomerUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID  string             `json:"id"`
		Req models.CrmCustomer `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().Update(ctx, req.ID, req.Req)
}

func crmCustomerDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().Delete(ctx, req.IDs)
}

func crmCustomerGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().Get(ctx, req.ID)
}

func crmCustomerGetDetail(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().GetDetail(ctx, req.ID)
}

func crmCustomerQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Keyword       string `json:"keyword"`
			OwnerUserID   string `json:"owner_user_id"`
			Source        string `json:"source"`
			OutFlowStatus any    `json:"out_flow_status"`
			PoolStatus    any    `json:"pool_status"`
			Category      string `json:"category"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	// out_flow_status 经 GET 查询串到达时是字符串（"0"/"1"），POST body 则可能是数字，需兼容两种。
	outFlow := 0
	switch v := req.Query.OutFlowStatus.(type) {
	case float64:
		outFlow = int(v)
	case string:
		outFlow, _ = strconv.Atoi(v)
	}
	// pool_status: -1=不过滤(全部), 0=非公海, 1=公海。默认 -1。
	poolStatus := -1
	switch v := req.Query.PoolStatus.(type) {
	case float64:
		poolStatus = int(v)
	case string:
		poolStatus, _ = strconv.Atoi(v)
	}
	items, total, err := services.NewCrmCustomer().Query(ctx, req.Query.Keyword, EffectiveOwnerFilter(ctx, req.Query.OwnerUserID), req.Query.Source, outFlow, poolStatus, req.Query.Category, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- crm_lead ----------

func crmLeadCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmLead
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLead().Create(ctx, identity.UserID(ctx), req)
}

func crmLeadUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID  string         `json:"id"`
		Req models.CrmLead `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLead().Update(ctx, req.ID, req.Req)
}

func crmLeadDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLead().Delete(ctx, req.IDs)
}

func crmLeadGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLead().Get(ctx, req.ID)
}

func crmLeadQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status      string `json:"status"`
			OwnerUserID string `json:"owner_user_id"`
			Keyword     string `json:"keyword"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmLead().Query(ctx, req.Query.Status, EffectiveOwnerFilter(ctx, req.Query.OwnerUserID), req.Query.Keyword, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- crm_pipeline ----------

func crmStageCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req models.CrmPipelineStage
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmPipelineStage().Create(ctx, req)
}

func crmStageUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID  string                  `json:"id"`
		Req models.CrmPipelineStage `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmPipelineStage().Update(ctx, req.ID, req.Req)
}

func crmStageDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmPipelineStage().Delete(ctx, req.IDs)
}

func crmStageList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	return services.NewCrmPipelineStage().List(ctx)
}

// ---------- crm_opportunity ----------

func crmOpportunityCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmOpportunity
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOpportunity().Create(ctx, identity.UserID(ctx), req)
}

func crmOpportunityUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID  string                `json:"id"`
		Req models.CrmOpportunity `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOpportunity().Update(ctx, req.ID, req.Req)
}

func crmOpportunityDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOpportunity().Delete(ctx, req.IDs)
}

func crmOpportunityGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOpportunity().Get(ctx, req.ID)
}

func crmOpportunityQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status      string `json:"status"`
			StageID     string `json:"stage_id"`
			OwnerUserID string `json:"owner_user_id"`
			Keyword     string `json:"keyword"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmOpportunity().Query(ctx, req.Query.Status, req.Query.StageID, EffectiveOwnerFilter(ctx, req.Query.OwnerUserID), req.Query.Keyword, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmOpportunityMoveStage(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID      string `json:"id"`
		StageID string `json:"stage_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOpportunity().MoveStage(ctx, req.ID, req.StageID)
}

// ---------- crm_followup ----------

func crmFollowupCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmFollowup
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().Create(ctx, identity.UserID(ctx), req)
}

func crmFollowupDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().Delete(ctx, req.IDs)
}

func crmFollowupListByCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().ListByCustomer(ctx, req.CustomerID)
}

func crmFollowupListByOpportunity(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		OpportunityID string `json:"opportunity_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().ListByOpportunity(ctx, req.OpportunityID)
}

func crmFollowupListByLead(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		LeadID string `json:"lead_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFollowup().ListByLead(ctx, req.LeadID)
}

// ---------- crm_order ----------

func crmOrderCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmOrder
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOrder().Create(ctx, identity.UserID(ctx), req)
}

func crmOrderUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID  string          `json:"id"`
		Req models.CrmOrder `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOrder().Update(ctx, req.ID, req.Req)
}

func crmOrderDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmOrder().Delete(ctx, req.IDs)
}

func crmOrderQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status      string `json:"status"`
			OwnerUserID string `json:"owner_user_id"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmOrder().Query(ctx, req.Query.Status, EffectiveOwnerFilter(ctx, req.Query.OwnerUserID), req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- crm_report ----------

func crmReportSalesSummary(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().SalesSummary(ctx)
}

func crmReportPipelineFunnel(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		OwnerUserID string `json:"owner_user_id"`
	}
	_ = Bind(params, &req)
	return services.NewCrmReport().PipelineFunnel(ctx, EffectiveOwnerFilter(ctx, req.OwnerUserID))
}

func crmReportPipelineInspection(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		OwnerUserID string `json:"owner_user_id"`
	}
	_ = Bind(params, &req)
	return services.NewCrmReport().PipelineInspection(ctx, EffectiveOwnerFilter(ctx, req.OwnerUserID))
}

func crmReportChannelSummary(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().ChannelSummary(ctx)
}

// ---------- crm_activity ----------

func crmActivityCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmActivity
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmActivity().Create(ctx, identity.UserID(ctx), req)
}

func crmActivityUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID  string             `json:"id"`
		Req models.CrmActivity `json:"req"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmActivity().Update(ctx, req.ID, req.Req)
}

func crmActivitySetStatus(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return nil, services.NewCrmActivity().SetStatus(ctx, req.ID, req.Status)
}

func crmActivityDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmActivity().Delete(ctx, req.IDs)
}

func crmActivityGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmActivity().Get(ctx, req.ID)
}

func crmActivityQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Keyword      string `json:"keyword"`
			ActivityType string `json:"activity_type"`
			Status       string `json:"status"`
			OwnerUserID  string `json:"owner_user_id"`
			Channel      string `json:"channel"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmActivity().Query(ctx, req.Query.Keyword, req.Query.ActivityType, req.Query.Status, req.Query.OwnerUserID, req.Query.Channel, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- crm_channel_tag ----------

func crmChannelTagList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Channel string `json:"channel"`
	}
	if len(params) > 0 {
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
	}
	tags, err := services.NewCrmChannelTag().List(ctx, req.Channel)
	if err != nil {
		return nil, err
	}
	return Result(tags, int64(len(tags))), nil
}

func crmChannelTagCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmChannelTag
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmChannelTag().Create(ctx, req)
}

func crmChannelTagDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmChannelTag().Delete(ctx, req.IDs)
}

// crmDictAddWay 下发各渠道的添加/承接方式字典（crm_customer.add_way 展示）。
func crmDictAddWay(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	return constants.AddWayByChannel, nil
}

// ---------- crm_activity 参与明细 ----------

func crmActivityRecord(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmActivityRecord
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmActivity().Record(ctx, identity.UserID(ctx), req)
}

func crmActivityDeleteRecord(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmActivity().DeleteRecord(ctx, req.IDs)
}

func crmActivityListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			ActivityID string `json:"activity_id"`
			RecordType string `json:"record_type"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmActivity().ListRecords(ctx, req.Query.ActivityID, req.Query.RecordType, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmActivityListByCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmActivity().ListByCustomer(ctx, req.CustomerID)
}

// ---------- crm_channel_interaction ----------

func crmChannelInteractionCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmChannelInteraction
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmChannelInteraction().Create(ctx, identity.UserID(ctx), req)
}

func crmChannelInteractionDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmChannelInteraction().Delete(ctx, req.IDs)
}

func crmChannelInteractionQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			CustomerID string `json:"customer_id"`
			Channel    string `json:"channel"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmChannelInteraction().Query(ctx, req.Query.CustomerID, req.Query.Channel, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- channel config（渠道接入配置）----------

func crmChannelConfigCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.ChannelConfigReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmChannelConfig().Create(ctx, req)
}

func crmChannelConfigUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
		services.ChannelConfigReq
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmChannelConfig().Update(ctx, req.ID, req.ChannelConfigReq)
}

func crmChannelConfigQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p QueryParams[struct{}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmChannelConfig().Query(ctx, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmChannelConfigDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmChannelConfig().Delete(ctx, req.IDs)
}

func crmWecomEventLogQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var p QueryParams[struct {
		CorpID string `json:"corp_id"`
	}]
	if err := Bind(params, &p); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmWecomEventLog().Query(ctx, p.Query.CorpID, p.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}
