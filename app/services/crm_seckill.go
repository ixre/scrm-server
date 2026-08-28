package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 秒杀引擎 ----------

type CrmSeckillService struct{}

func NewCrmSeckill() *CrmSeckillService { return &CrmSeckillService{} }

// SeckillCreateReq 秒杀创建请求。
type SeckillCreateReq struct {
	Seckill models.CrmSeckill `json:"seckill"`
}

// Create 创建秒杀活动。
func (s CrmSeckillService) Create(ctx context.Context, req models.CrmSeckill) (models.CrmSeckill, error) {
	tenantID := identity.TenantID(ctx)


	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.LimitPerUser <= 0 {
		req.LimitPerUser = 1
	}
	if req.TotalStock <= 0 {
		return req, errors.New("总库存必须大于0")
	}
	if req.SeckillPrice <= 0 || req.SeckillPrice >= req.OriginalPrice {
		return req, errors.New("秒杀价必须大于0且小于原价")
	}
	if err := createCRM(models.DB, &req, "ActivityID"); err != nil {
		log.Sugar.Errorw("seckill create failed", "err", err)
		return req, err
	}
	return req, nil
}

// Query 列出秒杀活动。
func (s CrmSeckillService) Query(ctx context.Context, status string, pager *app.Pager) ([]models.CrmSeckill, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmSeckill{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSeckill
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// Get 查单个秒杀活动。
func (s CrmSeckillService) Get(ctx context.Context, id string) (models.CrmSeckill, error) {


	var sk models.CrmSeckill
	err := models.MustFor(ctx).Where("id = ?", id).First(&sk).Error
	return sk, err
}

// Delete 删除秒杀活动（有记录的不能删）。
func (s CrmSeckillService) Delete(ctx context.Context, ids []string) (int64, error) {
	tenantID := identity.TenantID(ctx)


	var cnt int64
	models.DB.Model(&models.CrmSeckillRecord{}).Scopes(models.TenantScope(tenantID)).Where("seckill_id IN ?",  ids).Count(&cnt)
	if cnt > 0 {
		return 0, errors.New("有抢购记录的秒杀活动不能删除")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmSeckill{})
	return res.RowsAffected, res.Error
}

// Grab 抢购：事务性扣库存 + 每人限购校验 + 有效期校验。
func (s CrmSeckillService) Grab(ctx context.Context, seckillID, customerID string) (models.CrmSeckillRecord, error) {
	tenantID := identity.TenantID(ctx)


	var rec models.CrmSeckillRecord
	var sk models.CrmSeckill
	if err := models.MustFor(ctx).Where("id = ?", seckillID).First(&sk).Error; err != nil {
		return rec, errors.New("秒杀活动不存在")
	}
	if sk.Status != "active" {
		return rec, errors.New("秒杀活动已结束")
	}
	now := time.Now()
	if sk.StartAt != nil && now.Before(*sk.StartAt) {
		return rec, errors.New("秒杀尚未开始")
	}
	if sk.EndAt != nil && now.After(*sk.EndAt) {
		return rec, errors.New("秒杀已结束")
	}
	// 校验客户存在
	var c models.CrmCustomer
	if err := models.MustFor(ctx).Where("id = ?", customerID).First(&c).Error; err != nil {
		return rec, errors.New("客户不存在")
	}
	// 每人限购校验
	var userCnt int64
	models.MustFor(ctx).Model(&models.CrmSeckillRecord{}).
		Where("seckill_id = ? AND customer_id = ? AND status <> ?", seckillID, customerID, "cancelled").Count(&userCnt)
	if int(userCnt) >= sk.LimitPerUser {
		return rec, fmt.Errorf("超过每人限购%d次", sk.LimitPerUser)
	}

	rec = models.CrmSeckillRecord{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		SeckillID:  seckillID,
		CustomerID: customerID,
		Price:      sk.SeckillPrice,
		Status:     "grabbed",
		GrabbedAt:  &now,
	}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		// 乐观锁扣库存：UPDATE ... WHERE sold_count < total_stock
		res := tx.Model(&models.CrmSeckill{}).
			Where("tenant_id = ? AND id = ? AND sold_count < total_stock", tenantID, seckillID).
			UpdateColumn("sold_count", gorm.Expr("sold_count + 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("库存不足，抢购失败")
		}
		return tx.Create(&rec).Error
	})
	if err != nil {
		log.Sugar.Errorw("seckill grab failed", "err", err, "seckill", seckillID, "customer", customerID)
		return rec, err
	}
	return rec, nil
}

// ListRecords 秒杀记录列表。
func (s CrmSeckillService) ListRecords(ctx context.Context, seckillID, status string, pager *app.Pager) ([]models.CrmSeckillRecord, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if seckillID != "" {
		db = db.Where("seckill_id = ?", seckillID)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmSeckillRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSeckillRecord
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}
