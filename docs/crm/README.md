# CRM 业务操作流程文档

> 面向"实际使用本系统完成日常工作"的角色（销售经理 / 销售 / 运营 / 客服 / 老板 / 管理员）。
> 每篇文档以"目的 → 入口 → 数据模型 → 操作步骤 → 关联模块 → 代码位置 → 注意事项"为骨架。

## 模块索引

| 模块 | 文档 | 涉及模型 / 服务 |
|------|------|---------------|
| 客户管理 | [customer/](customer/) | CrmCustomer / CrmContact / CrmFollowup / CrmPool / CrmAutoTag / CrmDedup / CrmCustomerLoss / CrmScore / CrmInheritance |
| 销售管线 | [sales/](sales/) | CrmLead / CrmOpportunity / CrmQuote / CrmOrder / CrmReceivables / CrmWorkorder / CrmAftersales / CrmQuota |
| 营销活动 | [marketing/](marketing/) | 11 类活动 + CrmSop / CrmGroupSop / CrmSmartCard / CrmKnowledge |
| 多渠道 | [channel/](channel/) | 企微 / 小红书 / 公众号 / 抖音 / 跨渠道任务 |
| 基础数据 | [base/](base/) | Tenant / User / Role / CrmFieldDef / CrmScene |
| 办公协同 | [office/](office/) | CrmApproval / CrmAttendance / CrmLeave / CrmWorkReport / CrmNotice / CrmTodo |
| 财务与库存 | [finance/](finance/) | CrmProduct / CrmWarehouse / CrmStockIn / CrmStockOut / CrmFinanceRecord / CrmInvoice |
| 风控与合规 | [risk/](risk/) | CrmSensitiveWord / CrmInheritance / CrmRecycle |
| 报表仪表盘 | [dashboard/](dashboard/) | CrmDashboard / CrmReport / CrmStatistics |

## 业务架构图

```
                            ┌──────────────────┐
                            │  多渠道获客入口   │
                            │ 渠道活码/SOP/活动  │
                            └────────┬─────────┘
                                     ▼
┌──────────────┐    ┌─────────────────────────┐    ┌──────────────────┐
│  客户主档    │◄──│  客户 360（聚合层）      │──►│  跟进/沟通记录   │
│ CrmCustomer  │    │  - 跟进/订单/工单/互动  │    │  CrmFollowup     │
└──────┬───────┘    └─────────────────────────┘    └──────────────────┘
       │
       ├────► 客户池（CrmPool）
       ├────► 客户评分（CrmScore）
       ├────► 流失告警（CrmCustomerLoss）
       ├────► 离职继承（CrmInheritance）
       └────► 黑名单（CrmCustomer.IsBlacklist）
                │
                ▼
┌────────────────────────────────────────────────┐
│            销售管线（销售漏斗）                  │
│  Lead → 线索池 → 商机 → 阶段 → 报价 → 订单 → 回款  │
│  CrmLead → CrmOpportunity → CrmQuote → CrmOrder  │
│                  → CrmReceivables                │
└────────────────────────────────────────────────┘
                │
                ├────► 工单/售后（CrmWorkorder/CrmAftersales）
                ├────► 进销存（CrmProduct/CrmStock*）
                └────► 财务（CrmFinanceRecord/CrmInvoice）

┌────────────────────────────────────────────────┐
│            营销活动（11 类 + SOP）               │
│  优惠券/秒杀/抽奖/拼团/砍价/满减/红包/签到/裂变   │
│  /H5/智能名片 + CrmSop + CrmGroupSop + 话术库     │
└────────────────────────────────────────────────┘
```

## 设计哲学（贯穿所有流程）

1. **渠道无关优先**：所有客户/订单/活动数据通过 `ChannelLink` 解耦，企微 corp_id 降级为维度字段。
2. **多租户 SaaS**：所有 CRM 表 `tenant_id` 顶层分区，业务操作必带 `tenant_id` 隔离。
3. **本地 ID 主键**：CRM 模型主键均为本地 bigint，不依赖企微 external_userid。
4. **异步回调驱动**：企微 / 小红书事件以 webhook 为准同步进 CRM 主档。
5. **闭源核心 + 开放网关**：业务在 `scrm-server`（gRPC :9001），HTTP 在 `gateway` (:9002)，新业务通过 `dispatch` 注册表扩展。

## 角色速查

| 角色 | 主要使用模块 |
|------|-------------|
| 销售 | 客户 / 跟进 / 商机 / 订单 / 回款 / 智能名片 / 话术 |
| 销售经理 | 上述 + 销售漏斗 / 业绩 / 客户分配 / 工单分配 |
| 运营 | 营销活动 / SOP / 智能名片 / 公告 / 知识库 |
| 客服 | 工单 / 售后 / 敏感词 / 跨渠道聚合 |
| 老板 / 财务 | 仪表盘 / 财务记录 / 发票 / 回款 / 进销存 |
| 管理员 | 角色 / 员工 / 渠道配置 / 自定义字段 / 字典 |

## 调用约定

所有业务接口走 `dispatch` 注册表，HTTP 层在 `gateway`。约定调用形式：

```
POST /api/v1/biz/crm_customer/Create
Headers: Authorization: Bearer <JWT>
Body: {
  "name": "示例客户",
  "mobile": "13800000000"
}
```

返回：

```json
{
  "code": 0,
  "data": { ... }
}
```

完整接口清单参见 `internal/dispatch/crm.go` 的所有 `Reg.Register("crm_*", ...) `。
