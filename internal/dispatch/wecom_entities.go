package dispatch

import (
	"context"
	"core/api/pb"
	"core/app/models"
	"core/app/requests"
	"core/app/services"
	"core/common/app"
	"encoding/json"
)

// Typed HTTP endpoints use the same tenant/enterprise resolver as other actions.
func init() {
	Reg.Register("wecom_customer", "Get", func(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
		var req struct {
			ExtID string `json:"ext_id"`
		}
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
		item, err := services.NewCustomer().Get(ctx, req.ExtID, corpID, true)
		if err != nil {
			return nil, err
		}
		return wecomCustomerPB(item), nil
	})
	Reg.Register("wecom_customer", "Query", func(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
		var req QueryParams[requests.QueryCustomerReq]
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
		if req.Pager == nil {
			req.Pager = &app.Pager{}
		}
		req.Pager.SetDefault()
		items, total, err := services.NewCustomer().Query(ctx, req.Query, corpID, req.Pager)
		if err != nil {
			return nil, err
		}
		result := make([]*pb.Customer, 0, len(items))
		for _, item := range items {
			result = append(result, wecomCustomerPB(*item))
		}
		return Result(result, total), nil
	})
	Reg.Register("wecom_staff", "Get", func(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
		var req struct {
			ExtID string `json:"ext_id"`
		}
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
		item, err := services.NewStaffService().Get(ctx, req.ExtID, corpID)
		if err != nil {
			return nil, err
		}
		return wecomStaffPB(item), nil
	})
	Reg.Register("wecom_staff", "Query", func(ctx context.Context, corpID, _ string, params json.RawMessage) (any, error) {
		var req QueryParams[requests.QueryStaffReq]
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
		if req.Pager == nil {
			req.Pager = &app.Pager{}
		}
		if req.Sorter == nil {
			req.Sorter = &app.Sorter{}
		}
		req.Pager.SetDefault()
		items, total, err := services.NewStaffService().Query(ctx, req.Query, corpID, req.Sorter, req.Pager)
		if err != nil {
			return nil, err
		}
		result := make([]*pb.Staff, 0, len(items))
		for _, item := range items {
			result = append(result, wecomStaffPB(item))
		}
		return Result(result, total), nil
	})
}

func wecomCustomerPB(item models.Customer) *pb.Customer {
	return &pb.Customer{ExtId: item.ExtID, Name: item.Name, CorpId: item.ExtCorpID, Avatar: item.Avatar, CreatedAt: item.CreatedAt.Unix()}
}
func wecomStaffPB(item *models.Staff) *pb.Staff {
	return &pb.Staff{ExtId: item.ExtID, Name: item.Name, ExtCorpId: item.ExtCorpID, AvatarUrl: item.AvatarURL, Mobile: item.Mobile, RoleType: item.RoleType, Status: int32(item.Status)}
}
