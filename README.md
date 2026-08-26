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
- 覆盖 39 个业务域、130+ 个动作（CRUD/Query/Get/Update/Delete/统计/导出等），已注册清单见 `internal/dispatch/*.go` 的 `init()`。
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
1. 分别启动本仓库（闭源核心）与 gateway（需 MySQL + Redis 就绪，配置 `conf/config.yaml`）。
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

## 运行
```bash
go run .   # 需 MySQL + Redis 就绪，config.yaml 配好
```

## 与 gateway 的关系
- gateway 是开放端（`scrm-app/gateway`），通过 gRPC 调本服务，**不接触** `app/models`、`common`、`pkg` 实现。
- 每次契约变更：改 proto → 重新生成 → 同步 `.pb.go` 到 gateway → 两端各重新编译。

## 迁移说明
- 本仓库由 openscrm fork 与独立闭源仓库 core 合并而成：openscrm 的 110 提交 + core 的 5 提交历史完整保留于 git log；内容采用 core 那版（module `core`）。
- openscrm 的 `app`(models/services/constants/entities/requests/responses)、`common`、`pkg`、`conf` 已整体迁入，模块路径 `openscrm/` → `core/`，编译通过。
- openscrm 的 HTTP 层（`routers/`、HTTP `main.go`、`test/`、`docker/`、`LICENSE` 等）已被 gateway 路径映射取代，原源码压缩备份于 `~/git/_archive/` 并删除。
- 业务仍继承 openscrm 的全局单例结构（`conf.Settings`、共享 gorm DB）。customer/staff 暴露类型化 Get/Query；其余业务经 `internal/dispatch` 的通用 JSON 透传分发暴露。新增业务动作：在 `internal/dispatch/<域>.go` 的 `init()` 里 `Reg.Register("域","动作",handler)` 即可。
- 试点 main 未启动 consumers/tasks/delay_queue（依赖企微回调），生产需按 openscrm `main.go` 完整初始化。
