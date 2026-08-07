#!/usr/bin/env bash
# 契约同步：core 是 .proto 的唯一事实来源。
# 1) 用 protoc 重新生成 pb 代码（本仓库 api/pb）
# 2) 把生成的 .pb.go 同步到 gateway/api/pb（模拟"从私有 registry 拉客户端 SDK"）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PB_DIR="$ROOT/api/pb"
GATEWAY_PB_DIR="$ROOT/../gateway/api/pb"

if ! command -v protoc >/dev/null || ! command -v protoc-gen-go >/dev/null || ! command -v protoc-gen-go-grpc >/dev/null; then
  echo "缺少 protoc / protoc-gen-go / protoc-gen-go-grpc" >&2
  exit 1
fi

cd "$ROOT"
for proto in api/pb/*.proto; do
  protoc \
    --go_out=. --go_opt=paths=source_relative \
    --go-grpc_out=. --go-grpc_opt=paths=source_relative \
    -I . \
    "$proto"
done

# 同步所有生成的契约代码到 gateway
mkdir -p "$GATEWAY_PB_DIR"
for f in "$PB_DIR"/*.pb.go; do
  [ -e "$f" ] || continue
  cp "$f" "$GATEWAY_PB_DIR/"
done

echo "契约已生成并同步到 gateway/api/pb:"
ls -1 "$PB_DIR"/*.pb.go "$GATEWAY_PB_DIR"/*.pb.go
echo "记得在 core 与 gateway 分别 go mod tidy && go build ./... 验证。"
