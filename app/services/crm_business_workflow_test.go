package services

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"core/app/models"
)

func TestCustomerLifecycleRegression(t *testing.T) {
	db := regressionDB(t)
	sales := testIdentity("1", "10")
	other := testIdentity("1", "11")
	admin := testIdentity("1", "12")
	svc := NewCrmCustomer()
	pool := NewCrmPool()
	c, err := svc.Create(sales, "10", models.CrmCustomer{Name: "公海新客户", OwnerUserID: "10", PoolStatus: 1})
	if err != nil {
		t.Fatal(err)
	}
	if c.OwnerUserID != "" || c.PoolStatus != 1 || c.PoolAt == nil {
		t.Fatalf("pool flag ignored: %+v", c)
	}
	if ok, fail, err := svc.BatchImport(sales, "10", []map[string]string{{"姓名": "导入公海客户", "手机": "13000000001"}}); err != nil || ok != 1 || fail != 0 {
		t.Fatalf("import: %d %d %v", ok, fail, err)
	}
	_, total, err := pool.ListPool(sales, "", nil)
	if err != nil || total != 2 {
		t.Fatalf("invisible imports: %d %v", total, err)
	}
	var wins atomic.Int64
	var wg sync.WaitGroup
	for _, ctx := range []context.Context{sales, other} {
		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()
			user := "10"
			if ctx == other {
				user = "11"
			}
			if n, err := pool.Claim(ctx, user, []string{c.ID}); err == nil && n == 1 {
				wins.Add(1)
			}
		}(ctx)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("double claim: %d", wins.Load())
	}
	if err = db.First(&c, "id = ?", c.ID).Error; err != nil {
		t.Fatal(err)
	}
	owner := testIdentity("1", c.OwnerUserID)
	if n, err := pool.Recycle(owner, []string{c.ID}); err != nil || n != 1 {
		t.Fatalf("release own customer: %d %v", n, err)
	}
	var released models.CrmCustomer
	db.First(&released, "id = ?", c.ID)
	if released.OwnerUserID != "" || released.PrevOwnerUserID != c.OwnerUserID || released.PoolStatus != 1 {
		t.Fatalf("recycle inconsistent: %+v", released)
	}
	if _, err = pool.Assign(admin, "12", "20", []string{c.ID}); err == nil {
		t.Fatal("cross tenant assignment accepted")
	}
	if _, err = pool.Assign(admin, "12", "10", []string{c.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = NewCrmTransfer().Transfer(admin, "12", []string{c.ID}, "11", "销售交接"); err != nil {
		t.Fatal(err)
	}
	db.First(&released, "id = ?", c.ID)
	if released.OwnerUserID != "11" || released.ReassignStatus != 1 || released.ReassignFrom != "10" {
		t.Fatalf("missing receiving step: %+v", released)
	}
	if err = NewCrmCustomerExt().AcceptReassign(other, "11", c.ID); err != nil {
		t.Fatal(err)
	}
	next := time.Now().Add(time.Hour)
	followup, err := NewCrmFollowup().Create(other, "11", models.CrmFollowup{CustomerID: c.ID, Content: "确认采购需求", NextFollowupAt: &next})
	if err != nil {
		t.Fatal(err)
	}
	if followup.UserID != "11" {
		t.Fatal("followup author incorrect")
	}
	db.First(&released, "id = ?", c.ID)
	if released.NextFollowupAt == nil || released.NextFollowupAt.Unix() != next.Unix() {
		t.Fatal("followup reminder not updated")
	}
	if _, err = NewCrmFollowup().Create(sales, "10", models.CrmFollowup{CustomerID: c.ID, Content: "越权"}); err == nil {
		t.Fatal("former owner can follow up")
	}
	if _, err = NewCrmFollowup().Create(other, "11", models.CrmFollowup{CustomerID: c.ID, Content: "本次跟进完成"}); err != nil {
		t.Fatal(err)
	}
	var fresh models.CrmCustomer
	db.First(&fresh, "id = ?", c.ID)
	if fresh.NextFollowupAt != nil {
		t.Fatal("completed reminder remained overdue")
	}
}

func TestPoolQuotaAndRecycleRegression(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "pool", Status: 1})
	ctx := testIdentity("1", "10")
	admin := testIdentity("1", "12")
	pool := NewCrmPool()
	mustCreate(t, db, &models.CrmPoolRule{Model: models.Model{ID: "900"}, TenantID: "1", ClaimLimit: 2, ClaimCooldownDays: 3})
	ids := []string{}
	for _, name := range []string{"甲", "乙"} {
		c, err := NewCrmCustomer().Create(admin, "12", models.CrmCustomer{Name: name, PoolStatus: 1})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, c.ID)
	}
	var wins atomic.Int64
	var wg sync.WaitGroup
	for _, id := range ids {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			if _, err := pool.Claim(ctx, "10", []string{id}); err == nil {
				wins.Add(1)
			}
		}(id)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("quota race: %d", wins.Load())
	}
	if _, err := pool.Recycle(ctx, []string{"100", "101"}); err == nil {
		t.Fatal("partial unauthorized recycling accepted")
	}
	var own models.CrmCustomer
	db.First(&own, "id = ?", "100")
	if own.OwnerUserID != "10" {
		t.Fatal("batch partially recycled")
	}
	if _, err := pool.Recycle(ctx, []string{"100"}); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Claim(ctx, "10", []string{"100"}); err == nil {
		t.Fatal("cooldown bypassed")
	}
	// No-deal recycling must retain a signed contract even with no open opportunity.
	old := time.Now().AddDate(0, 0, -30)
	db.Model(&models.CrmCustomer{}).Where("id = ?", "101").Update("created_at", old)
	mustCreate(t, db, &models.CrmOrder{Model: models.Model{ID: "901"}, TenantID: "1", CustomerID: "101", UserID: "11", OrderNo: "keep", Amount: 100, Status: "signed"})
	db.Model(&models.CrmPoolRule{}).Where("id = ?", "900").Updates(map[string]any{"enabled": true, "no_deal_days": 7})
	if _, err := pool.AutoRecycle(testIdentity("1", "")); err != nil {
		t.Fatal(err)
	}
	var kept models.CrmCustomer
	db.First(&kept, "id = ?", "101")
	if kept.PoolStatus != 0 {
		t.Fatal("signed customer recycled as no deal")
	}
}

func TestReceivablesLifecycleRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "12")
	svc := NewCrmReceivables()
	now := time.Now()
	future := now.AddDate(0, 0, 7)
	order, err := NewCrmOrder().Create(ctx, "12", models.CrmOrder{Title: "年度服务合同", CustomerID: "100", Amount: 1000, Status: "signed"}, "")
	if err != nil {
		t.Fatal(err)
	}
	plans := []models.CrmReceivablesPlan{{OrderID: order.ID, PeriodNo: 1, Amount: 400, PlanDate: &future}, {OrderID: order.ID, PeriodNo: 2, Amount: 600, PlanDate: &future}}
	if err = NewCrmReceivablesPlan().BatchCreate(ctx, plans); err != nil {
		t.Fatal(err)
	}
	assertBalance := func(want float64, status, planState string) {
		t.Helper()
		var o models.CrmOrder
		var p models.CrmReceivablesPlan
		db.First(&o, "id = ?", order.ID)
		db.First(&p, "id = ?", plans[0].ID)
		if o.PaidAmount != want || o.PayStatus != status || p.Status != planState {
			t.Fatalf("balance paid=%v status=%s plan=%s", o.PaidAmount, o.PayStatus, p.Status)
		}
	}
	r1 := models.CrmReceivables{OrderID: order.ID, PlanID: plans[0].ID, Amount: 100, ReceivedAt: &now}
	if err = svc.Create(ctx, &r1); err != nil {
		t.Fatal(err)
	}
	assertBalance(100, "partial", "pending")
	r2 := models.CrmReceivables{OrderID: order.ID, PlanID: plans[0].ID, Amount: 300, ReceivedAt: &now}
	if err = svc.Create(ctx, &r2); err != nil {
		t.Fatal(err)
	}
	assertBalance(400, "partial", "received")
	if err = svc.Void(ctx, r2.ID); err != nil {
		t.Fatal(err)
	}
	assertBalance(100, "partial", "pending")
	if err = svc.Void(ctx, r2.ID); err != nil {
		t.Fatal(err)
	}
	assertBalance(100, "partial", "pending")
	r1.Amount = 150
	if err = svc.Update(ctx, r1.ID, &r1); err != nil {
		t.Fatal(err)
	}
	assertBalance(150, "partial", "pending")
	bad := models.CrmReceivables{OrderID: order.ID, CustomerID: "102", Amount: 10, ReceivedAt: &now}
	if err = svc.Create(ctx, &bad); err == nil {
		t.Fatal("foreign customer accepted")
	}
	r3 := models.CrmReceivables{OrderID: order.ID, PlanID: plans[1].ID, Amount: 500, ReceivedAt: &now, ApproveStatus: "pending"}
	if err = svc.Create(ctx, &r3); err != nil {
		t.Fatal(err)
	}
	assertBalance(150, "partial", "pending")
	if err = NewCrmReceivablesExt().Approve(testIdentity("1", "10"), r3.ID, "10"); err == nil {
		t.Fatal("sales approved receipt")
	}
	if err = NewCrmReceivablesExt().Approve(ctx, r3.ID, "12"); err != nil {
		t.Fatal(err)
	}
	assertBalance(650, "partial", "pending")
	if _, err = NewCrmOrder().Update(ctx, order.ID, models.CrmOrder{Title: "改价", CustomerID: "100", Amount: 600, UserID: "12", Status: "signed"}); err == nil {
		t.Fatal("lowered contract below collections")
	}
	tooMuch := models.CrmReceivables{OrderID: order.ID, Amount: 400, ReceivedAt: &now}
	if err = svc.Create(ctx, &tooMuch); err == nil {
		t.Fatal("overpayment accepted")
	}
	assertBalance(650, "partial", "pending")
	if err = NewCrmReceivablesPlan().Update(ctx, plans[0].ID, &models.CrmReceivablesPlan{OrderID: order.ID, PeriodNo: 1, Amount: 100, PlanDate: &future}); err == nil {
		t.Fatal("plan below received amount accepted")
	}
	pending := models.CrmReceivables{OrderID: order.ID, Amount: 100, ReceivedAt: &now, ApproveStatus: "pending"}
	if err = svc.Create(ctx, &pending); err != nil {
		t.Fatal(err)
	}
	if err = NewCrmReceivablesExt().Reject(ctx, pending.ID, "12", "凭证不符"); err != nil {
		t.Fatal(err)
	}
	assertBalance(650, "partial", "pending")
	if _, err = svc.Delete(ctx, []string{r1.ID}); err == nil {
		t.Fatal("confirmed receipt deleted without void")
	}
	if err = svc.Void(ctx, r3.ID); err != nil {
		t.Fatal(err)
	}
	assertBalance(150, "partial", "pending")
}

func TestPaymentConcurrencyRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "12")
	now := time.Now()
	order, err := NewCrmOrder().Create(ctx, "12", models.CrmOrder{Title: "并发回款", CustomerID: "100", Amount: 100}, "")
	if err != nil {
		t.Fatal(err)
	}
	var wins atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := models.CrmReceivables{OrderID: order.ID, Amount: 30, ReceivedAt: &now}
			if err := NewCrmReceivables().Create(ctx, &r); err == nil {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 3 {
		t.Fatalf("confirmed %d receipts, want 3", wins.Load())
	}
	var got models.CrmOrder
	db.First(&got, "id = ?", order.ID)
	if got.PaidAmount != 90 {
		t.Fatalf("paid amount=%v", got.PaidAmount)
	}
}

func TestOrderCouponAtomicRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "12")
	mustCreate(t, db, &models.CrmCoupon{Model: models.Model{ID: "900"}, TenantID: "1", Name: "抵扣", CouponType: "cash", FaceValue: 50, Threshold: 100, Status: "active", TotalCount: 5})
	coupon, err := NewCrmCoupon().Issue(ctx, "900", "100")
	if err != nil {
		t.Fatal(err)
	}
	order, err := NewCrmOrder().Create(ctx, "12", models.CrmOrder{Title: "已有合同", CustomerID: "100", Amount: 200, OrderNo: "same"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewCrmOrder().Create(ctx, "12", models.CrmOrder{Title: "失败合同", CustomerID: "100", Amount: 200, OrderNo: order.OrderNo}, coupon.Code); err == nil {
		t.Fatal("duplicate number accepted")
	}
	var rec models.CrmCouponRecord
	db.First(&rec, "id = ?", coupon.ID)
	if rec.Status != "issued" {
		t.Fatal("failed order consumed coupon")
	}
	if _, err = NewCrmOrder().Create(ctx, "12", models.CrmOrder{Title: "错客户", CustomerID: "101", Amount: 200}, coupon.Code); err == nil {
		t.Fatal("used another customer's coupon")
	}
	good, err := NewCrmOrder().Create(ctx, "12", models.CrmOrder{Title: "优惠合同", CustomerID: "100", Amount: 200}, coupon.Code)
	if err != nil {
		t.Fatal(err)
	}
	db.First(&rec, "id = ?", coupon.ID)
	if rec.OrderID != good.ID || rec.Status != "used" || good.CouponAmount != 50 {
		t.Fatal("order/coupon not linked atomically")
	}
}

func TestWorkflowEditingGuardsRegression(t *testing.T) {
	db := regressionDB(t)
	sales := testIdentity("1", "10")
	admin := testIdentity("1", "12")
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	// Legacy unassigned imports must remain discoverable and claimable without a data migration.
	mustCreate(t, db, &models.CrmCustomer{Model: models.Model{ID: "999"}, TenantID: "1", Name: "历史导入"})
	if _, total, err := NewCrmPool().ListPool(sales, "历史导入", nil); err != nil || total != 1 {
		t.Fatalf("legacy pool: %d %v", total, err)
	}
	if _, err := NewCrmPool().Claim(sales, "10", []string{"999"}); err != nil {
		t.Fatal(err)
	}
	// Editing a customer's owner to blank must release it and keep a previous-owner audit trail.
	updated, err := NewCrmCustomer().Update(sales, "100", models.CrmCustomer{Name: "释放客户"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.OwnerUserID != "" || updated.PoolStatus != 1 || updated.PrevOwnerUserID != "10" {
		t.Fatalf("edit release inconsistent: %+v", updated)
	}
	if _, err = NewCrmPool().Assign(admin, "12", "10", []string{"100"}); err != nil {
		t.Fatal(err)
	}
	if err = NewCrmCustomerExt().Lock(sales, []string{"100"}, "保护期"); err != nil {
		t.Fatal(err)
	}
	if _, err = NewCrmCustomer().Update(sales, "100", models.CrmCustomer{Name: "绕过锁定"}); err == nil {
		t.Fatal("edit bypasses lock")
	}
	order, err := NewCrmOrder().Create(admin, "12", models.CrmOrder{Title: "编辑验证", CustomerID: "101", Amount: 200}, "")
	if err != nil {
		t.Fatal(err)
	}
	p := models.CrmReceivablesPlan{OrderID: order.ID, PeriodNo: 1, Amount: 100, PlanDate: &tomorrow}
	if err = NewCrmReceivablesPlan().Create(admin, &p); err != nil {
		t.Fatal(err)
	}
	p.Amount = 150
	if err = NewCrmReceivablesPlan().Update(admin, p.ID, &p); err != nil {
		t.Fatal(err)
	}
	var got models.CrmReceivablesPlan
	db.First(&got, "id = ?", p.ID)
	if got.Amount != 150 {
		t.Fatal("plan edit did not persist")
	}
	r := models.CrmReceivables{OrderID: order.ID, PlanID: p.ID, Amount: 50, ReceivedAt: &now}
	if err = NewCrmReceivables().Create(admin, &r); err != nil {
		t.Fatal(err)
	}
	if err = NewCrmOrderExt().Change(admin, order.ID, map[string]any{"amount": 40}); err == nil {
		t.Fatal("change bypasses balance guard")
	}
	if err = NewCrmOrderExt().Revoke(admin, order.ID, "撤销合同"); err == nil {
		t.Fatal("revoked order with payments")
	}
	if err = NewCrmReceivables().Void(admin, r.ID); err != nil {
		t.Fatal(err)
	}
	if err = NewCrmOrderExt().Revoke(admin, order.ID, "撤销合同"); err != nil {
		t.Fatal(err)
	}
	if err = NewCrmReceivables().Void(admin, r.ID); err != nil {
		t.Fatal(err)
	}
	db.First(&got, "id = ?", p.ID)
	if got.Status != "cancelled" {
		t.Fatal("void retry reopened cancelled plan")
	}
}
