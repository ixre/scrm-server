package services

import (
	"context"
	"time"

	"core/app/models"
	"core/common/app"
)

// GroupChatStatsService 客户群统计服务。
type GroupChatStatsService struct{}

func NewGroupChatStats() *GroupChatStatsService { return &GroupChatStatsService{} }

// GroupChatStatsResult 群统计概览。
type GroupChatStatsResult struct {
	TotalGroups        int64   `json:"total_groups"`          // 群总数
	ActiveGroups       int64   `json:"active_groups"`         // 活跃群数（未解散）
	TotalMembers       int64   `json:"total_members"`         // 群成员总数
	TodayJoin          int64   `json:"today_join"`            // 今日入群总数
	TodayQuit          int64   `json:"today_quit"`            // 今日退群总数
	ExternalMembers    int64   `json:"external_members"`      // 外部联系人总数
	InternalMembers    int64   `json:"internal_members"`      // 企业成员总数
	AvgMembersPerGroup float64 `json:"avg_members_per_group"` // 平均群人数
}

// Stats 获取群统计概览。
func (s GroupChatStatsService) Stats(ctx context.Context, extCorpID string) (GroupChatStatsResult, error) {
	var result GroupChatStatsResult
	db := models.DB.Model(&models.GroupChat{}).Where("ext_corp_id = ?", extCorpID)

	// 群总数
	db.Count(&result.TotalGroups)

	// 活跃群数（未解散）
	models.DB.Model(&models.GroupChat{}).Where("ext_corp_id = ? AND status = 2", extCorpID).Count(&result.ActiveGroups)

	// 群成员总数 + 今日入退群
	models.DB.Model(&models.GroupChat{}).Where("ext_corp_id = ? AND status = 2", extCorpID).
		Select("COALESCE(SUM(total), 0)").Scan(&result.TotalMembers)
	models.DB.Model(&models.GroupChat{}).Where("ext_corp_id = ? AND status = 2", extCorpID).
		Select("COALESCE(SUM(today_join_member_num), 0)").Scan(&result.TodayJoin)
	models.DB.Model(&models.GroupChat{}).Where("ext_corp_id = ? AND status = 2", extCorpID).
		Select("COALESCE(SUM(today_quit_member_num), 0)").Scan(&result.TodayQuit)

	// 成员构成（通过group_chat_member表统计）
	models.DB.Model(&models.GroupChatMember{}).
		Joins("JOIN group_chat gc ON gc.ext_chat_id = group_chat_member.ext_chat_id").
		Where("gc.ext_corp_id = ? AND gc.status = 2 AND group_chat_member.type = 2", extCorpID).
		Count(&result.ExternalMembers)
	models.DB.Model(&models.GroupChatMember{}).
		Joins("JOIN group_chat gc ON gc.ext_chat_id = group_chat_member.ext_chat_id").
		Where("gc.ext_corp_id = ? AND gc.status = 2 AND group_chat_member.type = 1", extCorpID).
		Count(&result.InternalMembers)

	// 平均群人数
	if result.ActiveGroups > 0 {
		result.AvgMembersPerGroup = float64(result.TotalMembers) / float64(result.ActiveGroups)
	}

	return result, nil
}

// ListMembers 查询群成员列表。
func (s GroupChatStatsService) ListMembers(ctx context.Context, extCorpID, extChatID string, memberType int, pager *app.Pager) ([]models.GroupChatMember, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Model(&models.GroupChatMember{}).
		Joins("JOIN group_chat gc ON gc.ext_chat_id = group_chat_member.ext_chat_id").
		Where("gc.ext_corp_id = ? AND group_chat_member.ext_chat_id = ?", extCorpID, extChatID)
	if memberType > 0 {
		db = db.Where("group_chat_member.type = ?", memberType)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.GroupChatMember
	err := db.Order("group_chat_member.join_time desc").
		Offset(pager.GetOffset()).Limit(pager.GetLimit()).Find(&items).Error
	return items, total, err
}

// GroupTrendItem 群增长趋势项。
type GroupTrendItem struct {
	Date      string `json:"date" gorm:"column:date"`
	NewGroups int64  `json:"new_groups"`
	TodayJoin int64  `json:"today_join"`
	TodayQuit int64  `json:"today_quit"`
}

// Trend 群增长趋势（最近N天）。
func (s GroupChatStatsService) Trend(ctx context.Context, extCorpID string, days int) ([]GroupTrendItem, error) {
	if days <= 0 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	deadline := time.Now().AddDate(0, 0, -days)
	var items []GroupTrendItem
	err := models.DB.Model(&models.GroupChat{}).
		Select("TO_CHAR(created_at, 'YYYY-MM-DD') as date, COUNT(*) as new_groups, COALESCE(SUM(today_join_member_num),0) as today_join, COALESCE(SUM(today_quit_member_num),0) as today_quit").
		Where("ext_corp_id = ? AND created_at >= ?", extCorpID, deadline).
		Group("date").Order("date desc").Limit(days).
		Scan(&items).Error
	return items, err
}
