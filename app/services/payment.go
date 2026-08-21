package services

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"core/app/models"
	"core/common/id_generator"

	qrcode "github.com/skip2/go-qrcode"

	"gorm.io/gorm"
)

// PaymentConfigService 支付方式配置服务（平台超管调用）。
type PaymentConfigService struct{}

func NewPaymentConfigService() *PaymentConfigService { return &PaymentConfigService{} }

// PaymentConfigCreateReq 创建支付配置入参。
type PaymentConfigCreateReq struct {
	Method    string            `json:"method"`
	Name      string            `json:"name"`
	SortOrder int               `json:"sort_order"`
	IsSandbox int               `json:"is_sandbox"`
	Config    map[string]string `json:"config"`
}

// PaymentConfigUpdateReq 更新支付配置入参。
type PaymentConfigUpdateReq struct {
	Name      string            `json:"name"`
	Status    int               `json:"status"`
	SortOrder int               `json:"sort_order"`
	IsSandbox int               `json:"is_sandbox"`
	Config    map[string]string `json:"config"`
}

// validMethods 支持的支付方式
var validPaymentMethods = map[string]bool{
	models.PaymentMethodWechat: true,
	models.PaymentMethodAlipay: true,
}

// wechatRequiredFields 微信支付必填字段
var wechatRequiredFields = []string{"app_id", "mch_id", "api_v3_key", "notify_url"}

// alipayRequiredFields 支付宝必填字段
var alipayRequiredFields = []string{"app_id", "private_key", "notify_url"}

// validateConfig 校验支付配置
func validateConfig(method string, config map[string]string) error {
	if config == nil {
		return errors.New("支付配置不能为空")
	}
	var required []string
	switch method {
	case models.PaymentMethodWechat:
		required = wechatRequiredFields
	case models.PaymentMethodAlipay:
		required = alipayRequiredFields
	default:
		return errors.New("不支持的支付方式: " + method)
	}
	for _, f := range required {
		if strings.TrimSpace(config[f]) == "" {
			return fmt.Errorf("缺少必填配置项: %s", f)
		}
	}
	return nil
}

// fillParsed 把 JSON Config 解析到 ConfigParsed
func fillPaymentParsed(p *models.PaymentConfig) {
	if p.Config != "" {
		_ = json.Unmarshal([]byte(p.Config), &p.ConfigParsed)
	}
	if p.ConfigParsed == nil {
		p.ConfigParsed = map[string]string{}
	}
}

// Create 创建支付配置
func (s PaymentConfigService) Create(req PaymentConfigCreateReq) (models.PaymentConfig, error) {
	var p models.PaymentConfig
	method := strings.TrimSpace(req.Method)
	if !validPaymentMethods[method] {
		return p, errors.New("不支持的支付方式")
	}
	if strings.TrimSpace(req.Name) == "" {
		return p, errors.New("显示名称不能为空")
	}
	if err := validateConfig(method, req.Config); err != nil {
		return p, err
	}
	// 唯一性校验
	var cnt int64
	models.DB.Model(&models.PaymentConfig{}).Where("method = ?", method).Count(&cnt)
	if cnt > 0 {
		return p, errors.New("该支付方式已配置")
	}
	configJSON, _ := json.Marshal(req.Config)
	p = models.PaymentConfig{
		Model:     models.Model{ID: id_generator.StringID()},
		Method:    method,
		Name:      req.Name,
		Status:    1,
		SortOrder: req.SortOrder,
		IsSandbox: req.IsSandbox,
		Config:    string(configJSON),
	}
	if err := models.DB.Create(&p).Error; err != nil {
		return p, err
	}
	fillPaymentParsed(&p)
	return p, nil
}

// List 列出所有支付配置
func (s PaymentConfigService) List() ([]models.PaymentConfig, error) {
	var configs []models.PaymentConfig
	err := models.DB.Order("sort_order, id").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	for i := range configs {
		fillPaymentParsed(&configs[i])
	}
	return configs, nil
}

// ListEnabled 列出启用的支付方式（租户端用）
func (s PaymentConfigService) ListEnabled() ([]models.PaymentConfig, error) {
	var configs []models.PaymentConfig
	err := models.DB.Where("status = 1").Order("sort_order, id").Find(&configs).Error
	if err != nil {
		return nil, err
	}
	for i := range configs {
		fillPaymentParsed(&configs[i])
		// 租户端不返回敏感配置详情
		configs[i].ConfigParsed = nil
	}
	return configs, nil
}

// Update 更新支付配置
func (s PaymentConfigService) Update(id string, req PaymentConfigUpdateReq) error {
	// 先查出现有记录获取 method
	var existing models.PaymentConfig
	if err := models.DB.Where("id = ?", id).First(&existing).Error; err != nil {
		return errors.New("支付配置不存在")
	}
	// 校验配置
	if err := validateConfig(existing.Method, req.Config); err != nil {
		return err
	}
	status := req.Status
	if status != 1 && status != 2 {
		status = 1
	}
	configJSON, _ := json.Marshal(req.Config)
	res := models.DB.Model(&models.PaymentConfig{}).Where("id = ?", id).Updates(map[string]any{
		"name":       req.Name,
		"status":     status,
		"sort_order": req.SortOrder,
		"is_sandbox": req.IsSandbox,
		"config":     string(configJSON),
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("支付配置不存在")
	}
	return nil
}

// SetStatus 启用/停用
func (s PaymentConfigService) SetStatus(id string, status int) error {
	if status != 1 && status != 2 {
		return errors.New("status 只能是 1 或 2")
	}
	res := models.DB.Model(&models.PaymentConfig{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("支付配置不存在")
	}
	return nil
}

// Delete 删除支付配置
func (s PaymentConfigService) Delete(id string) error {
	res := models.DB.Where("id = ?", id).Delete(&models.PaymentConfig{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("支付配置不存在")
	}
	return nil
}

// GetByMethod 按支付方式获取配置（内部调用）
func (s PaymentConfigService) GetByMethod(method string) (*models.PaymentConfig, error) {
	var p models.PaymentConfig
	if err := models.DB.Where("method = ? AND status = 1", method).First(&p).Error; err != nil {
		return nil, fmt.Errorf("支付方式 %s 未配置或已停用", method)
	}
	fillPaymentParsed(&p)
	return &p, nil
}

// SeedDefaultPaymentConfigs 幂等创建默认支付配置占位（需平台超管填入真实参数）。
func SeedDefaultPaymentConfigs() {
	var cnt int64
	models.DB.Model(&models.PaymentConfig{}).Count(&cnt)
	if cnt > 0 {
		return
	}
	defaults := []PaymentConfigCreateReq{
		{
			Method:    models.PaymentMethodWechat,
			Name:      "微信支付",
			SortOrder: 1,
			IsSandbox: 1,
			Config: map[string]string{
				"app_id":      "",
				"mch_id":      "",
				"api_v3_key":  "",
				"cert_serial": "",
				"private_key": "",
				"notify_url":  "",
			},
		},
		{
			Method:    models.PaymentMethodAlipay,
			Name:      "支付宝",
			SortOrder: 2,
			IsSandbox: 1,
			Config: map[string]string{
				"app_id":            "",
				"private_key":       "",
				"alipay_public_key": "",
				"gateway_url":       "https://openapi.alipay.com/gateway.do",
				"notify_url":        "",
			},
		},
	}
	for _, d := range defaults {
		// 跳过校验直接插入（占位记录，status=2 停用状态）
		configJSON, _ := json.Marshal(d.Config)
		p := models.PaymentConfig{
			Model:     models.Model{ID: id_generator.StringID()},
			Method:    d.Method,
			Name:      d.Name,
			Status:    2, // 默认停用，需超管配置参数后启用
			SortOrder: d.SortOrder,
			IsSandbox: d.IsSandbox,
			Config:    string(configJSON),
		}
		models.DB.Create(&p)
	}
}

// ---------- 租户订单服务 ----------

// TenantOrderService 租户订单服务（租户端调用）。
type TenantOrderService struct{}

func NewTenantOrderService() *TenantOrderService { return &TenantOrderService{} }

// OrderCreateReq 创建订单入参（租户端）。
type OrderCreateReq struct {
	PlanID        string `json:"plan_id"`
	Type          string `json:"type"`           // purchase/renew/upgrade
	BillingPeriod string `json:"billing_period"` // monthly/quarterly/yearly
	Years         int    `json:"years"`          // 购买年数
	PaymentMethod string `json:"payment_method"` // wechat/alipay
}

// OrderCreateResult 创建订单结果。
type OrderCreateResult struct {
	Order        models.TenantOrder `json:"order"`
	QrCodeBase64 string             `json:"qr_code_base64"` // base64 编码的二维码图片
}

// calculatePrice 根据套餐、计费周期、年数计算价格。
func calculatePrice(plan models.Plan, orderType string, billingPeriod string, years int) (original, discount, amount int64, giftYears int) {
	if years < 1 {
		years = 1
	}
	// 免费套餐或试用套餐价格为 0，直接返回
	if plan.IsFree == 1 || plan.IsTrial == 1 {
		return 0, 0, 0, 0
	}
	// 选择基础单价
	var basePrice int64
	var renewPrice int64
	switch billingPeriod {
	case models.BillingMonthly:
		basePrice = plan.MonthlyPrice
		renewPrice = plan.MonthlyRenewPrice
	case models.BillingQuarterly:
		basePrice = plan.QuarterlyPrice
		renewPrice = plan.QuarterlyRenewPrice
	case models.BillingYearly:
		basePrice = plan.YearlyPrice
		renewPrice = plan.YearlyRenewPrice
	}
	// 续费用续费价，新购/升级用新购价
	unitPrice := basePrice
	if orderType == models.OrderTypeRenew && renewPrice > 0 {
		unitPrice = renewPrice
	}
	// 原始金额 = 单价 × 年数
	original = unitPrice * int64(years)

	// 查找多年折扣
	var matchedDiscount *models.PlanMultiYearDiscount
	discounts := []models.PlanMultiYearDiscount{}
	if plan.MultiYearDiscounts != "" {
		_ = json.Unmarshal([]byte(plan.MultiYearDiscounts), &discounts)
	}
	// 按年数降序匹配最优规则
	sort.Slice(discounts, func(i, j int) bool { return discounts[i].Years > discounts[j].Years })
	for i := range discounts {
		if years >= discounts[i].Years {
			matchedDiscount = &discounts[i]
			break
		}
	}

	amount = original
	if matchedDiscount != nil {
		discountPct := matchedDiscount.Discount
		if discountPct <= 0 || discountPct > 100 {
			discountPct = 100
		}
		amount = original * int64(discountPct) / 100
		discount = original - amount
		giftYears = matchedDiscount.GiftYears
	}
	return
}

// generateOrderNo 生成订单号
func generateOrderNo() string {
	return fmt.Sprintf("SCRM%s", id_generator.StringID())
}

// generateQrCodeBase64 用 go-qrcode 生成 base64 编码的 PNG 二维码
func generateQrCodeBase64(content string) (string, error) {
	png, err := qrcode.Encode(content, qrcode.Medium, 256)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}

// Create 创建订单（租户端调用）
func (s TenantOrderService) Create(tenantID string, req OrderCreateReq) (OrderCreateResult, error) {
	var result OrderCreateResult
	if tenantID == "" {
		return result, errors.New("缺少租户身份")
	}
	if req.PlanID == "" {
		return result, errors.New("请选择套餐")
	}
	if req.Type != models.OrderTypePurchase && req.Type != models.OrderTypeRenew && req.Type != models.OrderTypeUpgrade {
		return result, errors.New("无效的订单类型")
	}
	if req.BillingPeriod != models.BillingMonthly && req.BillingPeriod != models.BillingQuarterly && req.BillingPeriod != models.BillingYearly {
		return result, errors.New("无效的计费周期")
	}
	// 获取套餐
	var plan models.Plan
	if err := models.DB.Where("id = ? AND status = 1", req.PlanID).First(&plan).Error; err != nil {
		return result, errors.New("套餐不存在或已停用")
	}

	// 免费/试用套餐允许 payment_method=none，付费套餐必须选 wechat/alipay
	isFreeOrTrial := plan.IsFree == 1 || plan.IsTrial == 1
	if isFreeOrTrial {
		if req.PaymentMethod != "none" && req.PaymentMethod != models.PaymentMethodWechat && req.PaymentMethod != models.PaymentMethodAlipay {
			return result, errors.New("无效的支付方式")
		}
	} else {
		if req.PaymentMethod != models.PaymentMethodWechat && req.PaymentMethod != models.PaymentMethodAlipay {
			return result, errors.New("无效的支付方式")
		}
	}
	if req.Years < 1 {
		return result, errors.New("购买年数必须大于0")
	}

	// 计算价格
	original, discount, amount, giftYears := calculatePrice(plan, req.Type, req.BillingPeriod, req.Years)

	// 创建订单
	orderNo := generateOrderNo()
	expireAt := time.Now().Add(30 * time.Minute) // 30 分钟过期

	order := models.TenantOrder{
		Model:          models.Model{ID: id_generator.StringID()},
		TenantID:       tenantID,
		OrderNo:        orderNo,
		Type:           req.Type,
		PlanID:         plan.ID,
		PlanCode:       plan.Code,
		PlanName:       plan.Name,
		BillingPeriod:  req.BillingPeriod,
		Years:          req.Years,
		GiftYears:      giftYears,
		OriginalAmount: original,
		DiscountAmount: discount,
		Amount:         amount,
		PaymentMethod:  req.PaymentMethod,
		PayStatus:      models.PayStatusPending,
		ExpireAt:       &expireAt,
	}

	// 免费/试用套餐：自动标记为已支付，直接更新租户套餐
	if isFreeOrTrial {
		now := time.Now()
		order.PayStatus = models.PayStatusPaid
		order.PaidAt = &now
		order.TransactionID = fmt.Sprintf("FREE_%s", orderNo)
		// 保存订单
		if err := models.DB.Create(&order).Error; err != nil {
			return result, err
		}
		// 直接更新租户套餐和到期时间
		s.applyOrderToTenant(order, plan)
		result.Order = order
		return result, nil
	}

	// 付费套餐：生成二维码
	qrContent := fmt.Sprintf("scrm://pay?order_no=%s&amount=%d&method=%s&plan=%s",
		orderNo, amount, req.PaymentMethod, plan.Code)
	order.QrCodeContent = qrContent
	qrBase64, err := generateQrCodeBase64(qrContent)
	if err != nil {
		return result, fmt.Errorf("生成二维码失败: %v", err)
	}

	// 保存订单
	if err := models.DB.Create(&order).Error; err != nil {
		return result, err
	}

	result.Order = order
	result.QrCodeBase64 = qrBase64
	return result, nil
}

// applyOrderToTenant 将订单效果应用到租户（更新套餐和到期时间）。
func (s TenantOrderService) applyOrderToTenant(order models.TenantOrder, plan models.Plan) {
	var tenant models.Tenant
	if err := models.DB.Where("id = ?", order.TenantID).First(&tenant).Error; err != nil {
		return
	}
	// 试用套餐：按 trial_days 设置到期，从当前时间开始
	if plan.IsTrial == 1 && plan.TrialDays > 0 {
		base := time.Now().AddDate(0, 0, plan.TrialDays)
		models.DB.Model(&tenant).Updates(map[string]any{
			"plan":      order.PlanCode,
			"expire_at": &base,
		})
		return
	}
	// 永久免费套餐（free_days=0）：到期时间设为 NULL（永久）
	if plan.IsFree == 1 && plan.FreeDays == 0 {
		models.DB.Model(&tenant).Updates(map[string]any{
			"plan":      order.PlanCode,
			"expire_at": nil,
		})
		return
	}
	// 限期免费套餐（free_days>0）：按 free_days 设置到期
	if plan.IsFree == 1 && plan.FreeDays > 0 {
		base := time.Now().AddDate(0, 0, plan.FreeDays)
		models.DB.Model(&tenant).Updates(map[string]any{
			"plan":      order.PlanCode,
			"expire_at": &base,
		})
		return
	}
	// 付费套餐：按计费周期和年数延长
	base := time.Now()
	if order.Type == models.OrderTypeRenew && tenant.ExpireAt != nil && tenant.ExpireAt.After(base) {
		base = *tenant.ExpireAt
	}
	totalYears := order.Years + order.GiftYears
	switch order.BillingPeriod {
	case models.BillingMonthly, models.BillingQuarterly:
		base = base.AddDate(0, totalYears*12, 0)
	case models.BillingYearly:
		base = base.AddDate(totalYears, 0, 0)
	}
	models.DB.Model(&tenant).Updates(map[string]any{
		"plan":      order.PlanCode,
		"expire_at": &base,
	})
}

// ListOrders 列出租户订单
func (s TenantOrderService) ListOrders(tenantID string) ([]models.TenantOrder, error) {
	var orders []models.TenantOrder
	err := models.DB.Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&orders).Error
	return orders, err
}

// GetOrder 获取订单详情
func (s TenantOrderService) GetOrder(tenantID, orderID string) (models.TenantOrder, error) {
	var order models.TenantOrder
	err := models.DB.Where("id = ? AND tenant_id = ?", orderID, tenantID).First(&order).Error
	return order, err
}

// GetOrderByNo 按订单号获取
func (s TenantOrderService) GetOrderByNo(orderNo string) (models.TenantOrder, error) {
	var order models.TenantOrder
	err := models.DB.Where("order_no = ?", orderNo).First(&order).Error
	return order, err
}

// QueryPayStatus 查询支付状态（租户端轮询）
func (s TenantOrderService) QueryPayStatus(tenantID, orderID string) (models.TenantOrder, error) {
	var order models.TenantOrder
	err := models.DB.Where("id = ? AND tenant_id = ?", orderID, tenantID).First(&order).Error
	if err != nil {
		return order, err
	}
	// 检查是否过期
	if order.PayStatus == models.PayStatusPending && order.ExpireAt != nil && order.ExpireAt.Before(time.Now()) {
		models.DB.Model(&order).Update("pay_status", models.PayStatusExpired)
		order.PayStatus = models.PayStatusExpired
	}
	return order, nil
}

// GetQrCode 获取订单二维码（租户端）
func (s TenantOrderService) GetQrCode(tenantID, orderID string) (string, error) {
	order, err := s.GetOrder(tenantID, orderID)
	if err != nil {
		return "", err
	}
	if order.PayStatus != models.PayStatusPending {
		return "", errors.New("订单状态不允许支付")
	}
	if order.ExpireAt != nil && order.ExpireAt.Before(time.Now()) {
		models.DB.Model(&order).Update("pay_status", models.PayStatusExpired)
		return "", errors.New("订单已过期")
	}
	// 重新生成二维码
	qrBase64, err := generateQrCodeBase64(order.QrCodeContent)
	if err != nil {
		return "", err
	}
	return qrBase64, nil
}

// ---------- 平台端订单管理 ----------

// ListAllOrders 平台端列出所有订单（分页）
func (s TenantOrderService) ListAllOrders(req OrderListReq) (OrderListResult, error) {
	var result OrderListResult
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 200 {
		req.PageSize = 200
	}
	q := models.DB.Model(&models.TenantOrder{})
	if req.TenantID != "" {
		q = q.Where("tenant_id = ?", req.TenantID)
	}
	if req.PayStatus > 0 {
		q = q.Where("pay_status = ?", req.PayStatus)
	}
	if req.Method != "" {
		q = q.Where("payment_method = ?", req.Method)
	}
	q.Count(&result.Total)
	offset := (req.Page - 1) * req.PageSize
	var orders []models.TenantOrder
	q.Order("created_at DESC").Offset(offset).Limit(req.PageSize).Find(&orders)
	result.List = orders
	return result, nil
}

// OrderListReq 订单列表查询参数
type OrderListReq struct {
	TenantID  string `json:"tenant_id"`
	PayStatus int    `json:"pay_status"`
	Method    string `json:"method"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
}

// OrderListResult 订单列表结果
type OrderListResult struct {
	Total int64                `json:"total"`
	List  []models.TenantOrder `json:"list"`
}

// ManualConfirmPay 平台端手动确认支付（模拟支付回调，用于测试/线下收款）
func (s TenantOrderService) ManualConfirmPay(orderID string) error {
	var order models.TenantOrder
	if err := models.DB.Where("id = ?", orderID).First(&order).Error; err != nil {
		return errors.New("订单不存在")
	}
	if order.PayStatus == models.PayStatusPaid {
		return errors.New("订单已支付，请勿重复操作")
	}
	if order.PayStatus != models.PayStatusPending {
		return errors.New("订单状态不允许确认支付")
	}
	// 允许对已过期订单手动确认（线下收款场景），但给出提示
	now := time.Now()
	return models.DB.Transaction(func(tx *gorm.DB) error {
		// 更新订单状态
		if err := tx.Model(&order).Updates(map[string]any{
			"pay_status":     models.PayStatusPaid,
			"paid_at":        &now,
			"transaction_id": fmt.Sprintf("MANUAL_%s", order.OrderNo),
		}).Error; err != nil {
			return err
		}
		// 如果是续费/新购/升级，更新租户套餐和到期时间
		if order.Type == models.OrderTypePurchase || order.Type == models.OrderTypeRenew || order.Type == models.OrderTypeUpgrade {
			var tenant models.Tenant
			if err := tx.Where("id = ?", order.TenantID).First(&tenant).Error; err != nil {
				return err
			}
			// 更新套餐
			tenant.Plan = order.PlanCode
			// 计算新的到期时间
			// 续费：从当前到期时间延长（如果未过期）；新购/升级：从当前时间开始
			base := time.Now()
			if order.Type == models.OrderTypeRenew && tenant.ExpireAt != nil && tenant.ExpireAt.After(base) {
				base = *tenant.ExpireAt
			}
			// 按计费周期和年数延长
			totalYears := order.Years + order.GiftYears
			switch order.BillingPeriod {
			case models.BillingMonthly:
				// 按月购买：totalYears 年 = totalYears*12 个月
				base = base.AddDate(0, totalYears*12, 0)
			case models.BillingQuarterly:
				// 按季购买：totalYears 年 = totalYears*4 个季度 = totalYears*12 个月
				base = base.AddDate(0, totalYears*12, 0)
			case models.BillingYearly:
				base = base.AddDate(totalYears, 0, 0)
			}
			if err := tx.Model(&tenant).Updates(map[string]any{
				"plan":      order.PlanCode,
				"expire_at": &base,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// CancelOrder 取消订单
func (s TenantOrderService) CancelOrder(tenantID, orderID string) error {
	res := models.DB.Model(&models.TenantOrder{}).
		Where("id = ? AND tenant_id = ? AND pay_status = ?", orderID, tenantID, models.PayStatusPending).
		Update("pay_status", models.PayStatusCancelled)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("订单不存在或状态不允许取消")
	}
	return nil
}
