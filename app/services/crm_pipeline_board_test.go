package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/migrations"
	"gorm.io/gorm"
)

func pipelineReadDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := regressionDB(t)
	ddl, err := migrations.OpportunityReadIndexStatements()
	requireNoError(t, err)
	for i := 0; i < 2; i++ {
		for _, stmt := range ddl {
			requireNoError(t, db.Exec(stmt).Error)
		}
	}
	for _, stage := range []models.CrmPipelineStage{
		{Model: models.Model{ID: "200"}, TenantID: "1", Name: "确认需求", Probability: 30},
		{Model: models.Model{ID: "201"}, TenantID: "1", Name: "方案", Probability: 70, Sort: 1},
		{Model: models.Model{ID: "202"}, TenantID: "1", Name: "赢单", Probability: 100, IsWon: true, Sort: 2},
		{Model: models.Model{ID: "300"}, TenantID: "2", Name: "Foreign secret", Probability: 50},
	} {
		copy := stage
		mustCreate(t, db, &copy)
	}
	return db
}
func pipelineColumn(t *testing.T, board PipelineBoard, id string) PipelineColumn {
	t.Helper()
	for _, column := range board.Columns {
		if column.StageID == id {
			return column
		}
	}
	t.Fatalf("missing column %s", id)
	return PipelineColumn{}
}
func TestPipelineBoardCompleteTotalsAndPaging(t *testing.T) {
	db := pipelineReadDB(t)
	ctx := testIdentity("1", "10")
	svc := NewCrmOpportunity()
	row := models.CrmOpportunity{Model: models.Model{ID: "1000"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "200", Title: "deal-0000", Status: "open", Amount: 0.01, Probability: 50}
	mustCreate(t, db, &row)
	small, err := countPaymentStatements(db, func() error { _, err := svc.Board(ctx, PipelineFilter{}); return err })
	requireNoError(t, err)
	var rows []models.CrmOpportunity
	for i := 1; i < 1005; i++ {
		copy := row
		copy.ID = fmt.Sprint(1000 + i)
		copy.Title = fmt.Sprintf("deal-%04d", i)
		rows = append(rows, copy)
	}
	requireNoError(t, db.Omit("LeadID").CreateInBatches(&rows, 100).Error)
	var board PipelineBoard
	large, err := countPaymentStatements(db, func() error { var err error; board, err = svc.Board(ctx, PipelineFilter{}); return err })
	requireNoError(t, err)
	if small != large || large > 10 {
		t.Fatalf("SQL calls grow per opportunity: %d/%d", small, large)
	}
	t.Logf("pipeline SQL calls with 1/1005 opportunities: %d/%d", small, large)
	column := pipelineColumn(t, board, "200")
	if column.Count != 1005 || column.Amount != "10.05" || column.WeightedAmount != "5.03" || len(column.Items) != 20 || !column.HasMore {
		t.Fatalf("summary truncated or rounded incorrectly: %+v", column)
	}
	if column.Items[0].Amount != "0.01" {
		t.Fatal("card money is not exact decimal text")
	}
	firstCursor := column.NextCursor
	copy := row
	copy.ID = "9000"
	mustCreate(t, db, &copy)
	seen := map[string]bool{}
	for _, card := range column.Items {
		seen[card.ID] = true
	}
	page := column.PipelinePage
	for page.HasMore {
		page, err = svc.BoardPage(ctx, PipelinePageRequest{StageID: "200", Cursor: page.NextCursor, PageSize: 1000})
		requireNoError(t, err)
		if len(page.Items) > 100 {
			t.Fatal("unbounded column page")
		}
		for _, card := range page.Items {
			if seen[card.ID] {
				t.Fatal("duplicate card")
			}
			seen[card.ID] = true
		}
	}
	if len(seen) != 1005 || seen["9000"] || !seen["1000"] {
		t.Fatal("older card missing or concurrent insertion shifted cursor", len(seen))
	}
	for _, req := range []PipelinePageRequest{
		{StageID: "201", Cursor: firstCursor}, {StageID: "200", Cursor: "invalid"}, {StageID: "200", Cursor: strings.Repeat("x", 4097)},
		{StageID: "200", Cursor: firstCursor, PipelineFilter: PipelineFilter{OwnerUserID: "11"}},
		{StageID: "200", Cursor: firstCursor, PipelineFilter: PipelineFilter{DateFrom: "2026-10-19", DateTo: "2026-10-19"}},
	} {
		if _, err = svc.BoardPage(ctx, req); err == nil {
			t.Fatal("cursor scope bypassed")
		}
	}
	if _, err = svc.BoardPage(testIdentity("1", "12"), PipelinePageRequest{StageID: "200", Cursor: firstCursor}); err == nil {
		t.Fatal("cursor changed actor")
	}
	requireNoError(t, db.Model(&models.CrmOpportunity{}).Where("id=1000").Update("owner_user_id", "11").Error)
	page, err = svc.BoardPage(ctx, PipelinePageRequest{StageID: "200", Cursor: firstCursor})
	requireNoError(t, err)
	for _, card := range page.Items {
		if card.ID == "1000" {
			t.Fatal("transferred card still visible")
		}
	}
}

func TestPipelineBoardDatesPermissionsAndOrphans(t *testing.T) {
	db := pipelineReadDB(t)
	svc := NewCrmOpportunity()
	ctx := testIdentity("1", "10")
	start := time.Date(2026, 10, 19, 0, 0, 0, 0, constants.PRCLocation)
	before, last, after := start.Add(-time.Second), start.Add(24*time.Hour-time.Nanosecond), start.AddDate(0, 0, 1)
	// PostgreSQL stores microseconds, so use the last representable instant of the day.
	last = start.Add(24*time.Hour - time.Microsecond)
	for i, when := range []*time.Time{&before, &start, &last, &after, nil} {
		mustCreate(t, db, &models.CrmOpportunity{Model: models.Model{ID: fmt.Sprint(1000 + i)}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "200", Status: "open", Amount: 1, Probability: 50, ExpectedCloseDate: when})
	}
	for _, row := range []models.CrmOpportunity{
		{Model: models.Model{ID: "1100"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", Status: "open", Amount: 2},
		{Model: models.Model{ID: "1101"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "999", Status: "open", Amount: 3},
		{Model: models.Model{ID: "1102"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "300", Status: "open", Amount: 4},
		{Model: models.Model{ID: "1200"}, TenantID: "1", CustomerID: "101", OwnerUserID: "11", StageID: "200", Status: "open", Amount: 999, ExpectedCloseDate: &start},
		{Model: models.Model{ID: "1201"}, TenantID: "2", CustomerID: "102", OwnerUserID: "20", StageID: "300", Status: "open", Amount: 9999, ExpectedCloseDate: &start},
		{Model: models.Model{ID: "1300"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "200", Status: "won", Amount: 888},
		{Model: models.Model{ID: "1301"}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "200", Status: "open", Amount: 888},
	} {
		copy := row
		mustCreate(t, db, &copy)
	}
	requireNoError(t, db.Where("id=1301").Delete(&models.CrmOpportunity{}).Error)
	board, err := svc.Board(ctx, PipelineFilter{})
	requireNoError(t, err)
	column := pipelineColumn(t, board, "200")
	if column.Count != 5 || column.Amount != "5.00" {
		t.Fatal("read boundary leaks other owner, closed or deleted row", column)
	}
	// Legacy funnel/inspection endpoints must not expose tenant-wide values to sales.
	for _, actor := range []struct {
		id   string
		want int64
	}{{"10", 5}, {"12", 6}} {
		funnel, err := NewCrmReport().PipelineFunnel(testIdentity("1", actor.id), "")
		requireNoError(t, err)
		for _, stage := range funnel {
			if stage["stage_id"] == "200" && stage["count"].(int64) != actor.want {
				t.Fatal("legacy funnel leaks another owner's totals", stage)
			}
		}
		inspection, err := NewCrmReport().PipelineInspection(testIdentity("1", actor.id), "")
		requireNoError(t, err)
		if inspection["total_open_count"].(int64) != actor.want+3 {
			t.Fatal("legacy inspection scope differs from board", inspection)
		}
	}
	inspection, err := NewCrmReport().PipelineInspection(ctx, "11")
	requireNoError(t, err)
	if inspection["total_open_count"].(int64) != 0 || inspection["won_count"].(int64) != 0 {
		t.Fatal("legacy owner filter escalates access")
	}
	orphan := pipelineColumn(t, board, unassignedPipelineStage)
	if orphan.Count != 3 || orphan.Amount != "9.00" {
		t.Fatal("unconfigured history is hidden", orphan)
	}
	for _, column := range board.Columns {
		if column.StageID == "300" || strings.Contains(column.Name, "Foreign") {
			t.Fatal("foreign stage exposed")
		}
	}
	filtered := PipelineFilter{DateFrom: "2026-10-19", DateTo: "2026-10-19"}
	board, err = svc.Board(ctx, filtered)
	requireNoError(t, err)
	column = pipelineColumn(t, board, "200")
	if column.Count != 2 || column.Amount != "2.00" || column.Items[0].ID != "1002" || column.Items[1].ID != "1001" {
		t.Fatal("China calendar boundary or SQL-side filtering incorrect", column)
	}
	board, err = svc.Board(testIdentity("1", "12"), filtered)
	requireNoError(t, err)
	if column = pipelineColumn(t, board, "200"); column.Count != 3 || column.Amount != "1001.00" {
		t.Fatal("manager tenant scope incorrect", column)
	}
	board, err = svc.Board(ctx, PipelineFilter{OwnerUserID: "11"})
	requireNoError(t, err)
	if pipelineColumn(t, board, "200").Count != 0 {
		t.Fatal("owner filter escalates permissions")
	}
	for _, filter := range []PipelineFilter{{DateFrom: "2026-10-19"}, {DateFrom: "2026-02-29", DateTo: "2026-03-01"}, {DateFrom: "2026-10-20", DateTo: "2026-10-19"}, {OwnerUserID: "bad"}} {
		if _, err = svc.Board(ctx, filter); err == nil {
			t.Fatal("invalid filter accepted")
		}
	}
	if _, err = svc.BoardPage(ctx, PipelinePageRequest{StageID: "300"}); err == nil {
		t.Fatal("foreign stage page accepted")
	}
	requireNoError(t, db.Where("id=201").Delete(&models.CrmPipelineStage{}).Error)
	if _, err = svc.BoardPage(ctx, PipelinePageRequest{StageID: "201"}); err == nil {
		t.Fatal("deleted stage still queryable")
	}
	orphanPage, err := svc.BoardPage(ctx, PipelinePageRequest{StageID: unassignedPipelineStage, PageSize: 1})
	requireNoError(t, err)
	mustCreate(t, db, &models.CrmPipelineStage{Model: models.Model{ID: "999"}, TenantID: "1", Name: "恢复阶段"})
	if _, err = svc.BoardPage(ctx, PipelinePageRequest{StageID: unassignedPipelineStage, Cursor: orphanPage.NextCursor}); err == nil {
		t.Fatal("changed stage membership did not invalidate orphan cursor")
	}
	for i, amount := range []float64{9e13, 9e13, 0.01} {
		mustCreate(t, db, &models.CrmOpportunity{Model: models.Model{ID: fmt.Sprint(3000 + i)}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "999", Status: "open", Amount: amount, Probability: 100})
	}
	board, err = svc.Board(ctx, PipelineFilter{})
	requireNoError(t, err)
	if column := pipelineColumn(t, board, "999"); column.Amount != "180000000000003.01" || column.WeightedAmount != "180000000000000.01" {
		t.Fatal("aggregate cents lost", column.Amount, column.WeightedAmount)
	}
	requireNoError(t, db.Where("tenant_id=1").Delete(&models.CrmPipelineStage{}).Error)
	board, err = svc.Board(ctx, PipelineFilter{})
	requireNoError(t, err)
	if len(board.Columns) != 1 || board.Columns[0].StageID != unassignedPipelineStage || board.Columns[0].Count != 11 {
		t.Fatal("tenant without stage configuration hides its open history")
	}
	requireNoError(t, db.Model(&models.User{}).Where("id=10").Update("status", 2).Error)
	if _, err = svc.Board(ctx, PipelineFilter{}); err == nil {
		t.Fatal("disabled actor accepted")
	}
	if _, err = NewCrmReport().PipelineFunnel(ctx, ""); err == nil {
		t.Fatal("legacy funnel accepts disabled actor")
	}
}

func TestOpportunityOptionsCustomerScopeAndHistory(t *testing.T) {
	db := pipelineReadDB(t)
	svc := NewCrmOpportunity()
	ctx := testIdentity("1", "10")
	var rows []models.CrmOpportunity
	for i := 0; i < 605; i++ {
		rows = append(rows, models.CrmOpportunity{Model: models.Model{ID: fmt.Sprint(1000 + i)}, TenantID: "1", CustomerID: "100", OwnerUserID: "10", StageID: "200", Title: fmt.Sprintf("deal-%04d", i), Status: "open"})
	}
	requireNoError(t, db.Omit("LeadID").CreateInBatches(&rows, 100).Error)
	for _, row := range []models.CrmOpportunity{
		{Model: models.Model{ID: "2000"}, TenantID: "1", CustomerID: "101", OwnerUserID: "10", Title: "wrong customer", Status: "open"},
		{Model: models.Model{ID: "2001"}, TenantID: "1", CustomerID: "100", OwnerUserID: "11", Title: "hidden owner", Status: "open"},
		{Model: models.Model{ID: "2002"}, TenantID: "2", CustomerID: "102", OwnerUserID: "20", Title: "foreign", Status: "open"},
	} {
		copy := row
		mustCreate(t, db, &copy)
	}
	page, err := svc.Options(ctx, ObjectOptionsRequest{CustomerID: "100", Purpose: "quote", PageSize: 1000})
	requireNoError(t, err)
	if len(page.Items) != 100 || !page.HasMore {
		t.Fatal("options are not bounded")
	}
	cursor := page.NextCursor
	count := len(page.Items)
	for page.HasMore {
		page, err = svc.Options(ctx, ObjectOptionsRequest{CustomerID: "100", Purpose: "quote", Cursor: page.NextCursor})
		requireNoError(t, err)
		count += len(page.Items)
	}
	if count != 605 {
		t.Fatal("older customer opportunities missing", count)
	}
	page, err = svc.Options(ctx, ObjectOptionsRequest{CustomerID: "100", Purpose: "quote", Keyword: "DEAL-0000"})
	requireNoError(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != "1000" {
		t.Fatal("old option not searchable")
	}
	page, err = svc.Options(ctx, ObjectOptionsRequest{CustomerID: "100", Purpose: "quote", IDs: []string{"1000", "2000", "2001", "2002"}})
	requireNoError(t, err)
	if len(page.Items) != 1 || page.Items[0].ID != "1000" || page.Items[0].Disabled {
		t.Fatal("selected resolution exceeds scope")
	}
	for _, req := range []ObjectOptionsRequest{{CustomerID: "101", Purpose: "quote", Cursor: cursor}, {CustomerID: "100", Purpose: "quote", Cursor: cursor, Keyword: "changed"}, {CustomerID: "100", Purpose: "plan"}, {Purpose: "quote"}} {
		if _, err = svc.Options(ctx, req); err == nil {
			t.Fatal("invalid option scope accepted")
		}
	}
	requireNoError(t, db.Model(&models.CrmCustomer{}).Where("id=100").Update("owner_user_id", "11").Error)
	page, err = svc.Options(ctx, ObjectOptionsRequest{CustomerID: "100", Purpose: "quote"})
	requireNoError(t, err)
	if len(page.Items) != 0 {
		t.Fatal("transferred customer remains assignable")
	}
	page, err = svc.Options(ctx, ObjectOptionsRequest{CustomerID: "100", Purpose: "quote", IDs: []string{"1000"}})
	requireNoError(t, err)
	if len(page.Items) != 1 || !page.Items[0].Disabled {
		t.Fatal("historical option lost or grants new association")
	}
	requireNoError(t, db.Where("id=1000").Delete(&models.CrmOpportunity{}).Error)
	page, err = svc.Options(ctx, ObjectOptionsRequest{CustomerID: "100", IDs: []string{"1000"}})
	requireNoError(t, err)
	if len(page.Items) != 0 {
		t.Fatal("soft deleted option exposed")
	}
}
