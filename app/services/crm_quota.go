package services

import (
	"core/common/identity"
	
	"context"
	
	"time"

	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// ---------- 销售业绩目标 ----------

type CrmQuotaService struct{}

func NewCrmQuota() *CrmQuotaService { return &CrmQuotaService{} }

// Set 设置销售月度目标（upsert）。
func (s CrmQuotaService) Set(ctx context.Context, userID, yearMonth string, targetAmount float64) (models.CrmQuota, error) {
	tenantID := identity.TenantID(ctx)


	var q models.CrmQuota
	err := models.MustFor(ctx).Where("user_id = ? AND year_month = ?",  userID,  yearMonth).First(&q).Error
	if err == nil {
		q.TargetAmount = targetAmount
		err = models.DB.Save(&q).Error
		return q, err
	}
	if err != gorm.ErrRecordNotFound {
		return q, err
	}
	q = models.CrmQuota{
		Model:        models.Model{ID: id_generator.StringID()},
		TenantID:     tenantID,
		UserID:       userID,
		YearMonth:    yearMonth,
		TargetAmount: targetAmount,
	}
	err = models.DB.Create(&q).Error
	return q, err
}

// Get 读取单条目标。
func (s CrmQuotaService) Get(ctx context.Context, userID, yearMonth string) (models.CrmQuota, error) {


	var q models.CrmQuota
	err := models.MustFor(ctx).Where("user_id = ? AND year_month = ?",  userID,  yearMonth).First(&q).Error
	return q, err
}

// QuotaRow 目标列表行（含销售名）。
type QuotaRow struct {
	models.CrmQuota
	UserName string `gorm:"column:user_name" json:"user_name"`
}

// List 列出某月所有销售目标（含销售名）。
func (s CrmQuotaService) List(ctx context.Context, yearMonth string) ([]QuotaRow, error) {
	tenantID := identity.TenantID(ctx)


	var rows []QuotaRow
	err := models.DB.Table("crm_quota").
		Select("crm_quota.*, \"user\".name AS user_name").
		Joins("LEFT JOIN \"user\" ON \"user\".id = crm_quota.user_id").Scopes(models.TenantScope(tenantID)).Where("crm_quota.year_month = ?",  yearMonth).
		Order("crm_quota.user_id").Find(&rows).Error
	return rows, err
}

// QuotaAchievement 达成情况。
type QuotaAchievement struct {
	UserID        string  `json:"user_id"`
	UserName      string  `json:"user_name"`
	YearMonth     string  `json:"year_month"`
	TargetAmount  float64 `json:"target_amount"`
	ActualAmount  float64 `json:"actual_amount"`
	AchieveRate   float64 `json:"achieve_rate"` // 达成率(%)，0目标时为0
}

// Achievement 查单个销售某月达成情况。
func (s CrmQuotaService) Achievement(ctx context.Context, userID, yearMonth string) (QuotaAchievement, error) {


	q, _ := s.Get(ctx, userID, yearMonth)
	actual := s.actualAmount(ctx, userID, yearMonth)
	var u models.User
	name := ""
	if models.MustFor(ctx).Where("id = ?", userID).First(&u).Error == nil {
		name = u.Name
	}
	return QuotaAchievement{
		UserID:       userID,
		UserName:     name,
		YearMonth:    yearMonth,
		TargetAmount: q.TargetAmount,
		ActualAmount: actual,
		AchieveRate:  calcAchievement(q.TargetAmount, actual),
	}, nil
}

// TeamAchievement 查全团队某月达成情况。
func (s CrmQuotaService) TeamAchievement(ctx context.Context, yearMonth string) ([]QuotaAchievement, error) {


	rows, err := s.List(ctx, yearMonth)
	if err != nil {
		return nil, err
	}
	out := make([]QuotaAchievement, 0, len(rows))
	for _, r := range rows {
		actual := s.actualAmount(ctx, r.UserID, yearMonth)
		out = append(out, QuotaAchievement{
			UserID:       r.UserID,
			UserName:     r.UserName,
			YearMonth:    yearMonth,
			TargetAmount: r.TargetAmount,
			ActualAmount: actual,
			AchieveRate:  calcAchievement(r.TargetAmount, actual),
		})
	}
	return out, nil
}

// actualAmount 某销售某月实际签约金额（CrmOrder status=signed/completed 且 signed_at 在该月）。
func (s CrmQuotaService) actualAmount(ctx context.Context, userID, yearMonth string) float64 {
	tenantID := identity.TenantID(ctx)


	start, end, ok := monthRange(yearMonth)
	if !ok {
		return 0
	}
	var amt struct {
		Total float64 `gorm:"column:total"`
	}
	models.DB.Model(&models.CrmOrder{}).Scopes(models.TenantScope(tenantID)).Where("user_id = ? AND status IN ? AND signed_at >= ? AND signed_at < ?",  userID,  []string{"signed", "completed"},  start,  end).
		Select("COALESCE(SUM(amount),0) AS total").Scan(&amt)
	return amt.Total
}

// monthRange 解析 YYYY-MM 为 [月初, 下月初)。
func monthRange(yearMonth string) (time.Time, time.Time, bool) {
	t, err := time.Parse("2006-01", yearMonth)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	start := t
	end := t.AddDate(0, 1, 0)
	return start, end, true
}

// calcAchievement 计算达成率(%)。纯函数，供单测。目标为0时返回0（避免除零）。
func calcAchievement(target, actual float64) float64 {
	if target <= 0 {
		return 0
	}
	return actual / target * 100
}
