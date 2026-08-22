# CRM 业务功能流程清单

> 基于当前系统实际实现整理，按 CRM 标准客户生命周期与销售漏斗流程组织。
> 每个功能标注实现状态：✅ 已实现 / ⚠️ 部分实现 / ❌ 未实现
> 对应后端接口为 `internal/dispatch/crm.go` 中注册的 `域名.方法`。

---

## 一、平台运营（SaaS 运营方使用）

### 1.1 平台账号管理
- ✅ 平台超管登录（`crm_platform.Login`）
- ✅ 平台账号 CRUD（`crm_platform_admin.List/Create/Update/Delete`）
- ✅ 平台账号重置密码（`crm_platform_admin.ResetPassword`）
- ✅ 平台账号启用/停用（`crm_platform_admin.SetStatus`）

### 1.2 租户管理
- ✅ 租户列表/分页查询（`crm_tenant.List/ListPaged`）
- ✅ 创建租户（含初始管理员账号）（`crm_tenant.Create`）
- ✅ 租户详情查看（`crm_tenant.GetDetail`）
- ✅ 租户信息编辑（`crm_tenant.Update`）
- ✅ 租户启用/停用（`crm_tenant.SetStatus`）
- ✅ 租户删除（`crm_tenant.Delete`）
- ✅ 重置租户管理员密码（`crm_tenant.ResetAdminPassword`）
- ✅ 租户续期（`crm_tenant.Renew`）
- ✅ 查看租户下用户列表（`crm_tenant.ListUsers`）
- ✅ 获取当前租户信息（`crm_tenant.GetCurrentInfo`）

### 1.3 套餐管理
- ✅ 套餐 CRUD（`crm_plan.Create/List/Update/Delete`）
- ✅ 套餐模块配置（`crm_plan.ListModules`）
- ✅ 套餐启用/停用（`crm_plan.SetStatus`）
- ✅ 套餐详情（`crm_plan.Get`）

### 1.4 支付与订单
- ✅ 支付方式配置（`crm_payment.Create/List/Update/Delete/SetStatus`）
- ✅ 平台订单总览（`crm_order.ListAll`）
- ✅ 手动确认收款（`crm_order.ManualConfirmPay`）
- ✅ 租户侧下单（`crm_tenant_order.Create`）
- ✅ 租户侧订单列表/详情（`crm_tenant_order.List/Get`）
- ✅ 支付状态轮询（`crm_tenant_order.QueryPayStatus`）
- ✅ 获取支付二维码（`crm_tenant_order.GetQrCode`）
- ✅ 取消订单（`crm_tenant_order.Cancel`）
- ✅ 获取可用支付方式（`crm_tenant_order.ListPaymentMethods`）

### 1.5 平台统计与日志
- ✅ 平台概览统计（`crm_platform_stats.Overview`）
- ✅ 租户用量统计（`crm_platform_stats.TenantUsage`）
- ✅ 全部租户用量统计（`crm_platform_stats.AllTenantUsage`）
- ✅ 操作日志查询（`crm_platform_log.List`）

---

## 二、租户初始化与企业管理

### 2.1 租户登录与认证
- ✅ 租户用户登录（`crm_auth.Login`）
- ✅ 修改密码（`crm_auth.ChangePassword`）

### 2.2 员工管理
- ✅ CRM 用户创建（`crm_user.Create`）
- ✅ CRM 用户查询（`crm_user.Query`）
- ✅ 设置/修改密码（`crm_user.SetPassword`）
- ✅ 用户信息编辑（`crm_user.Update`）
- ✅ 企微账号绑定（`crm_user.BindWecom`）
- ✅ 企微账号解绑（`crm_user.UnbindWecom`）
- ⚠️ 坐席数限制（套餐 max_users 校验，逻辑存在但前端无明确提示）

### 2.3 权限管理
- ✅ 角色列表/创建/编辑（前端 Role 页面）
- ✅ 角色权限分配（authority 配置）
- ✅ 菜单权限控制（permission.ts + menu.ts）

### 2.4 系统设置
- ✅ 自定义字段管理（`crm_field_def.Create/Update/Delete/List`）
- ✅ 字段值读写（`crm_field_value.Get`）
- ✅ 列表场景管理（`crm_scene.*`）
- ✅ 审批引擎配置（`crm_approval_flow.Create/Update/Delete/List`）
- ✅ 审批提交/审批/驳回/撤回（`crm_approval.Submit/Approve/Reject/Void`）
- ✅ 消费记录（`crm_consume.*`）
- ✅ 回收站（`crm_recycle.List/Restore/Clear`）
- ✅ 批量操作（`crm_batch.Execute`，支持 lock/unlock/edit_field/blacklist/remove_blacklist）
- ✅ 系统参数设置（`crm_customer_setting.*`）
- ✅ 套餐与账单查看（`crm_tenant.GetCurrentInfo` + `crm_tenant_order.List`）
- ✅ 自动分配规则配置（`crm_auto_allocation.List/Create/Update/Delete/ToggleStatus`）

### 2.5 公海规则配置
- ✅ 公海回收规则设置（`crm_pool.SetRule`）
- ✅ 公海回收规则查询（`crm_pool.GetRule`）
- ✅ 自动回收定时任务（`crm_pool.AutoRecycleAll`）

---

## 三、引流获客（线索生成阶段）

### 3.1 渠道活码
- ✅ 渠道活码列表/创建/编辑/复制（前端 ContactWay 页面）
- ✅ 渠道设置（`crm_channel_config.*`）

### 3.2 引流任务
- ✅ 引流任务管理（`crm_channel_task.*`）

### 3.3 裂变活动
- ✅ 裂变活动创建/管理（`crm_fission.Create/Query`）
- ✅ 邀请记录查询（`crm_fission.ListInvites`）
- ❌ 分享链接生成（`crm_fission.GetShareLink` — 未在 dispatch 注册）
- ❌ 裂变落地页（`crm_fission.Landing` — 未在 dispatch 注册）

### 3.4 H5 落地页
- ✅ 落地页创建/管理（`crm_landing_page.Create/Query`）
- ❌ 浏览追踪（`crm_landing_page.TrackView` — 未在 dispatch 注册）
- ❌ 表单提交（`crm_landing_page.Submit` — 未在 dispatch 注册）
- ❌ 提交记录查询（`crm_landing_page.ListSubmits` — 未在 dispatch 注册）
- ❌ 落地页统计（`crm_landing_page.Stats` — 未在 dispatch 注册）

### 3.5 智能名片
- ✅ 名片创建/管理（`crm_smart_card.Create/Query`）
- ❌ 浏览追踪（`crm_smart_card.TrackView` — 未在 dispatch 注册）
- ❌ 添加联系人（`crm_smart_card.AddContact` — 未在 dispatch 注册）
- ❌ 生成分享（`crm_smart_card.GenerateShare` — 未在 dispatch 注册）
- ❌ 名片统计（`crm_smart_card.Stats` — 未在 dispatch 注册）

### 3.6 渠道互动
- ✅ 渠道互动记录查询（`crm_channel_interaction.Query`）
- ✅ 渠道标签管理（`crm_channel_tag.Create/List`）

---

## 四、线索管理（线索池阶段）

### 4.1 线索录入
- ✅ 手动创建线索（`crm_lead.Create`）
- ✅ 线索查询/编辑/删除（`crm_lead.Query/Update/Delete`）
- ❌ 线索批量导入（前端无导入入口，后端无 `crm_lead.*Import` 接口）

### 4.2 线索池
- ✅ 线索池列表（`crm_lead_ext.ListPool`）
- ✅ 移入线索池（`crm_lead_ext.MoveToPool`）
- ✅ 从线索池领取（`crm_lead_ext.ReceiveFromPool`）
- ✅ 线索转移（`crm_lead_ext.Transfer`）

### 4.3 线索跟进与转化
- ✅ 线索转化为客户（`crm_lead.Convert`）
- ✅ 线索状态流转（new → contacted → qualified → lost）

---

## 五、客户管理（客户建档阶段）

### 5.1 客户录入
- ✅ 手动创建客户（`crm_customer.Create`）
- ✅ 创建时可选"放入公海池"（前端开关）
- ✅ 线索转化生成客户（`crm_lead.Convert`）
- ⚠️ 客户批量导入（`crm_customer.BatchImport`）— 文本框粘贴 CSV，无文件上传/模板下载
- ✅ 客户导出（`crm_customer.Export`）

### 5.2 客户列表与查询
- ✅ 客户列表查询（`crm_customer.Query`）
- ✅ 多维筛选（关键字/归属销售/来源/公海状态）
- ✅ 客户详情 360（`crm_customer.GetDetail`）

### 5.3 客户编辑与维护
- ✅ 客户信息编辑（`crm_customer.Update`）
- ✅ 客户删除（`crm_customer.Delete`）
- ✅ 客户查重（`crm_dedup.FindDuplicates`）
- ✅ 客户合并（`crm_dedup.Merge`）

### 5.4 客户标签
- ✅ 客户标签管理（`crm_channel_tag.Create/List`）
- ✅ 自动打标签规则（`crm_auto_tag.CreateRule/QueryRules/DeleteRules/ApplyToCustomer`）

### 5.5 客户群
- ✅ 客户群列表（前端 GroupChat 页面）
- ✅ 客户群标签（前端 GroupChatTag 页面）

### 5.6 客户转移与继承
- ✅ 客户转移（`crm_customer.Transfer`）
- ✅ 离职继承（`crm_inheritance.*`）
- ✅ 转移日志查询（`crm_customer.ListTransferLog`）

---

## 六、公海池管理（客户流转阶段）

### 6.1 数据进入公海
- ✅ 创建客户时不选归属销售 → 自动入公海
- ✅ 创建客户时勾选"放入公海池"
- ✅ 客户列表单个"移入公海"（`crm_pool.Recycle`）
- ✅ 客户列表批量"移入公海"（`crm_pool.Recycle`）
- ✅ 自动回收超期未跟进客户（`crm_pool.AutoRecycle`）
- ✅ 批量导入无归属客户 → 自动入公海

### 6.2 公海池操作
- ✅ 公海池列表查询（`crm_pool.List`）
- ✅ 单个认领（`crm_pool.Claim`）
- ✅ 批量认领（`crm_pool.Claim`）
- ✅ 公海回收规则配置（`crm_pool.SetRule`）

---

## 七、销售跟进（客户跟进阶段）

### 7.1 跟进记录
- ✅ 写跟进（`crm_followup.Create`）
- ✅ 跟进列表-按客户（`crm_followup.ListByCustomer`）
- ✅ 跟进列表-按商机（`crm_followup.ListByOpportunity`）
- ✅ 跟进删除（`crm_followup.Delete`）
- ✅ 跟进评论（`crm_followup_comment.Create`）
- ✅ 快捷跟进（`crm_quick_followup.Create/List`）
- ✅ 快捷备注（`crm_quick_remark.Create/List/Update/Delete`）

### 7.2 待办管理
- ✅ 待办查询（`crm_todo.Query`）
- ✅ 待办处理（`crm_todo.Handle`）
- ✅ 今日待跟进（`crm_followup_view.TodayTodo`）
- ✅ 今日已跟进（`crm_followup_view.TodayDone`）
- ✅ 从未跟进（`crm_followup_view.NeverFollowup`）

### 7.3 拜访签到
- ✅ 签到记录（`crm_signin.Create`）

### 7.4 SOP 自动化
- ✅ SOP 规则创建/更新（`crm_sop.CreateRule/UpdateRule`）
- ✅ SOP 任务列表（`crm_sop.ListTasks`）
- ✅ 群 SOP（`crm_group_sop.Create/List/ListTasks`）
- ⚠️ SOP 触发类型仅 2 种（stage_change / tag_added），缺 pool_claim / new_customer

### 7.5 客户运营
- ✅ 客户群发（前端 CustomerMassMsg 页面）
- ✅ 客户欢迎语（前端 CustomerWelcomeMsg 页面）
- ✅ 素材库（前端 MaterialLibrary 页面）
- ✅ 话术库（`crm_script.Create/Recommend`）
- ✅ 流失提醒（`crm_customer_loss.List/Handle`）

---

## 八、商机与管线（商机推进阶段）

### 8.1 销售管线
- ✅ 管线/阶段 CRUD（`crm_pipeline.Create/Update/Delete/List`）
- ✅ 阶段排序与终态配置

### 8.2 商机管理
- ✅ 商机 CRUD（`crm_opportunity.Create/Update/Delete/Get/Query`）
- ✅ 商机阶段移动（`crm_opportunity.MoveStage`）
- ✅ 商机扩展字段（`crm_opportunity_ext.*`）
- ✅ 销售规则配置（`crm_sales_rule.*`，跟进强制校验等）

### 8.3 业绩目标
- ✅ 业绩目标设置（`crm_quota.Set`）
- ✅ 业绩达成查询（`crm_quota.Achievement`）

---

## 九、商务合同（成交阶段）

### 9.1 报价管理
- ✅ 报价单 CRUD（`crm_quote.Create/Update/Query/Get`）
- ✅ 报价状态变更（`crm_quote.ChangeStatus`）
- ✅ 报价转移（`crm_quote.Transfer`）
- ✅ 报价扩展字段（`crm_quote_ext.*`）
- ❌ 报价转订单（`crm_quote.ConvertToOrder` — 待实现）

### 9.2 合同订单
- ✅ 订单 CRUD（`crm_order.Create/Update/Delete`）
- ✅ 订单详情查看（`crm_order.Get`）
- ✅ 订单进度跟踪（`crm_order_progress.Create/List`）
- ✅ 订单附件（`crm_order_attachment.Create/List/Delete`）
- ✅ 订单产品明细（`crm_order_product.*`）
- ✅ 订单作废（`crm_order_ext.Revoke`）
- ✅ 订单扩展字段（`crm_order_ext.*`）

### 9.3 回款管理
- ✅ 回款计划 CRUD（`crm_receivables_plan.Create/BatchCreate`）
- ✅ 回款记录（`crm_receivables.*`）
- ✅ 回款扩展字段（`crm_receivables_ext.*`）
- ❌ 回款汇总（`crm_receivables.Summary` — 待实现）
- ❌ 逾期标记（`crm_receivables_plan.MarkOverdue` — 待实现）

---

## 十、营销工具（营销转化阶段）

### 10.1 营销活动
- ✅ 营销活动 CRUD（`crm_activity.Create/Update/Query`）
- ✅ 活动漏斗分析（`crm_report.ActivityFunnel`）
- ✅ 营销活动扩展（`crm_marketing_ext.*`）
- ✅ 活动状态流转（active / off / ended）

### 10.2 优惠券
- ✅ 优惠券 CRUD（`crm_coupon.Create/Update/Delete/Get`）
- ✅ 优惠券发放（`crm_coupon.Issue`）
- ✅ 领取记录查询（`crm_coupon.ListRecords`）

### 10.3 秒杀
- ✅ 秒杀活动 CRUD（`crm_seckill.Create/Query/Get`）
- ✅ 秒杀抢购（`crm_seckill.Grab`）
- ✅ 秒杀记录查询（`crm_seckill.ListRecords`）

### 10.4 抽奖
- ✅ 抽奖活动 CRUD（`crm_lottery.Create/Query/Get`）
- ✅ 抽奖执行（`crm_lottery.Draw`）
- ✅ 抽奖记录查询（`crm_lottery.ListRecords`）

### 10.5 拼团
- ⚠️ 后端已实现（`crm_group_buy.List/Create/Delete/OpenGroup/JoinGroup/ListRecords`）
- ❌ 前端无独立页面

### 10.6 砍价
- ⚠️ 后端已实现（`crm_bargain.List/Create/Delete/Start/Help/ListRecords`）
- ❌ 前端无独立页面

### 10.7 满减
- ⚠️ 后端已实现（`crm_full_reduction.List/Create/Delete/CalculateDiscount`）
- ❌ 前端无独立页面

### 10.8 红包
- ⚠️ 后端已实现（`crm_red_packet.List/Create/Delete/Claim/ListRecords`）
- ❌ 前端无独立页面

---

## 十一、数据分析（经营分析阶段）

### 11.1 工作台
- ✅ 工作台首页（`/console/workbench`）
- ✅ 系统概览（`/console/crm/overview`）
- ✅ 简报数据（`crm_dashboard.Briefing`）
- ✅ 趋势数据（`crm_dashboard.Trend`）
- ✅ 排行榜（`crm_dashboard.Ranking`）

### 11.2 数据看板
- ✅ CRM 仪表盘（`crm_dashboard.*`）
- ✅ 统计报表（`crm_statistics.*`）
- ✅ 销售漏斗（`crm_report.PipelineFunnel`）
- ✅ 渠道对比（`crm_report.ChannelCompare`）
- ❌ 阶段停留时长（`crm_report.StageDuration` — 未注册）
- ❌ 输单原因分析（`crm_report.LostReasons` — 未注册）
- ❌ 渠道汇总（`crm_report.ChannelSummary` — 未注册）
- ❌ 多触点归因（`crm_report.MultiTouchAttribution` — 未注册）

### 11.3 客户评分
- ✅ 评分重算-单个（`crm_score.Recalculate`）
- ✅ 评分重算-全部（`crm_score.RecalculateAll`，定时任务）
- ✅ 高分客户排行（`crm_score.TopByScore`）
- ⚠️ 评分公式：客户等级分(15) + 互动频次分(30) + 商机金额分(30) + 订单金额分(25)
- ⚠️ 无业务事件实时触发评分更新

### 11.4 AI 分析
- ⚠️ 基础框架已实现（`crm_ai_analysis.List/Delete/AnalyzeCustomer/Stats`）
- ⚠️ 输入客户ID触发分析，输出 summary/score，分析维度有限

---

## 十二、风控合规（风险管控阶段）

### 12.1 消息存档
- ✅ 聊天会话记录（前端 ChatSession 页面）
- ✅ 删人提醒（前端 DeleteCustomerRecord 页面）

### 12.2 敏感词
- ✅ 敏感词库管理（`crm_sensitive_word.Create/Query`）
- ✅ 敏感词告警记录（`crm_sensitive_alert.List`）
- ❌ 敏感词批量导入（`crm_sensitive_word.Import` — 未注册）

### 12.3 风控中心
- ✅ 风控中心页面（前端 RiskCenter 页面）
- ✅ 回复超时（`crm_reply_timeout.List/Handle`）

---

## 十三、业务扩展模块

### 13.1 项目管理
- ✅ 项目 CRUD（`crm_project.*`）
- ✅ 项目事件（`crm_project_event.*`）
- ✅ 项目文档（`crm_project_file.*`）
- ✅ 项目任务（`crm_project_task.*`）

### 13.2 财务管理
- ✅ 财务记录（`crm_finance_record.*`）
- ✅ 收入分类（`crm_income_category.*`）
- ✅ 其他收入（`crm_other_income.*`）
- ✅ 其他支出（`crm_other_expense.*`）
- ✅ 资金转账（`crm_fund_transfer.Create`）
- ✅ 发票管理（`crm_invoice.Create/Void`）
- ✅ 财务扩展（`crm_finance_ext.*`）
- ❌ 发票审批/开具（`crm_invoice.Approve/Reject/MarkIssued` — 未注册）
- ❌ 财务月报（`finance.MonthlyReport` — 未注册）

### 13.3 进销存
- ✅ 商品管理（`crm_product.*`）
- ✅ 仓库管理（`crm_warehouse.*`）
- ✅ 库存流水（`crm_stock_record.*`）
- ✅ 入库单（`crm_stock_in.*`）
- ✅ 出库单（`crm_stock_out.*`）
- ✅ 调拨单（`crm_stock_transfer.*`）
- ✅ 盘点单（`crm_stock_check.*`）
- ✅ 供应商（`crm_supplier.*`）
- ✅ 采购订单（`crm_purchase_order.*`）
- ❌ 库存余额表（无当前库存量查看）
- ❌ 库存预警（无低库存提醒）
- ❌ 进销存报表

### 13.4 办公 OA
- ✅ 工作报告（`crm_work_report.Create/List/Review/Stats`）
- ✅ 报告评论（`crm_work_report_comment.*`）
- ✅ 公告（`crm_notice.Create/List`）
- ✅ 知识库（`crm_knowledge.Create/List`）
- ✅ 知识库分类（`crm_knowledge_type.*`）
- ✅ 考勤（`crm_attendance.Create`）
- ✅ 请假（`crm_leave.Create/Approve/Reject`）
- ❌ 知识库全文搜索（`knowledge.Search` — 未注册）
- ❌ 知识库点赞/收藏（`knowledge.Like/Favorite` — 未注册）
- ❌ 公告已读标记（`announcement.MarkRead` — 未注册）
- ❌ 考勤月报（`attendance.MonthlyReport` — 未注册）

### 13.5 工单与售后
- ✅ 工单 CRUD（`crm_workorder.*`）
- ✅ 工单池（`crm_workorder_pool.List/MoveToPool/Receive/BatchReceive`）
- ✅ 工单日志（`crm_workorder_log.List/Add`）
- ✅ 工单类型（`crm_workorder_type.*`）
- ✅ 售后管理（`crm_aftersales.*`）
- ✅ 售后类型（`crm_aftersales_type.*`）
- ✅ 售后评论（`crm_aftersales_comment.*`）

### 13.6 呼叫中心
- ✅ 通话记录 CRUD（`crm_call_record.*`）
- ✅ 通话统计（`crm_call_record.Stats`）
- ✅ 待拨电话列表（`crm_call_list.*`）
- ⚠️ 仅通话记录管理，未对接外部呼叫系统（符合预期，外部系统不对接）

---

## 十四、待完善功能优先级

### 高优先级
1. **线索批量导入** — 补充前端导入入口 + 后端 `crm_lead.BatchImport` 接口
2. **营销活动4玩法前端页面** — 拼团/砍价/满减/红包，后端已就绪，补前端页面
3. **进销存库存余额** — 补充库存余额查询 + 库存预警

### 中优先级
4. **客户导入升级** — 文件上传 + 模板下载 + 字段映射
5. **客户评分实时触发** — 成单/跟进/商机推进后自动重算
6. **知识库搜索** — 全文搜索 + 分类管理
7. **SOP 触发类型扩展** — 补充 pool_claim / new_customer

### 低优先级
8. **AI 分析维度** — 明确分析类型（画像/流失预测/价值评估/跟进建议）
9. **审批流可视化** — JSON 配置升级为流程设计器
10. **数据看板补充** — 阶段停留时长/输单原因/渠道汇总
