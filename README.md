# SCRM 项目

基于企业微信的私域流量管理（SCRM）系统。仓库根目录按权限边界组织多个独立的本地 git 仓库。

## 目录结构

```
scrm/
├── core/         闭源 gRPC 业务核心（独立本地仓库，本仓库）
├── gateway/      开放 HTTP 网关（独立本地仓库）
├── scrm-server/  openscrm 上游后端（fork，独立仓库，含 origin + upstream）
└── scrm-app/     前端 + 会话存档服务（独立仓库）
```

## 各仓库说明

### core/ —— 闭源 gRPC 核心（本仓库）
- **定位**：承载真实业务逻辑，源码私有，仅核心团队可访问。
- **对外**：只暴露 `.proto` 契约 + 生成的 pb 客户端代码（相当于私有 registry 的 SDK），不暴露实现。
- **技术**：Go + gRPC，监听 `:9002`。已迁移 openscrm 的 `app`/`common`/`pkg`/`conf` 地基，模块路径 `openscrm/` → `core/`。
- **已接入业务域**：customer、staff（Get/Query）。

### gateway/ —— 开放 HTTP 网关
- **定位**：对前端暴露 HTTP API，鉴权在 HTTP 层完成，业务转发给闭源 core。源码对前端/后端团队开放，不含业务实现。
- **技术**：Go + gin，监听 `:9001`，通过 gRPC client 调 core（`:9002`）。
- **已接入端点**：customer、staff 的列表/详情。
- 关键目录：`internal/controller/`、`internal/middleware/auth.go`（JWT，密钥取 `GATEWAY_JWT_SECRET`）、`internal/router/`、`api/pb/`（从 core 同步的契约）、`hack/gentoken/`。

### scrm-server/ —— 后端源仓库（openscrm fork）
- 上游 openscrm 的 API server（Go 单体），当前核心业务的地基来源。
- `origin` = 你们的主仓库，`upstream` = openscrm 官方（用于同步更新）。
- core 已把其中 `app`/`common`/`pkg`/`conf` 迁入并改造为 gRPC 核心，本仓库保留作为上游跟踪源。

### scrm-app/ —— 前端与会话存档
- `dashboard/` — React 管理后台；`sidebar/` — React 侧边栏；`msg-server/` — Go 会话存档服务（含企微专属 SDK，敏感）。
- 三者目前装在同一个仓库里，若需按权限隔离，应拆成独立仓库（msg-server 尤其涉敏感 SDK）。

## 调用链路

```
前端(dashboard/sidebar)
   │  HTTP (JSON)
   ▼
gateway(开放，:9001，JWT 鉴权)
   │  gRPC（身份经 metadata 透传）
   ▼
core(闭源，:9002，真实业务)
```

## 安全边界
- 鉴权在 gateway 的 HTTP 层；core 只应监听内网、仅允许 gateway 可达，不得对外直接暴露，否则可绕过鉴权。
- 闭源靠"仓库权限 + 契约/SDK 分发"，防的是协作同事看源码，防不了运维/反编译。
- `conf/config.yaml` 含密钥，已 gitignore 不入库。

## 快速开始（开发）
1. 分别启动 core 与 gateway（需 MySQL + Redis 就绪，core 配置 `conf/config.yaml`）。
2. `cd core && ./scripts/sync-pb.sh` 同步契约。
3. `cd gateway && go run ./hack/gentoken` 生成 JWT，调用 HTTP 接口。

---

# core（闭源核心）—— 内部结构与维护

## 结构
```
core/
├── api/pb/customer.proto      # 契约唯一事实来源（闭源）
├── api/pb/staff.proto         # 契约唯一事实来源（闭源）
├── api/pb/*.pb.go             # protoc 生成代码（与 gateway 同步）
├── app/                       # 从 openscrm 迁入的真实业务（models/services/constants/...）
├── common/                    # 共享基础设施（log/redis/db/validator/...）
├── pkg/easywework/            # 企微 SDK 封装（敏感）
├── conf/                      # 配置（config.yaml 已 gitignore，含密钥不入库）
├── internal/grpcserver/       # gRPC 服务实现，映射真实 models → 契约 pb
├── scripts/sync-pb.sh         # 契约生成 + 同步到 gateway
└── main.go                    # 初始化 conf/db/redis + gRPC server :9002
```

## 契约生成（唯一事实来源）
```bash
./scripts/sync-pb.sh   # 重新生成 customer/staff 的 pb，并同步到 gateway/api/pb
```
改 `.proto` 后跑一次，再在 core 与 gateway 分别 `go build ./...` 验证。

## 运行
```bash
go run .   # 需 MySQL + Redis 就绪，config.yaml 配好
```

## 与 gateway 的关系
- gateway 是开放端，通过 gRPC 调本服务，**不接触** `app/models`、`common`、`pkg` 实现。
- 每次契约变更：改 proto → 重新生成 → 同步 `.pb.go` 到 gateway → 两端各重新编译。

## 迁移说明
- 已把 openscrm 的 `app`(models/services/constants/entities/requests/responses)、`common`、`pkg`、`conf` 整体迁入本仓库，模块路径 `openscrm/` → `core/`，编译通过。
- 业务仍继承 openscrm 的全局单例结构（`conf.Settings`、共享 gorm DB）。已暴露 customer、staff 两个域的 Get/Query 接口，其余业务按 `internal/grpcserver/*.go` 模板逐域扩展。
- 试点 main 未启动 consumers/tasks/delay_queue（依赖企微回调），生产需按 openscrm `main.go` 完整初始化。
