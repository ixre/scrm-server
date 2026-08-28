package services

import (
	"core/app/constants"
	"core/app/models"
	"core/common/identity"
	"core/conf"
	"core/migrations"
	"strings"
	"testing"
)

func TestPermissionPointsIgnoreRoleNames(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "11")
	mustCreate(t, db, &models.CrmRole{Model: models.Model{ID: "900"}, TenantID: "1", Code: "custom_reviewer", Perms: constants.StringArrayField{"crm:customer:read", "crm:data:all", "crm:risk:full"}})
	requireNoError(t, db.Model(&models.User{}).Where("id=11").Update("role_id", "900").Error)
	requireNoError(t, ValidateBusinessPermission(ctx, "crm_customer", "Query"))
	if ValidateBusinessPermission(ctx, "crm_customer", "Delete") == nil {
		t.Fatal("read permission allowed mutation")
	}
	_, all, err := customerActor(ctx, models.DB)
	requireNoError(t, err)
	if !all {
		t.Fatal("custom role's explicit data permission ignored")
	}
	requireNoError(t, ValidateBusinessPermission(ctx, "crm_sensitive_alert", "Handle"))
	requireNoError(t, RequireBusinessManagement(WithBusinessPermission(ctx, "crm_sensitive_alert", "Handle")))
	// Renaming a role to admin confers no privileges; removing grants revokes immediately.
	requireNoError(t, db.Model(&models.CrmRole{}).Where("id=2").Update("perms", constants.StringArrayField{}).Error)
	if ValidateBusinessPermission(testIdentity("1", "12"), "crm_customer", "Query") == nil {
		t.Fatal("admin name granted implicit permission")
	}
	if HasCRMDataAccess(testIdentity("1", "12")) {
		t.Fatal("admin name granted tenant-wide data access")
	}
	requireNoError(t, db.Model(&models.CrmRole{}).Where("id=900").Update("perms", constants.StringArrayField{}).Error)
	if ValidateBusinessPermission(ctx, "crm_customer", "Query") == nil {
		t.Fatal("removed permission stayed cached")
	}
}

func TestAccountAndRoleManagementCannotEscalate(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	admin := testIdentity("1", "12")
	role, err := SaveCRMRole(admin, "", CRMRoleInput{Code: "reviewer", Name: "审核员", Perms: []string{"crm:customer:read", "crm:role:manage", "crm:user:create"}})
	requireNoError(t, err)
	requireNoError(t, db.Model(&models.User{}).Where("id=11").Update("role_id", role.ID).Error)
	custom := testIdentity("1", "11")
	if _, err := SaveCRMRole(custom, "", CRMRoleInput{Code: "escalated", Name: "越权", Perms: []string{"crm:*:full"}}); err == nil {
		t.Fatal("delegated operator granted unowned permissions")
	}
	if _, _, err := NewUser().Create(custom, UserCreateReq{Username: "escalated", RoleID: "2", Password: "StrongPass123"}); err == nil {
		t.Fatal("operator created stronger account")
	}
	if err := NewCrmAuth().SetPassword(custom, "12", "StrongPass123"); err == nil {
		t.Fatal("operator reset stronger account")
	}
	if err := NewUser().Update(admin, "12", UserUpdateReq{RoleID: role.ID}); err == nil {
		t.Fatal("last tenant administrator demoted")
	}
	if err := DeleteCRMRole(admin, role.ID); err == nil {
		t.Fatal("assigned role deleted")
	}
	if _, err := SaveCRMRole(admin, "2", CRMRoleInput{Code: "admin", Name: "admin", Perms: []string{}}); err == nil {
		t.Fatal("operator edited own role")
	}
}

func TestAccountSessionVersionsRevokeOldTokens(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	mustCreate(t, db, &models.PlatformAdmin{Model: models.Model{ID: "900"}, Username: "operator", Status: 1})
	requireNoError(t, db.AutoMigrate(&models.Staff{}))
	content, err := migrations.Files.ReadFile("0033_account_session_versions.sql")
	requireNoError(t, err)
	for _, stmt := range strings.Split(string(content), "-- migrate:statement") {
		requireNoError(t, db.Exec(stmt).Error)
	}
	ctx := testIdentity("1", "10")
	if _, err := ValidateBusinessSession(ctx, false, ""); err == nil {
		t.Fatal("unversioned local token allowed")
	}
	_, err = ValidateBusinessSession(ctx, false, "1")
	requireNoError(t, err)
	if err := NewCrmAuth().SetPassword(ctx, "12", "StrongPass123"); err == nil {
		t.Fatal("sales reset another account")
	}
	if err := NewCrmAuth().ChangePassword(ctx, "11", "anything", "StrongPass123"); err == nil {
		t.Fatal("changed another user's password")
	}
	requireNoError(t, NewCrmAuth().SetPassword(testIdentity("1", "12"), "10", "StrongPass123"))
	if _, err := ValidateBusinessSession(ctx, false, "1"); err == nil {
		t.Fatal("password reset preserved token")
	}
	_, err = ValidateBusinessSession(ctx, false, "2")
	requireNoError(t, err)
	requireNoError(t, db.Model(&models.User{}).Where("id=10").Update("status", 2).Error)
	requireNoError(t, db.Model(&models.User{}).Where("id=10").Update("status", 1).Error)
	if _, err := ValidateBusinessSession(ctx, false, "2"); err == nil {
		t.Fatal("reactivation revived old session")
	}
	requireNoError(t, Logout(ctx))
	if _, err := ValidateBusinessSession(ctx, false, "4"); err == nil {
		t.Fatal("logout preserved session")
	}
	requireNoError(t, db.Model(&models.Tenant{}).Where("id=1").Update("status", 2).Error)
	_, err = ValidateLogoutSession(ctx, "5")
	requireNoError(t, err)
	requireNoError(t, Logout(ctx))
	if _, err := ValidateLogoutSession(ctx, "5"); err == nil {
		t.Fatal("disabled tenant prevented logout revocation")
	}
	requireNoError(t, db.Model(&models.Tenant{}).Where("id=1").Update("status", 1).Error)
	ownHash, hashErr := hashPassword("StrongPass123")
	requireNoError(t, hashErr)
	requireNoError(t, db.Model(&models.User{}).Where("id=11").Update("password", ownHash).Error)
	own := testIdentity("1", "11")
	requireNoError(t, NewCrmAuth().ChangePassword(own, "11", "StrongPass123", "AnotherPass456"))
	if _, err := ValidateBusinessSession(own, false, "2"); err == nil {
		t.Fatal("self-service password change preserved old session")
	}
	_, err = ValidateBusinessSession(own, false, "3")
	requireNoError(t, err)
	staff := models.Staff{Model: models.Model{ID: "800"}, ExtCorpID: "corp-a", ExtID: "staff", Name: "staff", RoleID: "1"}
	requireNoError(t, db.Create(&staff).Error)
	requireNoError(t, LogoutWecom(ctx, "corp-a", "staff"))
	requireNoError(t, db.Where("id=800").First(&staff).Error)
	if staff.AuthVersion != 2 {
		t.Fatal("WeCom logout did not revoke sessions", staff.AuthVersion)
	}
	requireNoError(t, db.Model(&models.Staff{}).Where("id=800").Update("role_id", "2").Error)
	requireNoError(t, db.Where("id=800").First(&staff).Error)
	if staff.AuthVersion != 3 {
		t.Fatal("WeCom role change did not revoke sessions", staff.AuthVersion)
	}
	platform := identity.WithIdentity(ctx, identity.Identity{IsPlatform: true, PlatformID: "900"})
	_, err = ValidateBusinessSession(platform, false, "1")
	requireNoError(t, err)
	requireNoError(t, db.Model(&models.PlatformAdmin{}).Where("id=900").Update("password", "new-hash").Error)
	if _, err := ValidateBusinessSession(platform, false, "1"); err == nil {
		t.Fatal("platform password reset preserved token")
	}
}

func TestPasswordPolicy(t *testing.T) {
	for _, password := range []string{"123456", "abcdefghij", "12345678901", "short123", strings.Repeat("a", 65) + "1"} {
		if validatePassword(password) == nil {
			t.Fatal("weak password accepted")
		}
	}
	requireNoError(t, validatePassword("StrongPass123"))
	for n := 0; n < 50; n++ {
		password, err := randPassword(12)
		requireNoError(t, err)
		requireNoError(t, validatePassword(password))
	}
}

func TestForceLoginRequiresExplicitDevelopmentOptIn(t *testing.T) {
	previous := conf.Settings.App.Env
	t.Cleanup(func() { conf.Settings.App.Env = previous })
	conf.Settings.App.Env = "TEST"
	t.Setenv("CRM_ENABLE_FORCE_LOGIN", "")
	if _, err := (&Login{}).ForceConsoleLogin("corp", "staff"); err == nil {
		t.Fatal("development force-login enabled implicitly")
	}
	conf.Settings.App.Env = "PROD"
	t.Setenv("CRM_ENABLE_FORCE_LOGIN", "true")
	if _, err := (&Login{}).ForceConsoleLogin("corp", "staff"); err == nil {
		t.Fatal("production force-login enabled")
	}
}
