package services

import (
	"context"
	"core/app/models"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

type UserOptionsRequest struct {
	Keyword         string   `json:"keyword"`
	AfterID         string   `json:"after_id"`
	PageSize        int      `json:"page_size"`
	IDs             []string `json:"ids"`
	IncludeDisabled bool     `json:"include_disabled"`
}
type UserOption struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Status   int    `json:"status"`
	Name     string `json:"-"`
	Username string `json:"-"`
}
type UserOptionsPage struct {
	Items      []UserOption `json:"items"`
	HasMore    bool         `json:"has_more"`
	NextCursor string       `json:"next_cursor"`
}

func (s UserService) Options(ctx context.Context, req UserOptionsRequest) (UserOptionsPage, error) {
	page := UserOptionsPage{Items: []UserOption{}}
	actor, _, err := customerActor(ctx, models.DB)
	if err != nil {
		return page, err
	}
	q := models.DB.WithContext(ctx).Model(&models.User{}).Select("id, name, username, status").Where("tenant_id = ?", actor.TenantID)
	if len(req.IDs) > 0 {
		if len(req.IDs) > 1000 {
			return page, errors.New("已选员工过多")
		}
		for _, id := range req.IDs {
			if n, err := strconv.ParseInt(id, 10, 64); err != nil || n <= 0 {
				return page, errors.New("员工ID无效")
			}
		}
		q = q.Where("id IN ?", req.IDs).Limit(1000)
	} else {
		if !req.IncludeDisabled {
			q = q.Where("status = 1")
		}
		keyword := strings.TrimSpace(req.Keyword)
		if utf8.RuneCountInString(keyword) > 128 {
			return page, errors.New("搜索词最多128字")
		}
		if keyword != "" {
			escaped := strings.NewReplacer("\\", "\\\\", "%", "\\%", "_", "\\_").Replace(keyword)
			q = q.Where("(name ILIKE ? OR username ILIKE ?)", "%"+escaped+"%", "%"+escaped+"%")
		}
		if req.AfterID != "" {
			if n, err := strconv.ParseInt(req.AfterID, 10, 64); err != nil || n <= 0 {
				return page, errors.New("员工游标无效")
			}
			q = q.Where("id > ?", req.AfterID)
		}
		if req.PageSize <= 0 {
			req.PageSize = 50
		}
		if req.PageSize > 100 {
			req.PageSize = 100
		}
		q = q.Limit(req.PageSize + 1)
	}
	if err = q.Order("id").Find(&page.Items).Error; err != nil {
		return page, err
	}
	if len(req.IDs) == 0 && len(page.Items) > req.PageSize {
		page.HasMore = true
		page.Items = page.Items[:req.PageSize]
		page.NextCursor = page.Items[len(page.Items)-1].ID
	}
	for i := range page.Items {
		u := &page.Items[i]
		u.Label = u.Name
		if u.Label == "" {
			u.Label = u.Username
		}
		if u.Label == "" {
			u.Label = u.ID
		}
	}
	return page, nil
}
