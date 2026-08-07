# core（闭源核心）

闭源 gRPC 业务核心。对外只暴露 `.proto` 契约与生成的客户端代码，源码私有（本地仓库）。

## 结构
```
core/
├── api/pb/customer.proto      # 契约唯一事实来源（闭源）
├── api/pb/*.pb.go             # protoc 生成代码（与 gateway 同步）
├── app/                       # 从 openscrm 迁入的真实业务（models/services/constants/...）
├── common/                    # 共享基础设施（log/redis/db/validator/...）
├── pkg/easywework/            # 企微 SDK 封装（敏感）
├── conf/                      # 配置（config.yaml 已 gitignore，含密钥不入库）
├── internal/grpcserver/       # gRPC 服务实现，映射真实 models → 契约 pb
└── main.go                    # 初始化 conf/db/redis + gRPC server :9002
```

## 契约生成
```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/pb/customer.proto
# 每次改契约，把生成的 *.pb.go 同步到 gateway（模拟私有 registry SDK）
```

## 运行
```bash
go run .   # 需 MySQL + Redis 就绪，config.yaml 配好
```

## 与 gateway 的关系
- gateway 是开放端，通过 gRPC 调本服务，**不接触** `app/models`、`common`、`pkg` 实现。
- 每次契约变更：改 proto → 重新生成 → 同步 `.pb.go` 到 gateway → 两端各重新编译。

## 迁移说明
- 已把 openscrm 的 `app`(models/services/constants/entities/requests/responses)、`common`、`pkg`、`conf` 整体迁入本仓库，模块路径 `openscrm/` → `core/`，编译通过。
- 业务仍继承 openscrm 的全局单例结构（`conf.Settings`、共享 gorm DB）。试点只暴露 customer 的 Get/Query 两条 gRPC 接口，其余业务按 `internal/grpcserver/customer.go` 模板逐域扩展。
- 试点 main 未启动 consumers/tasks/delay_queue（依赖企微回调），生产需按 openscrm `main.go` 完整初始化。
