package services

import (
	"core/app/models"
	"core/migrations"
	"fmt"
	"testing"
	"time"
)

func TestCustomerHistoryCursor(t *testing.T) {
	db := regressionDB(t)
	sql, err := migrations.Files.ReadFile("0018_bounded_crm_reads.sql")
	requireNoError(t, err)
	requireNoError(t, db.Exec(string(sql)).Error)
	requireNoError(t, db.Exec(string(sql)).Error)
	when := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 45; i++ {
		mustCreate(t, db, &models.CrmFollowup{Model: models.Model{ID: fmt.Sprint(1000 + i)}, Timestamp: models.Timestamp{CreatedAt: when}, TenantID: "1", CustomerID: "100", Content: fmt.Sprint(i)})
		mustCreate(t, db, &models.CrmActivityRecord{Model: models.Model{ID: fmt.Sprint(2000 + i)}, Timestamp: models.Timestamp{CreatedAt: when}, TenantID: "1", CustomerID: "100"})
	}
	mustCreate(t, db, &models.CrmFollowup{Model: models.Model{ID: "9000"}, TenantID: "1", CustomerID: "101", Content: "hidden"})
	ctx := testIdentity("1", "10")
	svc := NewCrmCustomer()
	page, err := svc.History(ctx, CustomerHistoryRequest{CustomerID: "100", Kind: "followups"})
	requireNoError(t, err)
	if len(page.Items.([]followupHistoryRow)) != 20 || !page.HasMore || page.NextCursor == "" {
		t.Fatal("initial page not bounded")
	}
	firstCursor := page.NextCursor
	seen := map[string]bool{}
	for _, r := range page.Items.([]followupHistoryRow) {
		seen[r.ID] = true
	}
	// A new record must not shift the older-record cursor or repeat prior results.
	mustCreate(t, db, &models.CrmFollowup{Model: models.Model{ID: "5000"}, Timestamp: models.Timestamp{CreatedAt: when.Add(time.Minute)}, TenantID: "1", CustomerID: "100"})
	for page.HasMore {
		page, err = svc.History(ctx, CustomerHistoryRequest{CustomerID: "100", Kind: "followups", Cursor: page.NextCursor})
		requireNoError(t, err)
		for _, r := range page.Items.([]followupHistoryRow) {
			if seen[r.ID] {
				t.Fatal("duplicate across cursor pages")
			}
			seen[r.ID] = true
		}
	}
	if len(seen) != 45 || seen["5000"] || seen["9000"] {
		t.Fatal("cursor missed or leaked history", len(seen))
	}
	for _, req := range []CustomerHistoryRequest{
		{CustomerID: "100", Kind: "activity_records", Cursor: firstCursor},
		{CustomerID: "101", Kind: "followups", Cursor: firstCursor},
		{CustomerID: "100", Kind: "followups", Cursor: "invalid"},
	} {
		if _, err = svc.History(ctx, req); err == nil {
			t.Fatal("invalid cursor/access accepted")
		}
	}
	page, err = svc.History(ctx, CustomerHistoryRequest{CustomerID: "100", Kind: "activity_records"})
	requireNoError(t, err)
	if len(page.Items.([]activityHistoryRow)) != 20 || !page.HasMore {
		t.Fatal("NULL happened_at not paged")
	}
	page, err = svc.History(ctx, CustomerHistoryRequest{CustomerID: "100", Kind: "activity_records", Cursor: page.NextCursor})
	requireNoError(t, err)
	if len(page.Items.([]activityHistoryRow)) != 20 || page.Items.([]activityHistoryRow)[0].ID != "2024" {
		t.Fatal("NULL timestamp cursor skipped records")
	}
	detail, err := svc.GetDetail(ctx, "100")
	requireNoError(t, err)
	if len(detail["followups"].([]followupHistoryRow)) != 20 || len(detail["activity_records"].([]activityHistoryRow)) != 20 {
		t.Fatal("detail still loads full history")
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id = 100").Update("owner_user_id", "11").Error)
	if _, err = svc.History(ctx, CustomerHistoryRequest{CustomerID: "100", Kind: "followups", Cursor: firstCursor}); err == nil {
		t.Fatal("cursor bypassed changed customer access")
	}
}

func TestUserOptionsBeyondFirst500(t *testing.T) {
	db := regressionDB(t)
	// More than 500 active employees must be reachable without loading employee profiles.
	var users []models.User
	for i := 0; i < 605; i++ {
		users = append(users, models.User{Model: models.Model{ID: fmt.Sprint(1000 + i)}, TenantID: "1", Username: fmt.Sprintf("member-%04d", i), Name: fmt.Sprintf("销售%04d", i), Status: 1})
	}
	requireNoError(t, db.Omit("RoleID", "DepartmentID").CreateInBatches(&users, 100).Error)
	mustCreate(t, db, &models.User{Model: models.Model{ID: "9000"}, TenantID: "1", Username: "disabled", Status: 2})
	ctx := testIdentity("1", "10")
	svc := NewUser()
	seen := map[string]bool{}
	cursor := ""
	for {
		page, err := svc.Options(ctx, UserOptionsRequest{AfterID: cursor, PageSize: 50})
		requireNoError(t, err)
		if len(page.Items) > 50 {
			t.Fatal("unbounded employee page")
		}
		for _, u := range page.Items {
			if seen[u.ID] || u.Status != 1 || u.ID == "20" {
				t.Fatal("duplicate/ineligible employee")
			}
			seen[u.ID] = true
		}
		if !page.HasMore {
			break
		}
		cursor = page.NextCursor
	}
	if len(seen) != 608 || !seen["1604"] {
		t.Fatal("employees beyond 500 unreachable", len(seen))
	}
	page, err := svc.Options(ctx, UserOptionsRequest{Keyword: "member-0604"})
	requireNoError(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != "1604" {
		t.Fatal("remote search failed")
	}
	page, err = svc.Options(ctx, UserOptionsRequest{IDs: []string{"1604", "9000", "20"}})
	requireNoError(t, err)
	if len(page.Items) != 2 || page.Items[1].Status != 2 {
		t.Fatal("selected lookup leaked tenant or lost disabled label")
	}
	page, err = svc.Options(ctx, UserOptionsRequest{Keyword: "%"})
	requireNoError(t, err)
	if len(page.Items) != 0 {
		t.Fatal("wildcard search unexpectedly matched every employee")
	}
	if _, err = svc.Options(testIdentity("2", "10"), UserOptionsRequest{}); err == nil {
		t.Fatal("foreign actor accepted")
	}
}
