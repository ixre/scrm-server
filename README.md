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
- 鉴权在 gateway 的 HTTP 层；本仓库只应监听内网、仅允许 gateway 可达，不得对外直接暴露。
- 内部调用认证：core 与 gateway 必须配置相同的 `SCRM_INTERNAL_SECRET`（至少 32 字节，与 `CRM_JWT_SECRET` 分开），缺失或过短时两端拒绝启动。gateway 对每个 gRPC 调用的「方法 + 时间戳 + 客户端 IP + 确定性序列化的请求体」做 HMAC-SHA256，放在 `x-scrm-auth-ts` / `x-scrm-auth-sig` metadata；core 拦截器（`internal/grpcserver/internal_auth.go`）验签并允许 ±60s 时钟偏差，未签名、篡改或过期的调用返回 `Unauthenticated`。因此旁路进程即使能连到 :9001 也无法伪造 `tenant_id`/`is_platform` 等身份字段。
- 签名覆盖整个请求体：proto 新增字段时须先发布 core、再发布 gateway（gateway 先行会因 core 不认识新字段导致验签失败）。签名不提供传输加密，跨主机部署仍应加 TLS/mTLS。
- 租户隔离默认拒绝（fail closed，`app/models/tenant_guard.go`）：GORM 插件拦截所有含 `tenant_id` 列的表，查询/更新/删除必须在 WHERE 顶层 AND 条件中带非空 `tenant_id`（括号外出现 OR、空值、仅出现在子查询中都不算），新增必须带 `TenantID`；更新/删除已加载的单条记录（`Model(&obj)` / `Delete(&obj)`）时自动按其 `TenantID` 追加条件。违规语句在执行前返回 `ErrTenantScopeMissing` 并记 WARN 日志。`TenantScope("")` 返回 `ErrMissingTenant`，不再退化为全表访问。
  - 业务代码使用 `models.MustFor(ctx)` / `models.For(ctx)` / `Scopes(models.TenantScope(t))`；确需跨租户（定时任务扫描、平台超管、回调按企业/分享码反查租户、种子与迁移）必须显式写 `models.CrossTenant()` / `models.CrossTenantDB(tx)` / `models.CrossTenantContext(ctx)`，便于检索审计。
  - 模式：`App.TenantGuard`（CRM 表，默认 `enforce`）与 `App.LegacyTenantGuard`（老企微表 customer/staff/department/group_chat/mass_msg/material/contact_way，默认 `log`，也接受 `ext_corp_id` 条件）。老企微表仍有按 `ext_id` 等非企业键的查询，随企微多企业改造收敛后再切 `enforce`。`log` 模式仅用于灰度排查，生产 CRM 表不要降级。
  - 守卫不覆盖 `Raw`/`Exec` 原生 SQL 与作为参数传入的子查询，这些须人工保证带租户条件。
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
- **模型结构同步**：改 Model 定义，`AutoMigration: true` 时启动会同步表、字段及索引，也可能调整已有列定义。正式环境关闭启动迁移，由部署流程控制。
- **需要版本化的字段调整和数据迁移**：在 `migrations/` 加 `.sql` 文件，然后执行：
  ```bash
  go run ./cmd/migrate -schema-only   # 仅执行结构迁移，不额外导入旧企微数据到 CRM
  ```
  以文件名为版本号记录进 `schema_migrations`，幂等；是否执行 AutoMigrate 取决于配置。本命令不初始化种子数据、不依赖 Redis；不加 `-schema-only` 时仍按原行为调用 `BackfillCRM` 导入旧企微数据，执行前应另行核查映射。

**旧版迁移兼容修复（2026-10-16）**：`0006_strict_tenant_id.sql` 保留历史文件名，内容已改为与运行中的企微模型兼容的 DDL：七张旧渠道表的 `tenant_id` 允许 NULL，保留已有值；恢复客户 `ext_id`、群聊 `ext_chat_id`、员工/部门 `(ext_corp_id, ext_id)` 的完整唯一索引，匹配现有 `ON CONFLICT`，移除旧版不兼容的租户唯一索引。不再引用群发、素材、渠道码上不存在的 `ext_id`。已登记旧 `0006` 的库由新增 `0014_legacy_tenant_compatibility.sql` 执行同一修复；未执行的库正常运行修正版 `0006` 后再执行 `0014`，无需手工篡改迁移记录。迁移事务使用共享 advisory lock，并在锁内复查版本，避免并发执行同一迁移。

`0006`/`0014` 是兼容过渡版本；最终严格租户改造由 `0015_legacy_tenant_enforcement.sql` 完成。若曾放宽旧业务键并产生重复历史记录，兼容迁移会在恢复唯一索引时失败并回滚，需核对重复记录的业务归属，不会自动删数据。

**严格租户改造（0015）**：`customer`、`staff`、`department`、`group_chat`、`mass_msg`、`material`、`contact_way` 的 `tenant_id` 回填后统一设为 NOT NULL。迁移检查所有历史记录（包括软删除记录）、有效企微渠道配置及租户映射，未知企业、重复映射、无效租户或已有租户值冲突都会让该版本完整回滚；不创建默认租户、不猜测归属。应先根据已核实的业务归属补齐 `channel_link(channel='wecom', entity_type='tenant', ext_id=企业ID, entity_id=tenant_id)`，再重试迁移。

数据库触发器覆盖 ORM、回调及直接 SQL 的新增写入：未提供租户时按企业映射补齐，提供了不一致租户则拒绝。历史记录禁止更改企业/租户，已生效的企业租户映射禁止删除、软删除或改绑，对应租户禁止物理删除或改主键（仍可停用、软删除）。映射查询使用新增的企业唯一索引，普通更新只比较归属字段，不额外查询租户表。保留与现有 `ON CONFLICT` 一致的业务唯一键，不新增物理外键，也不放开旧客户/群聊外部 ID 的全局唯一性。

新增或更新企微渠道配置会在同一事务内建立/核对租户映射，配置保存失败时映射一起回滚。已接入企微配置的企业 ID 和渠道不可直接改绑；可停用原配置，再建立新配置，历史归属保留。七个模型的租户字段由版本迁移管理，AutoMigrate 不会将 NOT NULL 改回可空。群聊列表同时按企业约束主表与员工关联，防止同名员工跨企业匹配。

客户和群聊 `Upsert` 同时校验冲突记录的企业，不能借全局外部 ID 覆盖另一企业；客户跨批次写入遇到归属冲突时整批回滚。群成员按实际的 `(ext_chat_id, userid)` 唯一键更新，空集合直接返回，批量核对所属企业群聊；群聊与成员写入在同一事务内，成员失败不会留下部分群聊更新。

升级顺序：备份 → 停止旧服务及同步/回调消费者 → 在副本检查并执行 `go run ./cmd/migrate -schema-only` → 在目标库执行同一命令 → 部署新版服务。`0015` 会锁定相关表、扫描和回填历史数据，百万级库应安排维护窗口并预估 WAL/磁盘空间；这是一次性升级成本，不是在线无停机迁移。不要在升级至 `0015` 后单独重跑 `0006`/`0014` 的 SQL；使用带版本账本的迁移入口。包含函数体的 SQL 使用文件首行及独立行 `-- migrate:statement` 分隔语句，执行器不会拆开函数体内的分号。

升级先备份并在副本执行上述命令。修复若需要重建已被删除的唯一索引，会扫描旧表并阻塞写入，应安排维护窗口。兼容性回归：`SCRM_TEST_DATABASE_URL='postgres://USER:PASSWORD@HOST:PORT/TEST_DB?sslmode=disable' go test -race ./internal/migrate -count=1`，覆盖未执行/已执行旧版本、实际写入、冲突回滚、幂等及并发迁移。

严格租户回归还覆盖回填失败回滚、映射删除/改绑保护、并发接入与租户删除、AutoMigrate 后约束保留、群聊企业隔离，以及 1 万条企业映射下的索引命中和 5000 条受保护写入。渠道接入事务回归：`SCRM_TEST_DATABASE_URL=... go test -race ./app/services -run '^TestWecomTenantProvisioningRegression$' -count=1`。

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

本轮没有一次性替换所有旧企微关联：客户与员工、员工部门、标签、群聊、会话消息、自动拉群码等仍有原有 GORM 关联，需分别覆盖同步/回调和历史数据迁移后再拆除。尤其自动拉群码的备用员工继承了普通员工的表名，需要先明确存量数据归属。`0006` 的历史兼容问题已按上文修复；此次重构不等于全部渠道功能已完成商用验收。

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

- Gateway 必须配置至少 32 字节的 `CRM_JWT_SECRET`，不再回退固定默认密钥。生成随机值后通过环境变量或密钥服务注入，所有网关实例使用同一个值。
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


### 0016：销售跟进与回款提醒闭环

产品评估、验收口径与后续缺口见 [CRM 产品评估](docs/crm/README.md#2026-10-17-产品补齐评估核心-crm--多渠道获客首期)；升级与提醒生命周期见 [待办中心](docs/crm/office/05-待办聚合.md)。新增事件去重、来源 ID、处理时间和租户复合索引，不新增物理外键。旧未处理跟进/回款提醒会保留为“已失效”，新调度基于当前来源重新生成，客户/合同/实际回款不变。

在备份与维护窗口中运行 `go run ./cmd/migrate -schema-only`，然后启动新服务使 5 分钟调度生效。先执行迁移再部署；不要只依靠 AutoMigrate 代替版本迁移。可用隔离 PostgreSQL 运行 `SCRM_TEST_DATABASE_URL=... go test -race ./app/services -run 'TestReminder|TestFollowupWorkday|TestQuickRemarkWorkflow' -count=1` 验证。


### 0017：渠道留资与人工引流任务闭环

迁移 `0017_channel_task_workflow.sql` 新增任务截止/处理时间、处理人、结果、版本和事件幂等键，新增任务日志表，并为渠道互动增加事件指纹。创建 9 个索引，覆盖租户任务列表、接收人/状态、历史任务查重、到期扫描、日志、客户渠道身份及事件唯一性。不增加物理外键，不删除旧任务或互动，不编造历史日志；旧任务缺少截止时间时按创建时间 +48 小时补齐，旧事件键保留为空。

备份后先在副本运行 `go run ./cmd/migrate -schema-only`，核对历史行及新索引，再升级目标库并同步部署 backend/dashboard。任务操作新增必填版本及处理说明，旧前端需刷新。AutoMigrate 不能代替历史截止时间回填。迁移的普通建索引和回填可能阻塞写入，百万/千万行环境安排维护窗口并在真实容量副本测量耗时；不能声称无停机升级。

数据库回归：`SCRM_TEST_DATABASE_URL=... go test -race ./app/services -run 'Test(ChannelTask|XhsLead)' -count=1`。业务规则、接口与边界见 [引流任务](docs/crm/channel/06-跨渠道任务.md) 和 [小红书](docs/crm/channel/03-小红书.md)。本轮提供人工任务闭环与标准回调事务，不代表真实渠道账号、通用自动分配规则、生产指标或千万行混合负载已验收。

本地升级记录（2026-10-17）：已备份 `tmp/db-backups/scrm-before-channel-workflow-20261017.dump`，在恢复副本验证版本迁移、AutoMigrate 和重复执行后，将本地 `scrm` 更新至 `0017`。校验 176 张既有表的行数和旧字段内容未变、9 个新索引有效、物理外键仍为 0。该本地库原有引流任务 0 条、渠道互动 5 条；历史任务截止时间回填另由隔离数据库用例覆盖。报告保存在 `tmp/channel-upgrade/scrm-verification.json`。尚未重启业务服务，启动新版服务和刷新 dashboard 后新功能才会生效。


### 0018：客户历史游标分页与员工远程选择

迁移 `0018_bounded_crm_reads.sql` 创建 6 个索引：跟进、活动参与、非企微互动及转移历史的租户/客户/有效时间/ID 索引，以及员工的租户/状态/ID、租户/ID 索引。历史时间为空时使用与查询相同的回退表达式。仅新增索引，不改业务字段、数据或物理外键；暂不删除旧索引，后续依据生产查询及写入成本评估合并。

备份后在副本运行 `go run ./cmd/migrate -schema-only`，验证后升级目标库，再同步部署 backend/dashboard。`GetDetail` 的历史数组改为最近 20 条，并返回分页元数据，新前端提供查看更早入口；旧客户端应同步升级。旧跟进/活动 `ListByCustomer` 超过 1000 条会明确报错，调用方需改用 `crm_customer.History`。普通 `CREATE INDEX` 会阻塞相关表写入，大容量环境应安排维护窗口并先测量实际耗时；本迁移不承诺在线无阻塞建索引。

本地升级记录（2026-10-17）：备份 `tmp/db-backups/scrm-before-paged-reads-20261017.dump`，恢复到独立副本验证版本迁移、AutoMigrate 及重复迁移后，将本地 `scrm` 更新至 `0018`。177 张业务表的行数与完整内容校验一致，6 个新增索引有效，物理外键仍为 0；报告为 `tmp/paged-upgrade/scrm-verification.json`。未重启业务服务，运行中的旧进程仍需按部署流程切换到新版。

验证：`go test ./...`、隔离 PostgreSQL 下完整服务层竞态回归、dashboard 类型检查及构建通过。专项用例：`SCRM_TEST_DATABASE_URL=... go test -race ./app/services -run 'Test(CustomerHistoryCursor|UserOptionsBeyondFirst500)$' -count=1`。测试覆盖游标边界和第 500 名之后的员工选择，但不等于千万行容量验证；员工包含搜索目前仅限制在本租户内，未配置全文索引。功能协议见 [客户画像 360](docs/crm/customer/08-客户画像360.md)。


### 0019：公海回收批处理与日志访问

`0019_pool_recycle_indexes.sql` 增加 4 个索引，覆盖待回收客户游标、有效商机存在性判断、租户公海日志和单客户日志。不新增字段、外键或修改业务行。普通建索引会阻塞相关表写入，生产需安排维护窗口并先在真实容量副本测量。

升级前备份，在副本执行 `go run ./cmd/migrate -schema-only` 验证后再升级目标库，同步部署 backend/dashboard。公海详情新增 `crm_pool.Get`；日志 POST 使用 `pager` 包装；页面分页顺序已修正。公海日志及今日操作数量按当前客户读取权限过滤。规则保存继续仅限管理员，并增加服务层校验、参数范围和原子 upsert。

自动回收每批最多 200 客户，客户状态和日志同事务；并发跳过占用中的行，下次调度重试。每租户最多执行 2 分钟，失败批次回滚且保留之前成功批次，日志包含已提交数量。停用/到期租户不参加全租户回收。具体业务条件与边界见 [客户公海与池](docs/crm/customer/05-客户公海与池.md)。通用自动分配规则执行器仍未交付。

本地升级（2026-10-17）：备份为 `tmp/db-backups/scrm-before-pool-recycle-20261017.dump`。副本验证版本迁移、AutoMigrate 和重复执行后，本地 `scrm` 已更新至 `0019`；177 张业务表行数与完整内容未变，4 个新增索引有效，物理外键仍为 0。校验报告为 `tmp/pool-upgrade/scrm-verification.json`。只执行数据库升级，没有启动业务调度或重启应用。

验证：`go test ./...`、隔离 PostgreSQL 全服务竞态回归及最后的 `TestPool` 专项通过；dashboard 请求契约测试 8 项、类型检查及构建通过。专项覆盖 450 客户多任务并发、200 条批次审计失败回滚/重试、规则并发首存、客户当前权限及租户生命周期。隔离测试库关闭磁盘同步等待，不作为持久性或千万行性能验收；真实浏览器与生产容量验证仍待执行。


### 0020：新获客自动分配

迁移 `0020_acquisition_allocation.sql` 为规则新增 `execution_enabled`（默认 false）、来源 `sources` 及轮询位置 `last_target_id`，新增 `crm_allocation_log` 和 4 个执行/日志/幂等索引。没有物理外键；已有规则的启用状态及其他旧字段保持不变，升级不会自动把旧配置变成执行规则。

备份后先在副本执行 `go run ./cmd/migrate -schema-only`，再升级目标库，最后同步部署 backend/dashboard。普通 ALTER/CREATE INDEX 可能阻塞写入，正式环境安排维护窗口并测量大容量副本耗时。旧客户端没有执行开关，保存规则时会将执行关闭，部署后应刷新 dashboard。

管理员/经理在系统设置新建或编辑规则：对象选客户，策略选轮询/随机，选择最多 100 名有效销售及小红书/落地页来源，同时开启规则与“执行新获客分配”。仅在两个入口首次创建新客户时执行；旧客户、再次留资、手工录入/导入、名片和其他渠道不重新分配。独立线索、权重、旧条件表达式暂不支持执行，接口会拒绝而不是忽略条件。含旧条件的规则建议保留并新建来源规则。

按优先级、规则 ID 倒序命中第一条规则；轮询按保存的目标顺序推进，随机策略在可用候选中选择。账号锁与公海认领配额检查一致，超过客户持有上限或停用销售不接收。最高优先级规则没有可接收销售时客户留在公海，并写明原因，不自动改投低优先级团队。后续需人工分配，不存在自动重试积压队列。

获客、归属、轮询位置、公海日志和分配日志同事务，失败时一并回滚；落地页线索及小红书引流任务继承分配后的客户负责人。执行记录接口仅管理员/经理可读，使用 ID 游标，默认 20、最多 100 条；不进行全量计数或 OFFSET 翻页。单客户仅一条首次分配记录，历史记录不回填、不编造。

验证覆盖 `TestAllocation*`、已有公开获客及小红书事务回归：轮询并发公平、重复事件幂等、渠道任务/线索负责人一致、来源与优先级、停用/跨租户目标、容量上限、公海兜底、审计失败及轮询位置回滚、人工认领并发争用。运行 `SCRM_TEST_DATABASE_URL=... go test -race ./app/services -run 'TestAllocation|TestPublicAcquisition|TestXhsLead' -count=1`。这些结果不代表真实渠道协议、浏览器流程或大租户吞吐已经验收。


本地升级记录（2026-10-17）：备份 `tmp/db-backups/scrm-before-acquisition-allocation-20261017.dump`；恢复副本验证版本迁移、AutoMigrate 和重复迁移后，本地 `scrm` 更新至 `0020`。177 张既有业务表行数和旧字段内容一致，新增日志表为空、4 个新索引有效，既有规则的执行开关全部保持 false，物理外键仍为 0。报告为 `tmp/allocation-upgrade/scrm-verification.json`。未重启应用、未开启任何业务规则。

本轮验证结果：后端 `go test ./...`、隔离 PostgreSQL 下全服务 `go test -race ./app/services -count=1` 通过；dashboard 类型检查、构建及 10 项请求契约/反馈测试通过。临时数据库已用于事务逻辑验证，不作为持久性和容量 SLA 依据。前端保留原有大包提示；真实浏览器及渠道账号联调尚未执行。


### 0021：客户成交与回款总览

迁移 `0021_customer_business_reads.sql` 新增 9 个索引，支持客户范围下的合同/报价/商机个人与管理视图、合同计划和回款游标，以及有效回款按计划核销求和。不新增物理外键、业务字段、缓存汇总或改写历史金额；旧索引暂保留，后续依据真实读写负载评估合并。

先备份，在恢复副本验证 `go run ./cmd/migrate -schema-only` 后升级目标库，再同步部署 core/dashboard。普通建索引会阻塞相关表写入，生产环境需维护窗口与实际容量副本耗时评估，不能承诺无停机升级。

客户详情新增“成交与回款”，汇总当前可见的有效签约/完成合同应收、已收、未收，显示逾期计划未核销，并分页查看报价、合同、计划、回款；合同链接进入现有合同详情继续处理。商机改为游标翻阅，修复原先前 50 条限制及错误名称字段。每个页面按需加载，切换客户或页面后忽略旧响应；接口错误显示失败和重试。

`crm_customer.BusinessSummary` POST `{customer_id}`，汇总采用只读一致性快照，金额输出定点十进制文本。`BusinessRecords` POST `{customer_id,kind,cursor,page_size}`，类型为 `opportunities/quotes/orders/plans/receipts`，每页默认 20、最多 100，ID 游标不执行全量 COUNT/OFFSET。游标绑定租户、客户及类别，每页重新检查客户读取权及业务负责人范围。共享客户不会扩展成其他销售合同/报价可读。

计划与回款以同租户、未删除父合同判定归属，历史客户快照不参与访问授权；核销仅计有效关联回款，排除待确认、作废、待审、驳回及软删除。合同已收保留历史登记余额，未关联期次的回款不自动分摊。故逾期计划未核销与合同未收分别展示，催款前须核对；草稿及作废合同不计汇总，历史明细保留。具体产品口径与验收见 [客户画像 360](docs/crm/customer/08-客户画像360.md)。

查询仅处理当前客户及可见合同，执行期限为 5 秒；大客户汇总仍需读取其相关业务行。索引与分页不能代替真实多租户、千万行混合读写验收。


本地升级记录（2026-10-17）：备份 `tmp/db-backups/scrm-before-customer-business-20261017.dump`。恢复副本验证版本迁移、AutoMigrate 和重复迁移后，本地 `scrm` 已更新至 `0021`。178 张既有业务表行数及完整内容一致，9 个新增索引有效，物理外键仍为 0。报告为 `tmp/business-upgrade/scrm-verification.json`。未重启业务应用、未开启额外分配规则，未改动客户或历史金额。

专项验证：`SCRM_TEST_DATABASE_URL=... go test -race ./app/services -run 'TestCustomerBusiness|TestCustomerAccess' -count=1` 通过。用例覆盖共享与父合同权限、草稿/作废/软删除排除、审批有效性、错误客户快照、跨合同核销、日期即时逾期、125 商机翻阅、100 条上限、交接撤权和定点金额精度；前端金额格式化及接口请求契约也已覆盖。真实浏览器、渠道账号和大容量混合负载尚需现场验收。


最终验证：后端 `go test ./...`、隔离 PostgreSQL 完整服务层 `go test -race ./app/services -count=1` 均通过；dashboard 最终类型检查和构建、13 项请求/金额测试通过。前端仍有原有大包提示。隔离测试库关闭磁盘同步等待以缩短初始化耗时，这些用例不验证断电持久性或生产容量 SLA。临时测试与迁移副本清理，保留备份和校验报告。


### 0022：回款请求去重与远程期次选择

本轮针对实际收款任务补齐三个问题：登记表单只取前 500 期、网络响应丢失后重复登记、切换合同后旧请求更新页面。规则、接口、状态与边界详见 [回款](docs/crm/sales/05-回款.md)，已修正该文档过时的字段及未实现路由描述。

迁移 `0022_receipt_requests.sql` 为 `crm_receivables` 增加可空 `client_request_id/request_digest`，创建租户/合同/请求键条件唯一索引及租户/合同/期次/ID 有效计划索引。请求键唯一性包含软删除记录；历史键和摘要保留 NULL，不编造去重依据、不改历史金额，不新增物理外键。

同租户同合同的相同键与原始登记内容重试复用已有记录，余额和新建事件仅处理一次；同键不同内容明确报冲突，作废或删除后拒绝通过重试恢复，失败事务不占用键。重试重新鉴权，并保留原记录当前审批/修改状态。旧客户端可不传键，维持兼容，但没有新去重保证。新版弹窗在本次打开期间保留键，关闭重开及刷新浏览器会生成新键，不能当作银行流水自动对账；遇到不确定结果应先查原记录。

新增 `crm_receivables_plan.Options` POST `{order_id,keyword,cursor,page_size}`，默认 50、最多 100，支持正整数期次精确搜索和 ID 游标，不执行全量 COUNT/OFFSET。选项按同租户同合同的有效回款计算未收金额，不依赖旧计划状态。游标绑定检索范围且每页重新检查合同权限；客户共享不会扩大合同读取权限。旧计划及回款 `List` 接口仍为传统分页，每页默认 20、最多 1000。

部署顺序为备份 → 恢复副本验证 `go run ./cmd/migrate -schema-only` → 数据库升级 → core/dashboard。普通建索引会阻塞相关表写入，大容量环境需维护窗口及真实数据副本测量，索引和分页不代表已经通过千万行多租户混合压测。

本地升级记录（2026-10-18）：备份 `tmp/db-backups/scrm-before-receipt-requests-20261018.dump`；在恢复副本验证版本迁移、AutoMigrate 及重复执行后，本地 `scrm` 已更新至 `0022`。178 张既有业务表行数及旧字段完整内容一致，两个新索引有效，历史请求字段均为空、物理外键仍为 0；报告为 `tmp/receipt-upgrade/scrm-verification.json`。仅启动本地数据库完成升级，未启动或重启业务服务及调度。

验证：后端 `go test ./...`、隔离 PostgreSQL 下完整服务层 `go test -race ./app/services -count=1` 和完整迁移回归通过；dashboard 类型检查、构建和 16 项请求/金额测试通过。专项用例覆盖并发同键仅一笔、金额冲突、失败回滚、审批状态保留、作废/删除不能恢复、交接撤权、605 期完整翻阅及第 501 期搜索。副本演练发现并修复 SQL 注释分号与历史拆分器不兼容的问题，已增加实际迁移执行器回归。前端仍有原有大包提示；测试不代替真实浏览器、渠道联调或生产容量与断电持久性验收。


### 多期合同的回款与计划批量写入优化（2026-10-18）

`reconcilePayments` 原先每期各执行一次有效回款求和及状态更新。现改为仅在当前租户/合同范围聚合、统一校验期次上限并批量更新变化的状态，继续使用父合同行锁，保持历史余额、审批有效性、作废和事务回滚口径。未变化期次不再重复更新 `updated_at`，减少无效写入。

`BatchCreate/GeneratePlan` 限制每次最多 1000 期；按合同统一核对输入内重复、已有期次和总额，每 200 行插入，任意合同或批次失败全部回滚。页面同步限制新增行数；批量编辑删除中间行后新增期次使用当前最大编号 +1，避免按行数编号产生重复。历史合同缺少客户时保留 NULL，普通销售与管理权限不变。

在隔离 PostgreSQL 专项中，创建 1/605 期的 SQL 调用数分别为 6/10，登记一笔关联回款的 SQL 调用数均为 11；不再随合同期次逐行增加往返。用例还验证了不变期次不重复写入、第二/后续插入批次和批量状态写入失败整体回滚、审批与作废、跨租户/合同及软删除排除、并发创建不超过合同金额。专项命令：`SCRM_TEST_DATABASE_URL=... go test -race ./app/services -run 'TestPaymentBatch|TestPlanBatch' -v -count=1`。

复用既有 0021/0022 索引，没有新增数据库变更，本地库保留 0022，未启动或重启业务服务。SQL 调用数是测试场景的证据，不替代真实多租户、千万行混合负载和生产延迟验收。

本轮最终验证：后端 `go test ./...`、隔离 PostgreSQL 完整服务层竞态回归及四项批量专项通过；dashboard 类型检查、构建和 16 项既有请求/金额测试通过。测试容器已清理，本地业务数据库与业务服务未改动，前端仍有既有大包构建提示。


### 0023：五轮财务工作台完善（2026-10-18）

本轮按核心 CRM 的实际任务完成五处补齐：回款审批单号前缀搜索；到期/逾期按有效回款核销并显示合同余额；中国时区完整日历日期；财务列表错误、重试和异步响应隔离；计划表单逐行验证及提交锁定。功能、请求及验收边界见 [回款](docs/crm/sales/05-回款.md)。

催款列表只包含当前有权查看且仍有未收余额的有效签约/完成合同，已收满合同不会因期次未分摊而继续催款。期次未核销和合同未收分别以十进制文本返回，待审、驳回、作废、软删除及跨合同回款不抵扣期次。`days` 表示今天及未来 N 天完整日期，范围 1 至 365、默认 7；后端返回中国日历的天数差，消除宿主机或浏览器时区影响。

迁移 `0023_finance_workbench_reads.sql` 新增两个索引：`idx_receipt_tenant_number_prefix` 支持租户范围内小写单号前缀查找，`idx_plan_tenant_calendar_scan` 支持不依赖旧状态的有效计划日期扫描。不改业务行、字段或物理外键，不删除旧索引。升级顺序为备份 → 恢复副本验证版本迁移及 AutoMigrate → 数据库迁移 → core/dashboard；普通建索引会阻塞相关表写入，需要维护窗口及真实容量副本测量。

本轮自动验证不能代替真实浏览器任务、渠道账号联调、生产恢复和千万行混合负载验收。计划创建仍不自动重试，响应不确定时先核对已有期次。

本地升级结果：已备份 `tmp/db-backups/scrm-before-finance-workbench-20261018.dump`，恢复副本完成版本迁移、AutoMigrate 和重复执行后，本地 `scrm` 升级至 0023。178 张业务表行数及全部字段内容一致，两个新增索引有效、物理外键仍为 0；报告为 `tmp/finance-workbench-upgrade/scrm-verification.json`。没有启动或重启业务服务及调度。

验证结果：后端全包测试、隔离 PostgreSQL 完整服务层与迁移层竞态回归、最终财务专项通过；dashboard 类型检查、构建和 23 项请求/日期/金额/表单测试通过。浏览器完整交互和生产大容量混合负载尚未验收，前端仍有既有大包构建提示。


### 0024：线索预约、提醒与转客户接续

基于 product-manager-toolkit 的实际任务评估，本轮补齐销售线索在转客户前的下一次联系安排：独立预约/取消、跟进时更新下一步、站内待办、交接和状态失效、转客户延续较早预约、完整历史分页及办理链接。导入新增有效操作人检查和 1000 行上限。优先级及尚未补齐项见 [产品评估](docs/crm/README.md)，使用规则见 [线索](docs/crm/sales/01-线索.md)。

迁移 `0024_lead_followup_workflow.sql` 为 `crm_lead` 增加可空 `next_followup_at/next_action`，新增租户预约扫描和跟进历史游标两个索引。历史预约保留 NULL，无数据回填、无新表或物理外键。主档以标量 ID 关联，提醒用既有事件唯一键去重，不逐租户建表，也不全量拉取历史。

升级顺序：备份 → 恢复副本验证版本迁移、AutoMigrate 和重复执行 → 本地数据库迁移 → core/dashboard。普通建索引会阻塞相关表写入，大表安排维护窗口并测量副本耗时。仅升级数据库不会启动新的 5 分钟线索提醒调度。旧 ListByLead 超过 1000 条将明确报错，客户端需同步改用 crm_lead.History。

客户与线索的预约在未转化前独立，转化时保留较早时间，并保留历史；它不是支持多条约会并存的日历。提醒属于站内待办，不等于给客户自动发送消息。通用业务下拉前 500 条限制、导入逐行失败反馈、其他通用列表错误视图和真实渠道验收仍列为后续重点，本轮不把整个 CRM/SCRM 宣称为全部商用验收通过。

本地升级记录（2026-10-18）：备份 `tmp/db-backups/scrm-before-lead-followup-20261018.dump`，恢复副本验证版本迁移、AutoMigrate 和重复执行后，本地 scrm 已升级 0024。178 张既有业务表行数与旧字段内容一致，历史线索预约保持 NULL，两个新索引有效、物理外键仍为 0。报告为 `tmp/lead-followup-upgrade/scrm-verification.json`。未启动或重启业务服务及对外调度。

验证：后端全包测试、完整隔离 PostgreSQL 服务层与迁移层竞态回归、最终线索专项通过；dashboard 类型检查、构建及 25 项请求/日期/金额/表单测试通过。专项验证 605 条线索提醒、1005 条历史分页和预约/交接/转客户生命周期；真实浏览器完整任务、渠道账号与生产容量仍待验收。


### 0025：核心客户与合同远程检索

核心商机、报价、合同、回款计划及回款工作台不再依赖前 500 条客户/合同缓存，改为按需检索、游标分页、独立解析已选项及失败重试。列表只解析当前页名称，回款计划自动使用合同客户；共享可读不会授予新增业务权限，合同仍按签约销售授权。扩展模块旧选择器、管线商机总量上限等尚待逐类处理，详见 [使用和验收说明](docs/crm/sales/08-业务对象选择.md)。

迁移 `0025_business_object_options.sql` 新增 9 个租户游标/负责人游标/前缀表达式索引，无业务行、字段、表或物理外键变更。先备份并在恢复副本验证版本迁移、AutoMigrate、重复执行和数据一致性，再升级本地数据库及 core/dashboard。普通索引构建需要生产维护窗口与容量副本测量；本轮不替代真实浏览器、渠道和千万行多租户负载验收。

本地升级记录：已备份 `tmp/db-backups/scrm-before-business-options-20261018.dump`，恢复副本验证版本迁移、AutoMigrate 和重复执行后，本地 scrm 已更新至 0025。178 张业务表全部字段内容及行数保持一致，9 个新增索引有效，物理外键仍为 0；报告为 `tmp/business-options-upgrade/scrm-verification.json`。未启动或重启业务服务和对外调度。

验证：后端全包测试、隔离 PostgreSQL 完整服务/迁移竞态回归及最终业务对象专项通过；dashboard 类型检查、构建及 27 项测试通过。真实浏览器任务和生产多租户容量仍待验收，前端存在既有大包提示。

### 0026：报价商机检索与完整管线汇总

报价解除关联商机前 100 条限制，按客户和商机权限远程检索、分页及回显。管线不再截取前 1000 条计算规模，改数据库完整汇总和每阶段 20/100 条卡片游标，日期按中国日历在数据库过滤，未知/删除阶段的历史商机进入待配置列。新看板金额以十进制文本返回；旧管线报表同步本人/主管范围。普通阶段变更和编辑不能绕过赢单/输单原因，须使用专用关闭操作。

迁移 `0026_opportunity_bounded_reads.sql` 仅增加 5 个索引，复用现有标量关联，不修改业务字段或行，无新增表与物理外键。先备份及恢复副本演练版本迁移、AutoMigrate、重复执行和全表内容校验，再更新本地库及 core/dashboard。大表普通建索引需要维护窗口及写放大/容量测量。协议、读取时间及并发分页边界见 [商机与管线](docs/crm/sales/02-商机与管线.md)，报价规则见 [报价](docs/crm/sales/03-报价.md)。

本地升级记录：备份 `tmp/db-backups/scrm-before-pipeline-20261018.dump`，恢复副本演练版本迁移、AutoMigrate 和重复执行后，本地 scrm 已更新 0026。178 张业务表全部字段内容及行数保持一致，5 个新增索引有效，物理外键仍为 0；报告为 `tmp/pipeline-upgrade/scrm-verification.json`。未启动或重启业务服务及对外调度。

验证：后端全包测试、最终完整隔离 PostgreSQL 服务/迁移竞态回归及阶段关闭专项通过；dashboard 类型检查、构建及 31 项测试通过。管线专项验证 1005 条商机、日期和权限范围、大额汇总保留分；1/1005 条商机在相同阶段配置下看板 SQL 调用数均为 5。真实浏览器完整交互、其他报表口径和生产多租户负载仍待验收，前端保留既有大包提示。
