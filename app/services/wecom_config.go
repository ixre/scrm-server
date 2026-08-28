package services

import (
	"context"
	"errors"

	"core/app/constants"
	"core/app/models"
	"core/common/identity"
)

// ResolveWecomConfig validates ownership before selecting an enterprise. A local
// CRM session can omit corpID when exactly one enabled enterprise is configured.
func ResolveWecomConfig(ctx context.Context, corpID string) (models.CrmChannelConfig, error) {
	var configs []models.CrmChannelConfig
	db := models.CrossTenant().Where("channel = ? AND enabled = ?", constants.ChannelWecom, 1)
	if tenantID := identity.TenantID(ctx); tenantID != "" {
		db = db.Where("tenant_id = ?", tenantID)
	} else if corpID == "" {
		return models.CrmChannelConfig{}, errors.New("请指定企微企业")
	}
	if corpID != "" {
		db = db.Where("corp_id = ?", corpID)
	}
	if err := db.Limit(2).Find(&configs).Error; err != nil {
		return models.CrmChannelConfig{}, err
	}
	if len(configs) == 0 {
		return models.CrmChannelConfig{}, errors.New("企微企业未配置、已停用或不属于当前租户")
	}
	if len(configs) != 1 {
		return models.CrmChannelConfig{}, errors.New("租户接入了多个企微企业，请指定企业")
	}
	if err := models.RequirePlanModules(models.DB.WithContext(ctx), configs[0].TenantID, "channel_wecom"); err != nil {
		return models.CrmChannelConfig{}, err
	}
	if err := registerWecomClient(configs[0]); err != nil {
		return models.CrmChannelConfig{}, err
	}
	return configs[0], nil
}

// EachWecomCorp continues with the next enterprise after an individual failure.
func EachWecomCorp(ctx context.Context, run func(context.Context, string) error) error {
	var configs []models.CrmChannelConfig
	if err := models.CrossTenant().Where("channel = ? AND enabled = ?", constants.ChannelWecom, 1).Find(&configs).Error; err != nil {
		return err
	}
	var result error
	for _, cfg := range configs {
		if err := models.RequirePlanModules(models.DB.WithContext(ctx), cfg.TenantID, "channel_wecom"); err != nil {
			continue
		}
		if err := registerWecomClient(cfg); err != nil {
			result = errors.Join(result, err)
			continue
		}
		corpCtx := SyncTenantCtx(identity.WithIdentity(ctx, identity.Identity{TenantID: cfg.TenantID}), cfg.CorpID)
		if err := run(corpCtx, cfg.CorpID); err != nil {
			result = errors.Join(result, errors.New(cfg.CorpID+": "+err.Error()))
		}
	}
	return result
}
