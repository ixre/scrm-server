package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- A 组：SCRM 风控 dispatch handlers ----------

// ---- 敏感词 ----

func crmSensitiveWordList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Category string `json:"category"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSensitiveWord().List(t, req.Category, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSensitiveWordCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmSensitiveWord
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSensitiveWord().Create(t, req)
}

func crmSensitiveWordDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSensitiveWord().Delete(t, req.IDs)
}

func crmSensitiveAlertList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSensitiveAlert().List(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSensitiveAlertHandle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSensitiveAlert().Handle(t, req.IDs, req.Status)
}

// ---- 客户继承 ----

func crmInheritanceExecute(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		FromUserID string `json:"from_user_id"`
		ToUserID   string `json:"to_user_id"`
		Type       string `json:"type"`
		Remark     string `json:"remark"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmInheritance().Execute(t, req.FromUserID, req.ToUserID, req.Type, req.Remark)
}

func crmInheritanceList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct{ *app.Pager }
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmInheritance().List(t, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 客户流失 ----

func crmCustomerLossList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		LossType string `json:"loss_type"`
		Status   string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmCustomerLoss().List(t, req.LossType, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCustomerLossHandle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomerLoss().Handle(t, req.IDs, req.Status)
}

// ---- 超时未回复 ----

func crmReplyTimeoutList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmReplyTimeout().List(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmReplyTimeoutHandle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs    []string `json:"ids"`
		Status string   `json:"status"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmReplyTimeout().Handle(t, req.IDs, req.Status)
}

// ---- 群SOP ----

func crmGroupSopList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct{ *app.Pager }
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmGroupSop().List(t, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmGroupSopCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmGroupSop
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupSop().Create(t, req)
}

func crmGroupSopDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupSop().Delete(t, req.IDs)
}

func crmGroupSopListTasks(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmGroupSop().ListTasks(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmGroupSopDoneTask(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if err := services.NewCrmGroupSop().DoneTask(t, req.ID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ---------- B 组：营销活动补充 dispatch handlers ----------

// ---- 拼团 ----

func crmGroupBuyList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmGroupBuy().Query(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmGroupBuyCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmGroupBuy
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupBuy().Create(t, req)
}

func crmGroupBuyDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupBuy().Delete(t, req.IDs)
}

func crmGroupBuyOpenGroup(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		GroupBuyID string `json:"group_buy_id"`
		LeaderID   string `json:"leader_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupBuy().OpenGroup(t, req.GroupBuyID, req.LeaderID)
}

func crmGroupBuyJoinGroup(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		RecordID string `json:"record_id"`
		MemberID string `json:"member_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupBuy().JoinGroup(t, req.RecordID, req.MemberID)
}

func crmGroupBuyListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		GroupBuyID string `json:"group_buy_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmGroupBuy().ListRecords(t, req.GroupBuyID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 砍价 ----

func crmBargainList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmBargain().Query(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmBargainCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmBargain
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmBargain().Create(t, req)
}

func crmBargainDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmBargain().Delete(t, req.IDs)
}

func crmBargainStart(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		BargainID  string `json:"bargain_id"`
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmBargain().Start(t, req.BargainID, req.CustomerID)
}

func crmBargainHelp(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		RecordID string `json:"record_id"`
		HelperID string `json:"helper_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmBargain().Help(t, req.RecordID, req.HelperID)
}

func crmBargainListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		BargainID string `json:"bargain_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmBargain().ListRecords(t, req.BargainID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 满减满件 ----

func crmFullReductionList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmFullReduction().Query(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFullReductionCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmFullReduction
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFullReduction().Create(t, req)
}

func crmFullReductionDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFullReduction().Delete(t, req.IDs)
}

func crmFullReductionCalculate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID    string  `json:"id"`
		Value float64 `json:"value"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	discount, err := services.NewCrmFullReduction().CalculateDiscount(t, req.ID, req.Value)
	if err != nil {
		return nil, err
	}
	return map[string]any{"discount": discount}, nil
}

// ---- 红包获客 ----

func crmRedPacketList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmRedPacket().Query(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmRedPacketCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmRedPacket
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmRedPacket().Create(t, req)
}

func crmRedPacketDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmRedPacket().Delete(t, req.IDs)
}

func crmRedPacketClaim(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		RedPacketID string `json:"red_packet_id"`
		CustomerID  string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmRedPacket().Claim(t, req.RedPacketID, req.CustomerID)
}

func crmRedPacketListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		RedPacketID string `json:"red_packet_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmRedPacket().ListRecords(t, req.RedPacketID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 签到送券 ----

func crmSignInList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Status string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSignIn().Query(t, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSignInCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmSignIn
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSignIn().Create(t, req)
}

func crmSignInDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSignIn().Delete(t, req.IDs)
}

func crmSignInCheckIn(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		SignInID   string `json:"sign_in_id"`
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSignIn().CheckIn(t, req.SignInID, req.CustomerID)
}

func crmSignInListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		SignInID string `json:"sign_in_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSignIn().ListRecords(t, req.SignInID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- C4 智能名片 dispatch handlers ----------

func crmSmartCardList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		UserID string `json:"user_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSmartCard().Query(t, req.UserID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSmartCardCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	uid := identity.UserID(ctx)
	var req models.CrmSmartCard
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.UserID == "" {
		req.UserID = uid
	}
	return services.NewCrmSmartCard().Create(t, req)
}

func crmSmartCardDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSmartCard().Delete(t, req.IDs)
}

func crmSmartCardListViews(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CardID string `json:"card_id"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmSmartCard().ListViews(t, req.CardID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}
