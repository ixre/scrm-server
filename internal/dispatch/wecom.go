package dispatch

import (
	"context"
	"core/app/services"
	"encoding/json"
)

func init() {
	Reg.Register("crm_wecom", "AuthorizeURL", func(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
		return services.WecomAuthorizeURL(ctx)
	})
	Reg.Register("crm_wecom", "CompleteAuthorization", func(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
		var req struct {
			State string `json:"state"`
			Code  string `json:"auth_code"`
		}
		if err := Bind(params, &req); err != nil {
			return nil, err
		}
		return services.CompleteWecomAuthorization(ctx, req.State, req.Code)
	})
}
