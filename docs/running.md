# 本地开发运行指南

SCRM 系统由四个服务组成，启动顺序：基础设施（PG/Redis）→ 闭源核心（gRPC :9002）→ 开放网关（HTTP :9001）→ 前端（dashboard :3000）。

## 架构与端口

| 服务 | 目录 | 端口 | 说明 |
|------|------|------|------|
| PostgreSQL 18 + zhparser | docker-compose | 5433→5432 | 中文全文检索（abcfy2/zhparser:18） |
| Redis 7 | docker-compose | 6380→6379 | 缓存 |
| core（闭源核心） | scrm-server | 9002 (gRPC) | 业务实现，AutoMigrate + 种子数据 |
| gateway（开放网关） | scrm-app/gateway | 9001 (HTTP) | 对前端暴露 HTTP API，转发到 core gRPC |
| dashboard（管理后台） | scrm-app/dashboard | 3000 (HTTP) | Ant Design Pro，proxy → :9001 |
| sidebar（侧边栏） | scrm-app/sidebar | 9000 (HTTP) | Ant Design Pro，proxy → :8080（需按实际改） |
| msg-server（会话存档） | scrm-app/msg-server | — | 依赖腾讯 .so，仅 Linux，可选 |
| 企微回调 webhook | scrm-server | 9003 (HTTP) | 企微事件回调，URL: /wecom/callback/:corp_id |
| 小红书回调 webhook | scrm-server | 9004 (HTTP) | 小红书事件回调，URL: /xhs/callback/:app_id |
| 公众号回调 webhook | scrm-server | 9004 (HTTP) | 公众号事件回调，URL: /gzh/callback/:app_id |
| 抖音回调 webhook | scrm-server | 9004 (HTTP) | 抖音事件回调，URL: /douyin/callback/:app_id |
| H5落地页表单 | scrm-server | 9004 (HTTP) | 落地页表单回收，URL: /landing/:slug/submit |

数据流：浏览器 → dashboard:3000 → (proxy /api/) → gateway:9001 → (gRPC) → core:9002 → PostgreSQL:5433 / Redis:6380

渠道回调：企微 → core:9003；小红书/公众号/抖音/落地页 → core:9004（独立 webhook 端口，验签即鉴权）

## 前置条件

- Go 1.21+
- Node.js 16+ / npm（dashboard 用 bun 也行：`bun run dev`）
- Docker（OrbStack / Docker Desktop）
- 端口 5433、6380、9001、9002、9003、9004、3000 空闲

## 1. 启动基础设施（PostgreSQL + Redis）

```bash
cd scrm-server
docker compose -f docs/docker-compose-dev.yaml up -d
```


## 2. 配置 scrm-server

首次运行需从示例配置生成本地配置：
```bash
cd scrm-server
cp conf/config.example.yaml conf/config.yaml
```

`conf/config.yaml` 关键项（已与 docker-compose 对齐）：
```yaml
DB:
  Host: localhost
  Port: 5433
  User: root
  Name: scrm
  Password: root123456
  SSLMode: disable
Redis:
  Host: localhost:6380
  Password: 123456
```

> 若本机 5432/6379 未被占用，可改回标准端口；docker-compose-dev.yaml 与 config.yaml 需同步修改。

## 测试账号

系统有两类登录入口：**平台超管**（管理租户）和**租户用户**（使用 CRM）。

### 平台超管

平台超管独立于租户，用于管理所有租户（创建/停用/删除/编辑租户、为租户创建管理员账号）。

核心首次启动会自动创建平台超管账号（仅非生产环境执行，见 `app/services/crm.go` 的 `SeedPlatformAdmin`）。

| 账号 | 密码 | 入口 | 功能 |
|------|------|------|------|
| `master` | `123456` | `/platform/login` | 租户管理（列表/创建/编辑/停用启用/删除） |

访问 `http://localhost:3000/#/platform/login`，填入 `master` / `123456` 即可进入平台管理后台。

> 平台超管也可通过 mock-data.sql 初始化（幂等），与 SeedPlatformAdmin 等效。

### 租户用户

核心首次启动会自动创建演示租户与登录账号（仅非生产环境执行，见 `app/services/crm.go` 的 `SeedDemoAccount`）。
运行 `docs/mock-data.sql` 后可初始化完整角色账号（admin/manager/sales），实现不同角色不同菜单权限。

| 租户编码 | 账号 | 密码 | 角色 | 菜单范围 |
|---------|------|------|------|----------|
| `demo` | `admin` | `123456` | 租户管理员 | 全部菜单 |
| `demo` | `manager` | `123456` | 销售经理 | 引流获客/客户管理/客户运营/销售管理/营销工具/风控合规（无企业管理） |
| `demo` | `sales` | `123456` | 销售 | 工作台/客户管理（客户/公海/群列表/渠道互动）/销售管理（线索/商机/智能名片/话术） |
| `demo` | `saler1` | `123456` | 销售 | 同 sales |
| `demo` | `saler2` | `123456` | 销售 | 同 sales |

在登录页选择「密码登录」，填入企业编码 `demo`、对应账号和密码 `123456` 即可进入 CRM 工作区。

> 初始化角色账号需执行 mock-data.sql：
> ```bash
> docker exec -i scrm-dev-postgres psql -U root -d scrm < docs/mock-data.sql
> ```
> `admin` 账号由 `SeedDemoAccount` 自动创建，mock-data.sql 会为其绑定 admin 角色并新增 manager/sales 账号。
> mock-data.sql 同时幂等创建平台超管 `master/123456`。

### SaaS 租户管理

系统支持 SaaS 多租户运营。平台超管通过 `/platform/login` 登录后，可在 `/platform/tenants` 页面管理所有租户：

| 操作 | 说明 |
|------|------|
| 新建租户 | 填写企业名称、租户编码、套餐、到期时间、联系人等，同时创建该租户的管理员账号（密码可指定或自动生成） |
| 编辑租户 | 修改名称、套餐、到期时间、联系人、行业、Logo、最大用户数等 |
| 停用/启用 | 停用后该租户所有用户无法登录（登录时校验 `tenant.status`） |
| 删除租户 | 软删除租户记录（不删业务数据） |

**安全机制**：所有 `crm_tenant/*` API 均要求 `requirePlatformAdmin`（JWT 中 `is_platform=true`），租户用户和匿名请求一律拒绝。

**数据隔离**：租户内所有 CRM 数据通过 `tenant_id` 分区，GORM `TenantScope` 自动追加 `WHERE tenant_id = ?`，详见 `app/models/tenant_scope.go`。


## 3. 启动闭源核心（gRPC :9002）

```bash
cd scrm-server
go run .
```

首次启动会执行 AutoMigrate（建 50 张表）+ 种子数据初始化（权限/角色/默认配置），约 1-2 分钟。
看到以下日志即启动成功：
```
[core] closed-source gRPC server (real business) listening on :9002
```

仅执行数据库迁移（不启动服务）：
```bash
go run ./cmd/migrate
```

## 4. 启动开放网关（HTTP :9001）

```bash
cd scrm-app/gateway
go run .
```

看到以下日志即启动成功：
```
[gateway] open HTTP gateway listening on :9001
```

网关默认连接 `127.0.0.1:9002`（core），如 core 不在本机需改 `internal/client/grpc.go` 的 `CoreAddr`。

## 5. 启动前端 dashboard（:3000）

```bash
cd scrm-app/dashboard
npm install        # 首次
npm run dev
```

启动后访问 http://localhost:3000

> `npm run dev` = `start:dev`，禁用 mock、连真实后端。前端 proxy 把 `/api/` 转发到 `http://127.0.0.1:9001/`（见 `config/proxy.ts`）。

## 6.（可选）启动前端 sidebar（:9000）

```bash
cd scrm-app/sidebar
npm install        # 首次
npm run dev
```

> sidebar 的 proxy 默认指向 `http://127.0.0.1:8080/`（`config/proxy.ts`），与当前 gateway:9001 不一致。
> 如需连本系统，改为 `http://127.0.0.1:9001/` 后再启动。

## 7.（可选）启动会话存档 msg-server

仅 Linux，依赖腾讯官方 .so：
```bash
cd scrm-app/msg-server
export LD_LIBRARY_PATH=$(pwd)/lib
go run .
```

## 一键启动（开发常用）

开 4 个终端分别执行：
```bash
# 终端1：基础设施
cd scrm-server && docker compose -f docs/docker-compose-dev.yaml up -d

# 终端2：核心
cd scrm-server && go run .

# 终端3：网关
cd scrm-app/gateway && go run .

# 终端4：前端
cd scrm-app/dashboard && npm run dev
```

## 验证服务可用

```bash
# core gRPC 端口
lsof -i :9002 | grep LISTEN

# gateway HTTP
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9001/api/v1/console/permissions
# 期望：200 或 401（鉴权未通过也算正常，说明路由通了）

# 前端
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:3000
# 期望：200
```

## 常见问题

**Q: 端口被占用？**
A: 5432/6379 常被其他容器占用，docker-compose-dev.yaml 已映射到 5433/6380。若 9001/9002/3000 被占用，改对应配置文件。

**Q: AutoMigrate 很慢？**
A: 首次启动 GORM 会逐表检查并同步结构（含 COMMENT），50 张表约 1-2 分钟，属正常。后续启动会快很多。

**Q: `column "identify" does not exist`？**
A: 已修复。permission 表的 `identity` 列因 GORM inflection 被误转为 `identify`，已用 `column:identity` tag 解决。

**Q: PG 18 启动报 volume 错误？**
A: PG 18+ 要求挂载到 `/var/lib/postgresql`（非 `/var/lib/postgresql/data`），docker-compose-dev.yaml 已处理。若用旧数据卷需 `down -v` 清空重建。
