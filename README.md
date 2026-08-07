# core（闭源核心）

闭源 gRPC 业务核心。对外只暴露 `.proto` 契约与生成的客户端代码，源码私有。

## 结构
```
core/
├── api/pb/customer.proto    # 契约的唯一事实来源（闭源）
├── api/pb/*.pb.go           # protoc 生成代码
├── internal/service/        # 业务逻辑（试点为内存占位，待迁移 openscrm）
└── main.go                  # gRPC server :9002
```

## 生成契约代码
```bash
protoc --go_out=. --go_opt=paths=source_relative \
       --go-grpc_out=. --go-grpc_opt=paths=source_relative \
       api/pb/customer.proto
```

## 运行
```bash
go run .
# gRPC listening on :9002
```

## 与 gateway 的关系
- gateway 是开放端，通过 gRPC 调用本服务的 CustomerService。
- 每次契约变更：改 `customer.proto` → 重新生成 → 把生成的 `*.pb.go` 同步到 gateway（模拟"从私有 registry 拉客户端 SDK"）。

## 后续迁移（试点占位 → 真实业务）
把 scrm-server 的 `app/services`、`app/models`、`common`、`pkg/easywework` 逐域迁入本仓库，替换 `internal/service` 里的内存 `customerStore`，接入 DB(gorm)/Redis/企微 SDK。
