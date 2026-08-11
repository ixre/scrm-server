package services

import (
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
)

// ---------- 优惠券引擎 ----------

type CrmCouponService struct{}

func NewCrmCoupon() *CrmCouponService { return &CrmCouponService{} }

// CouponCreateReq 优惠券创建请求。
type CouponCreateReq struct {
	Coupon models.CrmCoupon `json:"coupon"`
}

// Create 创建优惠券模板。
func (s CrmCouponService) Create(tenantID, userID string, req models.CrmCoupon) (models.CrmCoupon, error) {
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
func (s CrmCouponService) Update(tenantID, id string, req models.CrmCoupon) (models.CrmCoupon, error) {
	var exist models.CrmCoupon
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&exist).Error; err != nil {
		return req, errors.New("优惠券不存在")
	}
	if exist.IssuedCount > 0 && (req.TotalCount != exist.TotalCount || req.FaceValue != exist.FaceValue) {
		return req, errors.New("已发放的优惠券不能修改总量或面值")
	}
	updates := map[string]any{
		"name": req.Name, "coupon_type": req.CouponType, "face_value": req.FaceValue,
		"threshold": req.Threshold, "total_count": req.TotalCount,
		"start_at": req.StartAt, "end_at": req.EndAt, "status": req.Status, "remark": req.Remark,
	}
	if err := models.DB.Model(&models.CrmCoupon{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return req, err
	}
	return req, nil
}

// Query 列出优惠券。
func (s CrmCouponService) Query(tenantID, status string, pager *app.Pager) ([]models.CrmCoupon, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Scopes(models.TenantScope(tenantID))
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
func (s CrmCouponService) Get(tenantID, id string) (models.CrmCoupon, error) {
	var c models.CrmCoupon
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&c).Error
	return c, err
}

// Delete 删除优惠券（已发放的不能删）。
func (s CrmCouponService) Delete(tenantID string, ids []string) (int64, error) {
	var cnt int64
	models.DB.Model(&models.CrmCouponRecord{}).Scopes(models.TenantScope(tenantID)).Where("coupon_id IN ?",  ids).Count(&cnt)
	if cnt > 0 {
		return 0, errors.New("已发放的优惠券不能删除")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmCoupon{})
	return res.RowsAffected, res.Error
}

// Issue 向客户发放优惠券：生成唯一券码，写 CrmCouponRecord，更新 issued_count。
func (s CrmCouponService) Issue(tenantID, couponID, customerID string) (models.CrmCouponRecord, error) {
	var rec models.CrmCouponRecord
	var coupon models.CrmCoupon
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", couponID).First(&coupon).Error; err != nil {
		return rec, errors.New("优惠券不存在")
	}
	if coupon.Status != "active" {
		return rec, errors.New("优惠券已停用")
	}
	// 校验有效期
	now := time.Now()
	if coupon.StartAt != nil && now.Before(*coupon.StartAt) {
		return rec, errors.New("优惠券尚未生效")
	}
	if coupon.EndAt != nil && now.After(*coupon.EndAt) {
		return rec, errors.New("优惠券已过期")
	}
	// 校验库存
	if coupon.TotalCount > 0 && coupon.IssuedCount >= coupon.TotalCount {
		return rec, errors.New("优惠券已发完")
	}
	// 校验客户存在
	var c models.CrmCustomer
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", customerID).First(&c).Error; err != nil {
		return rec, errors.New("客户不存在")
	}

	code := genCouponCode()
	rec = models.CrmCouponRecord{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		CouponID:   couponID,
		CustomerID: customerID,
		Code:       code,
		Status:     "issued",
		IssuedAt:   &now,
	}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		return tx.Model(&models.CrmCoupon{}).Where("id = ?", couponID).
			UpdateColumn("issued_count", gorm.Expr("issued_count + 1")).Error
	})
	if err != nil {
		log.Sugar.Errorw("coupon issue failed", "err", err)
		return rec, err
	}
	return rec, nil
}

// Verify 核销优惠券（按券码）。
func (s CrmCouponService) Verify(tenantID, code, orderID string) (models.CrmCouponRecord, error) {
	var rec models.CrmCouponRecord
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("code = ?",  code).First(&rec).Error; err != nil {
		return rec, errors.New("券码不存在")
	}
	if rec.Status != "issued" {
		return rec, errors.New("优惠券状态不允许核销")
	}
	// 校验券模板有效期
	var coupon models.CrmCoupon
	if err := models.DB.Where("id = ?", rec.CouponID).First(&coupon).Error; err == nil {
		now := time.Now()
		if coupon.EndAt != nil && now.After(*coupon.EndAt) {
			models.DB.Model(&rec).Update("status", "expired")
			return rec, errors.New("优惠券已过期")
		}
	}
	now := time.Now()
	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.CrmCouponRecord{}).Where("id = ?", rec.ID).
			Updates(map[string]any{"status": "used", "used_at": &now, "order_id": orderID}).Error; err != nil {
			return err
		}
		return tx.Model(&models.CrmCoupon{}).Where("id = ?", rec.CouponID).
			UpdateColumn("used_count", gorm.Expr("used_count + 1")).Error
	})
	if err != nil {
		return rec, err
	}
	rec.Status = "used"
	rec.UsedAt = &now
	rec.OrderID = orderID
	return rec, nil
}

// ListRecords 列出发放/核销记录。
func (s CrmCouponService) ListRecords(tenantID, couponID, status string, pager *app.Pager) ([]models.CrmCouponRecord, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Scopes(models.TenantScope(tenantID))
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
