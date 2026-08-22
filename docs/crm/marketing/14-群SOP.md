# 群 SOP

## 目的
针对企业微信客户群，按群周期事件自动发消息/素材，提升群活跃度。

## 入口
- 客户管理 → 客户运营 → 群SOP
- 群运营后台

## 数据模型

`CrmGroupSop`（`models/crm.go:667-678`）：

| 字段 | 说明 |
|------|------|
| `name` | 群 SOP 名 |
| `trigger_type` | daily / new_member / keyword |
| `trigger_config` | 配置（时间/关键词等） |
| `actions` | 动作列表 |
| `status` | active / disabled |

## 3 种触发类型

`models/crm.go:706-` + `services/crm_risk.go:388-`：

| trigger_type | 含义 | 触发时机 |
|--------------|------|----------|
| `daily` | 每日定时 | 定时任务每天 N 点扫描 |
| `new_member` | 新成员入群 | 群新成员加入时 |
| `keyword` | 关键词触发 | 群消息命中关键词 |

## 操作步骤

### 1. 创建群 SOP（已实现）

**接口**：`crm_group_sop.Create`

```go
// services/crm_risk.go:353-426 中 CrmGroupSopService.Create 实现：
func (s CrmGroupSopService) Create(tenantID, triggerConfig, actions) error {
    return s.Create(...)
}
```

**示例 1：每日早报**

```json
{
  "name": "每日早报",
  "trigger_type": "daily",
  "trigger_config": {"time": "09:00", "groups": ["g1", "g2"]},
  "actions": [
    {
      "type": "send_group_message",
      "content_type": "image",
      "content": "https://cdn.example.com/daily-report.jpg"
    }
  ]
}
```

**示例 2：新成员欢迎**

```json
{
  "name": "新成员欢迎",
  "trigger_type": "new_member",
  "actions": [
    {
      "type": "send_group_message",
      "content": "欢迎 @新成员 加入～请看群公告"
    },
    {
      "type": "send_group_message",
      "content_type": "image",
      "content": "https://cdn.example.com/welcome.jpg"
    }
  ]
}
```

**示例 3：关键词自动答**

```json
{
  "name": "关键词回复",
  "trigger_type": "keyword",
  "trigger_config": {"keywords": ["价格", "优惠", "折扣"]},
  "actions": [
    {
      "type": "send_group_message",
      "content": "请扫码加客服 1v1 咨询～"
    }
  ]
}
```

### 2. 群 SOP 列表（已实现）

**接口**：`crm_group_sop.List`

### 3. 群 SOP 任务查询（已实现）

**接口**：`crm_group_sop.ListTasks(tenantID, sopID)`

返回 SOP 触发的群任务记录（哪些群已执行）。

### 4. 编辑/删除（已实现）

**接口**：`crm_group_sop.Update / Delete`

> ⚠️ `crm_group_sop.Update` **未在 dispatch 注册**（dispatch 仅注册 List/Create/Delete/ListTasks/DoneTask，无 Update）。`Delete` 已注册。

## 注意事项

- **群范围**：`daily` 类型可指定 `groups: [group_id...]`，如不指定则全群。
- **关键词冲突**：同一关键词多个 SOP 时，**触发顺序按创建时间**。
- **风控**：连续触发可能被微信风控（**当前未实现限频**）。
- **已实现位置**：`services/crm_risk.go:388-`（虽然文件命名 risk，但包含 CrmGroupSopService）。

## 当前限制（**产品补强点**）

- 无按"群标签/群等级"范围。
- 无"定时任务执行次数限制"防刷屏。
- 无"群成员不活跃时提醒"等触发器。

## 相关文档
- [13-SOP自动化.md](13-SOP自动化.md)（针对客户的 SOP）
- [wecom 群](../channel/02-企业微信.md)（群消息依赖）