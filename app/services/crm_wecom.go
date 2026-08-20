package services

import (
	"core/common/identity"
	
	"context"
	
	"errors"

	"core/app/constants"
	"core/app/models"
	"core/common/app"
	"core/common/id_generator"
	"core/common/log"
	"core/common/we_work"

	"gorm.io/gorm"
)

// ---------- 企微 → CRM 同步（回调驱动） ----------

// SyncFromWeCom 企微外部联系人事件落 CRM：按 (corp, ext_customer) 幂等 upsert
// CrmCustomer(source=wecom) + ChannelLink，并把跟进员工(企微 staff)映射到本地 User 作为 owner。
// 回调不带客户名，name 为空时用 EnrichFromWeCom 走 API 补名（失败仅 log 不阻塞）。
func (s CrmCustomerService) SyncFromWeCom(ctx context.Context, corpID, staffExtID, customerExtID, name string) error {
	tenantID := identity.TenantID(ctx)


	if customerExtID == "" {
		return errors.New("external_user_id 不能为空")
	}
	var c models.CrmCustomer
	err := models.MustFor(ctx).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?",  constants.ChannelWecom,  corpID,  customerExtID).First(&c).Error
	switch {
	case err == nil:
		// 已存在：更新名称并复位流失态
		if name != "" || c.Name == "" {
			if uerr := models.DB.Model(&models.CrmCustomer{}).Where("id = ?", c.ID).
				Updates(map[string]any{"name": name, "out_flow_status": 0}).Error; uerr != nil {
				log.Sugar.Errorw("wecom customer update failed", "err", uerr)
			}
		}
	case errors.Is(err, gorm.ErrRecordNotFound):
		c = models.CrmCustomer{
			Model:         models.Model{ID: id_generator.StringID()},
			TenantID:      tenantID,
			Name:          name,
			Source:        constants.ChannelWecom,
			ExtCorpID:     corpID,
			ExtCustomerID: customerExtID,
			AccountType:   "微信",
			OutFlowStatus: 0,
		}
		if cerr := createCRM(models.DB, &c, "OwnerUserID"); cerr != nil {
			return cerr
		}
	default:
		return err
	}

	// 归属：企微 staff → 本地 User（若已建映射）
	if ownerID, ok := resolveWecomStaffUserID(ctx, staffExtID); ok {
		if oerr := models.DB.Model(&models.CrmCustomer{}).Where("id = ?", c.ID).Update("owner_user_id", ownerID).Error; oerr != nil {
			log.Sugar.Errorw("wecom customer owner assign failed", "err", oerr)
		}
	}

	if lerr := upsertCustomerChannelLink(ctx, c); lerr != nil {
		log.Sugar.Errorw("sync wecom customer channel link failed", "corp_id", corpID, "err", lerr)
	}
	if name == "" {
		// 回调不带客户名，走 API 补名（失败仅记日志，不阻塞事件处理）
		if perr := s.EnrichFromWeCom(ctx, corpID, customerExtID); perr != nil {
			log.Sugar.Warnw("wecom customer enrich failed", "corp_id", corpID, "ext_customer", customerExtID, "err", perr)
		}
	}
	return nil
}

// EnrichFromWeCom 用企微 API 拉取外部联系人详情补全 CrmCustomer（name/gender/company/position）。
// 幂等：仅覆盖非空字段。客户端未注册或 API 失败时返回错误（调用方仅 log）。
func (s CrmCustomerService) EnrichFromWeCom(ctx context.Context, corpID, customerExtID string) error {
	tenantID := identity.TenantID(ctx)


	if customerExtID == "" {
		return nil
	}
	if we_work.Clients == nil {
		return errors.New("wecom clients 未初始化")
	}
	client, err := we_work.Clients.Get(corpID)
	if err != nil {
		return err
	}
	if client.Customer == nil {
		return errors.New("corp 客户联系客户端未初始化")
	}
	info, err := client.Customer.GetExternalContact(customerExtID)
	if err != nil {
		return err
	}
	ec := info.ExternalContact
	updates := buildWecomEnrichUpdates(ec.Name, int(ec.Gender), ec.CorpName, ec.Position)
	if len(updates) == 0 {
		return nil
	}
	return models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?",  constants.ChannelWecom,  corpID,  customerExtID).
		Updates(updates).Error
}

// buildWecomEnrichUpdates 组装补全字段（纯逻辑，可单测）：仅采纳非空/有效值，保证幂等——
// 空字段不覆盖，已有值不被清空。
func buildWecomEnrichUpdates(name string, gender int, corpName, position string) map[string]any {
	updates := map[string]any{}
	if name != "" {
		updates["name"] = name
	}
	if gender > 0 {
		updates["gender"] = gender
	}
	if corpName != "" {
		updates["company"] = corpName
	}
	if position != "" {
		updates["position"] = position
	}
	return updates
}

// MarkWecomCustomerLost 企微删除外部联系人/删除跟进成员 → 置流失，不硬删（保留主档与活动史）。
func (s CrmCustomerService) MarkWecomCustomerLost(ctx context.Context, corpID, customerExtID string) error {
	tenantID := identity.TenantID(ctx)


	if customerExtID == "" {
		return nil
	}
	return models.DB.Model(&models.CrmCustomer{}).Scopes(models.TenantScope(tenantID)).Where("source = ? AND ext_corp_id = ? AND ext_customer_id = ?",  constants.ChannelWecom,  corpID,  customerExtID).
		Update("out_flow_status", 1).Error
}

// resolveWecomStaffUserID 由企微 staff_ext_id 解析本地 User（经 ChannelLink entity_type=user）。
func resolveWecomStaffUserID(ctx context.Context, staffExtID string) (string, bool) {
	if staffExtID == "" {
		return "", false
	}
	var link models.ChannelLink
	if err := models.MustFor(ctx).Where("channel = ? AND entity_type = ? AND ext_id = ?", constants.ChannelWecom,  "user",  staffExtID).First(&link).Error; err != nil {
		return "", false
	}
	return link.EntityID, true
}

// EnsureWecomLink 企微员工事件：若租户内存在 username=staff_ext_id 的本地用户，则建立
// ChannelLink(entity_type=user)。不自动开户——账户由管理员在用户管理创建后，企微员工以 username 对齐。
func (s UserService) EnsureWecomLink(ctx context.Context, corpID, staffExtID string) error {
	tenantID := identity.TenantID(ctx)


	if staffExtID == "" {
		return nil
	}
	var u models.User
	if err := models.MustFor(ctx).Where("username = ?",  staffExtID).First(&u).Error; err != nil {
		return nil
	}
	var link models.ChannelLink
	err := models.DB.Where("channel = ? AND entity_type = ? AND entity_id = ?", constants.ChannelWecom, "user", u.ID).First(&link).Error
	if err == nil {
		return models.DB.Model(&models.ChannelLink{}).Where("id = ?", link.ID).
			Updates(map[string]any{"ext_id": staffExtID, "ext_corp_id": corpID}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return models.DB.Create(&models.ChannelLink{
		Model:      models.Model{ID: id_generator.StringID()},
		TenantID:   tenantID,
		Channel:    constants.ChannelWecom,
		EntityType: "user",
		EntityID:   u.ID,
		ExtID:      staffExtID,
		ExtCorpID:  corpID,
	}).Error
}

// ---------- 企微回调事件日志 ----------

type CrmWecomEventLogService struct{}

func NewCrmWecomEventLog() *CrmWecomEventLogService { return &CrmWecomEventLogService{} }

// Record 记录一条回调事件（best-effort：写失败仅 log，不阻塞回调主流程）。
func (s CrmWecomEventLogService) Record(ctx context.Context, corpID, event, changeType, extUserID, extCustomerID string, handled bool, errMsg string) {
	tenantID := identity.TenantID(ctx)


	entry := models.CrmWecomEventLog{
		Model:         models.Model{ID: id_generator.StringID()},
		TenantID:      tenantID,
		CorpID:        corpID,
		Event:         event,
		ChangeType:    changeType,
		ExtUserID:     extUserID,
		ExtCustomerID: extCustomerID,
		Handled:       handled,
		Error:         errMsg,
	}
	if derr := models.DB.Create(&entry).Error; derr != nil {
		log.Sugar.Errorw("record wecom event log failed", "corp_id", corpID, "event", event, "err", derr)
	}
}

// Query 按租户分页查事件日志，可按 corp_id 过滤（排障用）。
func (s CrmWecomEventLogService) Query(ctx context.Context, corpID string, pager *app.Pager) ([]models.CrmWecomEventLog, int64, error) {


	if pager == nil {
		pager = &app.Pager{}
	}
	pager.SetDefault()
	db := models.MustFor(ctx)
	if corpID != "" {
		db = db.Where("corp_id = ?", corpID)
	}
	var total int64
	if err := db.Model(&models.CrmWecomEventLog{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []models.CrmWecomEventLog
	err := db.Offset(pager.GetOffset()).Limit(pager.GetLimit()).Order("id desc").Find(&items).Error
	return items, total, err
}
