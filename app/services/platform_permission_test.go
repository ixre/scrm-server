package services

import (
	"core/app/constants"
	"core/app/models"
	"core/common/identity"
	"testing"
)

func TestPlatformPermissionsIgnoreRoleLabels(t *testing.T) {
	db := regressionDB(t)
	for _, admin := range []models.PlatformAdmin{
		{Model: models.Model{ID: "800"}, Username: "root", Role: "custom_root", Status: 1, Perms: constants.StringArrayField{"platform:*:full"}},
		{Model: models.Model{ID: "801"}, Username: "unprivileged", Role: "super_admin", Status: 1},
		{Model: models.Model{ID: "802"}, Username: "reviewer", Role: "reviewer", Status: 1, Perms: constants.StringArrayField{"platform:tenant:list"}},
	} {
		mustCreate(t, db, &admin)
	}
	root := identity.WithIdentity(testIdentity("", ""), identity.Identity{IsPlatform: true, PlatformID: "800"})
	nameOnly := identity.WithIdentity(root, identity.Identity{IsPlatform: true, PlatformID: "801"})
	reader := identity.WithIdentity(root, identity.Identity{IsPlatform: true, PlatformID: "802"})
	if _, err := RequirePlatformPermission(nameOnly, "platform:tenant:list"); err == nil {
		t.Fatal("platform role name granted privileges")
	}
	if _, err := RequirePlatformPermission(reader, "platform:tenant:list"); err != nil {
		t.Fatal(err)
	}
	if _, err := RequirePlatformPermission(reader, "platform:tenant:delete"); err == nil {
		t.Fatal("read grant allowed deletion")
	}
	svc := PlatformAdminService{}
	if err := svc.SetAdminStatus(root, "800", 2); err == nil {
		t.Fatal("last root disabled")
	}
	if err := svc.UpdateAdmin(root, "800", PlatformAdminUpdateReq{Role: "support"}); err == nil {
		t.Fatal("last root demoted")
	}
	if err := svc.DeleteAdmin(root, "800"); err == nil {
		t.Fatal("last root deleted")
	}
	perms := []string{"platform:tenant:list"}
	created, err := svc.CreateAdmin(root, PlatformAdminCreateReq{Username: "custom", Name: "审核", Password: "StrongPass123", Role: "custom", Perms: &perms})
	requireNoError(t, err)
	if len(created.Perms) != 1 || created.Perms[0] != perms[0] {
		t.Fatal("custom platform grants lost", created.Perms)
	}
	if _, err := svc.CreateAdmin(reader, PlatformAdminCreateReq{Username: "escalated", Password: "StrongPass123", Role: "super_admin"}); err == nil {
		t.Fatal("reader created root")
	}
	requireNoError(t, db.Model(&models.PlatformAdmin{}).Where("id=802").Update("perms", constants.StringArrayField{"platform:admin:create"}).Error)
	if _, err := svc.CreateAdmin(reader, PlatformAdminCreateReq{Username: "escalated", Password: "StrongPass123", Role: "super_admin"}); err == nil {
		t.Fatal("account manager granted unowned root permissions")
	}
}
