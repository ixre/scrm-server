# 销售管线（Sales Pipeline）

> 销售漏斗全流程：线索 → 商机 → 报价 → 订单 → 回款。附加：工单 / 售后 / 业绩目标。

## 子模块清单

| 文档 | 内容 |
|------|------|
| [01-线索.md](01-线索.md) | 线索录入、分配、池、转移、转客户 |
| [02-商机与管线.md](02-商机与管线.md) | 销售阶段定义、商机推进、赢/输 |
| [03-报价.md](03-报价.md) | 报价单生命周期、转订单 |
| [04-订单.md](04-订单.md) | 合同/订单、回款计划、进度节点 |
| [05-回款.md](05-回款.md) | 回款登记、计划、逾期标记 |
| [06-工单.md](06-工单.md) | 工单类型、池、分配、SLA |
| [07-售后.md](07-售后.md) | 售后类型、评论、评价 |

## 销售漏斗流程图

```
                获客渠道                    销售过程                  交付售后
    ┌──────────────────────────┐    ┌─────────────────────┐    ┌──────────────────┐
    │  线索（Lead）              │    │  商机（Opportunity） │    │  订单（Order）    │
    │  - 留资                   │    │  - 阶段推进          │    │  - 签约          │
    │  - 手动录入               │ ─► │  - 赢率更新          │ ─► │  - 进度          │
    │  - 活动参与               │    │  - 预计成交          │    │  - 回款计划      │
    └──────────────────────────┘    └──────────┬──────────┘    └────────┬─────────┘
                                                │                       │
                                                ▼                       ▼
                                       ┌─────────────────────┐    ┌──────────────────┐
                                       │  报价（Quote）        │    │  回款（Receivables）│
                                       │  - 价格              │    │  - 实际收款      │
                                       │  - 有效期            │ ─► │  - 逾期标记      │
                                       └─────────────────────┘    └──────────────────┘
                                                                          │
                                                                          ▼
                                                                ┌──────────────────┐
                                                                │  售后（Aftersales）│
                                                                │  + 工单（Workorder）│
                                                                └──────────────────┘
```

## 关键模型

| 模型 | 路径 | 说明 |
|------|------|------|
| CrmLead | `models/crm.go:104-125` | 线索 |
| CrmPipelineStage | `models/crm.go:128-137` | 销售阶段 |
| CrmOpportunity | `models/crm.go:140-162` | 商机 |
| CrmQuote | `models/crm.go:936-` | 报价单 |
| CrmOrder | `models/crm.go:183-214` | 合同/订单 |
| CrmOrderProduct | `models/crm.go:1015-` | 订单商品明细 |
| CrmOrderAttachment | `models/crm.go:` | 订单附件 |
| CrmOrderProgress | `models/crm.go:1004-` | 订单进度节点 |
| CrmReceivables | `models/crm.go:912-` | 回款登记 |
| CrmReceivablesPlan | `models/crm.go:897-` | 回款计划 |
| CrmWorkorder | `models/crm.go:1327-` | 工单 |
| CrmAftersales | `models/crm.go:1370-` | 售后 |
| CrmQuota | `models/crm.go:357-` | 销售业绩目标 |

## 默认阶段种子

`CrmPipelineStageSeed` 创建租户时自动播种 6 个阶段：

| 排序 | 名称 | 成交概率 |
|------|------|---------|
| 1 | 初步接洽 | 10% |
| 2 | 需求确认 | 25% |
| 3 | 方案报价 | 50% |
| 4 | 商务谈判 | 70% |
| 5 | 合同签订 | 90% |
| 6 | 已成交 | 100% |

## 关键操作入口

| 角色 | 入口 |
|------|------|
| 销售 | 工作台 → 我的线索/商机/订单 |
| 销售经理 | 工作台 → 销售漏斗 / 团队管线 |
| 财务 | 工作台 → 回款管理 / 发票管理 |
| 客服 | 工作台 → 工单 / 售后 |

## 业绩目标（`CrmQuota`）

### 创建目标

**接口**：`crm_quota.Set`

```json
{
  "user_id": "201",
  "year_month": "2026-08",
  "target_amount": 100000.00
}
```

### 业绩达成

**接口**：`crm_quota.Achievement`

返回个人目标 vs 实际回款。

### 团队业绩

**接口**：`crm_quota.TeamAchievement(tenantID, yearMonth)`

返回团队所有成员的达成情况。

### 计算方式

`actualAmount = SUM(CrmReceivables.amount WHERE user_id = ? AND created_at BETWEEN month_start AND month_end)`

## 销售规则（`CrmSalesRule`）

### 销售阶段推进规则
- 当前实现：`移动阶段前 N 天内需有跟进`（`crm_sales_rule.go:74`）

**接口**：`crm_sales_rule.CreateRule / QueryRules`

**支持的规则类型**（当前 1 种，建议扩展）：
- 跟进强制校验
- 商机阶段跳过禁止
- 输单原因必填
- 自动分配策略

### 自动分配规则（`CrmAutoAllocation`）
**接口**：`crm_auto_allocation.Create / Update / Delete / Query`

仅 CRUD，无触发执行器（**产品补强点**）。

## 相关文档
- [dashboard/02-销售漏斗.md](../dashboard/02-销售漏斗.md)
- [dashboard/04-业绩达成.md](../dashboard/04-业绩达成.md)