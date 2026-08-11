package migrate

import (
	"core/app/models"
	"core/common/id_generator"
	"core/common/log"

	"gorm.io/gorm"
)

// BackfillCRM 把存量企微数据回填进 CRM 原生表（幂等，可重复执行）。
// 映射：
//   - 企微 corp_id  → Tenant（本地租户）
//   - 企微 staff    → User（本地用户）+ ChannelLink(user, wecom, ext_staff_id)
//   - 企微 customer → CrmCustomer（source=wecom）+ ChannelLink(customer, wecom, ext_customer_id)
//
// 幂等依据：以 ChannelLink(tenant, channel=wecom, entity_type, ext_id) 是否存在判断。
// 企微存量表不动，CRM 核心通过 ChannelLink 反向引用，SCRM 渠道照常工作。
func BackfillCRM(db *gorm.DB) error {
	// 1) corp_id → Tenant
	corps, err := distinctCorps(db)
	if err != nil {
		return err
	}
	tenantByCorp := map[string]string{}
	for _, corpID := range corps {
		t, created, err := ensureTenant(db, corpID)
		if err != nil {
			return err
		}
		tenantByCorp[corpID] = t.ID
		if created {
			log.Sugar.Infow("backfill: created tenant", "corp_id", corpID, "tenant_id", t.ID)
		}
	}

	// 2) staff → User + ChannelLink(user)
	var staffs []models.Staff
	if err := db.Find(&staffs).Error; err != nil {
		return err
	}
	userByStaff := map[string]string{} // ext_staff_id -> user_id
	for _, st := range staffs {
		tenantID, ok := tenantByCorp[st.ExtCorpID]
		if !ok {
			continue
		}
		u, created, err := ensureUser(db, tenantID, st)
		if err != nil {
			return err
		}
		userByStaff[st.ExtID] = u.ID
		if created {
			log.Sugar.Infow("backfill: created user", "staff_id", st.ExtID, "user_id", u.ID)
		}
	}

	// 3) customer → CrmCustomer + ChannelLink(customer)
	var customers []models.Customer
	if err := db.Find(&customers).Error; err != nil {
		return err
	}
	for _, cu := range customers {
		tenantID, ok := tenantByCorp[cu.ExtCorpID]
		if !ok {
			continue
		}
		ownerUserID := ownerForCustomer(db, cu.ExtID, userByStaff)
		c, created, err := ensureCustomer(db, tenantID, cu, ownerUserID)
		if err != nil {
			return err
		}
		if created {
			log.Sugar.Infow("backfill: created crm customer", "ext_customer_id", cu.ExtID, "customer_id", c.ID)
		}
	}
	return nil
}

// distinctCorps 从 staff 表收集所有 distinct ext_corp_id。
func distinctCorps(db *gorm.DB) ([]string, error) {
	var corps []string
	err := db.Model(&models.Staff{}).Distinct().Pluck("ext_corp_id", &corps).Error
	return corps, err
}

// ensureTenant 创建（或复用）一个 corp_id 对应的租户，并写 ChannelLink(tenant, wecom, corp)。
func ensureTenant(db *gorm.DB, corpID string) (models.Tenant, bool, error) {
	var link models.ChannelLink
	err := db.Where("channel = ? AND entity_type = ? AND ext_id = ?", "wecom", "tenant", corpID).
		First(&link).Error
	if err == nil {
		var t models.Tenant
		if err := db.First(&t, "id = ?", link.EntityID).Error; err != nil {
			return t, false, err
		}
		return t, false, nil
	} else if err != gorm.ErrRecordNotFound {
		return models.Tenant{}, false, err
	}
	t := models.Tenant{
		Model:  models.Model{ID: id_generator.StringID()},
		Name:   "企业-" + corpID,
		Code:   corpID,
		Status: 1,
	}
	return t, true, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&t).Error; err != nil {
			return err
		}
		return tx.Create(&models.ChannelLink{
			Model:    models.Model{ID: id_generator.StringID()},
			TenantID: t.ID,
			Channel:  "wecom", EntityType: "tenant", EntityID: t.ID,
			ExtID: corpID,
		}).Error
	})
}

// ensureUser 创建（或复用）企微 staff 对应的本地 User，并写 ChannelLink(user, wecom, ext_staff_id)。
func ensureUser(db *gorm.DB, tenantID string, st models.Staff) (models.User, bool, error) {
	var link models.ChannelLink
	err := db.Where("channel = ? AND entity_type = ? AND ext_id = ?", "wecom", "user", st.ExtID).
		First(&link).Error
	if err == nil {
		var u models.User
		if err := db.First(&u, "id = ?", link.EntityID).Error; err != nil {
			return u, false, err
		}
		return u, false, nil
	} else if err != gorm.ErrRecordNotFound {
		return models.User{}, false, err
	}
	u := models.User{
		Model:    models.Model{ID: id_generator.StringID()},
		TenantID: tenantID,
		Username: st.ExtID,
		Name:     st.Name,
		Mobile:   st.Mobile,
		RoleID:   st.RoleID,
		Status:   1,
	}
	newLink := models.ChannelLink{
		Model:    models.Model{ID: id_generator.StringID()},
		TenantID: tenantID,
		Channel:  "wecom", EntityType: "user", EntityID: u.ID,
		ExtID: st.ExtID, ExtCorpID: st.ExtCorpID,
	}
	return u, true, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("role_id").Create(&u).Error; err != nil {
			return err
		}
		return tx.Create(&newLink).Error
	})
}

// ensureCustomer 创建（或复用）企微 customer 对应的本地 CrmCustomer，并写 ChannelLink。
func ensureCustomer(db *gorm.DB, tenantID string, cu models.Customer, ownerUserID string) (models.CrmCustomer, bool, error) {
	var link models.ChannelLink
	err := db.Where("channel = ? AND entity_type = ? AND ext_id = ?", "wecom", "customer", cu.ExtID).
		First(&link).Error
	if err == nil {
		var c models.CrmCustomer
		if err := db.First(&c, "id = ?", link.EntityID).Error; err != nil {
			return c, false, err
		}
		return c, false, nil
	} else if err != gorm.ErrRecordNotFound {
		return models.CrmCustomer{}, false, err
	}
	c := models.CrmCustomer{
		Model:         models.Model{ID: id_generator.StringID()},
		TenantID:      tenantID,
		Name:          cu.Name,
		Company:       cu.CorpName,
		Gender:        cu.Gender,
		Source:        "wecom",
		OwnerUserID:   ownerUserID,
		ExtCustomerID: cu.ExtID,
		ExtCorpID:     cu.ExtCorpID,
		AccountType:   wecomAccountType(cu.Type),
	}
	// 添加方式与企微标签：取客户第一条员工关系（历史数据通常同源同方式）。
	var cs models.CustomerStaff
	if err := db.Where("ext_customer_id = ?", cu.ExtID).First(&cs).Error; err == nil {
		c.AddWay = int(cs.AddWay)
		var staffTags []models.CustomerStaffTag
		if err := db.Where("customer_staff_id = ?", cs.ID).Find(&staffTags).Error; err == nil {
			seen := map[string]bool{}
			for _, t := range staffTags {
				if t.TagName != "" && !seen[t.TagName] {
					seen[t.TagName] = true
					c.ChannelTags = append(c.ChannelTags, t.TagName)
				}
			}
		}
	}
	newLink := models.ChannelLink{
		Model:    models.Model{ID: id_generator.StringID()},
		TenantID: tenantID,
		Channel:  "wecom", EntityType: "customer", EntityID: c.ID,
		ExtID: cu.ExtID, ExtCorpID: cu.ExtCorpID,
	}
	return c, true, db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Omit("owner_user_id").Create(&c).Error; err != nil {
			return err
		}
		return tx.Create(&newLink).Error
	})
}

// ownerForCustomer 通过客户-员工关系找到该客户的第一个所属员工对应的本地 user_id。
func ownerForCustomer(db *gorm.DB, extCustomerID string, userByStaff map[string]string) string {
	var rel models.CustomerStaff
	if err := db.Where("ext_customer_id = ?", extCustomerID).First(&rel).Error; err != nil {
		return ""
	}
	return userByStaff[rel.ExtStaffID]
}

// wecomAccountType 企微客户类型(1-微信用户 2-企业微信用户) → 账号类型展示名。
func wecomAccountType(t int) string {
	switch t {
	case 1:
		return "微信"
	case 2:
		return "企业微信"
	}
	return ""
}
