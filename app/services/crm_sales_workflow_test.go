package services

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"core/app/models"
	"gorm.io/gorm"
)

func TestLeadConversionRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	lead, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "网站咨询", Name: "张经理", Mobile: "13912340001", Email: "sales@example.com", Province: "浙江"})
	if err != nil {
		t.Fatal(err)
	}
	f, err := NewCrmFollowup().Create(ctx, "10", models.CrmFollowup{LeadID: lead.ID, Content: "确认需求"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan string, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := NewCrmLeadExt().ConvertToCustomer(ctx, "10", lead.ID, "", "有效线索")
			if err != nil {
				t.Error(err)
				return
			}
			results <- c.ID
		}()
	}
	wg.Wait()
	close(results)
	var customerID string
	for id := range results {
		if customerID != "" && id != customerID {
			t.Fatal("repeated conversion created different customers")
		}
		customerID = id
	}
	if customerID == "" {
		t.Fatal("conversion returned no customer")
	}
	var count int64
	db.Model(&models.CrmCustomer{}).Where("mobile = ?", lead.Mobile).Count(&count)
	if count != 1 {
		t.Fatalf("duplicate customer count %d", count)
	}
	db.First(&lead, "id = ?", lead.ID)
	db.First(&f, "id = ?", f.ID)
	if lead.Status != "qualified" || lead.CustomerID != customerID || lead.ConvertCustomerID != customerID || f.CustomerID != customerID {
		t.Fatal("conversion broke history/link")
	}
	for _, operation := range []func() error{
		func() error { return NewCrmLeadExt().MoveToPool(ctx, []string{lead.ID}) },
		func() error {
			_, err := NewCrmLead().Update(ctx, lead.ID, models.CrmLead{Title: "reset", Status: "new"})
			return err
		},
		func() error {
			_, err := NewCrmLeadExt().ConvertToCustomer(testIdentity("1", "11"), "11", lead.ID, "", "")
			return err
		},
	} {
		if operation() == nil {
			t.Fatal("converted or unauthorized lead mutated")
		}
	}
	// Different leads with the same mobile must converge too.
	var siblings []models.CrmLead
	for i := 0; i < 4; i++ {
		l, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "重复咨询", Mobile: "13912340002"})
		if err != nil {
			t.Fatal(err)
		}
		siblings = append(siblings, l)
	}
	var ids sync.Map
	for _, l := range siblings {
		wg.Add(1)
		go func(l models.CrmLead) {
			defer wg.Done()
			c, err := NewCrmLeadExt().ConvertToCustomer(ctx, "10", l.ID, "", "")
			if err != nil {
				t.Error(err)
				return
			}
			ids.Store(c.ID, true)
		}(l)
	}
	wg.Wait()
	n := 0
	ids.Range(func(_, _ any) bool { n++; return true })
	if n != 1 {
		t.Fatalf("same mobile produced %d customers", n)
	}
	foreign, _ := NewCrmLead().Create(testIdentity("1", "11"), "11", models.CrmLead{Title: "冲突手机号", Mobile: lead.Mobile})
	if _, err := NewCrmLeadExt().ConvertToCustomer(testIdentity("1", "11"), "11", foreign.ID, "", ""); err == nil {
		t.Fatal("duplicate conversion exposed another owner's customer")
	}
	// The callback emulates failure after inserting the customer, before linking the lead.
	rollbackLead, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "原子性", Mobile: "13912340003"})
	if err != nil {
		t.Fatal(err)
	}
	callback := "regression:lead-link-failure"
	if err = db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "crm_lead" {
			tx.AddError(errors.New("injected link failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, convertErr := NewCrmLeadExt().ConvertToCustomer(ctx, "10", rollbackLead.ID, "", "")
	db.Callback().Update().Remove(callback)
	if convertErr == nil {
		t.Fatal("injected failure ignored")
	}
	db.Model(&models.CrmCustomer{}).Where("mobile = ?", rollbackLead.Mobile).Count(&count)
	if count != 0 {
		t.Fatal("failed conversion left an orphan customer")
	}
}

func TestLeadPoolRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	l, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "公海咨询"})
	if err != nil {
		t.Fatal(err)
	}
	if err = NewCrmLeadExt().MoveToPool(ctx, []string{l.ID}); err != nil {
		t.Fatal(err)
	}
	var stored models.CrmLead
	db.First(&stored, "id = ?", l.ID)
	if stored.OwnerUserID != "" || stored.PoolStatus != 1 {
		t.Fatal("pool owner not cleared")
	}
	var wins atomic.Int64
	var wg sync.WaitGroup
	for _, user := range []string{"10", "11"} {
		wg.Add(1)
		go func(user string) {
			defer wg.Done()
			if err := NewCrmLeadExt().ReceiveFromPool(testIdentity("1", user), user, l.ID); err == nil {
				wins.Add(1)
			}
		}(user)
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("claim winners %d", wins.Load())
	}
	db.First(&stored, "id = ?", l.ID)
	ownerCtx := testIdentity("1", stored.OwnerUserID)
	if err = NewCrmLeadExt().MarkLost(ownerCtx, stored.OwnerUserID, l.ID, "预算延期"); err != nil {
		t.Fatal(err)
	}
	if _, err = NewCrmLeadExt().ConvertToCustomer(ownerCtx, stored.OwnerUserID, l.ID, "", ""); err == nil {
		t.Fatal("lost lead converted")
	}
	if err = NewCrmLeadExt().Reopen(ownerCtx, stored.OwnerUserID, l.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = NewCrmLeadExt().ConvertToCustomer(ownerCtx, stored.OwnerUserID, l.ID, "", ""); err != nil {
		t.Fatal(err)
	}
}

func TestQuoteWorkflowRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	svc := NewCrmQuote()
	today := dayStart(time.Now())
	q := models.CrmQuote{Subject: "年度服务", CustomerID: "100", Amount: 1000, Discount: 100, FinalAmount: 1, ValidUntil: &today}
	if err := svc.Create(ctx, &q); err != nil {
		t.Fatal(err)
	}
	if q.FinalAmount != 900 || q.OwnerUserID != "10" || q.Status != "draft" || q.QuoteNo == "" {
		t.Fatalf("bad quote defaults: %+v", q)
	}
	for _, bad := range []models.CrmQuote{
		{Subject: "非法状态", CustomerID: "100", Status: "accepted"},
		{Subject: "优惠过量", CustomerID: "100", Amount: 100, Discount: 101},
		{Subject: "负数", CustomerID: "100", Amount: -1},
		{Subject: "客户越权", CustomerID: "101", Amount: 100},
		{Subject: "负责人跨租户", CustomerID: "100", Amount: 100, OwnerUserID: "20"},
	} {
		if err := svc.Create(ctx, &bad); err == nil {
			t.Fatalf("invalid quote accepted: %+v", bad)
		}
	}
	q.Amount, q.Discount = 1200, 200
	if err := svc.Update(ctx, q.ID, &q); err != nil {
		t.Fatal(err)
	}
	if q.FinalAmount != 1000 {
		t.Fatal("edited net price not recalculated")
	}
	if _, err := svc.ConvertToOrder(ctx, q.ID); err == nil {
		t.Fatal("draft converted")
	}
	if err := svc.ChangeStatus(ctx, q.ID, "sent"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ChangeStatus(ctx, q.ID, "accepted"); err != nil {
		t.Fatal(err)
	}
	if err := NewCrmQuoteExt().Void(ctx, q.ID, "绕过终态"); err == nil {
		t.Fatal("accepted quote voided")
	}
	if err := svc.Update(ctx, q.ID, &q); err == nil {
		t.Fatal("accepted quote edited")
	}
	var wg sync.WaitGroup
	var ids sync.Map
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			o, err := svc.ConvertToOrder(ctx, q.ID)
			if err != nil {
				t.Error(err)
				return
			}
			if o.Amount != 1000 || o.Status != "draft" || o.CustomerID != "100" || o.UserID != "10" {
				t.Errorf("bad contract: %+v", o)
			}
			ids.Store(o.ID, true)
		}()
	}
	wg.Wait()
	n := 0
	var orderID string
	ids.Range(func(k, _ any) bool { n++; orderID = k.(string); return true })
	if n != 1 {
		t.Fatalf("conversion produced %d orders", n)
	}
	if _, err := NewCrmOrder().Delete(ctx, []string{orderID}); err == nil {
		t.Fatal("linked contract deleted")
	}
	if _, err := svc.Get(testIdentity("1", "11"), q.ID); err == nil {
		t.Fatal("quote read by another owner")
	}
	if _, err := svc.ConvertToOrder(testIdentity("2", "20"), q.ID); err == nil {
		t.Fatal("foreign conversion accepted")
	}
	// Missing legacy numbers are saved, and later calls retain the same number.
	legacy := models.CrmQuote{Model: models.Model{ID: "800"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", Subject: "旧草稿", Status: "draft"}
	mustCreate(t, db, &legacy)
	no, err := NewCrmQuoteExt().GenerateNo(ctx, legacy.ID)
	if err != nil {
		t.Fatal(err)
	}
	db.First(&legacy, "id = ?", legacy.ID)
	if no == "" || legacy.QuoteNo != no {
		t.Fatal("generated number not persisted")
	}
	if _, err := svc.Delete(ctx, []string{legacy.ID, q.ID}); err == nil {
		t.Fatal("mixed deletion accepted")
	}
	if _, err := svc.Get(ctx, legacy.ID); err != nil {
		t.Fatal("failed batch partially deleted")
	}
	if err = NewCrmQuoteExt().Void(ctx, legacy.ID, ""); err == nil {
		t.Fatal("void without reason")
	}
	if err = NewCrmQuoteExt().Void(ctx, legacy.ID, "重复报价"); err != nil {
		t.Fatal(err)
	}
	expired := models.CrmQuote{Subject: "过期报价", CustomerID: "100", Amount: 100}
	yesterday := today.AddDate(0, 0, -1)
	expired.ValidUntil = &yesterday
	if err = svc.Create(ctx, &expired); err != nil {
		t.Fatal(err)
	}
	if err = svc.ChangeStatus(ctx, expired.ID, "sent"); err == nil {
		t.Fatal("expired quote sent")
	}
}

func TestQuoteConversionRollbackRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	svc := NewCrmQuote()
	q := models.CrmQuote{Subject: "事务报价", CustomerID: "100", Amount: 100}
	if err := svc.Create(ctx, &q); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"sent", "accepted"} {
		if err := svc.ChangeStatus(ctx, q.ID, status); err != nil {
			t.Fatal(err)
		}
	}
	callback := "regression:quote-link-failure"
	if err := db.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "crm_quote" {
			tx.AddError(errors.New("injected quote link failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err := svc.ConvertToOrder(ctx, q.ID)
	db.Callback().Update().Remove(callback)
	if err == nil {
		t.Fatal("conversion ignored persistence failure")
	}
	var count int64
	db.Model(&models.CrmOrder{}).Count(&count)
	if count != 0 {
		t.Fatal("orphan contract on failed conversion")
	}
	if _, err = svc.ConvertToOrder(ctx, q.ID); err != nil {
		t.Fatal(err)
	}
}

func TestOpportunityWorkflowRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	for _, stage := range []models.CrmPipelineStage{
		{Model: models.Model{ID: "700"}, TenantID: "1", Name: "需求确认", Sort: 1, Probability: 10},
		{Model: models.Model{ID: "701"}, TenantID: "1", Name: "方案报价", Sort: 2, Probability: 50},
		{Model: models.Model{ID: "702"}, TenantID: "1", Name: "赢单", Sort: 3, IsWon: true},
		{Model: models.Model{ID: "703"}, TenantID: "1", Name: "输单", Sort: 4, IsLost: true},
	} {
		mustCreate(t, db, &stage)
	}
	svc := NewCrmOpportunity()
	ext := NewCrmOpportunityExt()
	for _, bad := range []models.CrmOpportunity{{Title: "无客户"}, {Title: "越权客户", CustomerID: "101"}, {Title: "无效阶段", CustomerID: "100", StageID: "999"}} {
		if _, err := svc.Create(ctx, "10", bad); err == nil {
			t.Fatal("invalid opportunity accepted")
		}
	}
	o, err := svc.Create(ctx, "10", models.CrmOpportunity{Title: "年度服务", CustomerID: "100", Amount: 1000})
	if err != nil {
		t.Fatal(err)
	}
	if o.StageID != "700" || o.Status != "open" || o.Probability != 10 || o.ExpectedRevenue != 100 {
		t.Fatalf("bad defaults: %+v", o)
	}
	mustCreate(t, db, &models.CrmSalesRule{Model: models.Model{ID: "710"}, TenantID: "1", RequireFollowup: true, FollowupDays: 3})
	changed := o
	changed.StageID = "701"
	for name, fn := range map[string]func() error{
		"move":   func() error { _, err := svc.MoveStage(ctx, o.ID, "701"); return err },
		"change": func() error { return ext.ChangeStage(ctx, o.ID, "701") },
		"batch":  func() error { return ext.BatchChangeStage(ctx, []string{o.ID}, "701") },
		"update": func() error { _, err := svc.Update(ctx, o.ID, changed); return err },
		"win":    func() error { return ext.Win(ctx, o.ID, "确认成交", 900) },
	} {
		if fn() == nil {
			t.Errorf("%s bypassed followup rule", name)
		}
	}
	if _, err = NewCrmFollowup().Create(ctx, "10", models.CrmFollowup{OpportunityID: o.ID, Content: "确认采购计划"}); err != nil {
		t.Fatal(err)
	}
	if err = ext.ChangeStage(ctx, o.ID, "701"); err != nil {
		t.Fatal(err)
	}
	o, _ = svc.Get(ctx, o.ID)
	if o.Status != "open" || o.Probability != 50 || o.ExpectedRevenue != 500 {
		t.Fatal("stage metrics diverged")
	}
	if err = ext.Win(ctx, o.ID, "采购签约", 900); err != nil {
		t.Fatal(err)
	}
	o, _ = svc.Get(ctx, o.ID)
	if o.StageID != "702" || o.Status != "won" || o.Probability != 100 || o.WonAt == nil || o.ActualAmount != 900 {
		t.Fatal("win stage/status inconsistent")
	}
	if err = ext.ChangeStage(ctx, o.ID, "700"); err == nil {
		t.Fatal("closed deal reopened through stage change")
	}
	if err = ext.Reopen(ctx, o.ID); err != nil {
		t.Fatal(err)
	}
	o, _ = svc.Get(ctx, o.ID)
	if o.Status != "open" || o.StageID != "700" || o.WonAt != nil || o.ActualAmount != 0 {
		t.Fatal("reopened deal retains closed state")
	}
	other, err := svc.Create(testIdentity("1", "11"), "11", models.CrmOpportunity{Title: "他人商机", CustomerID: "101"})
	if err != nil {
		t.Fatal(err)
	}
	if err = ext.BatchChangeStage(ctx, []string{o.ID, other.ID}, "701"); err == nil {
		t.Fatal("unauthorized batch accepted")
	}
	o, _ = svc.Get(ctx, o.ID)
	if o.StageID != "700" {
		t.Fatal("failed batch partially applied")
	}
	if err = ext.Lose(ctx, o.ID, "预算取消"); err != nil {
		t.Fatal(err)
	}
	o, _ = svc.Get(ctx, o.ID)
	if o.Status != "lost" || o.StageID != "703" || o.LostAt == nil || o.Probability != 0 {
		t.Fatal("loss state inconsistent")
	}
}

func TestSalesJourneyRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	lead, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "展会咨询", Name: "李经理", Mobile: "13912340009"})
	if err != nil {
		t.Fatal(err)
	}
	customer, err := NewCrmLeadExt().ConvertToCustomer(ctx, "10", lead.ID, "", "采购需求明确")
	if err != nil {
		t.Fatal(err)
	}
	opp, err := NewCrmOpportunity().Create(ctx, "10", models.CrmOpportunity{Title: "培训服务", CustomerID: customer.ID, LeadID: lead.ID, Amount: 2000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewCrmFollowup().Create(ctx, "10", models.CrmFollowup{OpportunityID: opp.ID, Content: "确认服务范围"}); err != nil {
		t.Fatal(err)
	}
	quote := models.CrmQuote{Subject: "培训报价", CustomerID: customer.ID, OpportunityID: opp.ID, Amount: 2000, Discount: 200}
	if err = NewCrmQuote().Create(ctx, &quote); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"sent", "accepted"} {
		if err = NewCrmQuote().ChangeStatus(ctx, quote.ID, status); err != nil {
			t.Fatal(err)
		}
	}
	order, err := NewCrmQuote().ConvertToOrder(ctx, quote.ID)
	if err != nil {
		t.Fatal(err)
	}
	order.Status = "signed"
	now := time.Now()
	order.SignedAt = &now
	updated, err := NewCrmOrder().Sign(ctx, order.ID, &now)
	if err != nil {
		t.Fatal(err)
	}
	receipt := models.CrmReceivables{OrderID: order.ID, Amount: 1800, ReceivedAt: &now}
	if err = NewCrmReceivables().Create(ctx, &receipt); err != nil {
		t.Fatal(err)
	}
	db.First(&updated, "id = ?", order.ID)
	if updated.PayStatus != "paid" || updated.PaidAmount != 1800 || updated.Amount != 1800 {
		t.Fatalf("journey balance: %+v", updated)
	}
	if _, err = NewCrmOrder().Get(testIdentity("1", "11"), order.ID); err == nil {
		t.Fatal("another owner read contract")
	}
	if _, err = NewCrmOrder().Sign(testIdentity("1", "11"), order.ID, &now); err == nil {
		t.Fatal("another owner signed contract")
	}
	if _, err = NewCrmOrder().Sign(ctx, order.ID, &now); err != nil {
		t.Fatal("repeated signing failed")
	}
	if _, err = NewCrmQuote().ConvertToOrder(ctx, quote.ID); err != nil {
		t.Fatal(fmt.Errorf("retry after signing: %w", err))
	}
}
