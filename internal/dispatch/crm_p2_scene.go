package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
	"core/common/app"
	"core/common/identity"
)

// ---------- P2: 场景 dispatch handlers ----------

func crmSceneList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		EntityType string `json:"entity_type"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmScene().List(t, u, req.EntityType)
}

func crmSceneCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var sc models.CrmScene
	if err := Bind(params, &sc); err != nil {
		return nil, err
	}
	return ok, services.NewCrmScene().Create(t, u, &sc)
}

func crmSceneUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var sc models.CrmScene
	if err := Bind(params, &sc); err != nil {
		return nil, err
	}
	return ok, services.NewCrmScene().Update(t, u, id, &sc)
}

func crmSceneDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{ _, e := services.NewCrmScene().Delete(t, u, req.IDs); return ok, e }
}

func crmSceneSetDefault(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	return ok, services.NewCrmScene().SetDefault(t, u, id)
}

func crmSceneDragSort(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	u := identity.UserID(ctx)
	var req struct {
		IDs []string `json:"ids"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	return ok, services.NewCrmScene().DragSort(t, u, req.IDs)
}

// ---------- P2: 客户消费 dispatch handlers ----------

func crmConsumeList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
		Type       string `json:"type"`
		Status     string `json:"status"`
		*app.Pager
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	q := services.ConsumeQueryReq{
		Page: req.Page, PageSize: req.PageSize,
		CustomerID: req.CustomerID, Type: req.Type, Status: req.Status,
	}
	items, total, err := services.NewCrmConsume().Query(t, q)
	if err != nil {
		return nil, err
	}
	return Result(items, total), nil
}

func crmConsumeCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmConsume
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmConsume().Create(t, &c)
}

func crmConsumeUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var c models.CrmConsume
	if err := Bind(params, &c); err != nil {
		return nil, err
	}
	return ok, services.NewCrmConsume().Update(t, id, &c)
}

func crmConsumeDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	if len(req.IDs) == 0 {
		return nil, errNeedIdentity
	}
	{ _, e := services.NewCrmConsume().Delete(t, req.IDs); return ok, e }
}

func crmConsumeRevoke(ctx context.Context, _, id string, _ json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	return ok, services.NewCrmConsume().Revoke(t, id)
}

func crmConsumeSummary(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		CustomerID string `json:"customer_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmConsume().Summary(t, req.CustomerID)
}
