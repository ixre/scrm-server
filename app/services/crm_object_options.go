package services

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"core/app/models"
	"gorm.io/gorm"
)

// Purpose limits new associations. ID resolution still uses read access so a
// historical selection can be displayed without becoming selectable again.
type ObjectOptionsRequest struct {
	CustomerID string   `json:"customer_id"`
	Purpose    string   `json:"purpose"`
	Keyword    string   `json:"keyword"`
	Cursor     string   `json:"cursor"`
	PageSize   int      `json:"page_size"`
	IDs        []string `json:"ids"`
}
type ObjectOption struct {
	ID         string `json:"id"`
	Label      string `json:"label"`
	Disabled   bool   `json:"disabled"`
	CustomerID string `json:"customer_id,omitempty"`
}
type ObjectOptionsPage struct {
	Items      []ObjectOption `json:"items"`
	HasMore    bool           `json:"has_more"`
	NextCursor string         `json:"next_cursor"`
}
type objectOptionCursor struct{ Tenant, Actor, Kind, Purpose, Keyword, Customer, ID string }

func (s CrmCustomerService) Options(ctx context.Context, req ObjectOptionsRequest) (ObjectOptionsPage, error) {
	return objectOptions(ctx, "customer", req)
}
func (s CrmOrderService) Options(ctx context.Context, req ObjectOptionsRequest) (ObjectOptionsPage, error) {
	return objectOptions(ctx, "order", req)
}

func (s CrmOpportunityService) Options(ctx context.Context, req ObjectOptionsRequest) (ObjectOptionsPage, error) {
	return objectOptions(ctx, "opportunity", req)
}

func validOptionID(id string) bool {
	n, err := strconv.ParseInt(id, 10, 64)
	return err == nil && n > 0 && strconv.FormatInt(n, 10) == id
}

func objectOptions(ctx context.Context, kind string, req ObjectOptionsRequest) (ObjectOptionsPage, error) {
	page := ObjectOptionsPage{Items: []ObjectOption{}}
	if req.Purpose == "" {
		req.Purpose = "read"
	}
	if req.Purpose != "read" && !(kind == "customer" && req.Purpose == "sales") && !(kind == "order" && req.Purpose == "plan") && !(kind == "opportunity" && req.Purpose == "quote") {
		return page, errors.New("选项用途无效")
	}
	if kind == "opportunity" {
		if !validOptionID(req.CustomerID) {
			return page, errors.New("请先选择有效客户")
		}
	} else if req.CustomerID != "" {
		return page, errors.New("该选项不支持客户范围")
	}
	req.Keyword = strings.ToLower(strings.TrimSpace(req.Keyword))
	if !utf8.ValidString(req.Keyword) || utf8.RuneCountInString(req.Keyword) > 128 || strings.ContainsRune(req.Keyword, 0) {
		return page, errors.New("检索词最多 128 个字符")
	}
	if len(req.IDs) > 100 || (len(req.IDs) > 0 && (req.Keyword != "" || req.Cursor != "")) {
		return page, errors.New("已选值最多 100 个，不能与检索条件混用")
	}
	for _, id := range req.IDs {
		if !validOptionID(id) {
			return page, errors.New("选项 ID 无效")
		}
	}
	if req.PageSize <= 0 {
		req.PageSize = 50
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}
	var cursor objectOptionCursor
	if req.Cursor != "" {
		if len(req.Cursor) > 2048 {
			return page, errors.New("选项游标无效")
		}
		raw, err := base64.RawURLEncoding.DecodeString(req.Cursor)
		if err != nil || json.Unmarshal(raw, &cursor) != nil || !validOptionID(cursor.ID) || cursor.Kind != kind || cursor.Purpose != req.Purpose || cursor.Keyword != req.Keyword || cursor.Customer != req.CustomerID {
			return page, errors.New("选项游标范围不匹配，请重新查询")
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, manager, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		if req.Cursor != "" && (cursor.Tenant != actor.TenantID || cursor.Actor != actor.ID) {
			return errors.New("选项游标范围不匹配，请重新查询")
		}
		customerEligible := true
		if kind == "opportunity" && req.Purpose == "quote" {
			var customer models.CrmCustomer
			err := customerAccessForActor(ctx, tx, actor, manager, true).Select("id").Where("id = ? AND pool_status = 0 AND reassign_status = 0 AND owner_user_id IS NOT NULL", req.CustomerID).First(&customer).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			customerEligible = err == nil
		}
		q := tx.WithContext(ctx)
		if kind == "customer" {
			q = customerAccessForActor(ctx, tx, actor, manager, len(req.IDs) == 0 && req.Purpose == "sales").Model(&models.CrmCustomer{})
			if len(req.IDs) == 0 && req.Purpose == "sales" {
				q = q.Where("pool_status = 0 AND reassign_status = 0 AND owner_user_id IS NOT NULL")
			}
		} else if kind == "opportunity" {
			q = q.Model(&models.CrmOpportunity{}).Where("tenant_id = ? AND customer_id = ?", actor.TenantID, req.CustomerID)
			if !manager {
				q = q.Where("owner_user_id = ?", actor.ID)
			}
			if len(req.IDs) == 0 && !customerEligible {
				return nil
			}
		} else {
			q = q.Model(&models.CrmOrder{}).Where("tenant_id = ?", actor.TenantID)
			if !manager {
				q = q.Where("user_id = ?", actor.ID)
			}
			if len(req.IDs) == 0 && req.Purpose == "plan" {
				q = q.Where("status IN ?", []string{"draft", "signed", "completed"})
			}
		}
		if len(req.IDs) > 0 {
			q = q.Where("id IN ?", req.IDs)
		} else {
			if req.Keyword != "" {
				// Escape wildcards. Prefix searches use the tenant expression indexes.
				prefix := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(req.Keyword) + "%"
				filter := "(lower(name) LIKE ? OR lower(company) LIKE ? OR lower(mobile) LIKE ?"
				args := []any{prefix, prefix, prefix}
				if kind == "order" {
					filter, args = "(lower(title) LIKE ? OR lower(order_no) LIKE ?", []any{prefix, prefix}
				}
				if kind == "opportunity" {
					filter, args = "(lower(title) LIKE ?", []any{prefix}
				}
				if validOptionID(req.Keyword) {
					filter += " OR id = ?"
					args = append(args, req.Keyword)
				}
				q = q.Where(filter+")", args...)
			}
			if req.Cursor != "" {
				q = q.Where("id < ?", cursor.ID)
			}
			q = q.Limit(req.PageSize + 1)
		}
		q = q.Order("id DESC")
		if kind == "customer" {
			var rows []models.CrmCustomer
			if err := q.Select("id,name,company,mobile,owner_user_id,pool_status,reassign_status").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				label := strings.TrimSpace(row.Name)
				if label == "" {
					label = strings.TrimSpace(row.Company)
				}
				if label == "" {
					label = row.ID
				}
				if row.Mobile != "" {
					label += " · " + row.Mobile
				}
				disabled := req.Purpose == "sales" && (row.PoolStatus != 0 || row.ReassignStatus != 0 || row.OwnerUserID == "" || (!manager && row.OwnerUserID != actor.ID))
				page.Items = append(page.Items, ObjectOption{ID: row.ID, Label: label, Disabled: disabled})
			}
		} else if kind == "opportunity" {
			var rows []models.CrmOpportunity
			if err := q.Select("id,title,status").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				label := strings.TrimSpace(row.Title)
				if label == "" {
					label = row.ID
				}
				status := map[string]string{"open": "进行中", "won": "赢单", "lost": "输单"}[row.Status]
				if status != "" {
					label += " · " + status
				}
				page.Items = append(page.Items, ObjectOption{ID: row.ID, Label: label, Disabled: !customerEligible, CustomerID: req.CustomerID})
			}
		} else {
			var rows []models.CrmOrder
			if err := q.Select("id,title,order_no,customer_id,status").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				label := strings.TrimSpace(row.Title)
				if label == "" {
					label = row.ID
				}
				if row.OrderNo != "" {
					label += " · " + row.OrderNo
				}
				disabled := req.Purpose == "plan" && row.Status != "draft" && row.Status != "signed" && row.Status != "completed"
				page.Items = append(page.Items, ObjectOption{ID: row.ID, Label: label, Disabled: disabled, CustomerID: row.CustomerID})
			}
		}
		if len(req.IDs) == 0 && len(page.Items) > req.PageSize {
			page.Items = page.Items[:req.PageSize]
			page.HasMore = true
			raw, err := json.Marshal(objectOptionCursor{Tenant: actor.TenantID, Actor: actor.ID, Kind: kind, Purpose: req.Purpose, Keyword: req.Keyword, Customer: req.CustomerID, ID: page.Items[len(page.Items)-1].ID})
			if err != nil {
				return err
			}
			page.NextCursor = base64.RawURLEncoding.EncodeToString(raw)
		}
		return nil
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	return page, err
}
