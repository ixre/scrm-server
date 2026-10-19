package services

import (
	"core/common/identity"

	"context"

	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"github.com/pkg/errors"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------- 客户公海池 ----------

type CrmPoolService struct{}

func NewCrmPool() *CrmPoolService { return &CrmPoolService{} }

// ListPool 公海客户列表（pool_status=1），按租户隔离，支持关键字搜索。
func (s CrmPoolService) ListPool(ctx context.Context, keyword string, pager *app.Pager) ([]models.CrmCustomer, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx).Where(availablePoolCustomers)
	if keyword != "" {
		like := "%" + keyword + "%"
		db = db.Where("name LIKE ? OR mobile LIKE ? OR company LIKE ?", like, like, like)
	}
	var total int64
	if err := db.Model(&models.CrmCustomer{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var cs []models.CrmCustomer
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("pool_at desc").Find(&cs).Error
	return cs, total, err
}

// Claim 认领公海客户：pool_status→0，owner→认领人。仅公海客户可认领。
// 校验：领取上限、前负责人冷却期。记录操作日志。
func (s CrmPoolService) Claim(ctx context.Context, userID string, customerIDs []string) (int64, error) {
	return s.takePool(ctx, userID, customerIDs, false)
}

// Assign 管理员分配公海客户给指定销售。
func (s CrmPoolService) Assign(ctx context.Context, operatorID string, targetUserID string, customerIDs []string) (int64, error) {
	if operatorID != identity.UserID(ctx) {
		return 0, ErrCustomerAccess
	}
	return s.takePool(ctx, targetUserID, customerIDs, true)
}

// Recycle 手动回收客户进公海：pool_status→1，owner 清空，pool_at=now，记录前负责人。
func (s CrmPoolService) Recycle(ctx context.Context, customerIDs []string) (int64, error) {
	ids, err := workflowIDs(customerIDs)
	if err != nil {
		return 0, err
	}
	err = models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		actor, _, err := customerActor(ctx, tx)
		if err != nil {
			return err
		}
		db, err := customerAccessDB(ctx, tx, true)
		if err != nil {
			return err
		}
		var customers []models.CrmCustomer
		if err = db.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).Order("id").Find(&customers).Error; err != nil {
			return err
		}
		if len(customers) != len(ids) {
			return ErrCustomerAccess
		}
		for _, c := range customers {
			if c.InPool() {
				return errors.New("客户已在公海，请刷新后重试")
			}
			if c.IsLocked {
				return errors.New("锁定客户不能回收，请先解锁")
			}
			if c.IsReassigning() {
				return errors.New("客户正在转交，接收完成后才能回收")
			}
			if err = tx.Model(&c).Updates(recycledCustomerChanges(c, time.Now())).Error; err != nil {
				return err
			}
			if err = poolLog(tx, actor.TenantID, c, "recycle", actor, "", "手动回收"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return int64(len(ids)), nil
}

// lastFollowupTime 取某客户最新跟进时间，无跟进则返回零值。
func lastFollowupTime(ctx context.Context, customerID string) time.Time {
	var f models.CrmFollowup
	if err := models.MustFor(ctx).Where("customer_id = ?", customerID).
		Order("created_at desc").Limit(1).First(&f).Error; err != nil {
		return time.Time{}
	}
	return f.CreatedAt
}

// shouldRecycle 判断是否应回收：基准时间（最近跟进或创建时间）早于阈值。
// 纯函数，供单测。
func shouldRecycle(baseTime time.Time, threshold time.Time) bool {
	if baseTime.IsZero() {
		return false
	}
	return baseTime.Before(threshold)
}

// ---------- 公海规则 ----------

// GetRule 读取租户公海规则，不存在则返回默认（不自动回收）。
func (s CrmPoolService) GetRule(ctx context.Context) (models.CrmPoolRule, error) {

	var r models.CrmPoolRule
	err := models.MustFor(ctx).First(&r).Error
	if err == nil {
		return r, nil
	}
	if err == gorm.ErrRecordNotFound {
		return models.CrmPoolRule{}, nil
	}
	return models.CrmPoolRule{}, err
}

// ---------- 公海操作记录 ----------

// ListLogs 查询公海操作记录，支持按客户ID筛选。
func (s CrmPoolService) ListLogs(ctx context.Context, customerID string, pager *app.Pager) ([]models.CrmPoolLog, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db, err := poolLogScope(ctx)
	if err != nil {
		return nil, 0, err
	}
	if customerID != "" {
		db = db.Where("customer_id = ?", customerID)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmPoolLog
	err = db.Order("created_at desc, id desc").Offset(pager.GetOffset()).Limit(pager.GetLimit()).Find(&items).Error
	return items, total, err
}

// writePoolLog 写入公海操作记录。
func (s CrmPoolService) writePoolLog(tenantID, customerID, customerName, action, operatorID, operatorName, targetUserID, remark string) {
	logEntry := models.CrmPoolLog{
		Model:        models.Model{ID: id_generator.StringID()},
		TenantID:     tenantID,
		CustomerID:   customerID,
		CustomerName: customerName,
		Action:       action,
		OperatorID:   operatorID,
		OperatorName: operatorName,
		TargetUserID: targetUserID,
		Remark:       remark,
	}
	if err := models.DB.Create(&logEntry).Error; err != nil {
		log.Sugar.Errorw("write pool log failed", "err", err)
	}
}

// ---------- 公海统计 ----------

// PoolStats 公海池统计概览。
type PoolStats struct {
	TotalInPool   int64 `json:"total_in_pool"`  // 当前公海客户数
	ClaimedToday  int64 `json:"claimed_today"`  // 今日认领数
	RecycledToday int64 `json:"recycled_today"` // 今日回收数
	AssignedToday int64 `json:"assigned_today"` // 今日分配数
}

// GetStats 获取公海池统计概览。
func (s CrmPoolService) GetStats(ctx context.Context) (PoolStats, error) {
	var stats PoolStats
	logs, err := poolLogScope(ctx)
	if err != nil {
		return stats, err
	}
	if err := models.MustFor(ctx).Model(&models.CrmCustomer{}).Where(availablePoolCustomers).Count(&stats.TotalInPool).Error; err != nil {
		return stats, err
	}
	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	err = logs.Select("COUNT(*) FILTER (WHERE action = 'claim') AS claimed_today, COUNT(*) FILTER (WHERE action = 'recycle') AS recycled_today, COUNT(*) FILTER (WHERE action = 'assign') AS assigned_today").Where("created_at >= ?", today).Scan(&stats).Error
	if err != nil {
		return PoolStats{}, err
	}
	return stats, nil
}

// ---------- 辅助函数 ----------

// hasActiveOpportunity 判断客户是否有进行中的商机。
func hasActiveOpportunity(ctx context.Context, customerID string) bool {
	var count int64
	models.MustFor(ctx).Model(&models.CrmOpportunity{}).
		Where("customer_id = ? AND status NOT IN ?", customerID, []string{"won", "lost"}).Count(&count)
	return count > 0
}
