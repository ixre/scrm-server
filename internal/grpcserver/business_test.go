package grpcserver

import "testing"

func TestPublicAndExpiredActionBoundaries(t *testing.T) {
	for _, tt := range []struct {
		service, action string
		public, expired bool
	}{
		{"crm_auth", "Login", true, false}, {"crm_platform", "Login", true, false},
		{"crm_smart_card", "PublicGet", true, false}, {"crm_smart_card", "PublicSubmitLead", true, false},
		{"crm_smart_card", "Delete", false, false}, {"crm_customer", "Query", false, false},
		{"crm_user", "GetCurrent", false, true}, {"crm_user", "Update", false, false},
		{"crm_tenant", "GetCurrentInfo", false, true}, {"crm_tenant", "Update", false, false},
		{"crm_tenant_order", "Create", false, true},
		{"crm_plan", "List", false, true}, {"crm_plan", "Update", false, false},
	} {
		if publicBusinessAction(tt.service, tt.action) != tt.public || expiredSessionAction(tt.service, tt.action) != tt.expired {
			t.Errorf("unexpected access policy for %s/%s", tt.service, tt.action)
		}
	}
}
