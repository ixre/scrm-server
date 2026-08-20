package dispatch

import (
	"context"
	"encoding/json"

	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- 活动效果漏斗 ----------

func crmReportActivityFunnel(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return services.NewCrmReport().ActivityFunnel(ctx)
}

// ---------- 优惠券 ----------

func crmCouponCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.CouponCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCoupon().Create(ctx, identity.UserID(ctx), req.Coupon)
}

func crmCouponUpdate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		ID string `json:"id"`
		services.CouponCreateReq
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCoupon().Update(ctx, req.ID, req.Coupon)
}

func crmCouponQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status string `json:"status"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmCoupon().Query(ctx, req.Query.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmCouponGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmCoupon().Get(ctx, req.ID)
}

func crmCouponDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmCoupon().Delete(ctx, req.IDs)
}

func crmCouponIssue(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CouponID   string `json:"coupon_id"`
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCoupon().Issue(ctx, req.CouponID, req.CustomerID)
}

func crmCouponVerify(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Code    string `json:"code"`
		OrderID string `json:"order_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCoupon().Verify(ctx, req.Code, req.OrderID)
}

func crmCouponListRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			CouponID string `json:"coupon_id"`
			Status   string `json:"status"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmCoupon().ListRecords(ctx, req.Query.CouponID, req.Query.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

// ---------- 裂变活动 ----------

func crmFissionCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.FissionCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFission().Create(ctx, req.Fission)
}

func crmFissionQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			Status string `json:"status"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmFission().Query(ctx, req.Query.Status, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFissionGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmFission().Get(ctx, req.ID)
}

func crmFissionDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmFission().Delete(ctx, req.IDs)
}

func crmFissionRecordInvite(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		FissionID          string `json:"fission_id"`
		InviterCustomerID  string `json:"inviter_customer_id"`
		InviteeCustomerID  string `json:"invitee_customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFission().RecordInvite(ctx, req.FissionID, req.InviterCustomerID, req.InviteeCustomerID)
}

func crmFissionListInvites(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			FissionID         string `json:"fission_id"`
			InviterCustomerID string `json:"inviter_customer_id"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmFission().ListInvites(ctx, req.Query.FissionID, req.Query.InviterCustomerID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmFissionStatsByInviter(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		FissionID         string `json:"fission_id"`
		InviterCustomerID string `json:"inviter_customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmFission().StatsByInviter(ctx, req.FissionID, req.InviterCustomerID)
}

// ---------- 落地页 ----------

func crmLandingPageCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req services.LandingPageCreateReq
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmLandingPage().Create(ctx, req.Page)
}

func crmLandingPageQuery(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	_, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		Query struct {
			ActivityID string `json:"activity_id"`
		} `json:"query"`
		Pager *app.Pager `json:"pager"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	items, total, err := services.NewCrmLandingPage().Query(ctx, req.Query.ActivityID, req.Pager)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmLandingPageGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmLandingPage().Get(ctx, req.ID)
}

func crmLandingPageDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	return services.NewCrmLandingPage().Delete(ctx, req.IDs)
}
