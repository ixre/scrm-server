package dispatch

import (
	"context"
	"core/app/services"
	"encoding/json"
)

func crmCustomerHistory(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req services.CustomerHistoryRequest
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().History(ctx, req)
}
func crmUserOptions(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req services.UserOptionsRequest
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewUser().Options(ctx, req)
}

func crmCustomerBusinessSummary(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().BusinessSummary(ctx, req.CustomerID)
}
func crmCustomerBusinessRecords(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	if _, err := requireIdentity(ctx); err != nil {
		return nil, err
	}
	var req services.CustomerBusinessRequest
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmCustomer().BusinessRecords(ctx, req)
}
