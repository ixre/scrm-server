package dispatch

import (
	"context"
	"encoding/json"

	"core/app/models"
	"core/app/services"
)

// ---------- P2: 动态字段/自定义表单 dispatch handlers ----------

// ---- 字段定义 ----

func crmFieldDefList(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		EntityType string `json:"entity_type"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmField().ListDefs(t, req.EntityType)
}

func crmFieldDefCreate(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var d models.CrmFieldDef
	if err := Bind(params, &d); err != nil {
		return nil, err
	}
	return ok, services.NewCrmField().CreateDef(t, &d)
}

func crmFieldDefUpdate(ctx context.Context, _, id string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var d models.CrmFieldDef
	if err := Bind(params, &d); err != nil {
		return nil, err
	}
	return ok, services.NewCrmField().UpdateDef(t, id, &d)
}

func crmFieldDefDelete(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
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
	{ _, e := services.NewCrmField().DeleteDef(t, req.IDs); return ok, e }
}

// ---- 字段值 ----

func crmFieldValueGet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return services.NewCrmField().GetValues(t, req.EntityType, req.EntityID)
}

func crmFieldValueSet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		EntityType string `json:"entity_type"`
		EntityID   string `json:"entity_id"`
		FieldKey   string `json:"field_key"`
		Value      string `json:"value"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmField().SetValue(t, req.EntityType, req.EntityID, req.FieldKey, req.Value)
}

func crmFieldValueBatchSet(ctx context.Context, _, _ string, params json.RawMessage) (any, error) {
	t, err := requireIdentity(ctx)
	if err != nil {
		return nil, err
	}
	var req struct {
		EntityType string            `json:"entity_type"`
		EntityID   string            `json:"entity_id"`
		Values     map[string]string `json:"values"`
	}
	if err := Bind(params, &req); err != nil {
		return nil, err
	}
	return ok, services.NewCrmField().BatchSetValues(t, req.EntityType, req.EntityID, req.Values)
}
