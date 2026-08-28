package services

import (
	"context"
	"core/migrations"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"core/app/models"
	"core/common/identity"
	"gorm.io/gorm"
)

func TestSessionLifecycleRegression(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Name: "Test", Status: 1})
	ctx := testIdentity("1", "10")
	if _, err := ValidateBusinessSession(ctx, false); err != nil {
		t.Fatal(err)
	}
	if code, _ := ValidateBusinessSession(testIdentity("2", "10"), false); code != 401 {
		t.Fatal("cross-tenant identity accepted")
	}
	db.Model(&models.User{}).Where("id = 10").Update("status", 0)
	if code, _ := ValidateBusinessSession(ctx, false); code != 401 {
		t.Fatal("disabled account remains authorized")
	}
	db.Model(&models.User{}).Where("id = 10").Update("status", 1)
	past := time.Now().Add(-time.Hour)
	db.Model(&models.Tenant{}).Where("id = 1").Update("expire_at", past)
	if code, _ := ValidateBusinessSession(ctx, false); code != 403 {
		t.Fatal("expired tenant accepted")
	}
	if _, err := ValidateBusinessSession(ctx, true); err != nil {
		t.Fatal("billing blocked for expired tenant", err)
	}
	db.Model(&models.Tenant{}).Where("id = 1").Update("status", 0)
	if code, _ := ValidateBusinessSession(ctx, true); code != 403 {
		t.Fatal("disabled tenant accepted")
	}
	mustCreate(t, db, &models.PlatformAdmin{Model: models.Model{ID: "99"}, Username: "platform", Status: 1})
	platform := identity.WithIdentity(context.Background(), identity.Identity{IsPlatform: true, PlatformID: "99"})
	if _, err := ValidateBusinessSession(platform, false); err != nil {
		t.Fatal(err)
	}
	db.Model(&models.PlatformAdmin{}).Where("id = 99").Update("status", 0)
	if code, _ := ValidateBusinessSession(platform, false); code != 401 {
		t.Fatal("disabled platform admin accepted")
	}
}

func TestContactDirectoryRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	svc := NewCrmContact()
	if err := svc.Create(ctx, &models.CrmContact{Name: "Foreign", CustomerID: "101"}); err == nil {
		t.Fatal("foreign customer contact created")
	}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := svc.Create(ctx, &models.CrmContact{Name: fmt.Sprint("Primary", i), CustomerID: "100", IsPrimary: true}); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	var n int64
	db.Model(&models.CrmContact{}).Where("customer_id = 100 AND is_primary = true").Count(&n)
	if n != 1 {
		t.Fatal("primary contacts", n)
	}
	other := models.CrmContact{Name: "Other", CustomerID: "101"}
	if err := svc.Create(testIdentity("1", "11"), &other); err != nil {
		t.Fatal(err)
	}
	rows, total, err := svc.Query(ctx, QueryReq{})
	if err != nil || total != 6 || len(rows) != 6 {
		t.Fatalf("contact scope: %d %d %v", len(rows), total, err)
	}
	if _, err := svc.Get(ctx, other.ID); err == nil {
		t.Fatal("foreign contact leaked")
	}
	c := models.CrmContact{Name: "Unlinked", Phone: "13912345550", Department: "采购部"}
	if err := svc.Create(ctx, &c); err != nil {
		t.Fatal(err)
	}
	var converted sync.Map
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, err := NewCrmContactExt().ConvertToCustomer(ctx, "10", c.ID)
			if err != nil {
				t.Error(err)
				return
			}
			converted.Store(row.ID, true)
		}()
	}
	wg.Wait()
	count := 0
	var customerID string
	converted.Range(func(key, value any) bool { count++; customerID = key.(string); return true })
	if count != 1 {
		t.Fatal("duplicate conversion", count)
	}
	var customer models.CrmCustomer
	if err := db.First(&customer, "id = ?", customerID).Error; err != nil {
		t.Fatal(err)
	}
	if customer.OwnerUserID != "10" || customer.Company != "" {
		t.Fatal("conversion lost owner or treated department as company", customer)
	}
	c.CustomerID = customerID
	f, err := NewCrmFollowup().Create(ctx, "10", models.CrmFollowup{CustomerID: customerID, ContactID: c.ID, Content: "需求确认"})
	if err != nil {
		t.Fatal(err)
	}
	c.CustomerID = ""
	if err := svc.Update(ctx, c.ID, &c); err == nil {
		t.Fatal("historical contact unlinked")
	}
	if _, err := svc.Delete(ctx, []string{c.ID}); err == nil {
		t.Fatal("historical contact deleted")
	}
	if _, err := NewCrmFollowup().Delete(ctx, []string{f.ID}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Update(ctx, c.ID, &c); err != nil {
		t.Fatal("unlink bigint", err)
	}
	stored, err := svc.Get(ctx, c.ID)
	if err != nil || stored.CustomerID != "" {
		t.Fatal("unlink not saved", err)
	}
	if _, err := svc.Delete(ctx, []string{c.ID}); err != nil {
		t.Fatal(err)
	}
	_, total, err = svc.Query(ctx, QueryReq{Keyword: "Unlinked"})
	if err != nil || total != 0 {
		t.Fatal("deleted contact visible", total, err)
	}
}

func TestFollowupBoundariesRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	svc := NewCrmFollowup()
	l, err := NewCrmLead().Create(ctx, "10", models.CrmLead{Title: "咨询"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(ctx, "10", models.CrmFollowup{LeadID: l.ID, CustomerID: "100", Content: "wrong association"}); err == nil {
		t.Fatal("unrelated customer attached to lead")
	}
	if err := NewCrmLeadExt().MoveToPool(ctx, []string{l.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Create(testIdentity("1", "12"), "12", models.CrmFollowup{LeadID: l.ID, Content: "pool"}); err == nil {
		t.Fatal("unclaimed pool followed")
	}
	next := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	first, err := svc.Create(ctx, "10", models.CrmFollowup{CustomerID: "100", Content: "first", NextFollowupAt: &next})
	if err != nil {
		t.Fatal(err)
	}
	last, err := svc.Create(ctx, "10", models.CrmFollowup{CustomerID: "100", Content: "last"})
	if err != nil {
		t.Fatal(err)
	}
	comments := NewCrmFollowupComment()
	if _, err := comments.Create(testIdentity("1", "11"), first.ID, "11", "外人评论"); err == nil {
		t.Fatal("comment bypassed followup permission")
	}
	if _, err := comments.Create(ctx, first.ID, "10", "跟进补充"); err != nil {
		t.Fatal(err)
	}
	if _, err := comments.ListByFollowup(testIdentity("1", "11"), first.ID); err == nil {
		t.Fatal("comment history leaked")
	}
	if _, err := svc.ListByCustomer(testIdentity("1", "11"), "100"); err == nil {
		t.Fatal("followup read bypassed customer scope")
	}
	if _, err := svc.Delete(testIdentity("1", "11"), []string{last.ID}); err == nil {
		t.Fatal("another sales deleted followup")
	}
	if _, err := svc.Delete(ctx, []string{last.ID}); err != nil {
		t.Fatal(err)
	}
	var c models.CrmCustomer
	db.First(&c, "id = 100")
	if c.NextFollowupAt == nil || !c.NextFollowupAt.Equal(next) {
		t.Fatal("previous reminder not restored", c.NextFollowupAt)
	}
	if _, err := svc.Delete(ctx, []string{first.ID}); err != nil {
		t.Fatal(err)
	}
	c = models.CrmCustomer{}
	db.First(&c, "id = 100")
	if c.NextFollowupAt != nil {
		t.Fatal("last deleted followup left reminder")
	}
	if _, err := svc.Create(ctx, "10", models.CrmFollowup{LeadID: l.ID, ContactID: "999", Content: "contact without customer"}); err == nil {
		t.Fatal("orphan contact accepted")
	}
}

func TestContractChildrenRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	other := testIdentity("1", "11")
	order, err := NewCrmOrder().Create(ctx, "10", models.CrmOrder{Title: "合同", CustomerID: "100", Amount: 200}, "")
	if err != nil {
		t.Fatal(err)
	}
	products := NewCrmOrderProduct()
	p := models.CrmOrderProduct{OrderID: order.ID, ProductName: "服务A", Qty: 2, UnitPrice: 100, Discount: 50, Amount: 999}
	if err := products.Create(other, &p); err == nil {
		t.Fatal("foreign product added")
	}
	if err := products.Create(ctx, &p); err != nil {
		t.Fatal(err)
	}
	if p.Amount != 100 {
		t.Fatal("spoofed subtotal", p.Amount)
	}
	now := time.Now()
	if _, err := NewCrmOrder().Sign(ctx, order.ID, &now); err == nil {
		t.Fatal("partial details signed")
	}
	p2 := p
	p2.ID = ""
	if err := products.Create(ctx, &p2); err != nil {
		t.Fatal(err)
	}
	excess := p
	if err := products.Create(ctx, &excess); err == nil {
		t.Fatal("excess details accepted")
	}
	if _, err := products.ListByOrder(other, order.ID); err == nil {
		t.Fatal("foreign order details leaked")
	}
	if _, err := products.Delete(ctx, []string{p.ID, "999"}); err == nil {
		t.Fatal("mixed missing delete accepted")
	}
	var n int64
	db.Model(&models.CrmOrderProduct{}).Where("order_id = ?", order.ID).Count(&n)
	if n != 2 {
		t.Fatal("partial product delete", n)
	}
	attachment := models.CrmOrderAttachment{OrderID: order.ID, FileName: "合同.pdf", FileURL: "javascript:alert(1)"}
	if err := NewCrmOrderAttachment().Create(ctx, &attachment); err == nil {
		t.Fatal("unsafe attachment URL accepted")
	}
	attachment.FileURL = "https://example.com/contract.pdf"
	attachment.UploadedBy = "11"
	if err := NewCrmOrderAttachment().Create(ctx, &attachment); err != nil {
		t.Fatal(err)
	}
	if attachment.UploadedBy != "10" {
		t.Fatal("uploader forged")
	}
	progress := models.CrmOrderProgress{OrderID: order.ID, NodeName: "交付", Status: "done"}
	ps := NewCrmOrderProgress()
	if err := ps.Create(ctx, &progress); err != nil {
		t.Fatal(err)
	}
	if progress.DoneAt == nil {
		t.Fatal("done date missing")
	}
	if err := ps.UpdateStatus(ctx, progress.ID, "doing"); err != nil {
		t.Fatal(err)
	}
	stored := models.CrmOrderProgress{}
	db.First(&stored, "id = ?", progress.ID)
	if stored.DoneAt != nil {
		t.Fatal("reopened progress kept done date")
	}
	if err := ps.UpdateStatus(ctx, progress.ID, "invalid"); err == nil {
		t.Fatal("invalid progress accepted")
	}
	if err := ps.UpdateStatus(other, progress.ID, "done"); err == nil {
		t.Fatal("foreign progress changed")
	}
	if _, err := NewCrmOrder().Sign(ctx, order.ID, &now); err != nil {
		t.Fatal(err)
	}
	if _, err := products.Delete(ctx, []string{p.ID}); err == nil {
		t.Fatal("signed products deleted")
	}
	if _, err := NewCrmOrderAttachment().Delete(ctx, []string{attachment.ID}); err == nil {
		t.Fatal("signed proof deleted")
	}
	if err := NewCrmOrderExt().Revoke(ctx, order.ID, "取消合作"); err != nil {
		t.Fatal(err)
	}
	if err := ps.UpdateStatus(ctx, progress.ID, "done"); err == nil {
		t.Fatal("cancelled progress mutated")
	}
}

func TestBatchAndRecycleRegression(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	ctx := testIdentity("1", "10")
	admin := testIdentity("1", "12")
	// Deliberately use the same numeric ID in the two tables to catch incorrect routing.
	mustCreate(t, db, &models.CrmLead{Model: models.Model{ID: "100"}, TenantID: "1", OwnerUserID: "10", Title: "线索", Status: "new"})
	batch, err := NewCrmBatch().Execute(ctx, "10", BatchExecReq{EntityType: "lead", EntityIDs: []string{"100", "100"}, OpType: "seas"})
	if err != nil {
		t.Fatal(err)
	}
	if batch.SuccessCount != 1 {
		t.Fatal("duplicate counted twice", batch)
	}
	var c models.CrmCustomer
	db.First(&c, "id = 100")
	if c.PoolStatus != 0 || c.OwnerUserID != "10" {
		t.Fatal("lead batch mutated customer")
	}
	if _, err := NewCrmBatch().Execute(ctx, "11", BatchExecReq{EntityType: "customer", EntityIDs: []string{"100"}, OpType: "lock"}); err == nil {
		t.Fatal("operator forged")
	}
	if _, err := batchUpdateField(ctx, "customer", []string{"100", "101"}, "remark", "partial"); err == nil {
		t.Fatal("foreign batch field accepted")
	}
	db.First(&c, "id = 100")
	if c.Remark != "" {
		t.Fatal("partial field update persisted")
	}
	if _, err := batchUpdateField(ctx, "lead", []string{"100"}, "level", "5"); err == nil {
		t.Fatal("nonexistent lead field accepted")
	}
	if _, err := batchUpdateField(ctx, "customer", []string{"100"}, "level", "VIP"); err == nil {
		t.Fatal("invalid level accepted")
	}
	if n, err := batchUpdateField(ctx, "customer", []string{"100", "100"}, "level", "5"); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if _, err := NewCrmCustomer().Delete(ctx, []string{"100"}); err != nil {
		t.Fatal(err)
	}
	recs, total, err := NewCrmRecycle().List(admin, "customer", RecycleQueryReq{Keyword: "Own"})
	if err != nil || total != 1 {
		t.Fatal(total, err)
	}
	if _, _, err := NewCrmRecycle().List(ctx, "", RecycleQueryReq{}); err == nil {
		t.Fatal("sales sees tenant recycle records")
	}
	if err := NewCrmRecycle().Restore(admin, recs[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := NewCrmRecycle().Clear(admin, recs[0].ID); err == nil {
		t.Fatal("restored entity permanently deleted")
	}
	if _, err := NewCrmCustomer().Delete(ctx, []string{"100"}); err != nil {
		t.Fatal(err)
	}
	recs, _, err = NewCrmRecycle().List(admin, "customer", RecycleQueryReq{})
	if err != nil || len(recs) != 1 {
		t.Fatal(err)
	}
	if _, err := NewCrmRecycle().DeleteRecord(admin, []string{recs[0].ID, "999999999999999999"}); err == nil {
		t.Fatal("mixed invalid clear accepted")
	}
	if err := NewCrmRecycle().Restore(admin, recs[0].ID); err != nil {
		t.Fatal("failed clear did not roll back", err)
	}
	contact := models.CrmContact{CustomerID: "100", Name: "联系人"}
	if err := NewCrmContact().Create(ctx, &contact); err != nil {
		t.Fatal(err)
	}
	if _, err := NewCrmCustomer().Delete(ctx, []string{"100"}); err == nil {
		t.Fatal("customer with contact deleted")
	}
	if _, err := NewCrmContact().Delete(ctx, []string{contact.ID}); err != nil {
		t.Fatal(err)
	}
	recs, _, err = NewCrmRecycle().List(admin, "contact", RecycleQueryReq{})
	if err != nil || len(recs) != 1 {
		t.Fatal(err)
	}
	if err := NewCrmRecycle().Clear(admin, recs[0].ID); err != nil {
		t.Fatal(err)
	}
	var n int64
	db.Unscoped().Model(&models.CrmContact{}).Where("id = ?", contact.ID).Count(&n)
	if n != 0 {
		t.Fatal("clear kept contact")
	}
}

func TestPipelineConfigurationRegression(t *testing.T) {
	db := regressionDB(t)
	admin := testIdentity("1", "12")
	svc := NewCrmPipelineStage()
	if _, err := svc.Create(testIdentity("1", "10"), models.CrmPipelineStage{Name: "终态", IsWon: true}); err == nil {
		t.Fatal("sales changed pipeline")
	}
	if _, err := svc.Create(admin, models.CrmPipelineStage{Name: "终态", IsWon: true, IsLost: true}); err == nil {
		t.Fatal("contradictory stage accepted")
	}
	stage, err := svc.Create(admin, models.CrmPipelineStage{Name: "沟通", Probability: 30})
	if err != nil {
		t.Fatal(err)
	}
	mustCreate(t, db, &models.CrmOpportunity{Model: models.Model{ID: "888"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: stage.ID, Title: "机会", Status: "open"})
	if _, err := svc.Delete(admin, []string{stage.ID}); err == nil {
		t.Fatal("in-use stage deleted")
	}
	stage.Probability = 80
	if _, err := svc.Update(admin, stage.ID, stage); err == nil {
		t.Fatal("in-use probability changed without opportunities")
	}
	stage.Probability = 30
	stage.Name = "需求沟通"
	if _, err := svc.Update(admin, stage.ID, stage); err != nil {
		t.Fatal(err)
	}
}

func TestPublicAcquisitionRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Name: "商家", Status: 1})
	svc := NewCrmSmartCard()
	codes := map[string]bool{}
	var card models.CrmSmartCard
	for i := 0; i < 8; i++ {
		var err error
		card, err = svc.Create(ctx, models.CrmSmartCard{Name: "销售", UserID: "10"})
		if err != nil {
			t.Fatal(err)
		}
		if codes[card.ShareCode] || len(card.ShareCode) != 32 {
			t.Fatal("colliding share code")
		}
		codes[card.ShareCode] = true
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := svc.SubmitLead(card.ShareCode, "访客", "139 1234 9001", "咨询"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	var n int64
	db.Model(&models.CrmLead{}).Where("mobile = ?", "13912349001").Count(&n)
	if n != 1 {
		t.Fatal("duplicate card lead", n)
	}
	if _, err := svc.Delete(testIdentity("1", "11"), []string{card.ID}); err == nil {
		t.Fatal("foreign card deleted")
	}
	mustCreate(t, db, &models.CrmActivity{Model: models.Model{ID: "600"}, TenantID: "1", Name: "推广", Status: "active"})
	page, err := NewCrmLandingPage().Create(ctx, models.CrmLandingPage{Title: "推广", ActivityID: "600"})
	if err != nil {
		t.Fatal(err)
	}
	customerIDs := sync.Map{}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, err := NewCrmLandingPage().SubmitForm(page.Slug, map[string]string{"name": "线索", "mobile": "13912349002"})
			if err != nil {
				t.Error(err)
				return
			}
			customerIDs.Store(c.ID, true)
		}()
	}
	wg.Wait()
	unique := 0
	customerIDs.Range(func(key, value any) bool { unique++; return true })
	if unique != 1 {
		t.Fatal("duplicate landing customers", unique)
	}
	db.Model(&models.CrmLead{}).Where("mobile = ?", "13912349002").Count(&n)
	if n != 1 {
		t.Fatal("duplicate landing leads", n)
	}
	var lead models.CrmLead
	db.First(&lead, "mobile = ?", "13912349002")
	if lead.PoolStatus != 1 || lead.CustomerID == "" {
		t.Fatal("landing lead not claimable or not linked")
	}
	db.First(&page, "id = ?", page.ID)
	if page.SubmitCount != 8 {
		t.Fatal("submission count", page.SubmitCount)
	}
	var activity models.CrmActivity
	db.First(&activity, "id = 600")
	if activity.ParticipateCount != 1 {
		t.Fatal("participation count", activity)
	}
	db.Model(&models.User{}).Where("id = 10").Update("status", 0)
	if _, err := svc.GetByShareCode(card.ShareCode); err == nil {
		t.Fatal("disabled staff still publicly collecting")
	}
	db.Model(&models.Tenant{}).Where("id = 1").Update("status", 0)
	if _, err := NewCrmLandingPage().GetBySlug(page.Slug); err == nil {
		t.Fatal("disabled tenant page available")
	}
}

func TestAcquisitionRollbackRegression(t *testing.T) {
	db := regressionDB(t)
	ctx := testIdentity("1", "10")
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Name: "商家", Status: 1})
	page, err := NewCrmLandingPage().Create(ctx, models.CrmLandingPage{Title: "推广"})
	if err != nil {
		t.Fatal(err)
	}
	callback := "regression:acquisition-failure"
	if err := db.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "crm_lead" {
			tx.AddError(errors.New("injected lead failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, submitErr := NewCrmLandingPage().SubmitForm(page.Slug, map[string]string{"name": "回滚", "mobile": "13912349003"})
	db.Callback().Create().Remove(callback)
	if submitErr == nil {
		t.Fatal("lead failure ignored")
	}
	var n int64
	db.Model(&models.CrmCustomer{}).Where("mobile = ?", "13912349003").Count(&n)
	if n != 0 {
		t.Fatal("partial customer persisted")
	}
	db.First(&page, "id = ?", page.ID)
	if page.SubmitCount != 0 {
		t.Fatal("failed submission counted")
	}
	if _, err := NewCrmLandingPage().SubmitForm(page.Slug, map[string]string{"name": "", "mobile": "x"}); err == nil {
		t.Fatal("invalid public form accepted")
	}
	for _, id := range []string{"701", "702"} {
		mustCreate(t, db, &models.CrmSmartCard{Model: models.Model{ID: id}, TenantID: "1", UserID: "10", Name: "legacy", ShareCode: "duplicate", Status: "active"})
	}
	if _, err := NewCrmSmartCard().GetByShareCode("duplicate"); err == nil {
		t.Fatal("ambiguous legacy share code accepted")
	}
}

func TestLegacyRecycleMigrationRegression(t *testing.T) {
	db := regressionDB(t)
	mustCreate(t, db, &models.Tenant{Model: models.Model{ID: "1"}, Code: "a", Status: 1})
	if err := db.Where("id = 100").Delete(&models.CrmCustomer{}).Error; err != nil {
		t.Fatal(err)
	}
	mustCreate(t, db, &models.CrmLead{Model: models.Model{ID: "999"}, TenantID: "1", OwnerUserID: "10", CustomerID: "100", Title: "历史线索", Status: "new"})
	if err := db.Where("id = 999").Delete(&models.CrmLead{}).Error; err != nil {
		t.Fatal(err)
	}
	raw, err := migrations.Files.ReadFile("0009_recycle_legacy_records.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Transaction(func(tx *gorm.DB) error {
			for _, stmt := range strings.Split(string(raw), ";") {
				if strings.TrimSpace(stmt) != "" {
					if err := tx.Exec(stmt).Error; err != nil {
						return err
					}
				}
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	admin := testIdentity("1", "12")
	rows, total, err := NewCrmRecycle().List(admin, "", RecycleQueryReq{})
	if err != nil || total != 2 {
		t.Fatal("legacy index not idempotent", total, err)
	}
	var customerRec, leadRec string
	for _, row := range rows {
		if row.DeletedBy != "" {
			t.Fatal("historical deleter fabricated")
		}
		if row.EntityType == "customer" {
			customerRec = row.ID
		} else {
			leadRec = row.ID
		}
	}
	if err := NewCrmRecycle().Restore(admin, leadRec); err == nil {
		t.Fatal("orphan lead restored")
	}
	if err := NewCrmRecycle().Restore(admin, customerRec); err != nil {
		t.Fatal(err)
	}
	if err := NewCrmRecycle().Restore(admin, leadRec); err != nil {
		t.Fatal(err)
	}
}
