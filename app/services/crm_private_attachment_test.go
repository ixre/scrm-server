package services

import (
	"strings"
	"sync"
	"testing"

	"core/app/models"
	"core/migrations"
)

func TestPrivateAttachmentWorkflowRegression(t *testing.T) {
	db := regressionDB(t)
	ctx, other, foreign := testIdentity("1", "10"), testIdentity("1", "11"), testIdentity("2", "20")
	order, err := NewCrmOrder().Create(ctx, "10", models.CrmOrder{Title: "私有附件合同", CustomerID: "100", Amount: 100}, "")
	if err != nil {
		t.Fatal(err)
	}
	svc := NewCrmOrderAttachment()
	req := PrivateAttachmentUpload{OrderID: order.ID, UploadID: strings.Repeat("a", 32), FileName: "合同.pdf", FileSize: 12, SHA256: strings.Repeat("b", 64)}
	if err := svc.AuthorizeUpload(other, order.ID, req.UploadID); err == nil {
		t.Fatal("foreign salesperson upload authorized")
	}
	if _, err := svc.RegisterUpload(foreign, req); err == nil {
		t.Fatal("foreign tenant registration authorized")
	}
	var wg sync.WaitGroup
	ids := make(chan string, 8)
	for i := 0; i < cap(ids); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := svc.RegisterUpload(ctx, req)
			if err != nil {
				t.Error(err)
				return
			}
			ids <- a.ID
		}()
	}
	wg.Wait()
	close(ids)
	var id string
	for got := range ids {
		if id != "" && got != id {
			t.Fatal("retry created duplicate attachments")
		}
		id = got
	}
	if id == "" {
		t.Fatal("no attachment registered")
	}
	var count int64
	db.Model(&models.CrmOrderAttachment{}).Where("order_id = ?", order.ID).Count(&count)
	if count != 1 {
		t.Fatal("duplicate attachment rows", count)
	}
	a, err := svc.ResolveDownload(ctx, id)
	if err != nil || a.FileURL != "" || a.StorageKey != "1/"+order.ID+"/"+req.UploadID {
		t.Fatalf("private metadata: %+v %v", a, err)
	}
	if _, err := svc.ResolveDownload(other, id); err == nil {
		t.Fatal("other salesperson downloaded attachment")
	}
	if _, err := svc.ResolveDownload(foreign, id); err == nil {
		t.Fatal("other tenant downloaded attachment")
	}
	if err := svc.Create(ctx, &a); err == nil {
		t.Fatal("public business route accepted private metadata")
	}
	changed := req
	changed.SHA256 = strings.Repeat("c", 64)
	if _, err := svc.RegisterUpload(ctx, changed); err == nil {
		t.Fatal("upload key overwritten")
	}
	for _, size := range []int64{0, MaxContractAttachmentSize + 1} {
		bad := req
		bad.FileSize = size
		if _, err := svc.RegisterUpload(ctx, bad); err == nil {
			t.Fatal("invalid size accepted", size)
		}
	}
	if err := db.Model(&models.CrmOrder{}).Where("id = ?", order.ID).Update("status", "completed").Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.AuthorizeUpload(ctx, order.ID, req.UploadID); err != nil {
		t.Fatal("completed upload retry denied", err)
	}
	if retried, err := svc.RegisterUpload(ctx, req); err != nil || retried.ID != id {
		t.Fatal("completed upload retry not idempotent", err)
	}
	newUpload := req
	newUpload.UploadID = strings.Repeat("c", 32)
	if _, err := svc.RegisterUpload(ctx, newUpload); err == nil {
		t.Fatal("new upload to closed contract accepted")
	}
	// Ownership changes must affect every subsequent download, including old uploads.
	if err := db.Model(&models.CrmOrder{}).Where("id = ?", order.ID).Updates(map[string]any{"user_id": "11", "status": "draft"}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveDownload(ctx, id); err == nil {
		t.Fatal("former owner retained download access")
	}
	if _, err := svc.ResolveDownload(other, id); err != nil {
		t.Fatal("new owner cannot download", err)
	}
	if _, err := svc.Delete(other, []string{id}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveDownload(other, id); err == nil {
		t.Fatal("deleted attachment downloadable")
	}
	if err := db.Model(&models.CrmOrder{}).Where("id = ?", order.ID).Update("user_id", "10").Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RegisterUpload(ctx, req); err == nil {
		t.Fatal("deleted upload resurrected by retry")
	}
}

func TestPrivateAttachmentMigrationRegression(t *testing.T) {
	db := regressionDB(t)
	legacy := models.CrmOrderAttachment{Model: models.Model{ID: "500"}, TenantID: "1", OrderID: "100", UploadedBy: "10", FileName: "历史合同.pdf", FileURL: "https://example.com/old.pdf"}
	mustCreate(t, db, &legacy)
	if err := db.Exec("ALTER TABLE crm_order_attachment DROP COLUMN storage_key, DROP COLUMN sha256").Error; err != nil {
		t.Fatal(err)
	}
	sql, err := migrations.Files.ReadFile("0010_private_contract_attachments.sql")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := db.Exec(string(sql)).Error; err != nil {
			t.Fatal(err)
		}
	}
	var restored models.CrmOrderAttachment
	if err := db.First(&restored, "id = ?", legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if restored.FileURL != legacy.FileURL || restored.StorageKey != "" || restored.SHA256 != "" {
		t.Fatal("migration changed legacy attachment", restored)
	}
	key := "1/100/" + strings.Repeat("a", 32)
	if err := db.Model(&models.CrmOrderAttachment{}).Where("id = ?", legacy.ID).Update("storage_key", key).Error; err != nil {
		t.Fatal(err)
	}
	duplicate := legacy
	duplicate.ID = "501"
	duplicate.StorageKey = key
	if err := createCRM(db, &duplicate); err == nil {
		t.Fatal("storage key unique index missing")
	}
}
