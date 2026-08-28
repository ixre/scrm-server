package services

import (
	"core/common/identity"
	
	"context"
	
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// ---------- B1 拼团 ----------

type CrmGroupBuyService struct{}

func NewCrmGroupBuy() *CrmGroupBuyService { return &CrmGroupBuyService{} }

func (s CrmGroupBuyService) Query(ctx context.Context, status string, p *app.Pager) ([]models.CrmGroupBuy, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmGroupBuy{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmGroupBuy
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmGroupBuyService) Get(ctx context.Context, id string) (models.CrmGroupBuy, error) {


	var item models.CrmGroupBuy
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	return item, err
}

func (s CrmGroupBuyService) Create(ctx context.Context, req models.CrmGroupBuy) (models.CrmGroupBuy, error) {
	tenantID := identity.TenantID(ctx)


	if req.Name == "" {
		return req, errors.New("活动名称不能为空")
	}
	if req.GroupSize < 2 {
		return req, errors.New("成团人数至少2人")
	}
	if req.GroupPrice <= 0 || req.OriginalPrice <= 0 {
		return req, errors.New("价格必须大于0")
	}
	if req.GroupPrice >= req.OriginalPrice {
		return req, errors.New("拼团价必须小于原价")
	}
	if req.TimeLimit <= 0 {
		req.TimeLimit = 24 // 默认24小时成团
	}
	if req.StartAt != nil && req.EndAt != nil && req.StartAt.After(*req.EndAt) {
		return req, errors.New("开始时间不能晚于结束时间")
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmGroupBuyService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmGroupBuy{})
	return res.RowsAffected, res.Error
}

// OpenGroup 开团：团长发起拼团。
func (s CrmGroupBuyService) OpenGroup(ctx context.Context, groupBuyID, leaderID string) (models.CrmGroupBuyRecord, error) {
	tenantID := identity.TenantID(ctx)


	if leaderID == "" {
		return models.CrmGroupBuyRecord{}, errors.New("团长ID不能为空")
	}
	gb, err := s.Get(ctx, groupBuyID)
	if err != nil {
		return models.CrmGroupBuyRecord{}, err
	}
	if gb.Status != "active" {
		return models.CrmGroupBuyRecord{}, errors.New("活动未开始或已结束")
	}
	now := time.Now()
	if gb.StartAt != nil && now.Before(*gb.StartAt) {
		return models.CrmGroupBuyRecord{}, errors.New("活动未开始")
	}
	if gb.EndAt != nil && now.After(*gb.EndAt) {
		return models.CrmGroupBuyRecord{}, errors.New("活动已结束")
	}
	if gb.MaxGroups > 0 && gb.OpenedGroups >= gb.MaxGroups {
		return models.CrmGroupBuyRecord{}, errors.New("已达最大开团数")
	}
	expireAt := now.Add(time.Duration(gb.TimeLimit) * time.Hour)
	rec := models.CrmGroupBuyRecord{
		TenantID:    tenantID,
		GroupBuyID:  groupBuyID,
		LeaderID:    leaderID,
		MemberIDs:   constants.StringArrayField{leaderID},
		CurrentSize: 1,
		Status:      "open",
		ExpireAt:    &expireAt,
	}
	rec.ID = id_generator.StringID()
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		if err := createCRM(tx, &rec); err != nil {
			return err
		}
		return tx.Model(&models.CrmGroupBuy{}).Where("tenant_id = ? AND id = ?", tenantID, groupBuyID).UpdateColumn("opened_groups", gorm.Expr("opened_groups + 1")).Error
	})
	return rec, err
}

// JoinGroup 参团：加入已有拼团。
func (s CrmGroupBuyService) JoinGroup(ctx context.Context, recordID, memberID string) (models.CrmGroupBuyRecord, error) {


	if memberID == "" {
		return models.CrmGroupBuyRecord{}, errors.New("成员ID不能为空")
	}
	var rec models.CrmGroupBuyRecord
	if err := models.MustFor(ctx).Where("id = ?",  recordID).First(&rec).Error; err != nil {
		return rec, err
	}
	if rec.Status != "open" {
		return rec, errors.New("团已结束")
	}
	if rec.ExpireAt != nil && time.Now().After(*rec.ExpireAt) {
		models.DB.Model(&rec).Update("status", "failed")
		return rec, errors.New("团已超时")
	}
	// 检查是否已参团
	for _, id := range rec.MemberIDs {
		if id == memberID {
			return rec, errors.New("已参团，不能重复参与")
		}
	}
	var gb models.CrmGroupBuy
	models.MustFor(ctx).Where("id = ?", rec.GroupBuyID).First(&gb)
	// 检查是否已满员
	if rec.CurrentSize >= gb.GroupSize {
		return rec, errors.New("团已满员")
	}
	newMembers := append(rec.MemberIDs, memberID)
	newSize := rec.CurrentSize + 1
	newStatus := "open"
	var successAt *time.Time
	if newSize >= gb.GroupSize {
		newStatus = "success"
		now := time.Now()
		successAt = &now
	}
	updates := map[string]any{"member_ids": newMembers, "current_size": newSize, "status": newStatus}
	if successAt != nil {
		updates["success_at"] = successAt
	}
	if err := models.DB.Model(&rec).Updates(updates).Error; err != nil {
		return rec, err
	}
	models.MustFor(ctx).Where("id = ?", rec.ID).First(&rec)
	return rec, nil
}

func (s CrmGroupBuyService) ListRecords(ctx context.Context, groupBuyID string, p *app.Pager) ([]models.CrmGroupBuyRecord, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if groupBuyID != "" {
		db = db.Where("group_buy_id = ?", groupBuyID)
	}
	var total int64
	if err := db.Model(&models.CrmGroupBuyRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmGroupBuyRecord
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- B2 砍价 ----------

type CrmBargainService struct{}

func NewCrmBargain() *CrmBargainService { return &CrmBargainService{} }

func (s CrmBargainService) Query(ctx context.Context, status string, p *app.Pager) ([]models.CrmBargain, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmBargain{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmBargain
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmBargainService) Get(ctx context.Context, id string) (models.CrmBargain, error) {


	var item models.CrmBargain
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	return item, err
}

func (s CrmBargainService) Create(ctx context.Context, req models.CrmBargain) (models.CrmBargain, error) {
	tenantID := identity.TenantID(ctx)


	if req.Name == "" {
		return req, errors.New("活动名称不能为空")
	}
	if req.OriginalPrice <= 0 || req.FloorPrice <= 0 {
		return req, errors.New("价格必须大于0")
	}
	if req.FloorPrice >= req.OriginalPrice {
		return req, errors.New("底价必须小于原价")
	}
	if req.MaxHelpers < 1 {
		return req, errors.New("最多帮砍人数至少1人")
	}
	if req.PerCutMin <= 0 || req.PerCutMax <= 0 || req.PerCutMin > req.PerCutMax {
		return req, errors.New("砍价金额范围不合法")
	}
	if req.TimeLimit <= 0 {
		req.TimeLimit = 24
	}
	if req.StartAt != nil && req.EndAt != nil && req.StartAt.After(*req.EndAt) {
		return req, errors.New("开始时间不能晚于结束时间")
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmBargainService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmBargain{})
	return res.RowsAffected, res.Error
}

// Start 发起砍价。
func (s CrmBargainService) Start(ctx context.Context, bargainID, customerID string) (models.CrmBargainRecord, error) {
	tenantID := identity.TenantID(ctx)


	if customerID == "" {
		return models.CrmBargainRecord{}, errors.New("客户ID不能为空")
	}
	b, err := s.Get(ctx, bargainID)
	if err != nil {
		return models.CrmBargainRecord{}, err
	}
	if b.Status != "active" {
		return models.CrmBargainRecord{}, errors.New("活动未开始或已结束")
	}
	now := time.Now()
	if b.StartAt != nil && now.Before(*b.StartAt) {
		return models.CrmBargainRecord{}, errors.New("活动未开始")
	}
	if b.EndAt != nil && now.After(*b.EndAt) {
		return models.CrmBargainRecord{}, errors.New("活动已结束")
	}
	// 幂等：同一客户同一活动只能发起一次
	var exist models.CrmBargainRecord
	if err := models.MustFor(ctx).Where("bargain_id = ? AND customer_id = ?",  bargainID,  customerID).First(&exist).Error; err == nil {
		return exist, nil
	}
	expireAt := now.Add(time.Duration(b.TimeLimit) * time.Hour)
	rec := models.CrmBargainRecord{
		TenantID:     tenantID,
		BargainID:    bargainID,
		CustomerID:   customerID,
		CurrentPrice: b.OriginalPrice,
		HelperIDs:    constants.StringArrayField{},
		HelperCnt:    0,
		Status:       "active",
		ExpireAt:     &expireAt,
	}
	rec.ID = id_generator.StringID()
	if err := createCRM(models.DB, &rec); err != nil {
		return rec, err
	}
	return rec, nil
}

// Help 帮砍一刀。
func (s CrmBargainService) Help(ctx context.Context, recordID, helperID string) (models.CrmBargainRecord, error) {


	if helperID == "" {
		return models.CrmBargainRecord{}, errors.New("帮砍者ID不能为空")
	}
	var rec models.CrmBargainRecord
	if err := models.MustFor(ctx).Where("id = ?",  recordID).First(&rec).Error; err != nil {
		return rec, err
	}
	if rec.Status != "active" {
		return rec, errors.New("砍价已结束")
	}
	if rec.ExpireAt != nil && time.Now().After(*rec.ExpireAt) {
		models.DB.Model(&rec).Update("status", "expired")
		return rec, errors.New("已超时")
	}
	var b models.CrmBargain
	models.MustFor(ctx).Where("id = ?", rec.BargainID).First(&b)
	// 检查是否已帮砍
	for _, id := range rec.HelperIDs {
		if id == helperID {
			return rec, errors.New("已帮砍过，不能重复帮砍")
		}
	}
	if b.MaxHelpers > 0 && rec.HelperCnt >= b.MaxHelpers {
		return rec, errors.New("已达最大帮砍人数")
	}
	// 砍价金额计算：确保最后一次能砍到底价
	var cut float64
	remainingPrice := rec.CurrentPrice - b.FloorPrice
	remainingHelpers := b.MaxHelpers - rec.HelperCnt
	if remainingHelpers <= 1 {
		// 最后一次帮砍：直接砍到剩余金额
		cut = remainingPrice
	} else {
		// 随机砍价，但不超过剩余金额
		cut = b.PerCutMin + rand.Float64()*(b.PerCutMax-b.PerCutMin)
		if cut > remainingPrice {
			cut = remainingPrice
		}
	}
	newPrice := rec.CurrentPrice - cut
	if newPrice < b.FloorPrice {
		newPrice = b.FloorPrice
	}
	newStatus := "active"
	if newPrice <= b.FloorPrice {
		newStatus = "success"
	}
	newHelpers := append(rec.HelperIDs, helperID)
	if err := models.DB.Model(&rec).Updates(map[string]any{
		"current_price": newPrice,
		"helper_ids":    newHelpers,
		"helper_cnt":    rec.HelperCnt + 1,
		"status":        newStatus,
	}).Error; err != nil {
		return rec, err
	}
	models.MustFor(ctx).Where("id = ?", rec.ID).First(&rec)
	return rec, nil
}

func (s CrmBargainService) ListRecords(ctx context.Context, bargainID string, p *app.Pager) ([]models.CrmBargainRecord, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if bargainID != "" {
		db = db.Where("bargain_id = ?", bargainID)
	}
	var total int64
	if err := db.Model(&models.CrmBargainRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmBargainRecord
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- B3 满减满件 ----------

type CrmFullReductionService struct{}

func NewCrmFullReduction() *CrmFullReductionService { return &CrmFullReductionService{} }

func (s CrmFullReductionService) Query(ctx context.Context, status string, p *app.Pager) ([]models.CrmFullReduction, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmFullReduction{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmFullReduction
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmFullReductionService) Create(ctx context.Context, req models.CrmFullReduction) (models.CrmFullReduction, error) {
	tenantID := identity.TenantID(ctx)


	if req.Name == "" {
		return req, errors.New("活动名称不能为空")
	}
	if req.RuleType != "full_amount" && req.RuleType != "full_count" {
		return req, errors.New("rule_type 只能为 full_amount 或 full_count")
	}
	if len(req.Rules) == 0 {
		return req, errors.New("rules 不能为空")
	}
	// 校验阶梯规则合理性：threshold 必须递增
	var prevThreshold float64
	for i, rStr := range req.Rules {
		var r struct {
			Threshold float64 `json:"threshold"`
			Discount  float64 `json:"discount"`
		}
		if err := json.Unmarshal([]byte(rStr), &r); err != nil {
			return req, fmt.Errorf("第%d条规则：JSON格式错误", i+1)
		}
		if r.Threshold <= 0 {
			return req, fmt.Errorf("第%d条规则：threshold 必须大于0", i+1)
		}
		if r.Discount <= 0 {
			return req, fmt.Errorf("第%d条规则：discount 必须大于0", i+1)
		}
		if r.Threshold <= prevThreshold {
			return req, fmt.Errorf("第%d条规则：threshold 必须递增", i+1)
		}
		// 满额减时 discount 不能超过 threshold
		if req.RuleType == "full_amount" && r.Discount > r.Threshold {
			return req, fmt.Errorf("第%d条规则：优惠金额不能超过门槛金额", i+1)
		}
		prevThreshold = r.Threshold
	}
	if req.StartAt != nil && req.EndAt != nil && req.StartAt.After(*req.EndAt) {
		return req, errors.New("开始时间不能晚于结束时间")
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmFullReductionService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmFullReduction{})
	return res.RowsAffected, res.Error
}

// CalculateDiscount 计算给定金额/件数的优惠金额。
// ruleType=full_amount 时 value=订单金额，ruleType=full_count 时 value=商品件数。
func (s CrmFullReductionService) CalculateDiscount(ctx context.Context, id string, value float64) (float64, error) {


	var fr models.CrmFullReduction
	if e := models.MustFor(ctx).Where("id = ? AND status = ?",  id,  "active").First(&fr).Error; e != nil {
		return 0, e
	}
	var discount float64
	for _, rStr := range fr.Rules {
		var r struct {
			Threshold float64 `json:"threshold"`
			Discount  float64 `json:"discount"`
		}
		if json.Unmarshal([]byte(rStr), &r) != nil {
			continue
		}
		if value >= r.Threshold {
			discount = r.Discount // 取最大满足的阶梯
		}
	}
	return discount, nil
}

// ---------- B4 红包获客 ----------

type CrmRedPacketService struct{}

func NewCrmRedPacket() *CrmRedPacketService { return &CrmRedPacketService{} }

func (s CrmRedPacketService) Query(ctx context.Context, status string, p *app.Pager) ([]models.CrmRedPacket, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmRedPacket{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmRedPacket
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmRedPacketService) Get(ctx context.Context, id string) (models.CrmRedPacket, error) {


	var item models.CrmRedPacket
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	return item, err
}

func (s CrmRedPacketService) Create(ctx context.Context, req models.CrmRedPacket) (models.CrmRedPacket, error) {
	tenantID := identity.TenantID(ctx)


	if req.Name == "" {
		return req, errors.New("活动名称不能为空")
	}
	if req.TotalCount <= 0 || req.PerAmount <= 0 {
		return req, errors.New("total_count 和 per_amount 必须大于0")
	}
	if req.StartAt != nil && req.EndAt != nil && req.StartAt.After(*req.EndAt) {
		return req, errors.New("开始时间不能晚于结束时间")
	}
	req.TotalAmount = float64(req.TotalCount) * req.PerAmount
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmRedPacketService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmRedPacket{})
	return res.RowsAffected, res.Error
}

// Claim 领取红包（乐观锁防超发，事务保证一致性）。
func (s CrmRedPacketService) Claim(ctx context.Context, redPacketID, customerID string) (models.CrmRedPacketRecord, error) {
	tenantID := identity.TenantID(ctx)


	if customerID == "" {
		return models.CrmRedPacketRecord{}, errors.New("客户ID不能为空")
	}
	rp, err := s.Get(ctx, redPacketID)
	if err != nil {
		return models.CrmRedPacketRecord{}, err
	}
	if rp.Status != "active" {
		return models.CrmRedPacketRecord{}, errors.New("活动未开始或已结束")
	}
	now := time.Now()
	if rp.StartAt != nil && now.Before(*rp.StartAt) {
		return models.CrmRedPacketRecord{}, errors.New("活动未开始")
	}
	if rp.EndAt != nil && now.After(*rp.EndAt) {
		return models.CrmRedPacketRecord{}, errors.New("活动已结束")
	}
	// 幂等：同一客户同一活动只能领一次
	var exist models.CrmRedPacketRecord
	if err := models.MustFor(ctx).Where("red_packet_id = ? AND customer_id = ?",  redPacketID,  customerID).First(&exist).Error; err == nil {
		return exist, nil
	}
	// 事务：乐观锁扣库存 + 创建领取记录
	var rec models.CrmRedPacketRecord
	err = models.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&models.CrmRedPacket{}).Where("tenant_id = ? AND id = ? AND issued_count < total_count", rp.TenantID, redPacketID).UpdateColumn("issued_count", gorm.Expr("issued_count + 1"))
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("红包已抢完")
		}
		rec = models.CrmRedPacketRecord{
			TenantID:    tenantID,
			RedPacketID: redPacketID,
			CustomerID:  customerID,
			Amount:      rp.PerAmount,
			Status:      "issued",
			IssuedAt:    &now,
		}
		rec.ID = id_generator.StringID()
		return createCRM(tx, &rec)
	})
	return rec, err
}

func (s CrmRedPacketService) ListRecords(ctx context.Context, redPacketID string, p *app.Pager) ([]models.CrmRedPacketRecord, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if redPacketID != "" {
		db = db.Where("red_packet_id = ?", redPacketID)
	}
	var total int64
	if err := db.Model(&models.CrmRedPacketRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmRedPacketRecord
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// ---------- B5 签到送券 ----------

type CrmSignInService struct{}

func NewCrmSignIn() *CrmSignInService { return &CrmSignInService{} }

func (s CrmSignInService) Query(ctx context.Context, status string, p *app.Pager) ([]models.CrmSignIn, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmSignIn{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSignIn
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

func (s CrmSignInService) Get(ctx context.Context, id string) (models.CrmSignIn, error) {


	var item models.CrmSignIn
	err := models.MustFor(ctx).Where("id = ?",  id).First(&item).Error
	return item, err
}

func (s CrmSignInService) Create(ctx context.Context, req models.CrmSignIn) (models.CrmSignIn, error) {
	tenantID := identity.TenantID(ctx)


	if req.Name == "" {
		return req, errors.New("活动名称不能为空")
	}
	if req.StartAt != nil && req.EndAt != nil && req.StartAt.After(*req.EndAt) {
		return req, errors.New("开始时间不能晚于结束时间")
	}
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if err := createCRM(models.DB, &req); err != nil {
		return req, err
	}
	return req, nil
}

func (s CrmSignInService) Delete(ctx context.Context, ids []string) (int64, error) {


	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?",  ids).Delete(&models.CrmSignIn{})
	return res.RowsAffected, res.Error
}

// CheckIn 签到：每日一次，计算连续天数，发券。
func (s CrmSignInService) CheckIn(ctx context.Context, signInID, customerID string) (models.CrmSignInRecord, error) {
	tenantID := identity.TenantID(ctx)


	if customerID == "" {
		return models.CrmSignInRecord{}, errors.New("客户ID不能为空")
	}
	si, err := s.Get(ctx, signInID)
	if err != nil {
		return models.CrmSignInRecord{}, err
	}
	if si.Status != "active" {
		return models.CrmSignInRecord{}, errors.New("活动未开始或已结束")
	}
	now := time.Now()
	if si.StartAt != nil && now.Before(*si.StartAt) {
		return models.CrmSignInRecord{}, errors.New("活动未开始")
	}
	if si.EndAt != nil && now.After(*si.EndAt) {
		return models.CrmSignInRecord{}, errors.New("活动已结束")
	}
	today := now.Format("2006-01-02")
	// 幂等：今日已签到
	var exist models.CrmSignInRecord
	if err := models.MustFor(ctx).Where("sign_in_id = ? AND customer_id = ? AND day = ?",  signInID,  customerID,  today).First(&exist).Error; err == nil {
		return exist, nil
	}
	// 查昨日签到记录计算连续天数（断签则重置为1）
	yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
	var yRec models.CrmSignInRecord
	continuous := 1
	if err := models.MustFor(ctx).Where("sign_in_id = ? AND customer_id = ? AND day = ?",  signInID,  customerID,  yesterday).First(&yRec).Error; err == nil {
		continuous = yRec.Continuous + 1
	}
	// 确定发券ID：检查连续签到奖励
	couponID := si.DailyCouponID
	if len(si.ContinuousRewards) > 0 {
		for _, crStr := range si.ContinuousRewards {
			var cr struct {
				Days     int    `json:"days"`
				CouponID string `json:"coupon_id"`
			}
			if json.Unmarshal([]byte(crStr), &cr) != nil {
				continue
			}
			if continuous >= cr.Days && cr.CouponID != "" {
				couponID = cr.CouponID // 取最高满足档位
			}
		}
	}
	rec := models.CrmSignInRecord{
		TenantID:   tenantID,
		SignInID:   signInID,
		CustomerID: customerID,
		Day:        today,
		Continuous: continuous,
		CouponID:   couponID,
	}
	rec.ID = id_generator.StringID()
	if err := createCRM(models.DB, &rec); err != nil {
		return rec, err
	}
	// 发券：创建券领取记录
	if couponID != "" {
		go func() {
			defer func() { recover() }()
			NewCrmCoupon().Issue(ctx, couponID, customerID)
		}()
	}
	return rec, nil
}

func (s CrmSignInService) ListRecords(ctx context.Context, signInID string, p *app.Pager) ([]models.CrmSignInRecord, int64, error) {


	if p == nil {
		p = &app.Pager{}
	}
	p.SetDefault()
	db := models.MustFor(ctx)
	if signInID != "" {
		db = db.Where("sign_in_id = ?", signInID)
	}
	var total int64
	if err := db.Model(&models.CrmSignInRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSignInRecord
	err := db.Offset(p.GetOffset()).Limit(p.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// 保留 encoding/json 引用（后续 JSON 字段序列化用）
var _ = json.Unmarshal
