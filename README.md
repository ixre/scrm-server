# SCRM 项目

基于企业微信的私域流量管理（SCRM）系统。仓库根目录按权限边界组织多个独立的本地 git 仓库。

## 目录结构

```
scrm/
├── scrm-server/  唯一闭源 gRPC 业务核心（本仓库，module `core`）
└── scrm-app/     开放侧（前端 dashboard/sidebar + 开放网关 gateway + 会话存档 msg-server）
```

> 历史：本仓库由 openscrm fork（scrm-server）与闭源核心（core）合并而来。openscrm 的 HTTP 层已被 gateway 的路径映射取代，原 HTTP 源码已压缩备份于 `~/git/_archive/`（`scrm-server-openscrm-*.tar.gz`）并删除，不再随本仓库维护。`upstream`（openscrm 官方）remote 已移除。

## 各仓库说明

### scrm-server/ —— 闭源 gRPC 核心（本仓库）
- **定位**：承载真实业务逻辑，源码私有，仅核心团队可访问。
- **对外**：只暴露 `.proto` 契约 + 生成的 pb 客户端代码（相当于私有 registry 的 SDK），不暴露实现。
- **技术**：Go + gRPC，监听 `:9001`。迁移自 openscrm 的 `app`/`common`/`pkg`/`conf` 地基，模块路径 `openscrm/` → `core/`。
- **已接入业务域**：customer、staff（类型化 Get/Query）+ 其余全部业务域（经通用 JSON 透传 BusinessService）。

### BusinessService —— 通用 JSON 透传
openscrm 有数十个业务域、上百个异构方法签名（io.ReadCloser、bytes.Buffer、多字符串参数等），为每个域建模类型化 proto 不现实。因此除 customer/staff 的类型化契约外，增加一个通用 `BusinessService.Invoke(BusinessRequest) → BusinessResponse`：
- `BusinessRequest{service, action, corp_id, staff_id, params}`，`params` 为该动作入参 JSON。
- 本仓库内的 `internal/dispatch` 按 `(service, action)` 分发到真实 service，结果序列化回 JSON。
- 覆盖 CRM 与企微兼容业务域，具体动作以注册表为准（CRUD/Query/Get/Update/Delete/统计/导出等），已注册清单见 `internal/dispatch/*.go` 的 `init()`。
- 内部/回调/Sync/批量拉取/二进制上传（bytes.Buffer、io.ReadCloser）方法不对外暴露，属核心内部。
- 若某域需要强类型契约，仍可按 customer/staff 模板叠加类型化 proto，BusinessService 保持兼容。

### scrm-app/ —— 开放侧（前端 + 网关 + 会话存档）
- `dashboard/` — React 管理后台；`sidebar/` — React 侧边栏。
- `gateway :9002`）。源码开放，不含业务实现。
  - 类型化端点（customer/staff 列表/详情）+ **路径映射层**（`internal/controller/pathmap*`）：把原 openscrm 全部业务端点逐条翻译成对核心的 `(service, action)` 调用，前端 dashboard 沿用原始路径即可零改动访问。
  - 通用 JSON 透传兜底 `/api/v1/biz/:service/:action`。
  - 未纳入路径映射的端点（登录/OAuth、文件流导出、multipart 上传、各域 Sync、内部动作）由网关其他机制承接，见 `scrm-app/gateway/README.md`。
- `msg-server/` — Go 会话存档服务（含企微专属 SDK，敏感），独立部署。

## 调用链路

```
前端(dashboard/sidebar)
   │  HTTP (JSON)
   ▼
gateway(开放，:9002，JWT 鉴权)   ← scrm-app/gateway
   │  gRPC（身份经 metadata 透传）
   ▼
scrm-server(闭源，:9001，真实业务)   ← 本仓库
   │  HTTP（仅 msg_arch/会话存档域）
   ▼
msg-server(独立会话存档服务，企微专属 .so，Linux-only)
```

> **msg-server 是独立部署的外部依赖**（`scrm-app/msg-server`，OpenSCRM 会话存档服务），不是本仓库的一部分，也不是 gateway 的一部分。
> 本仓库仅 `msg_arch` 域（`customer/chat-sessions`、`session-msgs`、`session-msg/action/search`）经 HTTP 反向调用它，地址取 `conf.Settings.Server.MsgArchSrvHost:MsgArchHttpPort`（见 `app/services/msg_arch.go`）。该调用关系是 openscrm 原有拓扑，迁入时原样保留。
> 其它业务域不依赖 msg-server；gateway 对 msg-server 完全透明，只认本仓库的 `msg_arch` 动作。**不启动 msg-server 时，仅会话存档相关端点报错，其余不受影响。**

## 安全边界
- 鉴权在 gateway 的 HTTP 层；本仓库只应监听内网、仅允许 gateway 可达，不得对外直接暴露，否则可绕过鉴权。
- 闭源靠"仓库权限 + 契约/SDK 分发"，防的是协作同事看源码，防不了运维/反编译。
- `conf/config.yaml` 含密钥，已 gitignore 不入库。

## 快速开始（开发）
1. 分别启动本仓库（闭源核心）与 gateway（需 PostgreSQL + Redis 就绪，配置 `conf/config.yaml`）。
2. `cd scrm-server && ./scripts/sync-pb.sh` 同步契约到 `scrm-app/gateway/api/pb`。
3. `cd scrm-app/gateway && go run ./hack/gentoken` 生成 JWT，调用 HTTP 接口。

---

# scrm-server（闭源核心）—— 内部结构与维护

## 结构
```
scrm-server/
├── api/pb/business.proto     # 通用 JSON 透传契约（覆盖其余全部业务域）
├── api/pb/customer.proto      # 契约唯一事实来源（闭源）
├── api/pb/staff.proto         # 契约唯一事实来源（闭源）
├── api/pb/*.pb.go             # protoc 生成代码（与 gateway 同步）
├── app/                       # 从 openscrm 迁入的真实业务（models/services/constants/...）
├── common/                    # 共享基础设施（log/redis/db/validator/...）
├── pkg/easywework/            # 企微 SDK 封装（敏感）
├── conf/                      # 配置（config.yaml 已 gitignore，含密钥不入库）
├── internal/dispatch/         # 通用 JSON 透传分发：注册表 + 各业务域适配器
├── internal/grpcserver/       # gRPC 服务实现，映射真实 models → 契约 pb
├── internal/migrate/          # 手工 SQL 迁移执行器
├── migrations/*.sql           # 手工迁移 SQL（改列定义等 AutoMigrate 覆盖不到的）
├── cmd/migrate/               # 迁移命令入口
├── scripts/sync-pb.sh         # 契约生成 + 同步到 gateway
└── main.go                    # 初始化 conf/db/redis + gRPC server :9001
```

## 契约生成（唯一事实来源）
```bash
./scripts/sync-pb.sh   # 重新生成 customer/staff 的 pb，并同步到 scrm-app/gateway/api/pb
```
改 `.proto` 后跑一次，再在本仓库与 gateway 分别 `go build ./...` 验证。

## 数据库迁移
- **新增字段/表**：改 Model 定义，`AutoMigration: true` 时每次启动自动补（幂等，只增不改删）。
- **改已有列定义**（类型/长度/索引等，AutoMigrate 做不到）：在 `migrations/` 加 `.sql` 文件，然后执行：
  ```bash
  go run ./cmd/migrate   # 在本仓库目录下；自动建库连接 + AutoMigrate + 应用手工 SQL
  ```
  以文件名为版本号记录进 `schema_migrations`，幂等；本命令只建库连接+迁移，不初始化种子数据、不依赖 redis。

**旧版升级限制（2026-10-16 核查）**：`0006_strict_tenant_id.sql` 尚不能直接用于当前旧渠道模型。群发实际使用 `ext_msg_id`，渠道码使用 `config_id`，素材没有统一的 `ext_id`；且部分旧模型的新增流程尚未写入 `tenant_id`。应先完成旧渠道模型和写入流程适配、检查关联表及唯一性，再执行严格约束迁移。不要把该版本提前写入 `schema_migrations`，也不要假定当前 `go run ./cmd/migrate` 可以无条件升级所有旧库。`0004`/`0005` 的可空租户字段回填、`0007` 的模型整理及 `0008`—`0010` 的核心 CRM 变更可以在备份副本验证后独立应用；`0007` 已兼容旧数字状态和已经转为 boolean 的状态列。

## 运行
```bash
go run .   # 需 PostgreSQL + Redis 就绪，config.yaml 配好
```

## 模型与显式聚合读写（2026-10-16）

原生 CRM 的 124 个模型按客户、销售、渠道、营销、自动化、财务、库存、办公、售后及配置拆分到 `app/models/crm_*_models.go`，`crm.go` 保留身份模型。结构体名称、字段、JSON 和表名保持兼容，业务关系使用 ID 字段。

渠道码（含员工、备用员工、排班）、话术分组/详情、分时欢迎语和群发任务已移除 9 处 GORM 隐式关联。嵌套展示字段标为 `gorm:"-"`，通过对应的 `*_store.go` 显式批量加载和事务保存；直接 `DB.Create(parent)` 不再保存这些嵌套字段，应调用模型的 `Create` / `Update` 方法。写入按企业和主记录校验子记录 ID，使用行锁、受限冲突更新和事务回滚。渠道码替换保留员工计数及创建时间，移除的排班与员工软删除并保留原主记录 ID。

此次同时修复了话术详情更新中的空批次/错误字段和非事务写入，以及批量读取群发任务、欢迎语时只返回第一条的问题。接口的嵌套字段名称不变。原有 `char(n)` 字段读取带空格，聚合匹配内部会规范化企业 ID，不改变对外字段类型。

`0011` 增加 21 个与实际筛选/排序匹配的索引（包括租户 + 共享客户 JSONB 的 GIN 组合索引，排除软删除记录），AutoMigrate 与手工迁移共享同一份定义。共享客户索引使用 PostgreSQL 官方 `btree_gin` 扩展，由迁移安装；部署迁移账号需要数据库的扩展创建权限。`0012` 从主记录补全历史子记录缺失的企业字段，不覆盖已有非空企业字段。`0013` 删除未使用且重复遮盖 JSON 企业 ID 的 `quick_reply.corp_id`，并把欢迎语标题从 `char(128)` 改为 `varchar(128)`，去除定长补空格。若旧 `corp_id` 仍有非空值，迁移主动失败并回滚，必须先核实和迁移其业务含义。升级旧库时，先备份并在副本验证，然后停止旧服务、执行脚本、启动新服务；旧代码仍写入已废弃的 `corp_id`，本次字段删除不支持旧新版同时运行。执行：

```bash
# PGHOST / PGPORT / PGUSER / PGDATABASE 指向已确认的目标数据库。
psql -X -v ON_ERROR_STOP=1 -f scripts/migrate_model_refactor.sql
```

该脚本在一个事务内执行 `0011`—`0013` 并登记版本，可重复执行，不执行或标记 `0006`。不删除任何业务表、业务 ID 或已有索引，也不新增物理外键。

已有百万级数据的正式环境应关闭启动时的 `AutoMigration`，由部署流程管理迁移。上述脚本使用普通 `CREATE INDEX`，建索引期间会阻塞对应表的写入；不能当作在线无停机升级。需要缩短停写时间时，可先按 `0011` 的相同定义，在事务外逐个执行 `CREATE INDEX CONCURRENTLY`，检查索引有效性后再安排字段迁移与服务切换。并发建索引失败可能留下无效的同名索引，必须处理后重试，不能只凭 `IF NOT EXISTS` 判断完成。

验证：`SCRM_TEST_DATABASE_URL` 指向隔离 PostgreSQL 后运行 `go test -race ./app/models ./app/services`。模型测试覆盖聚合读写、空集合清理、跨企业/跨主记录 ID 拒绝、并发 ID 冲突、回滚、计数保留、批量查询次数和迁移幂等。

多租户读写保护：通用 `Pager` 单页最多 1000 条（兼容现有看板/下拉列表），默认仍为 10 条，导出使用单独限额。渠道码员工筛选使用带企业条件的 `EXISTS`，不再将全部匹配关系读入 Go。连接池默认每进程最多 25 个连接、10 个空闲连接，最长使用 1800 秒、空闲 300 秒回收，可通过 `DB.MaxOpenConns`、`MaxIdleConns`、`ConnMaxLifetimeSeconds`、`ConnMaxIdleTimeSeconds` 调整。多副本连接预算应按所有进程总和计算。

可复现的数据库容量测试：设置 `SCRM_TEST_DATABASE_URL` 指向隔离测试库，运行 `SCRM_SCALE_ROWS=1000000 go test ./app/models -run '^TestCRMScaleQueries$' -v -count=1 -timeout=1200s`。生成 100 万客户、200 万跟进、1001 个租户，其中一个租户拥有 20 万客户；检查代表性执行计划并运行 32 路并发列表读取/按租户写入。测试结束自动删除专用 schema。这是数据库路径验证，不能替代真实 HTTP 流量、任意关键词搜索、深页 OFFSET 和租户权限组合的完整压测。千万级容量上线前还需按真实租户分布、写入量和部署资源复测；深分页应优先改游标，海量导出改后台分批任务，不按租户批量建表。

2026-10-16 本机 PostgreSQL 16 容器实测（上述数据量，连接池上限 25）：640 次列表读取的 P50 / P95 为 6.04 / 27.12 ms，160 次更新为 35.75 / 269.55 ms。租户列表、负责人列表、客户跟进的执行计划均使用对应组合索引；共享客户精确计数使用租户 + 负责人索引及租户 + JSONB 索引，两次执行分别为 148.59 / 57.70 ms。报告保存在本地 `tmp/performance/model-scale-1000000.json`。这是一轮短时合成负载验证，读取统计不包含额外的总数查询，写入尾延迟和精确计数仍需在真实持续负载下评估；这些数字不是生产容量或响应时间承诺。

本轮没有一次性替换所有旧企微关联：客户与员工、员工部门、标签、群聊、会话消息、自动拉群码等仍有原有 GORM 关联，需分别覆盖同步/回调和历史数据迁移后再拆除。尤其自动拉群码的备用员工继承了普通员工的表名，需要先明确存量数据归属。`0006` 的租户非空和唯一键改造仍按上文限制处理；此次重构不等于全部渠道功能已完成商用验收。

## 与 gateway 的关系
- gateway 是开放端（`scrm-app/gateway`），通过 gRPC 调本服务，**不接触** `app/models`、`common`、`pkg` 实现。
- 每次契约变更：改 proto → 重新生成 → 同步 `.pb.go` 到 gateway → 两端各重新编译。

## 迁移说明
- 本仓库由 openscrm fork 与独立闭源仓库 core 合并而成：openscrm 的 110 提交 + core 的 5 提交历史完整保留于 git log；内容采用 core 那版（module `core`）。
- openscrm 的 `app`(models/services/constants/entities/requests/responses)、`common`、`pkg`、`conf` 已整体迁入，模块路径 `openscrm/` → `core/`，编译通过。
- openscrm 的 HTTP 层（`routers/`、HTTP `main.go`、`test/`、`docker/`、`LICENSE` 等）已被 gateway 路径映射取代，原源码压缩备份于 `~/git/_archive/` 并删除。
- 业务仍继承 openscrm 的全局单例结构（`conf.Settings`、共享 gorm DB）。customer/staff 暴露类型化 Get/Query；其余业务经 `internal/dispatch` 的通用 JSON 透传分发暴露。新增业务动作：在 `internal/dispatch/<域>.go` 的 `init()` 里 `Reg.Register("域","动作",handler)` 即可。
- main 在实际服务进程中启动延迟队列、消费者和定时任务；热更新监督进程不执行任务。队列使用配置中的 Redis.DBNumber。企微任务按记录所属企业选择客户端，现有渠道配置在重启时恢复。

## 安全与回归验证

- Gateway 必须配置至少 32 字节的 `GATEWAY_JWT_SECRET`，不再回退固定默认密钥。生成随机值后通过环境变量或密钥服务注入，所有网关实例使用同一个值。
- 企微强制登录仅允许 `App.Env=DEV/TEST`；`DEMO/PROD` 使用正常账号登录。
- 客户列表、详情、导出和修改使用统一业务层权限：管理员/经理访问本租户，销售访问自己的客户；共享客户只读。批量写入遇到无权限客户时整体回滚。
- 客户合并保留渠道身份、迁移关联记录、合并活动参与档位并重算计数，所有操作处于同一事务。
- 优惠券发放锁定库存，核销锁定模板和券记录；同一非空订单重复核销幂等返回，不重复累计。
- 小红书/公众号/抖音的现有 JSON 适配回调必须携带 `X-Signature`（以渠道 CallbackToken 对原始 body 做 HMAC-SHA256，hex 编码）；该适配协议仍需与实际渠道接入方联调。
- SOP 的 `create_followup` 自动执行并保持事务幂等；`send_message` 暂由销售人工执行，任务保持待处理，不将日志记录当作发送成功。
- 延迟队列消费者将任务异常转为失败重试；真实企微发送、云存储以及遗留导出动作仍需相应配置和联调。未实现的导出任务返回错误，不再静默确认成功。

日常检查（不连接项目业务数据库，也不读取真实云存储凭据）：

```bash
go build ./...
go test ./...
```

数据库回归测试通过 `SCRM_TEST_DATABASE_URL` 显式指定**独立测试 PostgreSQL**，每个用例创建自己的 schema，结束后删除。覆盖销售/租户隔离、合并回滚、24 路并发发券与核销、SOP 幂等。Redis 用例通过 `SCRM_TEST_REDIS_ADDR` 指定独立测试实例；未设置时这两类集成测试会跳过。

```bash
SCRM_TEST_DATABASE_URL='postgres://USER:PASSWORD@127.0.0.1:PORT/TEST_DB?sslmode=disable' go test ./app/services -run Regression -count=1
SCRM_TEST_REDIS_ADDR='127.0.0.1:PORT' go test ./common/delay_queue -count=1
```

`internal/dispatch` 测试会检查重复动作及相邻 `scrm-app/dashboard` 中的静态 CRM 调用契约；独立检出 core 时跳过前端扫描。

## 日常业务流程与验收

当前已验证客户建档、公海流转、线索转化、商机阶段、报价转合同与回款。渠道真实发送、退款退券、进销存等模块仍需各自的场景验收。

- 新建客户默认归当前销售；勾选“放入公海池”提交 `pool_status=1`，明确清空归属。批量导入客户进入公海，历史无归属且未处于转交中的记录也会出现在公海，可直接认领，无需改写存量数据。
- 销售可以释放自己的未锁定客户；管理员/经理可以分配公海客户。认领上限、冷却期、归属变更与操作日志统一在事务内校验，多人争抢同一客户仅一人成功。客户批量转交后进入目标销售的待接收列表。
- 新跟进中的“下次跟进时间”同步到客户提醒；留空表示本次跟进结束、不另设下一次提醒。跟进人取登录账号，不能代填其他员工。
- 合同列表的“回款”入口支持登记、确认、作废和查看余额；新合同的已收金额从回款登记产生，不能直接编辑已收金额或收款状态。历史合同已有的手工已收余额在新增回款时保留，应在上线前核对其与历史流水的差额。
- 回款计划按 `period_no` 区分期次，不允许同一合同重复期次或计划总额超出应收。只有确认且无需审批/审批通过的回款计入已收；部分回款不会将整期标成已收清。作废、修改和审批会同步重算订单与计划，金额按分比较，禁止累计超额。
- 当前合同应收口径为 `amount - coupon_amount`。保存合同与优惠券核销处于同一事务；优惠券必须属于合同客户并达到使用门槛。取消合同不会自动退回优惠券，需另行制定退款退券流程。
- 有回款的合同必须先完成回款冲销，再作废；已有关联回款或计划的合同保留历史记录，禁止直接删除。作废合同的计划标记为 `cancelled`。
- 业务动作的记录 ID 从 JSON 请求体读取；表单日期接受 RFC3339、`YYYY-MM-DD` 和 `YYYY-MM-DD HH:mm:ss`。未带时区的日期按北京时间处理，空的可选时间转为 NULL。

在独立测试 PostgreSQL 中执行业务验收：

```bash
SCRM_TEST_DATABASE_URL='postgres://USER:PASSWORD@127.0.0.1:PORT/TEST_DB?sslmode=disable' go test ./app/services -run 'Test(CustomerLifecycle|PoolQuotaAndRecycle|ReceivablesLifecycle|PaymentConcurrency|OrderCouponAtomic|WorkflowEditingGuards)Regression$' -count=1
go test ./internal/dispatch -run 'Test(FormTimeBinding|ActionIDsComeFromPayload|DashboardContracts)$'
```

人工验收可依次操作：新建公海客户 → 销售认领 → 记录跟进及下次时间 → 创建合同 → 制定分期计划 → 登记部分回款 → 补足本期 → 作废一笔回款。每一步核对客户归属、跟进提醒、合同余额与计划状态。


## 销售链路验收（线索 → 商机 → 报价 → 合同）

- 线索新建默认 `new`。领取、转移、入池、标记联系/流失以及转客户均校验租户、有效员工与归属。转客户保留原线索负责人，池中线索默认归操作人；流失线索先恢复，已转化线索继续在客户中办理业务。
- 线索转客户在事务中锁定来源记录，重复提交返回同一客户。同手机号的并发线索转化串行去重；已关联客户直接复用。已有客户归属不同、在公海或手机号匹配冲突时明确报错，不暴露其他销售客户或抢占归属。跟进历史同步关联到客户；任何落库失败会回滚新客户。
- 商机需要关联可操作的客户。新商机默认进入第一个非终态阶段；无阶段配置时保持进行中。单条/批量改阶段、表单编辑、赢单/输单使用同一跟进规则；规则读取失败不放行。状态、阶段、概率、预计收入及赢单/输单时间同步保存。已关闭商机只能通过恢复操作回到首个非终态阶段；赢单/输单操作要求预先配置对应终态阶段。
- 报价 `discount` 的口径是**优惠金额（元）**，`final_amount = amount - discount`，使用分进行校验、后端计算。页面不再提交百分比或手工最终金额。历史版本页面曾把此字段标成百分比，已有报价需要在上线前核对原始约定价格；本次不自动改写历史金额。
- 报价必须先建草稿，草稿可编辑和删除；发送后可登记客户接受/拒绝。页面“标记已发送”只登记状态，不会自动给客户发消息。过期报价不能新发送或接受；作废仅限草稿/已发送状态，必须填写原因。单号生成不再依赖当日记录数量，已有报价的补号操作会实际保存。
- 已接受报价可一键转合同，复制主题、客户、商机、负责人及最终金额，生成**合同草稿**并保存 `crm_quote.order_id`。报价优惠已计入合同 `amount`，不能再扣减一次。重复点击返回原合同；合同与报价回链在同一事务中保存。已转合同报价和关联的合同保留业务记录，不能直接删除。
- 合同详情展示合同信息、金额及签约日期，可确认签约并打开回款管理。专用签约接口只变更锁定草稿的状态与签约日期，不回写客户端旧金额。

上线前需新增 `crm_quote.order_id`：开启 AutoMigration 时由模型迁移创建；关闭时应执行 `migrations/0008_quote_order_link.sql`，或按本仓库迁移流程运行 `go run ./cmd/migrate`。迁移只增加可空字段与索引，不回填或修改原报价金额。本轮测试使用无业务数据的临时 PostgreSQL，没有执行生产迁移。

```bash
SCRM_TEST_DATABASE_URL='postgres://USER:PASSWORD@127.0.0.1:PORT/TEST_DB?sslmode=disable' go test -race ./app/services -run 'Test(LeadConversion|LeadPool|QuoteWorkflow|QuoteConversionRollback|OpportunityWorkflow|SalesJourney)Regression$' -count=1
```

人工验收顺序：新建线索 → 标记已联系 → 转客户 → 创建关联商机并记录跟进 → 推进阶段 → 新建报价（1000 元、优惠 100 元，应付 900 元）→ 标记已发送 → 客户已接受 → 转合同 → 核对合同金额与来源 → 确认签约 → 登记 900 元回款。另验重复点击转客户/转合同、他人记录、超额优惠、过期报价以及批量操作中一条记录失败时的整批回滚。

## 首期商用范围与补充验收（2026-10-15）

首期范围已确定为**核心 CRM + 多渠道获客**。进销存、OA、复杂营销及售后等扩展模块分期验收。以下代码回归证明特定流程的正确性，不代表所有部署环境或真实渠道已经通过商业交付验收。

本轮已补齐：

- 每次 CRM 业务请求重新检查有效用户、平台管理员、企业状态与套餐有效期，停用账号的旧令牌不再继续访问业务。到期企业仍可读取本人及套餐信息并办理续费。网关保留 HTTP 401/403 语义。
- 联系人继承客户的数据权限，未关联联系人作为企业共享联系人池。主要联系人变更串行处理，转客户保留操作人归属并防止重复建档；有跟进/报价的联系人不能随意改绑客户。
- 跟进及其评论、拜访记录检查关联客户权限。线索与客户不匹配、未领取或已转化线索不能继续新增跟进。删除跟进后重新计算客户的下次提醒。
- 合同产品、附件、进度按父合同校验权限。产品小计由后端计算，折扣为百分数，100 为原价，0 为免费；有明细时签约要求合计等于合同金额。签约后锁定产品与附件删除，已完成/作废合同不再修改履约节点。撤回完成状态时清除完成时间。
- 批量操作使用正确的 `entity_type`、`entity_ids`、`op_type` 和 `field_key`，线索和客户分别调用对应工作流。输入 ID 去重，整批检查权限，不允许通过改字段绕过状态/负责人控制。先保存执行记录再执行业务，结果写入失败时提示先核对业务，不能把自动重试当作幂等保证。
- 客户、线索、商机、合同、联系人删除与回收站登记同一事务提交。管理员/经理可恢复或彻底删除，恢复前检查关联主档和负责人，彻底删除检查包含软删除历史的关联数据。回收站页面已修正合同分类、名称、删除时间和单条恢复参数。
- 销售阶段配置限管理员/经理维护，不能删除已有商机的阶段，或直接改变该阶段的终态/概率而留下不一致商机。
- 名片、落地页改用随机分享标识；历史冲突名片码拒绝模糊匹配，须重新创建名片并更新分享链接。公开页面检查企业状态，名片还检查有效销售。名片重复留资复用同一销售、手机号下的进行中线索；落地页创建客户、线索、活动参与和计数使用同一事务。留资响应不返回内部客户或线索资料，标题和模板文本做 HTML 转义。
- 合同详情支持直接上传非空、最大 32 MiB 的文件，网关保存到独立私有目录，每次下载重新检查用户状态、合同权限及文件摘要。上传标识与内容绑定，并发重试只登记一条附件；登记超时保留文件供原请求重试，删除的附件不能通过重试重新出现。历史 URL 继续显示为外部链接，不会自动获得私有访问控制。
- 名片公开接口和落地页使用 Redis 原子共享限额，同一 IP 的留资共用每分钟 20 次、每小时 200 次预算；点赞每分钟 20 次、每小时 100 次；浏览每分钟 120 次。超限返回 HTTP 429 与重试秒数，页面保留输入并显示倒计时。Redis 异常时公开入口返回 503，不进行无上限写入。网关默认不信任转发 IP 请求头。

升级时，除 `0008_quote_order_link.sql` 外，还应按仓库迁移流程应用 `0009_recycle_legacy_records.sql`。后者仅补录历史软删除记录的回收站索引，不恢复业务数据、不编造历史删除人；保留原删除时间，并可重复执行。历史补录使用保留的负数回收站 ID，与应用生成的正数 ID 分离。仅开启 AutoMigration 不会执行这项数据补录。先在备份副本验证迁移，再按部署流程执行。

本次还新增 `0010_private_contract_attachments.sql`，添加私有存储键、SHA-256 摘要及唯一索引，保留历史 URL。升级顺序为数据库迁移 → core 与 gateway → dashboard。`internal_crm_attachment` 仅供受信网关调用，通用 HTTP 业务入口拒绝所有 `internal_` 服务；core 的 gRPC 端口仍必须限制为内网网关可达。

网关通过 `CRM_PRIVATE_UPLOAD_DIR` 指定附件目录，默认 `private_uploads`。正式部署须挂载持久化卷，多个网关实例共享同一支持原子硬链接发布的文件系统；不能配置在公开 `uploads` 目录或由其他静态服务器暴露。数据库与附件应作为配套备份恢复。超时未登记文件、软删除附件暂保留，人工对账确认后再按保留策略清理，禁止按文件年龄直接删除可能仍有业务引用的文件。需监控磁盘容量；当前未提供企业存储配额和自动清理任务。

若经过反向代理，在 gateway 和提供 `/landing` 的 callback 服务分别设置 `TRUSTED_PROXY_CIDRS` 为实际受信代理的 IP/CIDR（逗号分隔），代理须覆盖客户端可伪造的转发头。未设置时只使用连接对端 IP；不能把所有网段配置为可信。公开入口共享同一 Redis，配置后应从公网验证不同终端的识别与限流效果。此策略仅覆盖名片和落地页，不表示登录、真实渠道回调及扩展营销端点均已完成防滥用验收。

新增 PostgreSQL 回归覆盖账号状态、联系人并发、跟进边界、合同子项、批量操作/回收站、阶段配置、重复获客、获客失败回滚和历史回收站迁移。运行全部服务回归（含竞态检查）：

```bash
SCRM_TEST_DATABASE_URL='postgres://USER:PASSWORD@127.0.0.1:PORT/TEST_DB?sslmode=disable' go test -race ./app/services -count=1
SCRM_TEST_REDIS_ADDR='127.0.0.1:TEST_REDIS_PORT' go test -race ./common/publiclimit ./internal/grpcserver ./app/callback -count=1
go test ./...
# 相邻 scrm-app/gateway：go test ./...
# 相邻 scrm-app/dashboard：npm run build
```

此前已完成 25 项 PostgreSQL 业务回归；本次增加私有附件并发、合同权限变化、旧数据迁移、上传响应丢失、损坏文件、代理 IP 与 Redis 共享限流验证。前端仍有大体积共享包构建提示，自动回归不替代真实浏览器及生产网络性能验收。测试仅使用独立临时 PostgreSQL/Redis，未操作生产数据库。

本次实测结果：27 项 PostgreSQL 业务回归与 Redis/公开入口测试在 `-race` 下通过；core、gateway 全包测试及网关关键路由竞态测试通过；本地前端构建、干净 Node 22 容器依赖安装与构建、Nginx 配置及 Compose 解析通过。临时前端容器的首页、登录 SPA 路由与 JavaScript 资源均返回 200；此检查仅验证资源服务，不代表页面端到端业务验收。测试容器检查后清理。

首期正式上线仍需完成以下交付项，不以已有菜单或模拟数据替代验收：

| 项目 | 当前边界 | 放行依据 |
| --- | --- | --- |
| 多渠道接入 | 企微及其他渠道需要真实凭据与回调环境，公众号/抖音/小红书的 JSON HMAC 适配不能直接当作各平台官方协议已验收 | 每个选定渠道完成新增/重复回调、归属、失败重试、停用与凭据失效测试，核对客户及来源记录 |
| 合同文件 | 新附件已受控上传/下载；历史 URL 仍是外部链接，尚未提供对象存储、配额与自动清理 | 迁移敏感历史文件，验证共享持久化卷、备份恢复、容量告警及实际上传下载 |
| 公网防滥用 | 名片/落地页已接 Redis 共享限流，尚无验证码和异常提交告警 | 配置可信代理并在实际入口压测，覆盖共享出口 IP 的正常咨询和 Redis 故障恢复 |
| 财务及历史数据 | 当前回款口径已校验，但历史手工已收余额、旧报价折扣需要核对，作废合同不会自动退款退券 | 财务确认金额口径和历史差异，明确退款、退券与冲销流程 |
| 上线运行 | 本轮使用临时 PostgreSQL/Redis 和前端容器，本地构建及回归没有替代生产演练 | 数据库/附件备份恢复、升级回滚、日志告警、渠道失败监控与业务量压测通过 |
| 角色及业务验收 | 核心按销售归属、客户共享、管理员/经理权限处理，扩展模块另行验收 | 用两个企业及管理员、经理、销售账号完成页面验收；首期菜单只开放已验收模块 |

建议人工验收补充路线：两个销售同时设置主要联系人 → 联系人转客户并重复提交 → 新建跟进及评论 → 添加合同产品明细并故意制造合计差额 → 修正后签约 → 批量线索入池 → 删除空主档 → 回收站恢复；再连续提交名片/落地页表单并核对客户、线索、活动参与和计数。以无权限账号重复相同操作，确认数据不泄露且无部分写入。

附件与公开入口补充验收：上传 PDF/表格 → 模拟登记响应丢失后原文件重试 → 换销售和企业尝试下载 → 转交合同后核对新旧负责人权限 → 删除草稿合同附件并再次访问 → 网关重启后下载；公开表单超限后确认 429、倒计时和输入保留，Redis 恢复后确认留资成功且没有重复线索。
