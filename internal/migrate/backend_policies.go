package migrate

import (
	"fmt"
	"gorm.io/gorm"
)

// AutoMigrate does not install quota/session triggers or migrate permission
// grants. Never start business traffic with only the new Go structs installed.
func VerifyBackendPolicies(db *gorm.DB) error {
	versions := []string{"0031_plan_entitlement_quotas.sql", "0032_crm_permission_points.sql", "0033_account_session_versions.sql", "0034_platform_permission_points.sql"}
	var count int64
	if err := db.Table(schemaMigrationsTable).Where("version IN ?", versions).Count(&count).Error; err != nil || count != int64(len(versions)) {
		return fmt.Errorf("backend policy migrations 0031–0034 are required; run go run ./cmd/migrate -schema-only")
	}
	return nil
}
