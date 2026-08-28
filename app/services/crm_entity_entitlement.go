package services

import (
	"context"
	"core/app/models"
	"core/common/identity"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var entityPlanModules = map[string]string{"customer": "customer", "contact": "customer", "lead": "lead", "opportunity": "opportunity", "order": "order"}

func requireEntityModule(ctx context.Context, db *gorm.DB, kind string) error {
	module, ok := entityPlanModules[kind]
	if !ok {
		return errors.New("实体类型无效")
	}
	if err := models.RequirePlanModules(db.WithContext(ctx), identity.TenantID(ctx), module); err != nil {
		return fmt.Errorf("%w：%w", ErrPermissionDenied, err)
	}
	return nil
}

func requireEntityOperation(ctx context.Context, db *gorm.DB, kind, action, mode string) error {
	if err := requireEntityModule(ctx, db, kind); err != nil {
		return err
	}
	perms, err := crmActorPermissions(ctx, db)
	if err != nil {
		return err
	}
	if HasCRMPermission(perms, "crm:"+kind+":"+action) || HasCRMPermission(perms, "crm:"+kind+":"+mode) {
		return nil
	}
	return ErrPermissionDenied
}

func readableEntityKinds(ctx context.Context, db *gorm.DB) ([]string, error) {
	rights, err := models.TenantEntitlement(db.WithContext(ctx), identity.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	perms, err := crmActorPermissions(ctx, db)
	if err != nil {
		return nil, err
	}
	modules := map[string]bool{}
	for _, module := range rights.Modules {
		modules[module] = true
	}
	kinds := []string{}
	for kind, module := range entityPlanModules {
		if modules[module] && HasCRMPermission(perms, "crm:"+kind+":read") {
			kinds = append(kinds, kind)
		}
	}
	return kinds, nil
}

// Custom fields inherit their parent record's scope and are locked together with
// parent mutations. A tenant ID alone cannot authorize access to another owner.
func fieldParentAccess(ctx context.Context, tx *gorm.DB, kind, id string, write bool) error {
	mode := "read"
	if write {
		mode = "write"
	}
	if err := requireEntityOperation(ctx, tx, kind, mode, mode); err != nil {
		return err
	}
	var q *gorm.DB
	var row any
	var err error
	switch kind {
	case "customer":
		q, err = customerAccessDB(ctx, tx, write)
		row = &models.CrmCustomer{}
	case "contact":
		q, err = contactAccessDB(ctx, tx, write)
		row = &models.CrmContact{}
	case "lead":
		q, err = salesAccessDB(ctx, tx, false)
		row = &models.CrmLead{}
	case "opportunity":
		q, err = salesAccessDB(ctx, tx, false)
		row = &models.CrmOpportunity{}
	case "order":
		q, err = salesAccessDB(ctx, tx, false)
		row = &models.CrmOrder{}
	default:
		return errors.New("实体类型无效")
	}
	if err != nil {
		return err
	}
	strength := "SHARE"
	if write {
		strength = "UPDATE"
	}
	q = q.Clauses(clause.Locking{Strength: strength})
	if err := q.Where("id=?", id).First(row).Error; err != nil {
		return ErrCustomerAccess
	}
	return nil
}
