package dispatch

import (
	"core/app/models"
	"core/app/services"
	"testing"
)

func TestEveryBusinessServiceHasPlanPolicy(t *testing.T) {
	valid := map[string]bool{}
	for _, def := range models.PlanModuleDefs {
		valid[def.Key] = true
	}
	for service, actions := range Reg {
		for action := range actions {
			modules, ok := services.BusinessPlanModules(service, action)
			if !ok {
				t.Errorf("missing entitlement policy for %s/%s", service, action)
			}
			for _, module := range modules {
				if !valid[module] {
					t.Errorf("unknown module %s for %s", module, service)
				}
			}
		}
	}
}
