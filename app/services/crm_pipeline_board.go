package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"core/app/constants"
	"core/app/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const unassignedPipelineStage = "unassigned"

type PipelineFilter struct {
	OwnerUserID string `json:"owner_user_id"`
	DateFrom    string `json:"date_from"`
	DateTo      string `json:"date_to"`
}
type PipelinePageRequest struct {
	PipelineFilter
	StageID  string `json:"stage_id"`
	Cursor   string `json:"cursor"`
	PageSize int    `json:"page_size"`
}
type PipelineCard struct {
	ID                string     `json:"id"`
	Title             string     `json:"title"`
	CustomerID        string     `json:"customer_id"`
	OwnerUserID       string     `json:"owner_user_id"`
	Amount            string     `json:"amount"`
	ExpectedCloseDate *time.Time `json:"expected_close_date"`
}
type PipelinePage struct {
	Items      []PipelineCard `json:"items"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor"`
}
type PipelineColumn struct {
	StageID        string `json:"stage_id"`
	Name           string `json:"name"`
	Color          string `json:"color"`
	Description    string `json:"description"`
	IsWon          bool   `json:"is_won"`
	IsLost         bool   `json:"is_lost"`
	Count          int64  `json:"count"`
	Amount         string `json:"amount"`
	WeightedAmount string `json:"weighted_amount"`
	PipelinePage
}
type PipelineBoard struct {
	Columns []PipelineColumn `json:"columns"`
	AsOf    time.Time        `json:"as_of"`
}
type pipelineCursor struct {
	Tenant, Actor, Stage, StagesHash, ID string
	Filter                               PipelineFilter
}
type pipelineReader struct {
	ctx         context.Context
	tx          *gorm.DB
	actor       models.User
	manager     bool
	filter      PipelineFilter
	from, until *time.Time
	stages      []models.CrmPipelineStage
	stageIDs    []string
	stagesHash  string
}

func pipelineDates(filter PipelineFilter) (*time.Time, *time.Time, error) {
	if filter.OwnerUserID != "" && !validOptionID(filter.OwnerUserID) {
		return nil, nil, errors.New("销售负责人 ID 无效")
	}
	if filter.DateFrom == "" && filter.DateTo == "" {
		return nil, nil, nil
	}
	from, err := time.ParseInLocation("2006-01-02", filter.DateFrom, constants.PRCLocation)
	if err != nil {
		return nil, nil, errors.New("请选择完整有效的预计成交日期范围")
	}
	to, err := time.ParseInLocation("2006-01-02", filter.DateTo, constants.PRCLocation)
	if err != nil || to.Before(from) {
		return nil, nil, errors.New("预计成交日期范围无效")
	}
	until := to.AddDate(0, 0, 1)
	return &from, &until, nil
}
func newPipelineReader(ctx context.Context, tx *gorm.DB, filter PipelineFilter) (*pipelineReader, error) {
	from, until, err := pipelineDates(filter)
	if err != nil {
		return nil, err
	}
	actor, manager, err := customerActor(ctx, tx)
	if err != nil {
		return nil, err
	}
	r := &pipelineReader{ctx: ctx, tx: tx, actor: actor, manager: manager, filter: filter, from: from, until: until}
	if err = tx.Where("tenant_id = ?", actor.TenantID).Order("sort,id").Limit(101).Find(&r.stages).Error; err != nil {
		return nil, err
	}
	if len(r.stages) > 100 {
		return nil, errors.New("销售阶段超过 100 个，请先整理阶段配置")
	}
	for _, stage := range r.stages {
		r.stageIDs = append(r.stageIDs, stage.ID)
	}
	r.stagesHash = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(r.stageIDs, ","))))
	return r, nil
}
func (r *pipelineReader) query() *gorm.DB {
	q := r.tx.WithContext(r.ctx).Model(&models.CrmOpportunity{}).Where("tenant_id = ? AND status = 'open'", r.actor.TenantID)
	if !r.manager {
		q = q.Where("owner_user_id = ?", r.actor.ID)
	}
	if r.filter.OwnerUserID != "" {
		q = q.Where("owner_user_id = ?", r.filter.OwnerUserID)
	}
	if r.from != nil {
		q = q.Where("expected_close_date >= ? AND expected_close_date < ?", *r.from, *r.until)
	}
	return q
}
func (r *pipelineReader) page(req PipelinePageRequest) (PipelinePage, error) {
	page := PipelinePage{Items: []PipelineCard{}}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	q := r.query()
	if req.StageID == unassignedPipelineStage {
		if len(r.stageIDs) > 0 {
			q = q.Where("(stage_id IS NULL OR stage_id NOT IN ?)", r.stageIDs)
		}
	} else {
		found := false
		for _, id := range r.stageIDs {
			if id == req.StageID {
				found = true
				break
			}
		}
		if !found {
			return page, errors.New("销售阶段不存在，请刷新管线")
		}
		q = q.Where("stage_id = ?", req.StageID)
	}
	if req.Cursor != "" {
		var c pipelineCursor
		if len(req.Cursor) > 4096 {
			return page, errors.New("管线游标无效")
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(raw, &c) != nil || !validOptionID(c.ID) || c.Tenant != r.actor.TenantID || c.Actor != r.actor.ID || c.Stage != req.StageID || c.StagesHash != r.stagesHash || c.Filter != r.filter {
			return page, errors.New("管线范围已变化，请刷新后重试")
		}
		q = q.Where("id < ?", c.ID)
	}
	if err := q.Select("id,title,customer_id,owner_user_id," + moneySQL("amount") + " AS amount,expected_close_date").Order("id DESC").Limit(req.PageSize + 1).Scan(&page.Items).Error; err != nil {
		return page, err
	}
	if len(page.Items) > req.PageSize {
		page.Items = page.Items[:req.PageSize]
		page.HasMore = true
		raw, err := json.Marshal(pipelineCursor{Tenant: r.actor.TenantID, Actor: r.actor.ID, Stage: req.StageID, StagesHash: r.stagesHash, ID: page.Items[len(page.Items)-1].ID, Filter: r.filter})
		if err != nil {
			return page, err
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
	}
	return page, nil
}

// Board aggregates every accessible open opportunity, then bounds each column's cards.
// Counts and money never depend on the number of cards the browser has loaded.
func (s CrmOpportunityService) Board(ctx context.Context, filter PipelineFilter) (PipelineBoard, error) {
	board := PipelineBoard{Columns: []PipelineColumn{}, AsOf: time.Now()}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r, err := newPipelineReader(ctx, tx, filter)
		if err != nil {
			return err
		}
		// Normalize null, deleted and unknown stages into one visible recovery column.
		group := "'unassigned'"
		var args []any
		if len(r.stageIDs) > 0 {
			group = "CASE WHEN stage_id IN ? THEN stage_id::text ELSE 'unassigned' END"
			args = append(args, r.stageIDs)
		}
		type summary struct {
			StageID                string
			Count                  int64
			Amount, WeightedAmount string
		}
		var sums []summary
		selectSQL := group + " AS stage_id,COUNT(*) AS count," + moneySQL("SUM(amount)") + " AS amount," + moneySQL("SUM(amount * probability) / 100") + " AS weighted_amount"
		if err = r.query().Select(selectSQL, args...).Clauses(clause.GroupBy{Columns: []clause.Column{{Name: "1", Raw: true}}}).Scan(&sums).Error; err != nil {
			return err
		}
		byStage := map[string]summary{}
		for _, sum := range sums {
			byStage[sum.StageID] = sum
		}
		for _, stage := range r.stages {
			board.Columns = append(board.Columns, PipelineColumn{StageID: stage.ID, Name: stage.Name, Color: stage.Color, Description: stage.Description, IsWon: stage.IsWon, IsLost: stage.IsLost})
		}
		if byStage[unassignedPipelineStage].Count > 0 {
			board.Columns = append(board.Columns, PipelineColumn{StageID: unassignedPipelineStage, Name: "待配置阶段", Color: "#86909c", Description: "未分配、阶段已删除或历史阶段失效的商机；可拖入有效阶段继续处理"})
		}
		for i := range board.Columns {
			column := &board.Columns[i]
			sum, ok := byStage[column.StageID]
			column.Amount, column.WeightedAmount = "0.00", "0.00"
			column.Items = []PipelineCard{}
			if !ok {
				continue
			}
			column.Count, column.Amount, column.WeightedAmount = sum.Count, sum.Amount, sum.WeightedAmount
			column.PipelinePage, err = r.page(PipelinePageRequest{StageID: column.StageID, PageSize: 20})
			if err != nil {
				return err
			}
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return board, err
}
func (s CrmOpportunityService) BoardPage(ctx context.Context, req PipelinePageRequest) (PipelinePage, error) {
	page := PipelinePage{Items: []PipelineCard{}}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		r, err := newPipelineReader(ctx, tx, req.PipelineFilter)
		if err != nil {
			return err
		}
		page, err = r.page(req)
		return err
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return page, err
}
