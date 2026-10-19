package models

import "testing"

func TestCrmLeadLifecycle(t *testing.T) {
	t.Run("新线索可跟进", func(t *testing.T) {
		l := CrmLead{Status: LeadStatusNew}
		if !l.Followable() || l.IsConverted() || l.IsLost() || l.InPool() {
			t.Fatal("new lead should be followable only")
		}
	})
	t.Run("已转化是终态", func(t *testing.T) {
		byStatus := CrmLead{Status: LeadStatusQualified}
		if !byStatus.IsConverted() || byStatus.Followable() {
			t.Fatal("qualified lead is converted and not followable")
		}
		byID := CrmLead{Status: LeadStatusContacted, ConvertCustomerID: "c1"}
		if !byID.IsConverted() {
			t.Fatal("lead with convert_customer_id is converted")
		}
	})
	t.Run("流失可恢复但不可跟进", func(t *testing.T) {
		l := CrmLead{Status: LeadStatusLost}
		if !l.IsLost() || l.Followable() || l.IsConverted() {
			t.Fatal("lost lead must be IsLost and not followable")
		}
	})
	t.Run("池中待领", func(t *testing.T) {
		l := CrmLead{Status: LeadStatusNew, PoolStatus: LeadPoolIn}
		if !l.InPool() {
			t.Fatal("pool_status=1 must be InPool")
		}
	})
}

func TestCrmCustomerLifecycle(t *testing.T) {
	t.Run("私海正常客户可回收", func(t *testing.T) {
		c := CrmCustomer{PoolStatus: CustomerPoolNone, OwnerUserID: "u1"}
		if !c.Recyclable() || c.InPool() || c.IsReassigning() {
			t.Fatal("private unlocked customer must be recyclable")
		}
	})
	t.Run("公海客户不可再回收", func(t *testing.T) {
		c := CrmCustomer{PoolStatus: CustomerPoolIn}
		if c.Recyclable() || !c.InPool() {
			t.Fatal("pool customer is not recyclable")
		}
	})
	t.Run("锁定客户不可回收", func(t *testing.T) {
		c := CrmCustomer{IsLocked: true}
		if c.Recyclable() {
			t.Fatal("locked customer is not recyclable")
		}
	})
	t.Run("转交中客户不可回收", func(t *testing.T) {
		c := CrmCustomer{ReassignStatus: CustomerReassignPending}
		if c.Recyclable() || !c.IsReassigning() {
			t.Fatal("reassigning customer is not recyclable")
		}
	})
}
