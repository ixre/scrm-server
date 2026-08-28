package services

import (
	"core/app/constants"
	"core/app/models"
	"core/migrations"
	"fmt"
	"gorm.io/gorm"
	"sync"
	"testing"
	"time"
)

func receiptRequestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := customerBusinessDB(t)
	content, err := migrations.Files.ReadFile("0022_receipt_requests.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(content)).Error)
	requireNoError(t, db.Exec(string(content)).Error)
	businessOrderFixture(t, db, "1000", "1", "100", "10", "signed", 10000, 0, 0)
	businessOrderFixture(t, db, "1001", "1", "101", "11", "signed", 10000, 0, 0)
	return db
}
func TestReceiptRequestConcurrentReplay(t *testing.T) {
	db := receiptRequestDB(t)
	ctx := testIdentity("1", "10")
	svc := NewCrmReceivables()
	now := dayStart(time.Now())
	factory := func() models.CrmReceivables {
		return models.CrmReceivables{OrderID: "1000", Amount: 100, ReceivedAt: &now, ClientRequestID: "concurrent-request-001"}
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	ids := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); r := factory(); err := svc.Create(ctx, &r); errs <- err; ids <- r.ID }()
	}
	wg.Wait()
	close(errs)
	close(ids)
	for err := range errs {
		requireNoError(t, err)
	}
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("retry returned a different receipt")
		}
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmReceivables{}).Count(&n).Error)
	if n != 1 {
		t.Fatal("duplicate receipts", n)
	}
	var order models.CrmOrder
	requireNoError(t, db.First(&order, "id = 1000").Error)
	if order.PaidAmount != 100 {
		t.Fatal("duplicate balance", order.PaidAmount)
	}
	// Timezone-equivalent inputs and nil/empty attachment lists represent the same intent.
	equivalent := factory()
	utc := now.UTC()
	equivalent.ReceivedAt = &utc
	equivalent.CustomerID = "100"
	equivalent.Attachments = constants.StringArrayField{}
	requireNoError(t, svc.Create(ctx, &equivalent))
	if equivalent.ID != first {
		t.Fatal("canonical replay failed")
	}
	changed := factory()
	changed.Amount = 101
	if err := svc.Create(ctx, &changed); err == nil {
		t.Fatal("changed body reused successful key")
	}
	if err := svc.Create(testIdentity("1", "11"), &equivalent); err == nil {
		t.Fatal("unauthorized replay exposed receipt")
	}
	requireNoError(t, svc.Void(ctx, first))
	retry := factory()
	if err := svc.Create(ctx, &retry); err == nil {
		t.Fatal("void receipt resurrected")
	}
	requireNoError(t, db.First(&order, "id = 1000").Error)
	if order.PaidAmount != 0 {
		t.Fatal("void replay restored balance")
	}
	requireNoError(t, db.Model(&models.CrmReceivables{}).Count(&n).Error)
	if n != 1 {
		t.Fatal("void replay created new receipt")
	}
}
func TestReceiptRequestFailureDeleteAndApproval(t *testing.T) {
	db := receiptRequestDB(t)
	svc := NewCrmReceivables()
	ctx := testIdentity("1", "10")
	now := time.Now()
	r := models.CrmReceivables{OrderID: "1000", Amount: 10001, ReceivedAt: &now, ClientRequestID: "rollback-request-001"}
	if err := svc.Create(ctx, &r); err == nil {
		t.Fatal("overpayment accepted")
	}
	var n int64
	requireNoError(t, db.Model(&models.CrmReceivables{}).Count(&n).Error)
	if n != 0 {
		t.Fatal("failed request consumed key")
	}
	// A failed transaction may retry with corrected values, since no intent committed.
	retry := models.CrmReceivables{OrderID: "1000", Amount: 100, ReceivedAt: &now, ClientRequestID: "rollback-request-001", ApproveStatus: "pending"}
	requireNoError(t, svc.Create(ctx, &retry))
	originalID := retry.ID
	requireNoError(t, db.Model(&models.CrmReceivables{}).Where("id = ?", originalID).Updates(map[string]any{"approve_status": "approved", "status": "confirmed"}).Error)
	again := models.CrmReceivables{OrderID: "1000", Amount: 100, ReceivedAt: &now, ClientRequestID: "rollback-request-001", ApproveStatus: "pending"}
	requireNoError(t, svc.Create(ctx, &again))
	if again.ID != originalID || again.ApproveStatus != "approved" || again.Status != "confirmed" {
		t.Fatal("replay reset approval")
	}
	pending := models.CrmReceivables{OrderID: "1000", Amount: 50, ReceivedAt: &now, Status: "pending", ClientRequestID: "deleted-request-001"}
	requireNoError(t, svc.Create(ctx, &pending))
	_, err := svc.Delete(ctx, []string{pending.ID})
	requireNoError(t, err)
	deletedRetry := models.CrmReceivables{OrderID: "1000", Amount: 50, ReceivedAt: &now, Status: "pending", ClientRequestID: "deleted-request-001"}
	if err = svc.Create(ctx, &deletedRetry); err == nil {
		t.Fatal("deleted receipt recreated")
	}
	bad := models.CrmReceivables{OrderID: "1000", Amount: 1, ReceivedAt: &now, ClientRequestID: "bad key!"}
	if err = svc.Create(ctx, &bad); err == nil {
		t.Fatal("invalid request key")
	}
	// Legacy clients without keys keep their existing multi-receipt behavior.
	for i := 0; i < 2; i++ {
		legacy := models.CrmReceivables{OrderID: "1000", Amount: 1, ReceivedAt: &now}
		requireNoError(t, svc.Create(ctx, &legacy))
	}
	requireNoError(t, db.Model(&models.CrmOrder{}).Where("id = 1000").Update("user_id", "11").Error)
	if err = svc.Create(ctx, &again); err == nil {
		t.Fatal("former owner retained replay rights")
	}
}
func TestPlanOptionsBeyond500AndPermissionChanges(t *testing.T) {
	db := receiptRequestDB(t)
	svc := NewCrmReceivablesPlan()
	ctx := testIdentity("1", "10")
	plans := make([]models.CrmReceivablesPlan, 0, 605)
	for i := 0; i < 605; i++ {
		plans = append(plans, models.CrmReceivablesPlan{Model: models.Model{ID: fmt.Sprint(10000 + i)}, TenantID: "1", OrderID: "1000", PeriodNo: i + 1, Amount: 10, Status: "received"})
	}
	requireNoError(t, db.Omit("CustomerID").Create(&plans).Error)
	for _, r := range []models.CrmReceivables{
		{Model: models.Model{ID: "20000"}, TenantID: "1", OrderID: "1000", PlanID: "10000", Amount: 10, Status: "confirmed"},
		{Model: models.Model{ID: "20001"}, TenantID: "1", OrderID: "1000", PlanID: "10001", Amount: 10, Status: "confirmed", ApproveStatus: "pending"},
		{Model: models.Model{ID: "20002"}, TenantID: "1", OrderID: "1000", PlanID: "10002", Amount: 4, Status: "confirmed"},
		{Model: models.Model{ID: "20003"}, TenantID: "2", OrderID: "1000", PlanID: "10002", Amount: 6, Status: "confirmed"},
		{Model: models.Model{ID: "20004"}, TenantID: "1", OrderID: "1001", PlanID: "10002", Amount: 6, Status: "confirmed"},
	} {
		mustCreate(t, db, &r)
	}
	page, err := svc.Options(ctx, PlanOptionsRequest{OrderID: "1000", PageSize: 1000})
	requireNoError(t, err)
	if len(page.Items) != 100 || !page.HasMore {
		t.Fatal("unbounded options page")
	}
	firstCursor := page.NextCursor
	seen := map[string]bool{}
	for {
		for _, p := range page.Items {
			if seen[p.ID] || p.ID == "10000" {
				t.Fatal("duplicate or paid option")
			}
			seen[p.ID] = true
			if p.ID == "10002" && p.RemainingAmount != "6.00" {
				t.Fatal("invalid effective payment sum")
			}
		}
		if !page.HasMore {
			break
		}
		page, err = svc.Options(ctx, PlanOptionsRequest{OrderID: "1000", Cursor: page.NextCursor})
		requireNoError(t, err)
	}
	if len(seen) != 604 {
		t.Fatal("options missing after 500", len(seen))
	}
	found, err := svc.Options(ctx, PlanOptionsRequest{OrderID: "1000", Keyword: "501"})
	requireNoError(t, err)
	if len(found.Items) != 1 || found.Items[0].ID != "10500" || found.Items[0].PeriodNo != 501 {
		t.Fatal("period search failed")
	}
	for _, req := range []PlanOptionsRequest{{OrderID: "1000", Keyword: "501", Cursor: firstCursor}, {OrderID: "1001", Cursor: firstCursor}, {OrderID: "1000", Keyword: "x"}} {
		if _, err = svc.Options(ctx, req); err == nil {
			t.Fatal("invalid cursor/search accepted")
		}
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Updates(map[string]any{"owner_user_id": "11", "shared_staff_ids": constants.StringArrayField{}}).Error)
	// Contract owners may still manage their contracts after customer-only handover.
	if _, err = svc.Options(ctx, PlanOptionsRequest{OrderID: "1000"}); err != nil {
		t.Fatal("customer handover revoked unrelated contract ownership")
	}
	if _, err = svc.Options(testIdentity("1", "11"), PlanOptionsRequest{OrderID: "1000"}); err == nil {
		t.Fatal("customer access expanded plan permission")
	}
	requireNoError(t, db.Model(&models.CrmOrder{}).Where("id = 1000").Update("user_id", "11").Error)
	if _, err = svc.Options(ctx, PlanOptionsRequest{OrderID: "1000", Cursor: firstCursor}); err == nil {
		t.Fatal("former contract owner retained cursor")
	}
	requireNoError(t, db.Model(&models.CrmOrder{}).Where("id = 1000").Update("status", "cancelled").Error)
	page, err = svc.Options(testIdentity("1", "11"), PlanOptionsRequest{OrderID: "1000"})
	requireNoError(t, err)
	if len(page.Items) != 0 {
		t.Fatal("cancelled order offered plans")
	}
}
