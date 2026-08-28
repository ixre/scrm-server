package dispatch

import (
	"context"
	"core/app/services"
	"encoding/json"
)

func init() {
	Reg.Register("crm_user", "RoleOptions", func(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
		return services.CRMRoleOptions(ctx)
	})
	Reg.Register("crm_role", "List", func(ctx context.Context, _, _ string, _ json.RawMessage) (any, error) {
		return services.ListCRMRoles(ctx)
	})
	for _, action := range []string{"Create", "Update"} {
		currentAction := action
		Reg.Register("crm_role", action, func(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
			var req struct {
				ID string `json:"id"`
				services.CRMRoleInput
			}
			if err := Bind(params, &req); err != nil {
				return nil, err
			}
			if currentAction == "Create" {
				req.ID = ""
			} else if req.ID == "" {
				return nil, services.ErrPermissionDenied
			}
			return services.SaveCRMRole(ctx, req.ID, req.CRMRoleInput)
		})
	}
	Reg.Register("crm_role", "Delete", func(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
		id, err := bindActionID(params)
		if err != nil {
			return nil, err
		}
		return nil, services.DeleteCRMRole(ctx, id)
	})
}
