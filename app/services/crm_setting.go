package services

import (
	"core/common/identity"
	"encoding/json"

	"context"

	"errors"
	"strings"

	"core/app/models"
	"core/common/id_generator"

	"gorm.io/gorm"
)

// ---------- 自动分配规则 ----------

// AutoAllocationQueryReq 自动分配规则查询参数。
type AutoAllocationQueryReq struct {
	Page       int
	PageSize   int
	EntityType string
}

// CrmAutoAllocationService 自动分配规则服务。
type CrmAutoAllocationService struct{}

func NewCrmAutoAllocation() *CrmAutoAllocationService { return &CrmAutoAllocationService{} }

// Create 创建自动分配规则。
func (s CrmAutoAllocationService) Create(ctx context.Context, a *models.CrmAutoAllocation) error {
	tenantID := identity.TenantID(ctx)

	if a == nil {
		return errors.New("规则不能为空")
	}
	if strings.TrimSpace(a.Name) == "" {
		return errors.New("规则名称不能为空")
	}
	if a.EntityType == "" {
		return errors.New("entity_type 不能为空")
	}
	a.ID = id_generator.StringID()
	a.TenantID = tenantID
	a.Name = strings.TrimSpace(a.Name)
	return createCRM(models.DB, a)
}

// Update 更新自动分配规则。
func (s CrmAutoAllocationService) Update(ctx context.Context, id string, a *models.CrmAutoAllocation) error {

	if a == nil {
		return errors.New("规则不能为空")
	}
	m := map[string]any{
		"entity_type": a.EntityType,
		"name":        a.Name,
		"conditions":  a.Conditions,
		"strategy":    a.Strategy,
		"target_ids":  a.TargetIDs,
		"is_enabled":  a.IsEnabled,
		"priority":    a.Priority,
	}
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除自动分配规则。
func (s CrmAutoAllocationService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmAutoAllocation{})
	return res.RowsAffected, res.Error
}

// Query 分页查询自动分配规则。
func (s CrmAutoAllocationService) Query(ctx context.Context, req AutoAllocationQueryReq) ([]models.CrmAutoAllocation, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if et := strings.TrimSpace(req.EntityType); et != "" {
		db = db.Where("entity_type = ?", et)
	}
	var total int64
	if err := db.Model(&models.CrmAutoAllocation{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmAutoAllocation
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).
		Order("priority desc, id desc").Find(&items).Error
	return items, total, err
}

// ToggleStatus 启用/停用自动分配规则。
func (s CrmAutoAllocationService) ToggleStatus(ctx context.Context, id string, enabled bool) error {
	tenantID := identity.TenantID(ctx)

	return models.DB.Model(&models.CrmAutoAllocation{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
		Update("is_enabled", enabled).Error
}

// ---------- 公海类型 ----------

// SeasTypeQueryReq 公海类型查询参数。
type SeasTypeQueryReq struct {
	Page     int
	PageSize int
	Keyword  string
}

// CrmSeasTypeService 公海类型服务。
type CrmSeasTypeService struct{}

func NewCrmSeasType() *CrmSeasTypeService { return &CrmSeasTypeService{} }

// Create 创建公海类型。
func (s CrmSeasTypeService) Create(ctx context.Context, st *models.CrmSeasType) error {
	tenantID := identity.TenantID(ctx)

	if st == nil {
		return errors.New("公海类型不能为空")
	}
	if strings.TrimSpace(st.Name) == "" {
		return errors.New("公海名称不能为空")
	}
	st.ID = id_generator.StringID()
	st.TenantID = tenantID
	st.Name = strings.TrimSpace(st.Name)
	return createCRM(models.DB, st)
}

// Update 更新公海类型。
func (s CrmSeasTypeService) Update(ctx context.Context, id string, st *models.CrmSeasType) error {

	if st == nil {
		return errors.New("公海类型不能为空")
	}
	m := map[string]any{
		"name":      st.Name,
		"rules":     st.Rules,
		"group_ids": st.GroupIDs,
		"staff_ids": st.StaffIDs,
		"remark":    st.Remark,
	}
	if err := models.MustFor(ctx).Where("id = ?", id).Updates(m).Error; err != nil {
		return err
	}
	return nil
}

// Delete 删除公海类型。
func (s CrmSeasTypeService) Delete(ctx context.Context, ids []string) (int64, error) {

	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmSeasType{})
	return res.RowsAffected, res.Error
}

// Query 分页查询公海类型。
func (s CrmSeasTypeService) Query(ctx context.Context, req SeasTypeQueryReq) ([]models.CrmSeasType, int64, error) {

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	db := models.MustFor(ctx)
	if kw := strings.TrimSpace(req.Keyword); kw != "" {
		db = db.Where("name LIKE ?", "%"+kw+"%")
	}
	var total int64
	if err := db.Model(&models.CrmSeasType{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmSeasType
	err := db.Offset((req.Page - 1) * req.PageSize).Limit(req.PageSize).Order("id desc").Find(&items).Error
	return items, total, err
}

// SetDefault 设为默认公海（同租户其他取消默认）。
func (s CrmSeasTypeService) SetDefault(ctx context.Context, id string) error {
	tenantID := identity.TenantID(ctx)

	var st models.CrmSeasType
	if err := models.MustFor(ctx).Where("id = ?", id).First(&st).Error; err != nil {
		return err
	}
	return models.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.CrmSeasType{}).Scopes(models.TenantScope(tenantID)).Where("id <> ?", id).
			Update("is_default", false).Error; err != nil {
			return err
		}
		return tx.Model(&models.CrmSeasType{}).Scopes(models.TenantScope(tenantID)).Where("id = ?", id).
			Update("is_default", true).Error
	})
}

// ---------- 客户设置 ----------

// CrmCustomerSettingService 客户设置服务。
type CrmCustomerSettingService struct{}

func NewCrmCustomerSetting() *CrmCustomerSettingService { return &CrmCustomerSettingService{} }

// Get 获取客户设置（不存在返回零值默认）。
func (s CrmCustomerSettingService) Get(ctx context.Context) (*models.CrmCustomerSetting, error) {
	tenantID := identity.TenantID(ctx)

	var item models.CrmCustomerSetting
	err := models.MustFor(ctx).First(&item).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &models.CrmCustomerSetting{TenantID: tenantID}, nil
		}
		return nil, err
	}
	return &item, nil
}

// Save 保存客户设置（upsert：存在则更新，不存在则新建）。
func (s CrmCustomerSettingService) Save(ctx context.Context, cs *models.CrmCustomerSetting) error {
	tenantID := identity.TenantID(ctx)

	if cs == nil {
		return errors.New("设置不能为空")
	}
	cs.TenantID = tenantID
	var existing models.CrmCustomerSetting
	err := models.MustFor(ctx).First(&existing).Error
	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}
	if err == gorm.ErrRecordNotFound {
		cs.ID = id_generator.StringID()
		return createCRM(models.DB, cs)
	}
	m := map[string]any{
		"max_customers": cs.MaxCustomers,
		"max_locked":    cs.MaxLocked,
		"lock_days":     cs.LockDays,
	}
	return models.DB.Model(&models.CrmCustomerSetting{}).Scopes(models.TenantScope(tenantID)).Updates(m).Error
}

// ---------- 线索来源 ----------

// DefaultLeadSources 系统预置的默认线索来源（管理员未配置时兜底）。
var DefaultLeadSources = []string{
	"展会", "转介绍", "广告", "搜索引擎", "社交媒体", "电话营销",
	"邮件营销", "网站表单", "线下活动", "合作伙伴", "主动开发", "其他",
}

// CrmLeadSourceService 线索来源服务。
type CrmLeadSourceService struct{}

func NewCrmLeadSource() *CrmLeadSourceService { return &CrmLeadSourceService{} }

// List 查询当前租户的线索来源选项。
// 若租户未配置任何来源，返回系统默认列表（以 DefaultLeadSources 生成记录，不落库）。
func (s CrmLeadSourceService) List(ctx context.Context) ([]models.CrmLeadSource, error) {
	var items []models.CrmLeadSource
	if err := models.MustFor(ctx).Order("sort asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return items, nil
	}
	// 兜底：返回默认列表
	for i, name := range DefaultLeadSources {
		items = append(items, models.CrmLeadSource{
			Name:    name,
			Sort:    i,
			Enabled: true,
		})
	}
	return items, nil
}

// Create 新建线索来源。
func (s CrmLeadSourceService) Create(ctx context.Context, src *models.CrmLeadSource) error {
	if src == nil {
		return errors.New("来源不能为空")
	}
	if strings.TrimSpace(src.Name) == "" {
		return errors.New("来源名称不能为空")
	}
	src.ID = id_generator.StringID()
	src.TenantID = identity.TenantID(ctx)
	src.Name = strings.TrimSpace(src.Name)
	return createCRM(models.DB, src)
}

// Update 更新线索来源。
func (s CrmLeadSourceService) Update(ctx context.Context, id string, src *models.CrmLeadSource) error {
	if src == nil {
		return errors.New("来源不能为空")
	}
	if strings.TrimSpace(src.Name) == "" {
		return errors.New("来源名称不能为空")
	}
	m := map[string]any{
		"name":    strings.TrimSpace(src.Name),
		"sort":    src.Sort,
		"enabled": src.Enabled,
	}
	return models.MustFor(ctx).Where("id = ?", id).Updates(m).Error
}

// Delete 删除线索来源。
func (s CrmLeadSourceService) Delete(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmLeadSource{})
	return res.RowsAffected, res.Error
}

// ---------- 赢单/输单原因字典 ----------

// DefaultWinReasons 系统预置的默认赢单原因。
var DefaultWinReasons = []string{
	"价格优势", "产品功能匹配", "客户关系良好", "竞争对手退出", "决策周期短", "其他",
}

// DefaultLoseReasons 系统预置的默认输单原因。
var DefaultLoseReasons = []string{
	"价格过高", "竞品胜出", "需求不匹配", "决策周期过长", "预算不足", "客户流失", "其他",
}

// CrmCustomerTagService 客户标签服务。
type CrmCustomerTagService struct{}

func NewCrmCustomerTag() *CrmCustomerTagService { return &CrmCustomerTagService{} }

// List 查询客户标签列表，支持按分组筛选。
func (s CrmCustomerTagService) List(ctx context.Context, group string) ([]models.CrmCustomerTag, error) {
	var items []models.CrmCustomerTag
	db := models.MustFor(ctx).Model(&models.CrmCustomerTag{})
	if group != "" {
		db = db.Where("tag_group = ?", group)
	}
	err := db.Order("sort asc, id asc").Find(&items).Error
	return items, err
}

// Create 新建标签。
func (s CrmCustomerTagService) Create(ctx context.Context, item *models.CrmCustomerTag) error {
	if item == nil {
		return errors.New("标签不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("标签名称不能为空")
	}
	item.ID = id_generator.StringID()
	item.TenantID = identity.TenantID(ctx)
	item.Name = strings.TrimSpace(item.Name)
	if item.Color == "" {
		item.Color = "arcoblue"
	}
	return createCRM(models.DB, item)
}

// Update 更新标签。
func (s CrmCustomerTagService) Update(ctx context.Context, id string, item *models.CrmCustomerTag) error {
	if item == nil {
		return errors.New("标签不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("标签名称不能为空")
	}
	m := map[string]any{
		"name":      strings.TrimSpace(item.Name),
		"tag_group": strings.TrimSpace(item.Group),
		"color":     item.Color,
		"sort":      item.Sort,
		"enabled":   item.Enabled,
	}
	return models.MustFor(ctx).Model(&models.CrmCustomerTag{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除标签，同时清除客户表中的引用。
func (s CrmCustomerTagService) Delete(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	tenantID := identity.TenantID(ctx)
	// 清除客户表中的标签引用（从 tags 数组中移除被删除的标签ID）
	for _, tagID := range ids {
		models.DB.Scopes(models.TenantScope(tenantID)).
			Exec(`UPDATE crm_customer SET tags = COALESCE(tags, '[]'::jsonb) - ? WHERE tenant_id = ?`, tagID, tenantID)
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmCustomerTag{})
	return res.RowsAffected, res.Error
}

// BatchTag 批量给客户打标签或取消标签。
// Action: "add" 打标签, "remove" 取消标签。
func (s CrmCustomerTagService) BatchTag(ctx context.Context, action string, customerIDs []string, tagIDs []string) (int64, error) {
	if len(customerIDs) == 0 || len(tagIDs) == 0 {
		return 0, errors.New("customer_ids 和 tag_ids 不能为空")
	}
	if action != "add" && action != "remove" {
		return 0, errors.New("action 只能为 add 或 remove")
	}
	tenantID := identity.TenantID(ctx)
	var affected int64
	for _, cid := range customerIDs {
		var cust models.CrmCustomer
		if err := models.DB.Scopes(models.TenantScope(tenantID)).Where("id = ?", cid).First(&cust).Error; err != nil {
			continue
		}
		currentSet := make(map[string]bool)
		for _, t := range cust.Tags {
			currentSet[t] = true
		}
		if action == "add" {
			for _, tid := range tagIDs {
				currentSet[tid] = true
			}
		} else {
			for _, tid := range tagIDs {
				delete(currentSet, tid)
			}
		}
		newTags := make([]string, 0, len(currentSet))
		for t := range currentSet {
			newTags = append(newTags, t)
		}
		res := models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).
			Where("id = ?", cid).Update("tags", newTags)
		if res.Error == nil {
			affected += res.RowsAffected
		}
	}
	return affected, nil
}

// Stats 标签使用统计：每个标签有多少客户在使用。
func (s CrmCustomerTagService) Stats(ctx context.Context) ([]map[string]any, error) {
	var tags []models.CrmCustomerTag
	if err := models.MustFor(ctx).Order("sort asc, id asc").Find(&tags).Error; err != nil {
		return nil, err
	}
	tenantID := identity.TenantID(ctx)
	result := make([]map[string]any, 0, len(tags))
	for _, t := range tags {
		var count int64
		jsonbVal, _ := json.Marshal([]string{t.ID})
		models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).
			Where("tags @> ?::jsonb", string(jsonbVal)).Count(&count)
		result = append(result, map[string]any{
			"id":             t.ID,
			"name":           t.Name,
			"group":          t.Group,
			"color":          t.Color,
			"sort":           t.Sort,
			"enabled":        t.Enabled,
			"customer_count": count,
		})
	}
	return result, nil
}

// CrmReasonDictService 赢单/输单原因字典服务。
type CrmReasonDictService struct{}

func NewCrmReasonDict() *CrmReasonDictService { return &CrmReasonDictService{} }

// List 查询指定分类的原因列表。未配置时返回系统默认列表（不落库）。
func (s CrmReasonDictService) List(ctx context.Context, category string) ([]models.CrmReasonDict, error) {
	if category != "win" && category != "lose" {
		return nil, errors.New("category 必须为 win 或 lose")
	}
	var items []models.CrmReasonDict
	if err := models.MustFor(ctx).Where("category = ?", category).Order("sort asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return items, nil
	}
	// 兜底：返回默认列表
	defaults := DefaultWinReasons
	if category == "lose" {
		defaults = DefaultLoseReasons
	}
	for i, name := range defaults {
		items = append(items, models.CrmReasonDict{
			Category: category,
			Name:     name,
			Sort:     i,
			Enabled:  true,
		})
	}
	return items, nil
}

// Create 新建原因。
func (s CrmReasonDictService) Create(ctx context.Context, item *models.CrmReasonDict) error {
	if item == nil {
		return errors.New("原因不能为空")
	}
	if item.Category != "win" && item.Category != "lose" {
		return errors.New("category 必须为 win 或 lose")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("原因名称不能为空")
	}
	item.ID = id_generator.StringID()
	item.TenantID = identity.TenantID(ctx)
	item.Name = strings.TrimSpace(item.Name)
	return createCRM(models.DB, item)
}

// Update 更新原因。
func (s CrmReasonDictService) Update(ctx context.Context, id string, item *models.CrmReasonDict) error {
	if item == nil {
		return errors.New("原因不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("原因名称不能为空")
	}
	m := map[string]any{
		"name":    strings.TrimSpace(item.Name),
		"sort":    item.Sort,
		"enabled": item.Enabled,
	}
	return models.MustFor(ctx).Where("id = ?", id).Updates(m).Error
}

// Delete 删除原因。
func (s CrmReasonDictService) Delete(ctx context.Context, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.MustFor(ctx).Where("id IN ?", ids).Delete(&models.CrmReasonDict{})
	return res.RowsAffected, res.Error
}

// CrmIndustryDictService 行业字典服务（平台级）。
type CrmIndustryDictService struct{}

func NewCrmIndustryDict() *CrmIndustryDictService { return &CrmIndustryDictService{} }

// DefaultIndustries 系统默认行业列表（未配置时兜底返回）。
var DefaultIndustries = []string{
	"互联网/IT", "电子商务", "计算机软件", "通信/硬件", "半导体/集成电路",
	"金融", "银行", "保险", "证券/基金", "投资/创投",
	"房地产", "建筑", "物业管理", "建材",
	"制造业", "机械", "汽车", "能源/化工", "环保",
	"医疗/医药", "生物技术", "医疗器械", "大健康",
	"教育", "培训", "在线教育",
	"传媒/广告", "营销", "公关", "出版",
	"餐饮", "酒店", "旅游", "物流", "快递",
	"零售", "批发", "贸易", "进出口",
	"农业", "林业", "渔业",
	"法律", "咨询", "会计/审计", "人力资源",
	"政府/非营利", "其他",
}

// List 查询行业列表。未配置时返回系统默认列表（不落库）。
func (s CrmIndustryDictService) List(ctx context.Context) ([]models.CrmIndustryDict, error) {
	var items []models.CrmIndustryDict
	if err := models.DB.Order("sort asc, id asc").Find(&items).Error; err != nil {
		return nil, err
	}
	if len(items) > 0 {
		return items, nil
	}
	for i, name := range DefaultIndustries {
		items = append(items, models.CrmIndustryDict{
			Name:    name,
			Sort:    i,
			Enabled: true,
		})
	}
	return items, nil
}

// Create 新建行业。
func (s CrmIndustryDictService) Create(item *models.CrmIndustryDict) error {
	if item == nil {
		return errors.New("行业不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("行业名称不能为空")
	}
	item.ID = id_generator.StringID()
	item.Name = strings.TrimSpace(item.Name)
	return models.DB.Create(item).Error
}

// Update 更新行业。
func (s CrmIndustryDictService) Update(id string, item *models.CrmIndustryDict) error {
	if item == nil {
		return errors.New("行业不能为空")
	}
	if strings.TrimSpace(item.Name) == "" {
		return errors.New("行业名称不能为空")
	}
	m := map[string]any{
		"name":    strings.TrimSpace(item.Name),
		"sort":    item.Sort,
		"enabled": item.Enabled,
	}
	return models.DB.Model(&models.CrmIndustryDict{}).Where("id = ?", id).Updates(m).Error
}

// Delete 删除行业。
func (s CrmIndustryDictService) Delete(ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 不能为空")
	}
	res := models.DB.Where("id IN ?", ids).Delete(&models.CrmIndustryDict{})
	return res.RowsAffected, res.Error
}
