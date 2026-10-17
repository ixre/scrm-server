package services

import (
	"context"
	"fmt"
	"testing"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/migrations"
	"gorm.io/gorm"
)

func customerBusinessDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := regressionDB(t)
	sql, err := migrations.Files.ReadFile("0021_customer_business_reads.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(sql)).Error)
	requireNoError(t, db.Exec(string(sql)).Error)
	return db
}
func businessOrderFixture(t *testing.T, db *gorm.DB, id, tenant, customer, owner, status string, amount, coupon, paid float64) {
	t.Helper()
	mustCreate(t, db, &models.CrmOrder{Model: models.Model{ID: id}, TenantID: tenant, CustomerID: customer, UserID: owner, OrderNo: "O" + id, Title: "Contract " + id, Status: status, Amount: amount, CouponAmount: coupon, PaidAmount: paid})
}
func TestCustomerBusinessSummaryAmountsAndPermissions(t *testing.T) {
	db := customerBusinessDB(t)
	businessOrderFixture(t, db, "1000", "1", "100", "10", "signed", 1000, 100, 400)
	businessOrderFixture(t, db, "1001", "1", "100", "10", "completed", 200, 0, 200)
	businessOrderFixture(t, db, "1002", "1", "100", "10", "draft", 3000, 0, 100)
	businessOrderFixture(t, db, "1003", "1", "100", "10", "cancelled", 4000, 0, 4000)
	businessOrderFixture(t, db, "1004", "1", "100", "11", "signed", 8000, 0, 0)
	businessOrderFixture(t, db, "1005", "2", "100", "10", "signed", 9000, 0, 0)
	businessOrderFixture(t, db, "1006", "1", "100", "10", "signed", 10000, 0, 0)
	requireNoError(t, db.Delete(&models.CrmOrder{}, "id = 1006").Error)
	yesterday := dayStart(time.Now()).AddDate(0, 0, -1)
	// Null/wrong customer snapshots must not change parent-contract attribution.
	mustCreate(t, db, &models.CrmReceivablesPlan{Model: models.Model{ID: "2000"}, TenantID: "1", CustomerID: "101", OrderID: "1000", Amount: 500, PlanDate: &yesterday, Status: "received"})
	mustCreate(t, db, &models.CrmReceivablesPlan{Model: models.Model{ID: "2001"}, TenantID: "1", OrderID: "1000", Amount: 400, PlanDate: &yesterday})
	for i, r := range []models.CrmReceivables{
		{Amount: 100, Status: "confirmed", ApproveStatus: "approved"},
		{Amount: 20, Status: "confirmed", ApproveStatus: "none"},
		{Amount: 100, Status: "confirmed", ApproveStatus: "pending"},
		{Amount: 100, Status: "confirmed", ApproveStatus: "rejected"},
		{Amount: 100, Status: "pending", ApproveStatus: "none"},
		{Amount: 100, Status: "void", ApproveStatus: "none"},
		{Amount: 100, Status: "confirmed", ApproveStatus: "none"},
	} {
		r.ID = fmt.Sprint(3000 + i)
		r.TenantID = "1"
		r.OrderID = "1000"
		r.CustomerID = "102"
		r.PlanID = "2000"
		mustCreate(t, db, &r)
	}
	requireNoError(t, db.Delete(&models.CrmReceivables{}, "id = 3006").Error)
	mustCreate(t, db, &models.CrmReceivables{Model: models.Model{ID: "3010"}, TenantID: "2", OrderID: "1000", PlanID: "2000", Status: "confirmed", Amount: 100})
	mustCreate(t, db, &models.CrmReceivables{Model: models.Model{ID: "3011"}, TenantID: "1", OrderID: "1004", PlanID: "2000", Status: "confirmed", Amount: 100})
	summary, err := NewCrmCustomer().BusinessSummary(testIdentity("1", "10"), "100")
	requireNoError(t, err)
	if summary.SignedCount != 2 || summary.DueAmount != "1100.00" || summary.PaidAmount != "600.00" || summary.OutstandingAmount != "500.00" || summary.OverduePlanCount != 2 || summary.OverduePlanUnsettled != "780.00" {
		t.Fatalf("incorrect summary scope or cents: %+v", summary)
	}
	page, err := NewCrmCustomer().BusinessRecords(testIdentity("1", "10"), CustomerBusinessRequest{CustomerID: "100", Kind: "orders"})
	requireNoError(t, err)
	if len(page.Items) != 4 || page.Items[3].ID != "1000" || page.Items[3].DueAmount != "900.00" || page.Items[3].RemainingAmount != "500.00" {
		t.Fatalf("contract list scope or amount mismatch: %+v", page)
	}
	manager, err := NewCrmCustomer().BusinessSummary(testIdentity("1", "12"), "100")
	requireNoError(t, err)
	if manager.SignedCount != 3 || manager.DueAmount != "9100.00" {
		t.Fatalf("manager scope: %+v", manager)
	}
	if _, err = NewCrmCustomer().BusinessSummary(testIdentity("1", "11"), "100"); err == nil {
		t.Fatal("private customer accessible")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("shared_staff_ids", constants.StringArrayField{"11"}).Error)
	shared, err := NewCrmCustomer().BusinessSummary(testIdentity("1", "11"), "100")
	requireNoError(t, err)
	if shared.SignedCount != 1 || shared.DueAmount != "8000.00" || shared.OverduePlanCount != 0 {
		t.Fatalf("shared access expanded contract permissions: %+v", shared)
	}
	mustCreate(t, db, &models.User{Model: models.Model{ID: "13"}, TenantID: "1", Username: "shared-reader", RoleID: "1", Status: 1})
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("shared_staff_ids", constants.StringArrayField{"11", "13"}).Error)
	reader := testIdentity("1", "13")
	empty, err := NewCrmCustomer().BusinessSummary(reader, "100")
	requireNoError(t, err)
	if empty.SignedCount != 0 || empty.DueAmount != "0.00" || empty.PaidAmount != "0.00" || empty.OverduePlanCount != 0 {
		t.Fatal("shared-only reader learned financial totals")
	}
	for _, kind := range []string{"orders", "quotes", "opportunities", "plans", "receipts"} {
		p, err := NewCrmCustomer().BusinessRecords(reader, CustomerBusinessRequest{CustomerID: "100", Kind: kind})
		requireNoError(t, err)
		if len(p.Items) != 0 {
			t.Fatal("shared-only reader learned business records", kind)
		}
	}

}
func TestCustomerBusinessParentAttributionAndPlanStatus(t *testing.T) {
	db := customerBusinessDB(t)
	businessOrderFixture(t, db, "1000", "1", "100", "10", "signed", 1000, 0, 100)
	businessOrderFixture(t, db, "1001", "1", "101", "11", "signed", 1000, 0, 0)
	businessOrderFixture(t, db, "1002", "1", "100", "10", "cancelled", 1000, 0, 0)
	businessOrderFixture(t, db, "1003", "1", "100", "10", "draft", 1000, 0, 0)
	yesterday := dayStart(time.Now()).AddDate(0, 0, -1)
	tomorrow := dayStart(time.Now()).AddDate(0, 0, 1)
	for _, p := range []models.CrmReceivablesPlan{
		{Model: models.Model{ID: "2000"}, TenantID: "1", CustomerID: "102", OrderID: "1000", PeriodNo: 1, Amount: 200, PlanDate: &yesterday, Status: "received"},
		{Model: models.Model{ID: "2001"}, TenantID: "1", CustomerID: "100", OrderID: "1001", Amount: 900, PlanDate: &yesterday},
		{Model: models.Model{ID: "2002"}, TenantID: "2", CustomerID: "100", OrderID: "1000", Amount: 900, PlanDate: &yesterday},
		{Model: models.Model{ID: "2003"}, TenantID: "1", OrderID: "1000", PeriodNo: 2, Amount: 100, PlanDate: &tomorrow, Status: "overdue"},
		{Model: models.Model{ID: "2004"}, TenantID: "1", OrderID: "1000", PeriodNo: 3, Amount: 50, PlanDate: &yesterday},
		{Model: models.Model{ID: "2005"}, TenantID: "1", OrderID: "1002", Amount: 50, PlanDate: &yesterday},
		{Model: models.Model{ID: "2006"}, TenantID: "1", OrderID: "1003", Amount: 50, PlanDate: &yesterday},
	} {
		mustCreate(t, db, &p)
	}
	for _, r := range []models.CrmReceivables{
		{Model: models.Model{ID: "3000"}, TenantID: "1", OrderID: "1000", PlanID: "2000", CustomerID: "101", Amount: 50, Status: "confirmed"},
		{Model: models.Model{ID: "3001"}, TenantID: "1", OrderID: "1001", PlanID: "2000", CustomerID: "100", Amount: 800, Status: "confirmed"},
		{Model: models.Model{ID: "3002"}, TenantID: "2", OrderID: "1000", CustomerID: "100", Amount: 800, Status: "confirmed"},
		{Model: models.Model{ID: "3003"}, TenantID: "1", OrderID: "1000", PlanID: "2004", Amount: 50, Status: "confirmed"},
	} {
		mustCreate(t, db, &r)
	}
	ctx := testIdentity("1", "10")
	svc := NewCrmCustomer()
	page, err := svc.BusinessRecords(ctx, CustomerBusinessRequest{CustomerID: "100", Kind: "plans"})
	requireNoError(t, err)
	states := map[string]string{}
	for _, row := range page.Items {
		states[row.ID] = row.Status
		if row.ID == "2000" && (row.RemainingAmount != "150.00" || row.PaidAmount != "50.00") {
			t.Fatal("cross-order payment counted against plan")
		}
	}
	if fmt.Sprint(states) != "map[2000:overdue 2003:pending 2004:received 2005:cancelled 2006:draft]" {
		t.Fatalf("invalid live plan status/parent filter: %v", states)
	}
	page, err = svc.BusinessRecords(ctx, CustomerBusinessRequest{CustomerID: "100", Kind: "receipts"})
	requireNoError(t, err)
	if len(page.Items) != 2 || page.Items[0].OrderID != "1000" {
		t.Fatal("receipt scope trusts customer snapshot or leaked foreign parent")
	}
	requireNoError(t, db.Delete(&models.CrmOrder{}, "id = 1000").Error)
	page, err = svc.BusinessRecords(ctx, CustomerBusinessRequest{CustomerID: "100", Kind: "receipts"})
	requireNoError(t, err)
	if len(page.Items) != 0 {
		t.Fatal("deleted parent receipts leaked")
	}
}
func TestCustomerBusinessCursorAndRevocation(t *testing.T) {
	db := customerBusinessDB(t)
	svc := NewCrmCustomer()
	ctx := testIdentity("1", "10")
	for i := 0; i < 125; i++ {
		mustCreate(t, db, &models.CrmOpportunity{Model: models.Model{ID: fmt.Sprint(1000 + i)}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", Title: fmt.Sprint("Opportunity ", i), Amount: 0, Status: "open"})
	}
	page, err := svc.BusinessRecords(ctx, CustomerBusinessRequest{CustomerID: "100", Kind: "opportunities", PageSize: 1000})
	requireNoError(t, err)
	if len(page.Items) != 100 || page.Items[0].Title != "Opportunity 124" || page.Items[0].Amount != "0.00" || !page.HasMore {
		t.Fatal("page cap/title/zero amount incorrect")
	}
	firstCursor := page.NextCursor
	mustCreate(t, db, &models.CrmOpportunity{Model: models.Model{ID: "5000"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", Title: "New"})
	next, err := svc.BusinessRecords(ctx, CustomerBusinessRequest{CustomerID: "100", Kind: "opportunities", Cursor: page.NextCursor, PageSize: 100})
	requireNoError(t, err)
	if len(next.Items) != 25 || next.HasMore || next.Items[0].ID != "1024" {
		t.Fatal("cursor duplicated or omitted preexisting rows")
	}
	for _, req := range []CustomerBusinessRequest{{CustomerID: "100", Kind: "orders", Cursor: firstCursor}, {CustomerID: "101", Kind: "opportunities", Cursor: firstCursor}, {CustomerID: "100", Kind: "opportunities", Cursor: "invalid"}, {CustomerID: "100", Kind: "unsupported"}} {
		if _, err = svc.BusinessRecords(ctx, req); err == nil {
			t.Fatal("invalid scope/cursor accepted")
		}
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("owner_user_id", "11").Error)
	if _, err = svc.BusinessRecords(ctx, CustomerBusinessRequest{CustomerID: "100", Kind: "opportunities", Cursor: firstCursor}); err == nil {
		t.Fatal("handover did not revoke cursor")
	}
	if _, err = svc.BusinessSummary(ctx, "100"); err == nil {
		t.Fatal("handover did not revoke summary")
	}
	cancelled, cancel := context.WithCancel(testIdentity("1", "11"))
	cancel()
	if _, err = svc.BusinessSummary(cancelled, "100"); err == nil {
		t.Fatal("cancelled query returned a successful zero summary")
	}
}
func TestCustomerBusinessQuoteAndDecimalPrecision(t *testing.T) {
	db := customerBusinessDB(t)
	for _, q := range []models.CrmQuote{
		{Model: models.Model{ID: "4000"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", Subject: "Visible", FinalAmount: 100},
		{Model: models.Model{ID: "4001"}, TenantID: "1", CustomerID: "100", OwnerUserID: "11", Subject: "Hidden", FinalAmount: 900},
		{Model: models.Model{ID: "4002"}, TenantID: "2", CustomerID: "100", OwnerUserID: "10", Subject: "Foreign", FinalAmount: 900},
	} {
		mustCreate(t, db, &q)
	}
	page, err := NewCrmCustomer().BusinessRecords(testIdentity("1", "10"), CustomerBusinessRequest{CustomerID: "100", Kind: "quotes"})
	requireNoError(t, err)
	if len(page.Items) != 1 || page.Items[0].Title != "Visible" {
		t.Fatal("quote scope or title incorrect")
	}
	businessOrderFixture(t, db, "1000", "1", "100", "10", "signed", 1, 0, 0)
	requireNoError(t, db.Exec("UPDATE crm_order SET amount = 99999999999999.99, paid_amount = 0.01 WHERE id = 1000").Error)
	summary, err := NewCrmCustomer().BusinessSummary(testIdentity("1", "10"), "100")
	requireNoError(t, err)
	if summary.DueAmount != "99999999999999.99" || summary.OutstandingAmount != "99999999999999.98" {
		t.Fatalf("aggregate lost decimal cents: %+v", summary)
	}
}
