package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 跟进扩展 dispatch handlers ----------

// ---- 跟进评论 ----

func crmFollowupCommentList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		FollowupID string `json:"followup_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, err := services.NewCrmFollowupComment().ListByFollowup(ctx, req.FollowupID)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmFollowupCommentCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		FollowupID string `json:"followup_id"`
		Content    string `json:"content"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	c, err := services.NewCrmFollowupComment().Create(ctx, req.FollowupID, u, req.Content)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func crmFollowupCommentDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmFollowupComment().Delete(ctx, u, req.ID)
}

// ---- 常用备注 ----

func crmQuickRemarkList(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	items, err := services.NewCrmQuickRemark().List(ctx, u)
	if err != nil {
		return nil, err
	}
	return Result(items, int64(len(items))), nil
}

func crmQuickRemarkCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		Content string `json:"content"`
		Sort    *int   `json:"sort"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	sort := []int{}
	if req.Sort != nil {
		sort = append(sort, *req.Sort)
	}
	r, err := services.NewCrmQuickRemark().Create(ctx, u, req.Content, sort...)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func crmQuickRemarkUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ID      string `json:"id"`
		Content string `json:"content"`
		Sort    *int   `json:"sort"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	sort := []int{}
	if req.Sort != nil {
		sort = append(sort, *req.Sort)
	}
	return ok, services.NewCrmQuickRemark().Update(ctx, u, req.ID, req.Content, sort...)
}

func crmQuickRemarkDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuickRemark().Delete(ctx, u, req.ID)
}

func crmQuickRemarkSetSort(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID   string `json:"id"`
		Sort int    `json:"sort"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuickRemark().SetSort(ctx, req.ID, req.Sort)
}

// ---- 跟进视图 ----

func crmFollowupViewTodayTodo(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		app.Pager
		Keyword string `json:"keyword"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PageReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmFollowupView().ListTodayTodo(ctx, u, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFollowupViewTodayDone(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		app.Pager
		Keyword string `json:"keyword"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PageReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmFollowupView().ListTodayDone(ctx, u, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFollowupViewNeverFollowup(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		app.Pager
		Keyword string `json:"keyword"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PageReq{Page: req.Page, PageSize: req.PageSize, Keyword: req.Keyword}
	items, total, err := services.NewCrmFollowupView().ListNeverFollowup(ctx, u, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- 商机扩展 dispatch handlers ----------

func crmOpportunityChangeStage(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return ok, services.NewCrmOpportunityExt().ChangeStage(ctx, req.ID, req.StageID)
}

func crmOpportunityWin(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID           string  `json:"id"`
		WonReason    string  `json:"won_reason"`
		ActualAmount float64 `json:"actual_amount"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOpportunityExt().Win(ctx, req.ID, req.WonReason, req.ActualAmount)
}

func crmOpportunityLose(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID         string `json:"id"`
		LostReason string `json:"lost_reason"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOpportunityExt().Lose(ctx, req.ID, req.LostReason)
}

func crmOpportunityBatchChangeStage(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs     []string `json:"ids"`
		StageID string   `json:"stage_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmOpportunityExt().BatchChangeStage(ctx, req.IDs, req.StageID)
}

func crmOpportunityReopen(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOpportunityExt().Reopen(ctx, req.ID)
}

// ---------- 合同扩展 dispatch handlers ----------

func crmOrderRevoke(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderExt().Revoke(ctx, req.ID, req.Reason)
}

func crmOrderChange(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID      string         `json:"id"`
		Updates map[string]any `json:"updates"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderExt().Change(ctx, req.ID, req.Updates)
}

func crmOrderLinkProject(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID        string `json:"id"`
		ProjectID string `json:"project_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderExt().LinkProject(ctx, req.ID, req.ProjectID)
}

func crmOrderLinkWorkorder(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID          string `json:"id"`
		WorkorderID string `json:"workorder_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmOrderExt().LinkWorkorder(ctx, req.ID, req.WorkorderID)
}

// ---------- 回款扩展 dispatch handlers ----------

func crmReceivablesGeneratePlan(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		OrderID string                `json:"order_id"`
		Periods []services.PlanPeriod `json:"periods"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivablesExt().GeneratePlan(ctx, req.OrderID, req.Periods)
}

func crmReceivablesListExpiring(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Days int `json:"days"`
		app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PageReq{Page: req.Page, PageSize: req.PageSize}
	items, total, err := services.NewCrmReceivablesExt().ListExpiring(ctx, req.Days, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmReceivablesListOverdue(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PageReq{Page: req.Page, PageSize: req.PageSize}
	items, total, err := services.NewCrmReceivablesExt().ListOverdue(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmReceivablesApprove(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ID string `json:"id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivablesExt().Approve(ctx, req.ID, u)
}

func crmReceivablesReject(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ID     string `json:"id"`
		Remark string `json:"remark"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmReceivablesExt().Reject(ctx, req.ID, u, req.Remark)
}

// ---------- 报价扩展 dispatch handlers ----------

func crmQuoteGenerateNo(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	no, err := services.NewCrmQuoteExt().GenerateNo(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return map[string]any{"quote_no": no}, nil
}

func crmQuoteVoid(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID     string `json:"id"`
		Reason string `json:"reason"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuoteExt().Void(ctx, req.ID, req.Reason)
}

func crmQuoteExtTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID       string `json:"id"`
		ToUserID string `json:"to_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmQuoteExt().Transfer(ctx, req.ID, req.ToUserID)
}

func crmQuoteBatchTransfer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		IDs      []string `json:"ids"`
		ToUserID string   `json:"to_user_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmQuoteExt().BatchTransfer(ctx, req.IDs, req.ToUserID)
}

// ---------- 联系人扩展 dispatch handlers ----------

func crmContactConvertToCustomer(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		ContactID string `json:"contact_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	c, err := services.NewCrmContactExt().ConvertToCustomer(ctx, u, req.ContactID)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func crmContactListPool(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.PageReq{Page: req.Page, PageSize: req.PageSize}
	items, total, err := services.NewCrmContactExt().ListPool(ctx, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}
