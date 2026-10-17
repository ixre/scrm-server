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

## 2026-10-17 产品补齐评估（核心 CRM + 多渠道获客首期）

本轮按 product-manager-toolkit 的 JTBD、MoSCoW 和任务成功率方法评估。证据来自路由、服务、模型、前端及数据库回归测试；没有客户访谈或线上使用量，因此不虚构 RICE 覆盖人数、商业收益或用户满意度。

目标用户与工作：销售每天知道该联系谁、何时联系、哪些合同该催款，并能够记录结果和预约下一步；主管能够确保交接后的待办归正确负责人。以“可完成真实业务任务”作为验收单位，而不是按菜单数量判断完整度。

| 优先级 | 用户问题与实现证据 | 处理及验收边界 |
| --- | --- | --- |
| Must / 本轮 | 跟进工作台只查询当日零点之前，今天已跟进会屏蔽同日再次预约；前端字段与返回字段不一致 | 今日含逾期及整天预约，取客户当前预约；补关键词检索、客户名、预约时间、快速预约和下一步行动 |
| Must / 本轮 | 跟进、回款提醒生成方法未注册到调度器，固定取前 500 条 | 每 5 分钟触发，按租户、到期时间和 ID 游标分批；数据库错误可见；单个租户或提醒类型超时不阻断后续租户 |
| Must / 本轮 | 仅按 pending 去重，处理后重复生成；多期回款按合同合成一条 | 来源记录、负责人、预约时间组成唯一事件；不同回款期独立提醒；显式处理后不重复生成同一事件 |
| Must / 本轮 | 同租户其他人可处理待办；交接、入海、改期、已回款后仍提醒 | 本人处理，整批校验；列表、计数和调度前核对来源；无效提醒保留历史并显示已失效 |
| Must / 本轮 | 常用备注 bigint 与空串比较导致列表失败；前端删除参数不匹配，排序未保存 | 修复 NULL 语义及接口字段；个人备注仅本人写，共享备注由管理角色写；排序随新增/编辑保存 |
| Must / 待下一专项验收 | 自动分配仅配置 CRUD（`app/services/crm_setting.go`），代码没有调用规则执行器 | 在补齐匹配、轮询/负载策略、无可分配人回退、并发认领和分配日志前，不把“自动分配规则”作为已交付能力 |
| Must / 待下一专项验收 | 跨渠道引流任务（`app/services/crm_channel_task.go`）分配、处理仅按租户和 ID 更新 | 需完善员工有效性、当前负责人权限、状态流转、分配审计和 webhook 重试下的幂等；需真实渠道凭据验收 |
| Should / 待补齐 | 客户 360（`CrmCustomerService.GetDetail`）仍未聚合合同/报价/回款，并无界读取跟进 | 在各业务权限范围内增加金额摘要和分页明细，避免共享客户权限自动扩大成财务权限 |
| Should / 待专项验证 | 部分旧业务文档仍描述尚未接入的推送、自动化和不存在的接口 | 逐模块对照路由和业务测试修正文档；本轮先更新跟进与待办文档 |
| Won't / 首期范围外 | 商城、会员积分、直播、CDP 旅程等独立产品能力 | 延续根目录 TODO 的分期范围，不把扩展菜单当作首期商用验收通过依据 |

本轮可执行验收标准：同一提醒事件并发生成仅一条；处理后重复扫描不再生成；同日二次预约不漏；601 条到期客户全部覆盖；同合同不同期次独立；跨租户/跨用户混合处理全部失败且无部分写入；来源失效后待处理计数立即归零；历史待办不删除。测试使用隔离 PostgreSQL。测试通过不能替代真实客户访谈、渠道联调、千万行压测和部署后的任务延迟监控。

上线后需要采集的产品指标：到期提醒生成延迟（目标正常负载下不超过一个 5 分钟调度周期加执行耗时）、重复事件数（目标 0）、无效提醒展示数（目标 0）、预约后实际跟进完成率、逾期预约积压量。当前未接入指标采集，不能宣称已达到生产 SLA。
