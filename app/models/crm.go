package models

import (
	"core/app/constants"
	"time"
)

// CRM 原生模型 —— 渠道无关的客户/员工/销售管线底座。
//
// 与企微模型的区别：主键均为本地 ID（id_generator 生成的 bigint），
// 顶层分区是 TenantID（本地租户），而不是 ExtCorpID（企微 corp_id）。
// 企微 external id 一律落到 ChannelLink 映射，不作为主键。
// 这样"去掉企微也能用"：CRM 核心不依赖任何企微字段。

// Tenant 企业/组织（本地租户），取代 corp_id 作为顶层数据分区。
type Tenant struct {
	Model
	Timestamp
	Name         string     `gorm:"type:varchar(128);comment:企业/组织名称" json:"name"`
	Code         string     `gorm:"type:varchar(64);uniqueIndex;comment:租户编码" json:"code"`
	Status       int        `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
	Remark       string     `gorm:"type:varchar(512);comment:备注" json:"remark"`
	Plan         string     `gorm:"type:varchar(32);default:free;comment:套餐 free/standard/pro/enterprise" json:"plan"`
	ExpireAt     *time.Time `gorm:"comment:套餐到期时间(NULL=永久)" json:"expire_at"`
	ContactName  string     `gorm:"type:varchar(64);comment:联系人姓名" json:"contact_name"`
	ContactPhone string     `gorm:"type:varchar(32);comment:联系人手机" json:"contact_phone"`
	ContactEmail string     `gorm:"type:varchar(128);comment:联系人邮箱" json:"contact_email"`
	Industry     string     `gorm:"type:varchar(64);comment:所属行业" json:"industry"`
	Logo         string     `gorm:"type:varchar(255);comment:企业Logo URL" json:"logo"`
	MaxUsers     int        `gorm:"type:int;default:0;comment:最大用户数(0=不限)" json:"max_users"`
	MaxCustomers int        `gorm:"type:int;default:0;comment:最大客户数(0=不限)" json:"max_customers"`
}

// PlatformAdmin 平台超管（独立于租户，管理所有租户）。
// 与租户内 User 解耦：不归属任何 tenant，专用登录入口，JWT 标记 is_platform。
type PlatformAdmin struct {
	Model
	Timestamp
	Username  string     `gorm:"type:varchar(64);uniqueIndex;comment:登录名(全局唯一)" json:"username"`
	Password  string     `gorm:"type:varchar(256);comment:登录密码(哈希)" json:"-"`
	Name      string     `gorm:"type:varchar(128);comment:显示名" json:"name"`
	Avatar    string     `gorm:"type:varchar(512);comment:头像URL" json:"avatar"`
	Mobile    string     `gorm:"type:varchar(32);comment:手机号" json:"mobile"`
	Email     string     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	Role      string     `gorm:"type:varchar(32);default:super_admin;comment:平台角色 super_admin/operator/finance/support" json:"role"`
	Status    int        `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
	LastLogin *time.Time `gorm:"comment:最后登录时间" json:"last_login_at"`
}

// PlatformOperationLog 平台操作日志（记录平台超管对租户/套餐/支付等资源的操作）。
type PlatformOperationLog struct {
	Model
	Timestamp
	AdminID    string `gorm:"type:bigint;index;comment:操作人(PlatformAdmin.ID)" json:"admin_id"`
	AdminName  string `gorm:"type:varchar(128);comment:操作人姓名" json:"admin_name"`
	AdminRole  string `gorm:"type:varchar(32);comment:操作人角色" json:"admin_role"`
	Action     string `gorm:"type:varchar(64);index;comment:操作类型(如 tenant.create/plan.delete)" json:"action"`
	Resource   string `gorm:"type:varchar(32);comment:资源类型(tenant/plan/payment/order/admin)" json:"resource"`
	ResourceID string `gorm:"type:varchar(64);comment:资源ID" json:"resource_id"`
	Detail     string `gorm:"type:text;comment:操作详情(JSON)" json:"detail"`
	IP         string `gorm:"type:varchar(64);comment:操作IP" json:"ip"`
}

// User 本地用户（销售/员工账号），取代企微 staff 作为鉴权与归属单元。
type User struct {
	Model
	Timestamp
	TenantID     string     `gorm:"type:bigint;index;uniqueIndex:idx_tenant_username,priority:1,where:deleted_at IS NULL;comment:归属租户ID" json:"tenant_id"`
	Username     string     `gorm:"type:varchar(64);index;uniqueIndex:idx_tenant_username,priority:2,where:deleted_at IS NULL;comment:登录名(租户内唯一)" json:"username"`
	Password     string     `gorm:"type:varchar(256);comment:登录密码(哈希)" json:"-"`
	Name         string     `gorm:"type:varchar(128);comment:显示名" json:"name"`
	Mobile       string     `gorm:"type:varchar(32);index;comment:手机号" json:"mobile"`
	Email        string     `gorm:"type:varchar(128);comment:邮箱" json:"email"`
	RoleID       string     `gorm:"type:bigint;index;comment:角色ID" json:"role_id"`
	Position     string     `gorm:"type:varchar(64);comment:职位" json:"position"`
	DepartmentID string     `gorm:"type:bigint;index;comment:部门ID" json:"department_id"`
	Avatar       string     `gorm:"type:varchar(255);comment:头像" json:"avatar"`
	LastLoginAt  *time.Time `gorm:"comment:最后登录时间" json:"last_login_at"`
	Status       int        `gorm:"type:smallint;default:1;comment:1-启用 2-停用" json:"status"`
}

// ChannelLink 渠道映射：把本地实体(租户/用户/客户)与外部渠道 ID(如企微 ext id)关联。
// 这是企微 external id 从"主键"降级为"渠道外部键"的载体。
type ChannelLink struct {
	Model
	Timestamp
	TenantID   string `gorm:"type:bigint;index;comment:本地租户ID" json:"tenant_id"`
	Channel    string `gorm:"type:varchar(32);index;uniqueIndex:idx_channel_ext,priority:1;comment:渠道标识,如 wecom" json:"channel"`
	EntityType string `gorm:"type:varchar(32);index;comment:实体类型 tenant/user/customer" json:"entity_type"`
	EntityID   string `gorm:"type:bigint;index;comment:本地实体主键" json:"entity_id"`
	ExtID      string `gorm:"type:varchar(128);index;uniqueIndex:idx_channel_ext,priority:3;comment:渠道外部ID(如企微 external_userid)" json:"ext_id"`
	ExtCorpID  string `gorm:"type:varchar(128);index;uniqueIndex:idx_channel_ext,priority:2;comment:渠道外部企业ID(如企微 corp_id)" json:"ext_corp_id"`
}

// CrmRole 角色（CRM 原生权限模型底座，tenant 分区）。
// 与企微 role 无关：本地 User.role_id 引用本表，支撑租户内权限校验（如管理员重置密码）。
type CrmRole struct {
	Model
	Timestamp
	TenantID  string                     `gorm:"type:bigint;index;uniqueIndex:idx_tenant_role_code,priority:1;comment:归属租户ID" json:"tenant_id"`
	Code      string                     `gorm:"type:varchar(64);uniqueIndex:idx_tenant_role_code,priority:2;comment:角色编码 admin/manager/sales" json:"code"`
	Name      string                     `gorm:"type:varchar(128);comment:角色名称" json:"name"`
	Perms     constants.StringArrayField `gorm:"type:jsonb;comment:权限点编码数组" json:"perms"`
	IsDefault bool                       `gorm:"comment:是否默认角色(新用户自动分配)" json:"is_default"`
}
