# 客户管理（Customer 360）

> 销售日常使用最频繁的模块。围绕 `CrmCustomer` 主档，提供"录入→标签→跟进→画像→流转→回收"全链路。

## 子模块清单

| 文档 | 内容 |
|------|------|
| [01-客户主档.md](01-客户主档.md) | 新建/编辑/查找/合并/锁定/共享/黑名单/查看 360 |
| [02-客户标签与自动打标.md](02-客户标签与自动打标.md) | 标签字典 + 内部标签 + 自动打标规则 |
| [03-客户去重与合并.md](03-客户去重与合并.md) | 重复客户合并与历史迁移 |
| [04-跟进沟通.md](04-跟进沟通.md) | 跟进记录 / 评论 / 快捷跟进 / 拜访签到 |
| [05-客户公海与池.md](05-客户公海与池.md) | 公海池规则 / 入海 / 认领 / 再分配 |
| [06-流失告警与超时.md](06-流失告警与超时.md) | 流失客户 / 超时未跟 / 超时未回 |
| [07-客户评分.md](07-客户评分.md) | 自动评分 / Top 客户榜 |
| [08-客户画像360.md](08-客户画像360.md) | 详情聚合卡 / 客户时间线 |
| [09-数据导入导出.md](09-数据导入导出.md) | Excel 导入 / 导出 / 字段映射 |

## 客户数据流向

```
                       入口（多渠道）
                          │
   ┌──────────────────────┼──────────────────────┐
   │                      │                      │
   ▼                      ▼                      ▼
 手动录入             渠道留资               H5 落地页
 CrmCustomer         ChannelLink             CrmLandingPage
   │                  4 渠道 webhook              │
   │                      │                      │
   └──────────────────────┼──────────────────────┘
                          ▼
                  CrmCustomer 主档
                          │
            ┌─────────────┼─────────────┐
            ▼             ▼             ▼
       自动打标       客户评分       跟进/沟通
       CrmAutoTag    CrmScore      CrmFollowup
            │             │             │
            └─────────────┼─────────────┘
                          ▼
                   销售过程记录
                   Lead/Quote/工单
                          │
                          ▼
                   客户 360 详情聚合
                          │
                          ▼
                   公海 / 流失 / 继承
```

## 关键数据模型

- **CrmCustomer**（`models/crm.go:58-101`）：主档 50+ 字段，含渠道快照、池状态、评分、黑名单、共享等。
- **ChannelLink**（`models/crm.go:46-55`）：本地客户 ↔ 渠道外部 ID 映射。
- **CrmContact**（`models/crm.go:877-`）：同一客户下多联系人（决策人 / 经办人）。
- **CrmFollowup**（`models/crm.go:165-180`）：跟进记录，含跟进类型/时长/位置/结果。
- **CrmCustomerSetting**（`models/crm.go:1534-`）：拥有客户数上限等设置。

## 关键代码

| 服务 | 路径 | 说明 |
|------|------|------|
| CrmCustomerService | `services/crm.go` | 主档 CRUD + GetDetail 360 |
| CrmCustomerExtService | `services/crm_customer_ext.go` | 锁客 / 黑名单 / 共享 / 再分配 |
| CrmFollowupService | `services/crm.go` + `crm_followup_ext*.go` | 跟进 + 评论 + 快捷 + 今日待办 |
| CrmPoolService | `services/crm_pool.go` | 公海池 + 自动回收 |
| CrmRecycleService | `services/crm_recycle.go` | 回收站 |
| CrmTransferService | `services/crm_transfer.go` | 转移 + 日志 |
| CrmDedupService | `services/crm_dedup.go` | 去重 + 合并 |
| CrmAutoTagService | `services/crm_auto_tag.go` | 自动打标 |
| CrmScoreService | `services/crm_score.go` | 客户评分 |
| CrmCustomerLossService | `services/crm_risk.go` | 流失告警 |
| CrmInheritanceService | `services/crm_risk.go` | 离职/在职继承 |

## 角色权限要点

| 操作 | 销售 | 销售经理 | 管理员 |
|------|------|---------|--------|
| 查看自己的客户 | ✅ | ✅ | ✅ |
| 查看团队客户 | ❌ | ✅ | ✅ |
| 锁客 / 黑名单 | ❌ | ✅ | ✅ |
| 共享客户 | ✅ | ✅ | ✅ |
| 转移客户 | ✅（自己的） | ✅ | ✅ |
| 客户去重合并 | ❌ | ✅ | ✅ |
| 标记客户继承 | ❌ | ✅ | ✅ |
| 自定义字段配置 | ❌ | ❌ | ✅ |

## 客户 360 详情卡（GetDetail）

`crm.go:898-937` 返回字段：

```
customer                  // 主档（含企微渠道快照）
followups                 // 跟进记录 timeline
wecom_messages            // 企微聊天记录（关联 ext_customer_id）
channel_interactions      // 非企微渠道互动（小红书/公众号/抖音）
activity_records          // 营销活动参与史
transfer_logs             // 转移历史
```

> **注**：当前未聚合订单/合同/工单/回款/报价（生产环境补强点）。