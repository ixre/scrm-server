package models

import (
	"core/app/constants"
	log "core/common/log"
	"core/conf"
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

var DB *gorm.DB // 默认数据库

type Timestamp struct {
	CreatedAt time.Time      `sql:"index" gorm:"comment:'创建时间'" json:"created_at"`
	UpdatedAt time.Time      `sql:"index" gorm:"comment:'更新时间'" json:"updated_at"`
	DeletedAt gorm.DeletedAt `sql:"index" gorm:"comment:'删除时间'" json:"deleted_at" swaggerignore:"true"`
}

type Model struct {
	ID string `gorm:"primaryKey;type:bigint;comment:'ID'" json:"id" validate:"int64"`
}

type ExtCorpModel struct {
	// ID
	ID string `json:"id" gorm:"primaryKey;type:bigint;comment:'ID'" validate:"int64"`
	// ExtCorpID 外部企业ID
	ExtCorpID string `json:"ext_corp_id" gorm:"index;type:char(18);comment:外部企业ID" validate:ext_corp_id"`
	// ExtCreatorID 创建者外部员工ID
	ExtCreatorID string `json:"ext_creator_id" gorm:"index;type:char(64);comment:创建者外部员工ID" validate:"word"`
}

// RefModel 关联表基本模型，ID仅用做唯一键，使用组合字段作为主键，方便去重，可实现Association replace保留原纪录
type RefModel struct {
	// ID
	ID string `json:"id" gorm:"unique;type:bigint;comment:'ID'" validate:"int64"`
	// ExtCorpID 外部企业ID
	ExtCorpID string `json:"ext_corp_id" gorm:"index;type:char(18);comment:外部企业ID" validate:ext_corp_id"`
}

// SetupDB Setup 用于初始化数据库连接
func SetupDB() {
	DB = initDB(conf.Settings.DB)
	SetupPermissions()     // 初始化的权限
	SetupRoles()           // 初始化默认角色
	SetupStaffRole()       // 初始化超级管理员权限
	SetupContactWayGroup() // 初始化默认渠道码分组
	SetupCustomerInfoDisplayRule()
}

// SetupSchema 只建立数据库连接并自动迁移表结构（AutoMigrate），
// 不做种子数据初始化，供迁移工具（cmd/migrate）使用，避免依赖 redis/id_generator。
func SetupSchema() {
	DB = initDB(conf.Settings.DB)
}

// AutoMigrate 用于根据结构体定义自动迁移表结构,只新增字段
func AutoMigrate(db *gorm.DB) error {
	if !conf.Settings.App.AutoMigration {
		return nil
	}

	err := db.AutoMigrate(
		// 此处顺序不可改动，以免触发外键约束
		&Customer{},
		&CustomerInfo{},
		&CustomerInfoDisplayRule{},
		&CustomerStaff{},
		&CustomerStaffRelationHistory{},
		&Staff{},
		&Department{},
		&MaterialLibTag{},
		&StaffDepartment{},
		&CustomerStaffTag{},
		&GroupChatTagGroup{},
		&CorpSetting{},
		&GroupChatTag{},
		&TagGroup{},
		&Tag{},
		&ChatMsg{},
		&ChatMsgContent{},
		&QuickReplyGroup{},
		&QuickReply{},
		&QuickReplyDetail{},
		&CustomerRemark{},
		&RemarkOption{},
		&CustomerEvent{},
		&InternalTag{},
		&Material{},
		&Remainder{},
		&MassMsg{},
		&MassMsgStaff{},
		&WelcomeMsg{},
		&EventNotify{},
		&GroupChat{},
		&GroupChatMember{},
		&GroupChatGroup{},
		&GroupChatAutoJoinCode{},
		&GroupChatQRCode{},
		&GroupChatAutoJoinCodeStaff{},
		&GroupChatWelcomeMsg{},
		&GroupChatMassMsg{},
		&CustomerStatistic{},
		&DataExport{},
		&ContactWayGroup{},
		&ContactWay{},
		&ContactWaySchedule{},
		&ContactWayScheduleStaff{},
		&ContactWayBackupStaff{},
		&ContactWayStaff{},
		&Permission{},
		&CustomerInfoDisplayRule{},
		&Tag{},
		&TagGroup{},
		&Staff{},
		&Role{},
		// CRM 原生模型（渠道无关，本地身份底座）
		&Tenant{},
		&PlatformAdmin{},
		&PlatformOperationLog{},
		&Plan{},
		&PaymentConfig{},
		&TenantOrder{},
		&User{},
		&ChannelLink{},
		&CrmCustomer{},
		&CrmLead{},
		&CrmPipelineStage{},
		&CrmOpportunity{},
		&CrmFollowup{},
		&CrmOrder{},
		&CrmRole{},
		&CrmActivity{},
		&CrmChannelInteraction{},
		&CrmChannelTag{},
		&CrmActivityRecord{},
		&CrmChannelConfig{},
		&CrmWecomEventLog{},
		// Phase 1 销售域深化
		&CrmPoolRule{},
		&CrmPoolLog{},
		&CrmSalesRule{},
		&CrmQuota{},
		&CrmTransferLog{},
		// Phase 2 小红书渠道接入
		&CrmChannelTask{},
		// Phase 3 营销活动落地
		&CrmCoupon{},
		&CrmCouponRecord{},
		&CrmFission{},
		&CrmFissionInvite{},
		&CrmLandingPage{},
		// Phase 4 自动化与智能化
		&CrmSopRule{},
		&CrmSopTask{},
		&CrmAutoTagRule{},
		&CrmSalesScript{},
		// Phase 6 营销活动深化
		&CrmSeckill{},
		&CrmSeckillRecord{},
		&CrmLottery{},
		&CrmLotteryRecord{},
		// A 组：SCRM 风控
		&CrmSensitiveWord{},
		&CrmSensitiveAlert{},
		&CrmInheritance{},
		&CrmCustomerLoss{},
		&CrmReplyTimeout{},
		&CrmGroupSop{},
		&CrmGroupSopTask{},
		// B 组：营销活动补充
		&CrmGroupBuy{},
		&CrmGroupBuyRecord{},
		&CrmBargain{},
		&CrmBargainRecord{},
		&CrmFullReduction{},
		&CrmRedPacket{},
		&CrmRedPacketRecord{},
		&CrmSignIn{},
		&CrmSignInRecord{},
		// C4 智能名片
		&CrmSmartCard{},
		&CrmSmartCardView{},
		// P1: CRM 核心补全
		&CrmContact{},
		&CrmReceivablesPlan{},
		&CrmReceivables{},
		&CrmQuote{},
		&CrmTodo{},
		&CrmOrderProduct{},
		&CrmOrderAttachment{},
		&CrmOrderProgress{},
		&CrmSignin{},
		&CrmQuickFollowup{},
		// P2: 重要增强
		&CrmFieldDef{},
		&CrmFieldValue{},
		&CrmScene{},
		&CrmApprovalFlow{},
		&CrmApprovalInstance{},
		&CrmApprovalRecord{},
		&CrmConsume{},
		// P3: 大模块扩展
		&CrmProject{},
		&CrmProjectTask{},
		&CrmAccount{},
		&CrmFinanceRecord{},
		&CrmInvoice{},
		&CrmProduct{},
		&CrmWarehouse{},
		&CrmStockRecord{},
		&CrmKnowledge{},
		&CrmNotice{},
		&CrmWorkReport{},
		&CrmWorkorder{},
		&CrmCallRecord{},
		&CrmAiAnalysis{},
		&CrmAftersales{},
		&CrmLeadPool{},
		&CrmRecycle{},
		&CrmBatchOp{},
		&CrmProjectEvent{},
		&CrmProjectFile{},
		&CrmSupplier{},
		&CrmPurchaseOrder{},
		&CrmAttendance{},
		&CrmLeave{},
		&CrmAutoAllocation{},
		&CrmSeasType{},
		&CrmCustomerSetting{},
		&CrmLeadSource{},
		&CrmReasonDict{},
		&CrmCustomerTag{},
		&CrmCareTemplate{},
		&CrmCustomerSegment{},
		&CrmIndustryDict{},
		// 财务扩展
		&CrmIncomeCategory{},
		&CrmOtherIncome{},
		&CrmOtherExpense{},
		&CrmFundTransfer{},
		// 进销存单据
		&CrmStockIn{},
		&CrmStockOut{},
		&CrmStockCheck{},
		&CrmStockTransfer{},
		// 工单扩展
		&CrmWorkorderType{},
		&CrmWorkorderPool{},
		&CrmWorkorderLog{},
		// 呼叫中心扩展
		&CrmCallList{},
		// 跟进扩展
		&CrmFollowupComment{},
		&CrmQuickRemark{},
		// 办公扩展
		&CrmKnowledgeType{},
		&CrmWorkReportComment{},
		// 售后扩展
		&CrmAftersalesType{},
		&CrmAftersalesComment{},
	)
	if err != nil {
		log.Sugar.Errorw(err.Error())
		return err
	}
	// 修改 Staff 的 Departments 字段的连接表为 StaffDepartment
	err = db.SetupJoinTable(&Staff{}, "Departments", &StaffDepartment{})
	if err != nil {
		return err
	}

	// 补充复合索引（涉及 Timestamp 嵌入字段，AutoMigrate 无法通过 struct tag 创建）
	for _, idx := range []string{
		`CREATE INDEX IF NOT EXISTS idx_channel_task_status_created ON crm_channel_task (status, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_reply_timeout_status_created ON crm_reply_timeout (status, created_at)`,
	} {
		if e := db.Exec(idx).Error; e != nil {
			log.Sugar.Warnw("create composite index failed", "sql", idx, "err", e.Error())
		}
	}

	return nil
}

// initDB 初始化数据库连接
func initDB(c conf.DBConfig) (db *gorm.DB) {
	var err error

	gormLogLevel := logger.Warn
	if conf.Settings.App.Env == constants.DEV {
		gormLogLevel = logger.Info
	}

	sslmode := c.SSLMode
	if sslmode == "" {
		sslmode = "disable"
	}
	tz := c.TimeZone
	if tz == "" {
		tz = "Asia/Shanghai"
	}
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s TimeZone=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, sslmode, tz)
	db, err = gorm.Open(
		postgres.Open(dsn),
		&gorm.Config{
			SkipDefaultTransaction:                   false,
			DisableForeignKeyConstraintWhenMigrating: true,
			Logger:                                   logger.Default.LogMode(gormLogLevel),
			NamingStrategy: schema.NamingStrategy{
				SingularTable: true, // use singular table name, table for `User` would be `user` with this option enabled
			}},
	)
	if err != nil {
		log.Sugar.Errorw("models.Setup failed", "err", err, "conf", c)
		os.Exit(1)
	}

	err = AutoMigrate(db)
	if err != nil {
		log.Sugar.Errorw("model auto migrate failed", "err", err, "conf", c)
		os.Exit(1)
	}

	return db
}
