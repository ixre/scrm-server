# 数据看板（Dashboard）

> 工作台/销售漏斗/渠道对比/业绩达成，数据可视化与决策支持。

## 子模块清单

| 文档 | 内容 |
|------|------|
| [01-工作台.md](01-工作台.md) | 个人工作台首页 |
| [02-销售漏斗.md](02-销售漏斗.md) | 商机阶段漏斗分析 |
| [03-渠道对比.md](03-渠道对比.md) | 多渠道效果对比 |
| [04-业绩达成.md](04-业绩达成.md) | 团队/个人业绩目标 |

## 模块定位

数据看板是 CRM 决策支持层，**前端 Arco Design 已完成迁移**（详见 memory: dashboard-arco-migration-done）。

## 关键代码位置

| 模块 | 文件 |
|------|------|
| 工作台 | `services/dashboard_workspace.go` |
| 销售漏斗 | `services/crm.go:1561 PipelineFunnel` |
| 渠道对比 | `services/crm.go:1664 ChannelCompare` |
| 渠道汇总 | `services/crm.go:1602 ChannelSummary` |
| 活动漏斗 | `services/crm.go:1738 ActivityFunnel` |
| 业绩 | `services/performance.go` |

## 相关文档
- [CRM 主文档](../README.md)