package services

import (
	"core/app/models"
	"core/migrations"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

func financeWorkbenchDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := receiptRequestDB(t)
	ddl, err := migrations.Files.ReadFile("0023_finance_workbench_reads.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(ddl)).Error)
	requireNoError(t, db.Exec(string(ddl)).Error)
	return db
}

func TestFinanceReceiptPrefixSearch(t *testing.T) {
	db := financeWorkbenchDB(t)
	for _, r := range []models.CrmReceivables{
		{Model: models.Model{ID: "2000"}, TenantID: "1", OrderID: "1000", ReceivablesNo: "RC-First", ApproveStatus: "pending"},
		{Model: models.Model{ID: "2001"}, TenantID: "1", OrderID: "1001", ReceivablesNo: "RC-ForeignOwner", ApproveStatus: "pending"},
		{Model: models.Model{ID: "2002"}, TenantID: "2", OrderID: "1000", ReceivablesNo: "RC-ForeignTenant", ApproveStatus: "pending"},
		{Model: models.Model{ID: "2003"}, TenantID: "1", OrderID: "1000", ReceivablesNo: "RC_%literal", ApproveStatus: "pending"},
		{Model: models.Model{ID: "2004"}, TenantID: "1", OrderID: "1000", ReceivablesNo: "RC-FirstDeleted", ApproveStatus: "pending"},
		{Model: models.Model{ID: "2005"}, TenantID: "1", OrderID: "1000", ReceivablesNo: "RC-FVoid", Status: "void", ApproveStatus: "pending"},
	} {
		r := r
		mustCreate(t, db, &r)
	}
	requireNoError(t, db.Delete(&models.CrmReceivables{}, "id=2004").Error)
	svc := NewCrmReceivables()
	items, total, err := svc.Query(testIdentity("1", "10"), RecvQueryReq{Keyword: " rC-f ", ApproveStatus: "pending"})
	requireNoError(t, err)
	if total != 1 || len(items) != 1 || items[0].ID != "2000" {
		t.Fatal("prefix search lost permission/deletion scope", total, items)
	}
	_, total, err = svc.Query(testIdentity("1", "12"), RecvQueryReq{Keyword: "RC-F", ApproveStatus: "pending"})
	requireNoError(t, err)
	if total != 2 {
		t.Fatal("manager search crossed tenants", total)
	}
	_, total, err = svc.Query(testIdentity("1", "10"), RecvQueryReq{Keyword: "RC-FVoid", ApproveStatus: "pending", Status: "void"})
	requireNoError(t, err)
	if total != 1 {
		t.Fatal("explicit audit status filter lost void history")
	}
	items, total, err = svc.Query(testIdentity("1", "10"), RecvQueryReq{Keyword: "RC_%"})
	requireNoError(t, err)
	if total != 1 || items[0].ID != "2003" {
		t.Fatal("wildcards treated as search operators")
	}
	if _, _, err := svc.Query(testIdentity("1", "10"), RecvQueryReq{Keyword: strings.Repeat("x", 65)}); err == nil {
		t.Fatal("unbounded keyword accepted")
	}
}

func TestFinanceDuePlansEffectiveAmountsAndScope(t *testing.T) {
	db := financeWorkbenchDB(t)
	for _, order := range []struct{ id, tenant, owner, status string }{
		{"1002", "1", "10", "draft"}, {"1003", "1", "10", "cancelled"},
		{"1004", "1", "10", "signed"}, {"1005", "2", "20", "signed"}, {"1006", "1", "10", "completed"},
	} {
		businessOrderFixture(t, db, order.id, order.tenant, "100", order.owner, order.status, 100, 0, 0)
	}
	requireNoError(t, db.Delete(&models.CrmOrder{}, "id=1004").Error)
	businessOrderFixture(t, db, "1007", "1", "100", "10", "signed", 100, 0, 100)
	today := dayStart(time.Now())
	end := today.AddDate(0, 0, 7).Add(23*time.Hour + 59*time.Minute)
	outside, past := today.AddDate(0, 0, 8), today.AddDate(0, 0, -1)
	for _, p := range []models.CrmReceivablesPlan{
		{Model: models.Model{ID: "2000"}, TenantID: "1", OrderID: "1000", PlanDate: &today},
		{Model: models.Model{ID: "2001"}, TenantID: "1", OrderID: "1000", PlanDate: &end},
		{Model: models.Model{ID: "2002"}, TenantID: "1", OrderID: "1000", PlanDate: &outside},
		{Model: models.Model{ID: "2003"}, TenantID: "1", OrderID: "1000", PlanDate: &past, Status: "received"},
		{Model: models.Model{ID: "2004"}, TenantID: "1", OrderID: "1000", PlanDate: &past},
		{Model: models.Model{ID: "2005"}, TenantID: "1", OrderID: "1000", PlanDate: &past, Status: "received"},
		{Model: models.Model{ID: "2006"}, TenantID: "1", OrderID: "1000"},
		{Model: models.Model{ID: "2010"}, TenantID: "1", OrderID: "1001", PlanDate: &past},
		{Model: models.Model{ID: "2011"}, TenantID: "1", OrderID: "1002", PlanDate: &past},
		{Model: models.Model{ID: "2012"}, TenantID: "1", OrderID: "1003", PlanDate: &past},
		{Model: models.Model{ID: "2013"}, TenantID: "1", OrderID: "1004", PlanDate: &past},
		{Model: models.Model{ID: "2014"}, TenantID: "2", OrderID: "1005", PlanDate: &past},
		{Model: models.Model{ID: "2015"}, TenantID: "1", OrderID: "1000", PlanDate: &past},
		{Model: models.Model{ID: "2016"}, TenantID: "1", OrderID: "1006", PlanDate: &past},
		{Model: models.Model{ID: "2017"}, TenantID: "1", OrderID: "1007", PlanDate: &past},
	} {
		p := p
		p.Amount = 10
		mustCreate(t, db, &p)
	}
	requireNoError(t, db.Delete(&models.CrmReceivablesPlan{}, "id=2015").Error)
	for _, r := range []models.CrmReceivables{
		{Model: models.Model{ID: "3000"}, TenantID: "1", OrderID: "1000", PlanID: "2000", Amount: 2, Status: "confirmed"},
		{Model: models.Model{ID: "3001"}, TenantID: "1", OrderID: "1000", PlanID: "2003", Amount: 4, Status: "confirmed"},
		{Model: models.Model{ID: "3002"}, TenantID: "2", OrderID: "1000", PlanID: "2003", Amount: 6, Status: "confirmed"},
		{Model: models.Model{ID: "3003"}, TenantID: "1", OrderID: "1001", PlanID: "2003", Amount: 6, Status: "confirmed"},
		{Model: models.Model{ID: "3004"}, TenantID: "1", OrderID: "1000", PlanID: "2004", Amount: 10, Status: "confirmed", ApproveStatus: "approved"},
		{Model: models.Model{ID: "3005"}, TenantID: "1", OrderID: "1000", PlanID: "2005", Amount: 10, Status: "confirmed", ApproveStatus: "pending"},
		{Model: models.Model{ID: "3006"}, TenantID: "1", OrderID: "1000", PlanID: "2005", Amount: 10, Status: "confirmed"},
	} {
		r := r
		mustCreate(t, db, &r)
	}
	requireNoError(t, db.Delete(&models.CrmReceivables{}, "id=3006").Error)
	svc := NewCrmReceivablesExt()
	ctx := testIdentity("1", "10")
	items, total, err := svc.ListExpiring(ctx, 7, DuePlanQuery{})
	requireNoError(t, err)
	if total != 2 || len(items) != 2 || items[0].ID != "2000" || items[0].RemainingAmount != "8.00" || items[1].ID != "2001" || items[1].DaysUntilDue != 7 {
		t.Fatalf("bad calendar window/amount: %d %+v", total, items)
	}
	items, total, err = svc.ListOverdue(ctx, DuePlanQuery{})
	requireNoError(t, err)
	if total != 3 || len(items) != 3 {
		t.Fatalf("overdue stale status or parent scope: %d %+v", total, items)
	}
	for _, p := range items {
		if p.Status != "overdue" || p.DaysUntilDue != -1 || p.OrderNo == "" || p.ID == "2004" || p.ID == "2010" {
			t.Fatal("bad overdue projection", p)
		}
		if p.ID == "2003" && p.RemainingAmount != "6.00" {
			t.Fatal("foreign receipts affected remaining amount")
		}
	}
	_, total, err = svc.ListOverdue(testIdentity("1", "12"), DuePlanQuery{})
	requireNoError(t, err)
	if total != 4 {
		t.Fatal("manager scope", total)
	}
	_, total, err = svc.ListOverdue(ctx, DuePlanQuery{OrderID: "1006"})
	requireNoError(t, err)
	if total != 1 {
		t.Fatal("contract filter ignored", total)
	}
	for _, days := range []int{-1, 366} {
		if _, _, err := svc.ListExpiring(ctx, days, DuePlanQuery{}); err == nil {
			t.Fatal("unbounded date range")
		}
	}
	requireNoError(t, db.Model(&models.CrmOrder{}).Where("id=1000").Update("user_id", "11").Error)
	_, total, err = svc.ListExpiring(ctx, 7, DuePlanQuery{})
	requireNoError(t, err)
	if total != 0 {
		t.Fatal("former contract owner kept access")
	}
}

func TestFinanceBusinessCalendar(t *testing.T) {
	d, err := parsePlanDate("2026-10-18")
	requireNoError(t, err)
	if d.Format(time.RFC3339) != "2026-10-18T00:00:00+08:00" {
		t.Fatal("host timezone changed plan date", d)
	}
	for _, value := range []string{"", "2026-02-30", "2026-10-18T00:00:00Z"} {
		if _, err := parsePlanDate(value); err == nil {
			t.Fatal("invalid calendar date accepted", value)
		}
	}
	for _, c := range []struct {
		now, due string
		want     int
	}{
		{"2026-10-18T15:59:59Z", "2026-10-18T16:00:00Z", 1},
		{"2026-10-18T16:00:01Z", "2026-10-18T15:59:59Z", -1},
		{"2028-02-28T00:00:00+08:00", "2028-03-01T23:00:00+08:00", 2},
	} {
		now, err := time.Parse(time.RFC3339, c.now)
		requireNoError(t, err)
		due, err := time.Parse(time.RFC3339, c.due)
		requireNoError(t, err)
		if calendarDaysUntil(now, due) != c.want {
			t.Fatal("calendar day differs by host/time of day", c)
		}
	}
}
