# SOP 自动化

## 目的
基于客户行为或状态变化，自动触发跟进/打标/发消息等操作。

## 入口
- 客户管理 → 客户运营 → SOP自动化
- 规则配置 + 执行日志

## 数据模型

`CrmSopRule`（`models/crm.go:473-486`）：

| 字段 | 说明 |
|------|------|
| `name` | SOP 名称 |
| `trigger_type` | 触发类型 |
| `trigger_config` | 触发配置 JSON |
| `actions` | 执行动作 JSON 数组 |
| `status` | active / disabled |

## 触发类型

当前已实现 `stage_change` 和 `tag_added` 两种触发匹配。

`models/crm.go:512-` + `services/crm_sop.go:matchTriggerCond`：

| trigger_type | 含义 | 触发时机 |
|--------------|------|----------|
| `stage_change` | 商机阶段变更 | Opportunity.ChangeStage 时 |
| `tag_added` | 添加标签 | CrmCustomerTag.Create 时 |

## 操作步骤

### 1. 创建 SOP 规则

**接口**：`crm_sop.Create`

**示例 1：新客户自动发��迎语**

```json
{
  "name": "新客户欢迎语",
  "trigger_type": "new_customer",
  "actions": [
    {
      "type": "send_message",
      "channel": "wecom",
      "template": "welcome",
      "content": "您好，欢迎咨询～"
    },
    {
      "type": "create_followup",
      "owner_id": "{{customer.owner_id}}",
      "content": "首日跟进",
      "due_at_offset_hours": 24
    }
  ]
}
```

**示例 2：商机阶段变更提醒销售**

```json
{
  "name": "商机阶段变更通知",
  "trigger_type": "stage_change",
  "trigger_config": {"from_stage": "qualifying", "to_stage": "proposal"},
  "actions": [
    {
      "type": "notify_staff",
      "to": "{{opportunity.owner_id}}",
      "channel": "wecom",
      "content": "商机 {{opportunity.name}} 进入方案阶段"
    }
  ]
}
```

### 2. SOP 触发（内部）

由 `matchRule` 函数根据 `trigger_type` 路由到对应业务触发器：

```go
// services/crm_sop.go:matchRule
func (s CrmSopService) matchRule(tenantID string, event SopEvent) {
    rules := listByTrigger(tenantID, event.Type)
    for _, rule := range rules {
        if rule.match(event) {
            for _, action := range rule.Actions {
                action.execute(tenantID, event)
            }
        }
    }
}
```

### 3. 动作类型

| type | 说明 |
|------|------|
| `send_message` | 发送消息（channel + template） |
| `create_followup` | 创建跟进任务 |
| `add_tag` | 自动加标签 |
| `remove_tag` | 自动减标签 |
| `notify_staff` | 通知员工 |
| `update_customer` | 更新客户字段 |
| `webhook` | 调用外部接口 |

## 模板变量

`{{customer.id}}` / `{{customer.name}}` / `{{opportunity.name}}` 等。

## 注意事项

- **执行幂等**：同一事件可能触发多次，**需要业务幂等**（如不重复发欢迎语）。
- **频率限制**：避免 SOP 风暴（**当前未实现**，**产品补强点**）。
- **跨租户禁止**。
- **回溯执行**：创建 SOP 时是否对历史数据回溯执行？**当前默认不回溯**。

## 当前限制（**产品补强点**）

- 仅 2 种触发类型（stage_change / tag_added），缺少：
  - `birthday`（生日触发）
  - `holiday`（节日触发）
  - `order_completed`（订单完成触发）
  - `inactive_days`（N 天未活跃触发）
- 无"动作失败重试"。
- 无 SOP 执行日志详情查询（仅基础日志）。

## 相关文档
- [14-群SOP.md](14-群SOP.md)（针对群的 SOP）
- [customer/02-客户标签与自动打标.md](../customer/02-客户标签与自动打标.md)（SOP 加标签）
- [marketing/01-营销活动总览.md](01-营销活动总览.md)