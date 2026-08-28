package models

import (
	"fmt"
	"strings"

	"core/common/id_generator"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// EnsureWecomTenantLink must run in the same transaction as channel provisioning.
// The unique mapping index also protects concurrent configuration requests.
func EnsureWecomTenantLink(tx *gorm.DB, tenantID, corpID string) error {
	corpID = strings.TrimSpace(corpID)
	if tenantID == "" || corpID == "" {
		return fmt.Errorf("租户和企业 ID 不能为空")
	}
	lookup := func() ([]ChannelLink, error) {
		var links []ChannelLink
		// 企业 → 租户映射全局唯一，查重必须跨租户。
		err := CrossTenantDB(tx).Where("channel = 'wecom' AND entity_type = 'tenant' AND ext_id = ?", corpID).Limit(2).Find(&links).Error
		return links, err
	}
	links, err := lookup()
	if err != nil {
		return err
	}
	if len(links) == 0 {
		link := ChannelLink{Model: Model{ID: id_generator.StringID()}, TenantID: tenantID, EntityID: tenantID,
			Channel: "wecom", EntityType: "tenant", ExtID: corpID, ExtCorpID: corpID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
			return err
		}
		links, err = lookup()
		if err != nil {
			return err
		}
	}
	if len(links) != 1 || links[0].TenantID != tenantID || links[0].EntityID != tenantID {
		return fmt.Errorf("该企业已绑定其他租户或存在无效映射")
	}
	return nil
}
