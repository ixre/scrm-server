# 多渠道（Channel）

> 4 个渠道（企业微信 / 小红书 / 公众号 / 抖音）统一接入 + 跨渠道任务。

## 子模块清单

| 文档 | 内容 |
|------|------|
| [01-渠道总览.md](01-渠道总览.md) | 渠道架构、ChannelLink、适配器模式 |
| [02-企业微信.md](02-企业微信.md) | 企微接入、好友/群/消息 |
| [03-小红书.md](03-小红书.md) | 小红书接入、私信/笔记 |
| [04-公众号.md](04-公众号.md) | 公众号接入、关注/菜单 |
| [05-抖音.md](05-抖音.md) | 抖音接入、私信/视频 |
| [06-跨渠道任务.md](06-跨渠道任务.md) | 跨渠道 SOP/群发 |
| [07-渠道统计.md](07-渠道统计.md) | 渠道对比、渠道漏斗 |

## 渠道支持矩阵

| 渠道 | 接入状态 | 主要功能 |
|------|----------|----------|
| 企业微信 | ✅ 已实现 | 好友/群/消息/SOP |
| 小红书 | ✅ 已实现 | 私信/笔记互动 |
| 公众号 | ✅ 已实现 | 关注/菜单/模板消息 |
| 抖音 | ✅ 已实现 | 私信/视频互动 |
| 视频号 | ⚠️ 未独立接入 | 复用企微 |
| 一般渠道 | ✅ 已实现 | 通用适配 |

## 渠道架构

```
┌──────────────────────────────────────────────┐
│                 业务层（CRM）                  │
│  CrmCustomer + CrmChannelLink + Interaction  │
└──────────────────────┬───────────────────────┘
                       │ 适配器接口
        ┌──────────────┼──────────────┐
        ▼              ▼              ▼
   wecom-adapter  xhs-adapter  gzh-adapter
        │              │              │
        ▼              ▼              ▼
   企微 API       小红书 API    公众号 API
```

## 渠道模型

### 5 个渠道常量
`constants/channel.go`：
- `wecom`（企业微信）
- `xiaohongshu`（小红书）
- `gzh`（公众号）
- `douyin`（抖音）
- `general`（通用）

### CrmChannelLink
`models/crm.go:200-`：

| 字段 | 说明 |
|------|------|
| `customer_id` | 客户本地 ID |
| `channel` | 渠道常量 |
| `channel_user_id` | 渠道侧用户 ID（如��微 external_userid） |
| `channel_union_id` | 跨账号唯一 ID |
| `nickname` | 渠道昵称 |
| `avatar` | 渠道头像 |
| `is_primary` | 是否主渠道 |

## ChannelLink 写入位置

所有渠道适配器在创建/更新客户时均写入 `CrmChannelLink`：

| 渠道 | 文件 | 行 |
|------|------|-----|
| 企微 | `services/crm_wecom.go` | line 42, 86 |
| 小红书 | `services/crm_xhs.go` | line 42, 94, 137 |
| 公众号 | `services/crm_gzh_douyin.go` | line 42, 86, 123 |
| 抖音 | `services/crm_gzh_douyin.go` | line 233, 268 |

## 适配器接口

`internal/channel/adapter.go`（示意）：

```go
type Adapter interface {
    Channel() string                                    // 渠道常量
    SendMessage(to, content) error                      // 发消息
    SendGroupMessage(groupID, content) error            // 群消息
    GetUserInfo(channelUserID) (User, error)            // 拉取用户
    HandleWebhook(payload) (events []Event, error)     // 回调处理
}
```

## 跨渠道任务

通过 `dispatch/crm.go` 注册，所有渠道走统一路由：

```
业务调用 dispatch.Send(channel, to, content)
        │
        ▼
查 channel → 找到 adapter
        │
        ▼
adapter.SendMessage(to, content)
```

## 注意事项

- **主渠道**：每个客户有 1 个主渠道（is_primary=true），主渠道是默认沟通渠道。
- **跨渠道**：同一客户可在多渠道存在（如小红书引流、企微成交）。
- **渠道断开**：某渠道断开不影响其他渠道（**当前未实现断开重连机制**）。

## 当前限制（**产品补强点**）

- 视频号、视频号小店未独立接入。
- 跨渠道客户行为串联（如抖音看视频 → 公众号留资料 → 企微成交）**未深度整合**。

## 相关文档
- [dashboard/03-渠道对比.md](../dashboard/03-渠道对比.md)（渠道效果分析）