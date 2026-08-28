package services

import (
	"core/common/identity"

	"context"

	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ---------- 优惠券引擎 ----------

type CrmCouponService struct{}

func NewCrmCoupon() *CrmCouponService { return &CrmCouponService{} }

// CouponCreateReq 优惠券创建请求。
type CouponCreateReq struct {
	Coupon models.CrmCoupon `json:"coupon"`
}

// Create 创建优惠券模板。
func (s CrmCouponService) Create(ctx context.Context, userID string, req models.CrmCoupon) (models.CrmCoupon, error) {
	tenantID := identity.TenantID(ctx)

	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.CouponType == "" {
		return req, errors.New("coupon_type 不能为空")
	}
	if err := createCRM(models.DB, &req, "ActivityID"); err != nil {
		log.Sugar.Errorw("coupon create failed", "err", err)
		return req, err
	}
	return req, nil
}

// Update 更新优惠券模板（已发放的券不允许改总量/面值）。
func (s CrmCouponService) Update(ctx context.Context, id string, req models.CrmCoupon) (models.CrmCoupon, error) {
	tenant := identity.TenantID(ctx)
	if tenant == "" {
		return req, models.ErrMissingTenant
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exist models.CrmCoupon
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenant, id).First(&exist).Error; err != nil {
			return err
		}
		if exist.IssuedCount > 0 && (req.TotalCount != exist.TotalCount || req.FaceValue != exist.FaceValue) {
			return errors.New("已发放的优惠券不能修改总量或面值")
		}
		return tx.Model(&exist).Updates(map[string]any{"name": req.Name, "coupon_type": req.CouponType, "face_value": req.FaceValue, "threshold": req.Threshold, "total_count": req.TotalCount, "start_at": req.StartAt, "end_at": req.EndAt, "status": req.Status, "remark": req.Remark}).Error
	})
	return req, err
}

// Query 列出优惠券。
func (s CrmCouponService) Query(ctx context.Context, status string, pager *app.Pager) ([]models.CrmCoupon, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmCoupon{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCoupon
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// Get 查单张优惠券。
func (s CrmCouponService) Get(ctx context.Context, id string) (models.CrmCoupon, error) {

	var c models.CrmCoupon
	err := models.MustFor(ctx).Where("id = ?", id).First(&c).Error
	return c, err
}

// Delete 删除优惠券（已发放的不能删）。
func (s CrmCouponService) Delete(ctx context.Context, ids []string) (int64, error) {
	tenant := identity.TenantID(ctx)
	if tenant == "" {
		return 0, models.ErrMissingTenant
	}
	var count int64
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var coupons []models.CrmCoupon
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id IN ?", tenant, ids).Order("id").Find(&coupons).Error; err != nil {
			return err
		}
		for _, coupon := range coupons {
			if coupon.IssuedCount > 0 {
				return errors.New("已发放的优惠券不能删除")
			}
		}
		res := tx.Where("tenant_id = ? AND id IN ?", tenant, ids).Delete(&models.CrmCoupon{})
		count = res.RowsAffected
		return res.Error
	})
	return count, err
}

// Issue 向客户发放优惠券：生成唯一券码，写 CrmCouponRecord，更新 issued_count。
func (s CrmCouponService) Issue(ctx context.Context, couponID, customerID string) (models.CrmCouponRecord, error) {
	var rec models.CrmCouponRecord
	tenant := identity.TenantID(ctx)
	if tenant == "" {
		return rec, models.ErrMissingTenant
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var coupon models.CrmCoupon
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenant, couponID).First(&coupon).Error; err != nil {
			return err
		}
		now := time.Now()
		if err := usableCoupon(coupon, now); err != nil {
			return err
		}
		if coupon.TotalCount > 0 && coupon.IssuedCount >= coupon.TotalCount {
			return errors.New("优惠券已发完")
		}
		var customer models.CrmCustomer
		if err := tx.Where("tenant_id = ? AND id = ?", tenant, customerID).First(&customer).Error; err != nil {
			return errors.New("客户不存在")
		}
		rec = models.CrmCouponRecord{Model: models.Model{ID: id_generator.StringID()}, TenantID: tenant, CouponID: couponID, CustomerID: customerID, Code: genCouponCode(), Status: "issued", IssuedAt: &now}
		if err := createCRM(tx, &rec, "OrderID"); err != nil {
			return err
		}
		return tx.Model(&models.CrmCoupon{}).Where("tenant_id = ? AND id = ?", tenant, couponID).UpdateColumn("issued_count", gorm.Expr("issued_count + 1")).Error
	})
	return rec, err
}

func usableCoupon(coupon models.CrmCoupon, now time.Time) error {
	if coupon.Status != "active" {
		return errors.New("优惠券已停用")
	}
	if coupon.StartAt != nil && now.Before(*coupon.StartAt) {
		return errors.New("优惠券尚未生效")
	}
	if coupon.EndAt != nil && now.After(*coupon.EndAt) {
		return errors.New("优惠券已过期")
	}
	return nil
}

// Verify 同一订单重试返回原核销结果；其他并发核销只能成功一次。
func (s CrmCouponService) Verify(ctx context.Context, code, orderID string) (models.CrmCouponRecord, error) {
	var rec models.CrmCouponRecord
	tenant := identity.TenantID(ctx)
	if tenant == "" {
		return rec, models.ErrMissingTenant
	}
	err := models.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("tenant_id = ? AND code = ?", tenant, code).First(&rec).Error; err != nil {
			return err
		}
		// 模板先加锁，与发券/修改采用相同锁顺序。
		var coupon models.CrmCoupon
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenant, rec.CouponID).First(&coupon).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenant, rec.ID).First(&rec).Error; err != nil {
			return err
		}
		if rec.Status == "used" && orderID != "" && rec.OrderID == orderID {
			return nil
		}
		if rec.Status != "issued" {
			return errors.New("优惠券状态不允许核销")
		}
		now := time.Now()
		if err := usableCoupon(coupon, now); err != nil {
			return err
		}
		if orderID != "" {
			var order models.CrmOrder
			if err := tx.Where("tenant_id = ? AND id = ? AND customer_id = ?", tenant, orderID, rec.CustomerID).First(&order).Error; err != nil {
				return errors.New("订单不存在或不属于该客户")
			}
		}
		changes := updatesCRM(map[string]any{"status": "used", "used_at": &now, "order_id": orderID}, "order_id")
		result := tx.Model(&models.CrmCouponRecord{}).Where("tenant_id = ? AND id = ? AND status = ?", tenant, rec.ID, "issued").Updates(changes)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("优惠券已被核销")
		}
		if err := tx.Model(&models.CrmCoupon{}).Where("tenant_id = ? AND id = ?", tenant, coupon.ID).UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error; err != nil {
			return err
		}
		rec.Status, rec.UsedAt, rec.OrderID = "used", &now, orderID
		return nil
	})
	return rec, err
}

// ListRecords 列出发放/核销记录。
func (s CrmCouponService) ListRecords(ctx context.Context, couponID, status string, pager *app.Pager) ([]models.CrmCouponRecord, int64, error) {

	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if couponID != "" {
		db = db.Where("coupon_id = ?", couponID)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmCouponRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmCouponRecord
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// genCouponCode 生成 12 位 hex 券码。
func genCouponCode() string {
	b := make([]byte, 6)
	rand.Read(b)
	return fmt.Sprintf("CP%s", hex.EncodeToString(b))
}
