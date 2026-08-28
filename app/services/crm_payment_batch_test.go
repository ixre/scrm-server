package services

import (
	"context"
	"core/app/models"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type paymentStatementCounter struct {
	logger.Interface
	count atomic.Int64
}

func (c *paymentStatementCounter) Trace(ctx context.Context, start time.Time, query func() (string, int64), err error) {
	c.count.Add(1)
	c.Interface.Trace(ctx, start, query, err)
}

func countPaymentStatements(db *gorm.DB, operation func() error) (int64, error) {
	counter := &paymentStatementCounter{Interface: db.Logger}
	previous := models.DB
	models.DB = db.Session(&gorm.Session{Logger: counter})
	defer func() { models.DB = previous }()
	err := operation()
	return counter.count.Load(), err
}

func TestPaymentBatchQueryBudgetAndLifecycle(t *testing.T) {
	db := receiptRequestDB(t)
	ctx := testIdentity("1", "10")
	businessOrderFixture(t, db, "1002", "1", "100", "10", "signed", 10000, 0, 50)
	now := time.Now()
	future, yesterday := dayStart(now).AddDate(0, 0, 7), dayStart(now).AddDate(0, 0, -1)
	small := []models.CrmReceivablesPlan{{OrderID: "1000", PeriodNo: 1, Amount: 10, PlanDate: &future}}
	large := make([]models.CrmReceivablesPlan, 605)
	for i := range large {
		large[i] = models.CrmReceivablesPlan{OrderID: "1002", PeriodNo: i + 1, Amount: 10, PlanDate: &future}
	}
	large[1].PlanDate = &yesterday
	smallWrites, err := countPaymentStatements(db, func() error { return NewCrmReceivablesPlan().BatchCreate(ctx, small) })
	requireNoError(t, err)
	largeWrites, err := countPaymentStatements(db, func() error { return NewCrmReceivablesPlan().BatchCreate(ctx, large) })
	requireNoError(t, err)
	if largeWrites > smallWrites+5 || largeWrites > 20 {
		t.Fatalf("plan creation still grows per row: small=%d large=%d", smallWrites, largeWrites)
	}
	// Unchanged periods must not be rewritten on every collection.
	stamp := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	requireNoError(t, db.Model(&models.CrmReceivablesPlan{}).Where("order_id = 1002").Update("updated_at", stamp).Error)
	one := models.CrmReceivables{OrderID: "1000", PlanID: small[0].ID, Amount: 10, ReceivedAt: &now}
	many := models.CrmReceivables{OrderID: "1002", PlanID: large[0].ID, Amount: 10, ReceivedAt: &now}
	smallQueries, err := countPaymentStatements(db, func() error { return NewCrmReceivables().Create(ctx, &one) })
	requireNoError(t, err)
	largeQueries, err := countPaymentStatements(db, func() error { return NewCrmReceivables().Create(ctx, &many) })
	requireNoError(t, err)
	if largeQueries != smallQueries || largeQueries > 20 {
		t.Fatalf("receipt reconciliation still grows per period: small=%d large=%d", smallQueries, largeQueries)
	}
	t.Logf("SQL calls: create 1/605 plans=%d/%d, receipt with 1/605 plans=%d/%d", smallWrites, largeWrites, smallQueries, largeQueries)
	assertPlans := func(first, second string, balance float64) {
		t.Helper()
		var plans []models.CrmReceivablesPlan
		requireNoError(t, db.Where("order_id = 1002").Order("period_no").Find(&plans).Error)
		if len(plans) != 605 || plans[0].Status != first || plans[1].Status != second {
			t.Fatal("incorrect bulk plan status")
		}
		for _, plan := range plans[1:] {
			if !plan.UpdatedAt.Equal(stamp) {
				t.Fatal("unchanged plan was rewritten", plan.PeriodNo)
			}
		}
		var order models.CrmOrder
		requireNoError(t, db.First(&order, "id = 1002").Error)
		if order.PaidAmount != balance {
			t.Fatalf("historical balance changed: %v", order.PaidAmount)
		}
	}
	assertPlans("received", "overdue", 60)
	over := models.CrmReceivables{OrderID: "1002", PlanID: large[0].ID, Amount: 0.01, ReceivedAt: &now}
	if err := NewCrmReceivables().Create(ctx, &over); err == nil {
		t.Fatal("per-period overpayment accepted")
	}
	assertPlans("received", "overdue", 60)
	var count int64
	requireNoError(t, db.Model(&models.CrmReceivables{}).Where("order_id = 1002").Count(&count).Error)
	if count != 1 {
		t.Fatal("failed receipt did not roll back")
	}
	requireNoError(t, NewCrmReceivables().Void(ctx, many.ID))
	assertPlans("pending", "overdue", 50)
	// Approval uses the same bulk reconciliation path and still observes the parent lock.
	pending := models.CrmReceivables{OrderID: "1002", PlanID: large[0].ID, Amount: 10, ReceivedAt: &now, ApproveStatus: "pending"}
	requireNoError(t, NewCrmReceivables().Create(ctx, &pending))
	assertPlans("pending", "overdue", 50)
	requireNoError(t, NewCrmReceivablesExt().Approve(testIdentity("1", "12"), pending.ID, "12"))
	assertPlans("received", "overdue", 60)
}

func TestPlanBatchValidationAndRollback(t *testing.T) {
	db := receiptRequestDB(t)
	ctx := testIdentity("1", "10")
	future := time.Now().AddDate(0, 0, 7)
	svc := NewCrmReceivablesPlan()
	batch := func(order string, periods ...int) []models.CrmReceivablesPlan {
		plans := make([]models.CrmReceivablesPlan, len(periods))
		for i, period := range periods {
			plans[i] = models.CrmReceivablesPlan{OrderID: order, PeriodNo: period, Amount: 10, PlanDate: &future}
		}
		return plans
	}
	assertCount := func(want int64) {
		t.Helper()
		var count int64
		requireNoError(t, db.Model(&models.CrmReceivablesPlan{}).Count(&count).Error)
		if count != want {
			t.Fatalf("partial plan batch persisted: %d, want %d", count, want)
		}
	}
	for _, plans := range [][]models.CrmReceivablesPlan{
		batch("1000", 1, 1),
		append(batch("1000", 1), batch("1001", 1)...), // Unauthorized second contract.
		{{OrderID: "1000", PeriodNo: 1, Amount: 10001, PlanDate: &future}},
		{{OrderID: "1000", CustomerID: "101", PeriodNo: 1, Amount: 10, PlanDate: &future}},
		{{OrderID: "1000", PeriodNo: 2147483648, Amount: 10, PlanDate: &future}},
	} {
		if err := svc.BatchCreate(ctx, plans); err == nil {
			t.Fatal("invalid batch accepted")
		}
		assertCount(0)
	}
	if err := svc.BatchCreate(ctx, make([]models.CrmReceivablesPlan, 1001)); err == nil {
		t.Fatal("oversized batch accepted")
	}
	if err := NewCrmReceivablesExt().GeneratePlan(ctx, "1000", make([]PlanPeriod, 1001)); err == nil {
		t.Fatal("oversized generated plan accepted")
	}
	// Force a later INSERT chunk to fail after earlier chunks have run.
	requireNoError(t, db.Exec("ALTER TABLE crm_receivables_plan ADD CONSTRAINT test_later_chunk CHECK (period_no <> 401)").Error)
	large := make([]models.CrmReceivablesPlan, 605)
	for i := range large {
		large[i] = batch("1000", i+1)[0]
	}
	if err := svc.BatchCreate(ctx, large); err == nil {
		t.Fatal("later chunk failure not reported")
	}
	assertCount(0)
	requireNoError(t, db.Exec("ALTER TABLE crm_receivables_plan DROP CONSTRAINT test_later_chunk").Error)
	requireNoError(t, svc.BatchCreate(ctx, batch("1000", 1)))
	if err := svc.BatchCreate(ctx, batch("1000", 1, 2)); err == nil {
		t.Fatal("existing period not checked")
	}
	assertCount(1)
	// No partial writes across contracts when a later contract fails validation.
	bad := append(batch("1000", 2), batch("1001", 1)...)
	bad[1].Amount = 10001
	if err := svc.BatchCreate(testIdentity("1", "12"), bad); err == nil {
		t.Fatal("aggregate amount validation omitted")
	}
	assertCount(1)
	// Legacy contracts can lack a customer. Mixed batches must preserve NULL, not bigint ''.
	businessOrderFixture(t, db, "1002", "1", "", "10", "signed", 100, 0, 0)
	requireNoError(t, svc.BatchCreate(ctx, append(batch("1000", 2), batch("1002", 1)...)))
	var nullCustomer int64
	requireNoError(t, db.Table("crm_receivables_plan").Where("order_id = 1002 AND customer_id IS NULL").Count(&nullCustomer).Error)
	if nullCustomer != 1 {
		t.Fatal("legacy customer NULL not preserved")
	}
	assertCount(3)
}

func TestPlanBatchConcurrentCapacity(t *testing.T) {
	db := receiptRequestDB(t)
	ctx := testIdentity("1", "10")
	future := time.Now().AddDate(0, 0, 7)
	requireNoError(t, db.Model(&models.CrmOrder{}).Where("id = 1000").Update("amount", 100).Error)
	var successes atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(period int) {
			defer wg.Done()
			plans := []models.CrmReceivablesPlan{{OrderID: "1000", PeriodNo: period, Amount: 40, PlanDate: &future}}
			if NewCrmReceivablesPlan().BatchCreate(ctx, plans) == nil {
				successes.Add(1)
			}
		}(i + 1)
	}
	wg.Wait()
	if successes.Load() != 2 {
		t.Fatalf("concurrent plan capacity: %d", successes.Load())
	}
	var total float64
	requireNoError(t, db.Model(&models.CrmReceivablesPlan{}).Select("SUM(amount)").Scan(&total).Error)
	if total != 80 {
		t.Fatal("plan total exceeded contract", total)
	}
}

func TestPaymentBatchScopeAndWriteFailure(t *testing.T) {
	db := receiptRequestDB(t)
	ctx := testIdentity("1", "10")
	now, future, past := time.Now(), dayStart(time.Now()).AddDate(0, 0, 7), dayStart(time.Now()).AddDate(0, 0, -1)
	for _, plan := range []models.CrmReceivablesPlan{
		{Model: models.Model{ID: "2000"}, TenantID: "1", OrderID: "1000", Amount: 10, PlanDate: &past},
		{Model: models.Model{ID: "2001"}, TenantID: "1", OrderID: "1000", Amount: 10, PlanDate: &future},
		{Model: models.Model{ID: "2002"}, TenantID: "1", OrderID: "1000", Amount: 10, PlanDate: &future},
		{Model: models.Model{ID: "2010"}, TenantID: "2", OrderID: "1000", Amount: 10, Status: "foreign"},
		{Model: models.Model{ID: "2011"}, TenantID: "1", OrderID: "1001", Amount: 10, Status: "other"},
		{Model: models.Model{ID: "2012"}, TenantID: "1", OrderID: "1000", Amount: -1, Status: "deleted"},
	} {
		p := plan
		mustCreate(t, db, &p)
	}
	requireNoError(t, db.Delete(&models.CrmReceivablesPlan{}, "id = 2012").Error)
	for i, receipt := range []models.CrmReceivables{
		{TenantID: "1", OrderID: "1000", PlanID: "2000", Amount: 4, Status: "confirmed"},
		{TenantID: "2", OrderID: "1000", PlanID: "2000", Amount: 6, Status: "confirmed"},
		{TenantID: "1", OrderID: "1001", PlanID: "2000", Amount: 6, Status: "confirmed"},
		{TenantID: "1", OrderID: "1000", PlanID: "2001", Amount: 10, Status: "confirmed", ApproveStatus: "approved"},
		{TenantID: "1", OrderID: "1000", PlanID: "2002", Amount: 10, Status: "pending"},
		{TenantID: "1", OrderID: "1000", PlanID: "2002", Amount: 10, Status: "confirmed", ApproveStatus: "pending"},
		{TenantID: "1", OrderID: "1000", PlanID: "2002", Amount: 10, Status: "confirmed", ApproveStatus: "rejected"},
		{TenantID: "1", OrderID: "1000", PlanID: "2002", Amount: 10, Status: "void"},
		{TenantID: "1", OrderID: "1000", PlanID: "2002", Amount: 10, Status: "confirmed"},
	} {
		r := receipt
		r.ID = fmt.Sprint(3000 + i)
		mustCreate(t, db, &r)
	}
	requireNoError(t, db.Model(&models.CrmReceivables{}).Where("id = 3000").Update("approve_status", nil).Error)
	requireNoError(t, db.Delete(&models.CrmReceivables{}, "id = 3008").Error)
	r := models.CrmReceivables{OrderID: "1000", Amount: 1, ReceivedAt: &now}
	requireNoError(t, NewCrmReceivables().Create(ctx, &r))
	for id, want := range map[string]string{"2000": "overdue", "2001": "received", "2002": "pending", "2010": "foreign", "2011": "other", "2012": "deleted"} {
		var plan models.CrmReceivablesPlan
		requireNoError(t, db.Unscoped().First(&plan, "id = ?", id).Error)
		if plan.Status != want {
			t.Fatalf("wrong scope or effective receipt status: plan=%s got=%s want=%s", id, plan.Status, want)
		}
	}
	assertBalance := func() {
		t.Helper()
		var order models.CrmOrder
		requireNoError(t, db.First(&order, "id = 1000").Error)
		if order.PaidAmount != 15 {
			t.Fatal("failed update changed balance", order.PaidAmount)
		}
		var count int64
		requireNoError(t, db.Model(&models.CrmReceivables{}).Count(&count).Error)
		if count != 9 { // 8 surviving fixtures + 1 committed registration.
			t.Fatal("failed update persisted receipt", count)
		}
	}
	assertBalance()
	// Existing received rows are intentionally not validated; only a new transition fails.
	requireNoError(t, db.Exec("ALTER TABLE crm_receivables_plan ADD CONSTRAINT test_status_failure CHECK(status <> 'received') NOT VALID").Error)
	failed := models.CrmReceivables{OrderID: "1000", PlanID: "2002", Amount: 10, ReceivedAt: &now}
	if err := NewCrmReceivables().Create(ctx, &failed); err == nil {
		t.Fatal("bulk status write failure not propagated")
	}
	assertBalance()
	requireNoError(t, db.Exec("ALTER TABLE crm_receivables_plan DROP CONSTRAINT test_status_failure").Error)
	requireNoError(t, db.Model(&models.CrmReceivablesPlan{}).Where("id = 2002").Update("amount", -1).Error)
	failed = models.CrmReceivables{OrderID: "1000", Amount: 1, ReceivedAt: &now}
	if err := NewCrmReceivables().Create(ctx, &failed); err == nil {
		t.Fatal("invalid historical amount accepted")
	}
	assertBalance()
}
