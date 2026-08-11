package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// ---------- 抽奖引擎 ----------

type CrmLotteryService struct{}

func NewCrmLottery() *CrmLotteryService { return &CrmLotteryService{} }

// LotteryCreateReq 抽奖创建请求。
type LotteryCreateReq struct {
	Lottery models.CrmLottery `json:"lottery"`
}

// LotteryPrize 奖品配置（存储在 CrmLottery.Prizes JSON 数组中）。
type LotteryPrize struct {
	Name   string `json:"name"`   // 奖品名称
	Type   string `json:"type"`   // 奖品类型 coupon/cash/gift/thanks
	Weight int    `json:"weight"` // 中奖权重
	Count  int    `json:"count"`  // 奖品库存(0=不限,thanks通常0)
	Issued int    `json:"issued"` // 已发放数
}

// Create 创建抽奖活动。
func (s CrmLotteryService) Create(tenantID string, req models.CrmLottery) (models.CrmLottery, error) {
	req.ID = id_generator.StringID()
	req.TenantID = tenantID
	if req.Status == "" {
		req.Status = "active"
	}
	if req.DailyLimit <= 0 {
		req.DailyLimit = 1
	}
	if len(req.Prizes) == 0 {
		return req, errors.New("奖品配置不能为空")
	}
	// 校验奖品配置
	prizes := s.parsePrizes(req.Prizes)
	if len(prizes) == 0 {
		return req, errors.New("奖品配置格式错误")
	}
	totalWeight := 0
	for _, p := range prizes {
		if p.Weight < 0 {
			return req, errors.New("奖品权重不能为负")
		}
		totalWeight += p.Weight
	}
	if totalWeight == 0 {
		return req, errors.New("总权重不能为0")
	}
	if err := createCRM(models.DB, &req, "ActivityID"); err != nil {
		log.Sugar.Errorw("lottery create failed", "err", err)
		return req, err
	}
	return req, nil
}

// Query 列出抽奖活动。
func (s CrmLotteryService) Query(tenantID, status string, pager *app.Pager) ([]models.CrmLottery, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Scopes(models.TenantScope(tenantID))
	if status != "" {
		db = db.Where("status = ?", status)
	}
	var total int64
	if err := db.Model(&models.CrmLottery{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmLottery
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// Get 查单个抽奖活动。
func (s CrmLotteryService) Get(tenantID, id string) (models.CrmLottery, error) {
	var l models.CrmLottery
	err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", id).First(&l).Error
	return l, err
}

// Delete 删除抽奖活动。
func (s CrmLotteryService) Delete(tenantID string, ids []string) (int64, error) {
	var cnt int64
	models.DB.Model(&models.CrmLotteryRecord{}).Scopes(models.TenantScope(tenantID)).Where("lottery_id IN ?",  ids).Count(&cnt)
	if cnt > 0 {
		return 0, errors.New("有抽奖记录的活动不能删除")
	}
	res := models.DB.Scopes(models.TenantScope(tenantID)).Where("id IN ?",  ids).Delete(&models.CrmLottery{})
	return res.RowsAffected, res.Error
}

// Draw 抽奖：权重随机 + 次数校验 + 库存校验。
func (s CrmLotteryService) Draw(tenantID, lotteryID, customerID string) (models.CrmLotteryRecord, error) {
	var rec models.CrmLotteryRecord
	var l models.CrmLottery
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", lotteryID).First(&l).Error; err != nil {
		return rec, errors.New("抽奖活动不存在")
	}
	if l.Status != "active" {
		return rec, errors.New("抽奖活动已结束")
	}
	now := time.Now()
	if l.StartAt != nil && now.Before(*l.StartAt) {
		return rec, errors.New("抽奖尚未开始")
	}
	if l.EndAt != nil && now.After(*l.EndAt) {
		return rec, errors.New("抽奖已结束")
	}
	// 校验客户
	var c models.CrmCustomer
	if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", customerID).First(&c).Error; err != nil {
		return rec, errors.New("客户不存在")
	}
	// 总次数校验
	if l.TotalLimit > 0 && l.DrawnCount >= l.TotalLimit {
		return rec, errors.New("抽奖次数已用完")
	}
	// 每日次数校验
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := todayStart.Add(24 * time.Hour)
	var todayCnt int64
	models.DB.Model(&models.CrmLotteryRecord{}).
		Where("lottery_id = ? AND customer_id = ? AND drawn_at >= ? AND drawn_at < ?", lotteryID, customerID, todayStart, todayEnd).Count(&todayCnt)
	if int(todayCnt) >= l.DailyLimit {
		return rec, fmt.Errorf("超过每日抽奖次数%d次", l.DailyLimit)
	}

	// 权重随机抽奖
	prizes := s.parsePrizes(l.Prizes)
	prize := s.weightedRandom(prizes)

	nowPtr := time.Now()
	rec = models.CrmLotteryRecord{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		LotteryID:  lotteryID,
		CustomerID: customerID,
		PrizeName:  prize.Name,
		PrizeType:  prize.Type,
		IsWin:      prize.Type != "thanks",
		DrawnAt:    &nowPtr,
	}

	err := models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&rec).Error; err != nil {
			return err
		}
		// 更新已抽次数
		if err := tx.Model(&models.CrmLottery{}).Where("id = ?", lotteryID).
			UpdateColumn("drawn_count", gorm.Expr("drawn_count + 1")).Error; err != nil {
			return err
		}
		// 更新奖品发放数（非 thanks 奖品）
		if prize.Type != "thanks" && prize.Count > 0 {
			prizes = s.updatePrizeIssued(prizes, prize.Name)
			newPrizesJSON, _ := json.Marshal(prizes)
			if err := tx.Model(&models.CrmLottery{}).Where("id = ?", lotteryID).
				Update("prizes", string(newPrizesJSON)).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Sugar.Errorw("lottery draw failed", "err", err, "lottery", lotteryID, "customer", customerID)
		return rec, err
	}
	return rec, nil
}

// ListRecords 抽奖记录列表。
func (s CrmLotteryService) ListRecords(tenantID, lotteryID string, pager *app.Pager) ([]models.CrmLotteryRecord, int64, error) {
	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.DB.Scopes(models.TenantScope(tenantID))
	if lotteryID != "" {
		db = db.Where("lottery_id = ?", lotteryID)
	}
	var total int64
	if err := db.Model(&models.CrmLotteryRecord{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmLotteryRecord
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}

// parsePrizes 解析奖品配置 JSON。
func (s CrmLotteryService) parsePrizes(raw constants.StringArrayField) []LotteryPrize {
	if len(raw) == 0 {
		return nil
	}
	// raw 是 StringArrayField，每个元素是一段 JSON 或字符串
	// 实际存储时整个奖品列表是一个 JSON 数组字符串
	var prizes []LotteryPrize
	// 尝试解析为 JSON 数组
	if len(raw) == 1 {
		if err := json.Unmarshal([]byte(raw[0]), &prizes); err == nil {
			return prizes
		}
	}
	// 尝试逐个解析
	for _, item := range raw {
		var p LotteryPrize
		if err := json.Unmarshal([]byte(item), &p); err == nil {
			prizes = append(prizes, p)
		}
	}
	return prizes
}

// weightedRandom 按权重随机选择奖品（库存耗尽的跳过）。
func (s CrmLotteryService) weightedRandom(prizes []LotteryPrize) LotteryPrize {
	// 过滤掉库存已耗尽的奖品（Count>0 且 Issued>=Count 的跳过）
	available := make([]LotteryPrize, 0, len(prizes))
	for _, p := range prizes {
		if p.Count > 0 && p.Issued >= p.Count {
			continue
		}
		available = append(available, p)
	}
	if len(available) == 0 {
		// 所有奖品都发完了，返回 thanks
		return LotteryPrize{Name: "谢谢参与", Type: "thanks", Weight: 1}
	}
	totalWeight := 0
	for _, p := range available {
		totalWeight += p.Weight
	}
	if totalWeight <= 0 {
		return LotteryPrize{Name: "谢谢参与", Type: "thanks", Weight: 1}
	}
	r := rand.Intn(totalWeight)
	for _, p := range available {
		r -= p.Weight
		if r < 0 {
			return p
		}
	}
	return available[len(available)-1]
}

// updatePrizeIssued 更新指定奖品的已发放数。
func (s CrmLotteryService) updatePrizeIssued(prizes []LotteryPrize, name string) []LotteryPrize {
	for i := range prizes {
		if prizes[i].Name == name {
			prizes[i].Issued++
			break
		}
	}
	return prizes
}
