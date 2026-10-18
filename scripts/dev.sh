#!/usr/bin/env bash
# 开发热更新启动脚本：core(:9001) 与 gateway(:9002) 以 -dev 参数运行，
# 由编译进代码的 dev.WatchAndRun 托管，改代码自动重建重启，无需 air。
# 用法：scripts/dev.sh start | stop | status | logs
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CORE_DIR="$ROOT/scrm-server"
GATEWAY_DIR="$ROOT/scrm-app/gateway"
CORE_BIN="$ROOT/.dev/core"
GATEWAY_BIN="$ROOT/.dev/gateway"

mkdir -p "$ROOT/.dev"

start() {
  # 未显式配置时，在 .dev/jwt_secret 初始化并复用一个随机开发密钥（已被 .gitignore 忽略）。
  if [ -z "${CRM_JWT_SECRET:-}" ]; then
    local secret_file="$ROOT/.dev/jwt_secret"
    if [ ! -f "$secret_file" ]; then
      openssl rand -hex 32 > "$secret_file"
      chmod 600 "$secret_file"
      echo "[dev] 已生成开发密钥: $secret_file"
    fi
    CRM_JWT_SECRET="$(cat "$secret_file")"
  fi
  if [ "${#CRM_JWT_SECRET}" -lt 32 ]; then
    echo "请先设置至少 32 字节的 CRM_JWT_SECRET" >&2
    return 1
  fi
  export CRM_JWT_SECRET
  stop 2>/dev/null || true

  echo "[dev] building core…"
  (cd "$CORE_DIR" && go build -o "$CORE_BIN" .)
  echo "[dev] building gateway…"
  (cd "$GATEWAY_DIR" && go build -o "$GATEWAY_BIN" .)

  echo "[dev] starting core under hot-reload (log: $ROOT/.dev/core.log)"
  (cd "$CORE_DIR" && nohup "$CORE_BIN" -dev > "$ROOT/.dev/core.log" 2>&1) &
  sleep 0.5
  pgrep -f "^$CORE_BIN( -dev)?$" > "$ROOT/.dev/core.pid" || true

  echo "[dev] starting gateway under hot-reload (log: $ROOT/.dev/gateway.log)"
  (cd "$GATEWAY_DIR" && nohup "$GATEWAY_BIN" -dev > "$ROOT/.dev/gateway.log" 2>&1) &
  sleep 0.5
  pgrep -f "^$GATEWAY_BIN( -dev)?$" > "$ROOT/.dev/gateway.pid" || true

  echo "[dev] core=:9001 gateway=:9002 就绪。改动 .go/.yaml/.toml 文件会自动重建重启。"
  echo "[dev] 停止: scripts/dev.sh stop   |   日志: scripts/dev.sh logs"
}

stop() {
  for n in core gateway; do
    local f="$ROOT/.dev/$n.pid"
    if [ -f "$f" ]; then
      local pid
      pid="$(cat "$f")"
      kill "$pid" 2>/dev/null && echo "[dev] stopped $n ($pid)"
      rm -f "$f"
    fi
  done
  # 清理监督者 spawn 的子进程
  pkill -f 'tmp/dev-main' 2>/dev/null || true
}

status() {
  for n in core gateway; do
    local f="$ROOT/.dev/$n.pid"
    if [ -f "$f" ] && kill -0 "$(cat "$f")" 2>/dev/null; then
      echo "$n: running (pid $(cat "$f"))"
    else
      echo "$n: stopped"
    fi
  done
}

logs() {
  tail -f "$ROOT/.dev/core.log" "$ROOT/.dev/gateway.log"
}

case "${1:-start}" in
  start) start ;;
  stop) stop ;;
  status) status ;;
  logs) logs ;;
  *) echo "用法: $0 {start|stop|status|logs}" >&2; exit 1 ;;
esac
