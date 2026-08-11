# 营销活动（Marketing）

> 11 类营销活动 + SOP + 话术 + 智能名片，构成私域营销完整工具集。

## 子模块清单

| 文档 | 内容 |
|------|------|
| [01-营销活动总览.md](01-营销活动总览.md) | 活动模型、活动状态机、参与记录 |
| [02-优惠券.md](02-优惠券.md) | 满减/折扣/兑换券 |
| [03-秒杀.md](03-秒杀.md) | 限时抢购 |
| [04-抽奖.md](04-抽奖.md) | 权重随机抽奖 |
| [05-拼团.md](05-拼团.md) | 多人成团 |
| [06-砍价.md](06-砍价.md) | 好友帮砍 |
| [07-满减.md](07-满减.md) | 满 X 减 Y |
| [08-红包.md](08-红包.md) | 现金红包 |
| [09-签到.md](09-签到.md) | 每日签到 |
| [10-裂变.md](10-裂变.md) | 邀请好友 |
| [11-H5落地页.md](11-H5落地页.md) | 表单落地页 |
| [12-智能名片.md](12-智能名片.md) | 销售社交名片 |
| [13-SOP自动化.md](13-SOP自动化.md) | 触发式自动化规则 |
| [14-群SOP.md](14-群SOP.md) | 群运营自动化 |
| [15-话术库.md](15-话术库.md) | 销售话术推荐 |

## 营销活动矩阵

| 活动 | 模型 | 核心玩法 | 适用场景 |
|------|------|---------|---------|
| 优惠券 | CrmCoupon | 满减/折扣/兑换 | 全场景通用 |
| 秒杀 | CrmSeckill | 限时+限量 | 引流爆款 |
| 抽奖 | CrmLottery | 权重随机 | 互动涨粉 |
| 拼团 | CrmGroupBuy | 多人成团 | 老带新 |
| 砍价 | CrmBargain | 好友帮砍 | 社交裂变 |
| 满减 | CrmFullReduction | 满 X 减 Y | 订单转化 |
| 红包 | CrmRedPacket | 现金激励 | 复购激活 |
| 签到 | CrmSignIn | 连续签到送券 | 日活 |
| 裂变 | CrmFission | 邀请人解锁奖励 | 低成本获客 |
| H5 | CrmLandingPage | 表单留资 | 渠道引流 |
| 智能名片 | CrmSmartCard | 销售社交 | B2B 拓客 |

## 营销自动化

| 模块 | 作用 |
|------|------|
| CrmSop | 客户行为 → 创建跟进/发消息（4 种触发器） |
| CrmGroupSop | 群运营 → 发消息/素材（3 种触发器） |
| CrmAutoTag | 客户行为 → 自动打标（3 种规则） |
| CrmSalesScript | 按场景 → 推荐话术 |

## 营销活动关联图

```
                       ┌──────────────┐
                       │  营销活动     │
                       │ CrmActivity  │
                       └──────┬───────┘
                              │ 1:N
            ┌─────────────────┼─────────────────┐
            ▼                 ▼                 ▼
       CrmCoupon       CrmSeckill       CrmLottery
            │                 │                 │
            └────────┬────────┴────────┬────────┘
                     │                 │
                     ▼                 ▼
              CrmActivityRecord (统一参与记录)
                     │
                     ▼
              CrmChannelInteraction (渠道互动)
                     │
                     ▼
                CrmCustomer (获客/激活)
```

## 关键代码位置

| 服务 | 文件 | 说明 |
|------|------|------|
| CrmActivity | `services/crm_activity.go` | 活动总览 |
| CrmCoupon | `services/crm_coupon.go` | 优惠券 |
| CrmSeckill | `services/crm_seckill.go` | 秒杀 |
| CrmLottery | `services/crm_lottery.go` | 抽奖 |
| CrmFission | `services/crm_fission.go` | 裂变 |
| CrmLandingPage | `services/crm_landing_page.go` | H5 |
| CrmSmartCard | `services/crm_smart_card.go` | 智能名片 |
| CrmSop | `services/crm_sop.go` | SOP |
| CrmGroupSop | `services/crm_risk.go:353-` | 群 SOP |
| CrmSalesScript | `services/crm_script.go` | 话术 |
| CrmAutoTag | `services/crm_auto_tag.go` | 自动打标 |

## 渠道投放

营销活动可指定投放渠道：

```json
{
  "channels": ["wecom", "xiaohongshu"],
  "audience": {
    "tag_filter": ["高价值客户"],
    "level_min": 3
  }
}
```

**当前限制**（**产品补强点**）：
- 无触达频控
- 无投放计划排期
- 无 A/B 测试

## 注意事项

- **活动状态**：所有活动都有 `status` 字段（active/disabled/ended）。
- **参与记录**：每类活动有独立的 Record 表，统一通过 `CrmActivityRecord` 汇总。
- **奖品类型**：奖品支持 coupon（券）/ cash（现金）/ gift（实物）/ thanks（谢谢参与）。
- **库存管理**：秒杀有库存，优惠券有 `TotalCount/IssuedCount`，其他活动无显式库存。
- **时间控制**：所有活动都有 `StartAt/EndAt` 字段。

## 相关文档
- [base/04-自定义字段.md](../base/04-自定义字段.md)（活动可扩展字段）
- [dashboard/03-活动漏斗.md](../dashboard/03-活动漏斗.md)（活动效果分析）