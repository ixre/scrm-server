# 基础管理（Base）

> 租户/员工/字典/自定义字段，构成 CRM 系统基础。

## 子模块清单

| 文档 | 内容 |
|------|------|
| [01-租户与权限.md](01-租户与权限.md) | 多租户、菜单权限、数据权限 |
| [02-角色与员工.md](02-角色与员工.md) | 角色管理、员工账号、组织架构 |
| [03-数据字典.md](03-数据字典.md) | 状态/类型/分类字典 |
| [04-自定义字段.md](04-自定义字段.md) | 客户/订单等模型字段扩展 |

## 多租户架构

所有表都带 `tenant_id`，是顶层隔离维度：

```
租户 A（公司 1）── 只能看 tenant_id=A 的数据
租户 B（公司 2）── 只能看 tenant_id=B 的数据
```

## 关键代码位置

| 模块 | 文件 |
|------|------|
| 租户/平台超管 | `models/crm.go`（Tenant/PlatformAdmin）、`services/crm.go`（TenantService） |
| CRM 原生角色 | `models/crm.go`（CrmRole）、`services/crm.go`（seedTenantDefaults） |
| 企微域角色 | `models/role.go`、`services/role.go` |
| 企微员工 | `models/staff.go`、`services/staff.go` |
| CRM 原生用户 | `models/crm.go`（User）、`services/crm.go`（UserService） |
| 部门 | `models/department.go`、`services/department.go` |
| 租户隔离 | `models/tenant_scope.go` |
| 字典 | `dispatch/crm.go`（crm_dict.AddWay，部分实现） |
| 自定义字段 | `models/crm.go`（CrmFieldDef/CrmFieldValue）、`services/crm_field.go` |
| 列表场景 | `models/crm.go`（CrmScene）、`services/crm_scene.go` |

## 相关文档
- [CRM 主文档](../README.md)