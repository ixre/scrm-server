package dispatch

import (
	"context"
	"encoding/json"
	"errors"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- A 组：SCRM 风控 dispatch handlers ----------

// ---- 敏感词 ----

func crmSensitiveWordList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmSensitiveWord().List(ctx, req.Category, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSensitiveWordCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmSensitiveWord
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSensitiveWord().Create(ctx, req)
}

func crmSensitiveWordDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSensitiveWord().Delete(ctx, req.IDs)
}

func crmSensitiveAlertList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmSensitiveAlert().List(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSensitiveAlertHandle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmSensitiveAlert().Handle(ctx, req.IDs, req.Status)
}

// ---- 客户继承 ----

func crmInheritanceExecute(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmInheritance().Execute(ctx, req.FromUserID, req.ToUserID, req.Type, req.Remark)
}

func crmInheritanceList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct{ *app.Pager }
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmInheritance().List(ctx, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 客户流失 ----

func crmCustomerLossList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmCustomerLoss().List(ctx, req.LossType, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCustomerLossHandle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmCustomerLoss().Handle(ctx, req.IDs, req.Status)
}

// ---- 超时未回复 ----

func crmReplyTimeoutList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmReplyTimeout().List(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmReplyTimeoutHandle(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmReplyTimeout().Handle(ctx, req.IDs, req.Status)
}

// ---- 群SOP ----

func crmGroupSopList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct{ *app.Pager }
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmGroupSop().List(ctx, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmGroupSopCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmGroupSop
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupSop().Create(ctx, req)
}

func crmGroupSopDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmGroupSop().Delete(ctx, req.IDs)
}

func crmGroupSopListTasks(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmGroupSop().ListTasks(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmGroupSopDoneTask(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if err := services.NewCrmGroupSop().DoneTask(ctx, req.ID); err != nil {
		return nil, err
	}
	return map[string]any{"ok": true}, nil
}

// ---------- B 组：营销活动补充 dispatch handlers ----------

// ---- 拼团 ----

func crmGroupBuyList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmGroupBuy().Query(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmGroupBuyCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmGroupBuy
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmGroupBuy().Create(ctx, req)
}

func crmGroupBuyDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmGroupBuy().Delete(ctx, req.IDs)
}

func crmGroupBuyOpenGroup(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmGroupBuy().OpenGroup(ctx, req.GroupBuyID, req.LeaderID)
}

func crmGroupBuyJoinGroup(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmGroupBuy().JoinGroup(ctx, req.RecordID, req.MemberID)
}

func crmGroupBuyListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmGroupBuy().ListRecords(ctx, req.GroupBuyID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 砍价 ----

func crmBargainList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmBargain().Query(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmBargainCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmBargain
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmBargain().Create(ctx, req)
}

func crmBargainDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmBargain().Delete(ctx, req.IDs)
}

func crmBargainStart(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmBargain().Start(ctx, req.BargainID, req.CustomerID)
}

func crmBargainHelp(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmBargain().Help(ctx, req.RecordID, req.HelperID)
}

func crmBargainListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmBargain().ListRecords(ctx, req.BargainID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 满减满件 ----

func crmFullReductionList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmFullReduction().Query(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFullReductionCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmFullReduction
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFullReduction().Create(ctx, req)
}

func crmFullReductionDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmFullReduction().Delete(ctx, req.IDs)
}

func crmFullReductionCalculate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	discount, err := services.NewCrmFullReduction().CalculateDiscount(ctx, req.ID, req.Value)
	if err != nil {
		return nil, err
	}
	return map[string]any{"discount": discount}, nil
}

// ---- 红包获客 ----

func crmRedPacketList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmRedPacket().Query(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmRedPacketCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmRedPacket
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmRedPacket().Create(ctx, req)
}

func crmRedPacketDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmRedPacket().Delete(ctx, req.IDs)
}

func crmRedPacketClaim(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmRedPacket().Claim(ctx, req.RedPacketID, req.CustomerID)
}

func crmRedPacketListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmRedPacket().ListRecords(ctx, req.RedPacketID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---- 签到送券 ----

func crmSignInList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmSignIn().Query(ctx, req.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSignInCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmSignIn
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSignIn().Create(ctx, req)
}

func crmSignInDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSignIn().Delete(ctx, req.IDs)
}

func crmSignInCheckIn(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmSignIn().CheckIn(ctx, req.SignInID, req.CustomerID)
}

func crmSignInListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmSignIn().ListRecords(ctx, req.SignInID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- C4 智能名片 dispatch handlers ----------

func crmSmartCardList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	uid := identity.UserID(ctx)
	var req struct {
		UserID string `json:"user_id"`
		Scope  string `json:"scope"` // "mine"=只看自己, "all"=全部(管理员), ""=默认看自己
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	// 默认只看自己的名片，除非显式传 scope=all
	filterUID := req.UserID
	if filterUID == "" && req.Scope != "all" {
		filterUID = uid
	}
	items, total, err := services.NewCrmSmartCard().Query(ctx, filterUID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSmartCardCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	return services.NewCrmSmartCard().Create(ctx, req)
}

func crmSmartCardUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req models.CrmSmartCard
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmSmartCard().Update(ctx, id, req)
}

func crmSmartCardDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmSmartCard().Delete(ctx, req.IDs)
}

func crmSmartCardListViews(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
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
	items, total, err := services.NewCrmSmartCard().ListViews(ctx, req.CardID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmSmartCardStats(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	uid := identity.UserID(ctx)
	var req struct {
		Scope string `json:"scope"`
	}
	_ = Bind(params, &req)
	filterUID := ""
	if req.Scope != "all" {
		filterUID = uid
	}
	stats, err := services.NewCrmSmartCard().Stats(ctx, filterUID)
	if err != nil {
		return nil, err
	}
	return stats, nil
}

// crmSmartCardPublicGet 公开获取名片（无需登录，按分享码或ID查询）。
func crmSmartCardPublicGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ShareCode string `json:"share_code"`
		ID        string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	svc := services.NewCrmSmartCard()
	if req.ShareCode != "" {
		card, err := svc.GetByShareCode(req.ShareCode)
		if err != nil {
			return nil, err
		}
		// 异步记录浏览
		go func() {
			defer func() { recover() }()
			ctx2 := identity.WithIdentity(context.Background(), identity.Identity{TenantID: card.TenantID})
			svc.RecordView(ctx2, card.ID, "", "", "", "share", 0)
		}()
		return card, nil
	}
	if req.ID != "" {
		return svc.GetByID(req.ID)
	}
	return nil, errors.New("share_code 或 id 不能为空")
}

// crmSmartCardPublicSubmitLead 公开留资（无需登录，客户在H5名片页提交线索）。
func crmSmartCardPublicSubmitLead(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	var req struct {
		ShareCode string `json:"share_code"`
		Name      string `json:"name"`
		Mobile    string `json:"mobile"`
		Remark    string `json:"remark"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if req.ShareCode == "" {
		return nil, errors.New("share_code 不能为空")
	}
	if req.Name == "" || req.Mobile == "" {
		return nil, errors.New("姓名和手机号不能为空")
	}
	// 通过分享码找到名片，推导出租户ID和归属销售
	card, err := services.NewCrmSmartCard().GetByShareCode(req.ShareCode)
	if err != nil {
		return nil, errors.New("名片不存在")
	}
	// 用名片的租户和销售身份创建线索
	ctx2 := identity.WithIdentity(context.Background(), identity.Identity{
		TenantID: card.TenantID,
		UserID:   card.UserID,
	})
	lead := models.CrmLead{
		Name:        req.Name,
		Mobile:      req.Mobile,
		Remark:      req.Remark,
		Source:      "smart_card",
		Status:      "new",
		OwnerUserID: card.UserID,
		Title:       "名片留资-" + req.Name,
	}
	created, err := services.CrmLeadService{}.Create(ctx2, card.UserID, lead)
	if err != nil {
		return nil, err
	}
	return created, nil
}
