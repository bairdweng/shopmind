#!/usr/bin/env bash
# shopmind 一键启停：Web 构建 + API，对外只需 start / stop
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export SHOPMIND_ROOT="$ROOT"

PID_FILE="${ROOT}/.run/shopmind.pid"
LOG_FILE="${ROOT}/.run/shopmind.log"
PORT="${SHOPMIND_PORT:-8788}"
URL="http://127.0.0.1:${PORT}"

LOCAL_IP="$(ipconfig getifaddr en0 2>/dev/null || echo "127.0.0.1")"
URL_LAN="http://${LOCAL_IP}:${PORT}"

mkdir -p "${ROOT}/data" "${ROOT}/.run" "${ROOT}/.local/keys"

log() {
  echo "[shopmind] $*"
}

open_browser() {
  local target_url="${URL_LAN}"
  if [[ "$(uname -s)" == "Darwin" ]]; then
    local already_open=false
    if osascript -e 'tell application "System Events" to (name of processes) contains "Google Chrome"' 2>/dev/null | grep -q "true"; then
      if osascript -e "tell application \"Google Chrome\" to get URL of tabs of windows" 2>/dev/null | grep -q "${PORT}"; then
        already_open=true
      fi
    fi
    if [[ "$already_open" == "false" ]] && osascript -e 'tell application "System Events" to (name of processes) contains "Safari"' 2>/dev/null | grep -q "true"; then
      if osascript -e "tell application \"Safari\" to get URL of tabs of windows" 2>/dev/null | grep -q "${PORT}"; then
        already_open=true
      fi
    fi
    if [[ "$already_open" == "true" ]]; then
      log "浏览器已有工作台标签，跳过打开"
      return
    fi
    open "$target_url" >/dev/null 2>&1 || true
  elif command -v xdg-open >/dev/null 2>&1; then
    xdg-open "$target_url" >/dev/null 2>&1 || true
  fi
}

wait_http() {
  local path="$1"
  local tries="${2:-60}"
  local i
  for ((i = 1; i <= tries; i++)); do
    if curl -sf "${URL}${path}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

is_ready() {
  wait_http "/api/health" 1
}

stop_shopmind() {
  if [[ -f "$PID_FILE" ]]; then
    local pid
    pid="$(cat "$PID_FILE")"
    if kill -0 "$pid" 2>/dev/null; then
      log "停止 API (pid $pid)"
      kill "$pid" 2>/dev/null || true
      for _ in $(seq 1 10); do
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.5
      done
      kill -9 "$pid" 2>/dev/null || true
    fi
    rm -f "$PID_FILE"
  fi

  local pids
  pids="$(lsof -nP -iTCP:"${PORT}" -sTCP:LISTEN -t 2>/dev/null || true)"
  for pid in $pids; do
    [[ -z "$pid" ]] && continue
    if ps -p "$pid" -o args= 2>/dev/null | grep -q "shopmind"; then
      log "停止占用 ${PORT} 的旧进程 (pid $pid)"
      kill "$pid" 2>/dev/null || true
      sleep 1
      kill -9 "$pid" 2>/dev/null || true
    fi
  done
}

needs_web_build() {
  [[ ! -f "${ROOT}/web/dist/index.html" ]] && return 0
  local newest_src
  newest_src="$(find "${ROOT}/web/src" -type f -print0 2>/dev/null | xargs -0 stat -f '%m' 2>/dev/null | sort -n | tail -1 || true)"
  local dist_mtime
  dist_mtime="$(stat -f '%m' "${ROOT}/web/dist/index.html" 2>/dev/null || echo 0)"
  [[ -z "$newest_src" || "$newest_src" -gt "$dist_mtime" ]]
}

ensure_web() {
  if [[ ! -d "${ROOT}/web/node_modules" ]]; then
    log "安装前端依赖…"
    (cd "${ROOT}/web" && npm install)
  fi
  if needs_web_build; then
    log "构建 Web…"
    (cd "${ROOT}/web" && npm run build)
  fi
}

ensure_go() {
  log "编译 shopmind…"
  (cd "$ROOT" && go build -o bin/shopmind ./cmd/shopmind)
}

cmd_start() {
  if is_ready; then
    log "已在运行，打开浏览器"
    open_browser
    exit 0
  fi

  stop_shopmind
  ensure_web
  ensure_go

  if lsof -nP -iTCP:"${PORT}" -sTCP:LISTEN >/dev/null 2>&1; then
    log "端口 ${PORT} 已被占用，请先执行: make stop" >&2
    lsof -nP -iTCP:"${PORT}" -sTCP:LISTEN >&2 || true
    exit 1
  fi

  log "启动工作台…"
  # 默认走 HuggingFace 国内镜像，避免首次下载对齐模型时在官方源卡死；
  # 显式设置了 HF_ENDPOINT 则尊重用户选择。
  export HF_ENDPOINT="${HF_ENDPOINT:-https://hf-mirror.com}"
  nohup "${ROOT}/bin/shopmind" serve --port "${PORT}" >>"$LOG_FILE" 2>&1 &
  echo $! >"$PID_FILE"
  disown -h "$!" 2>/dev/null || true

  log "等待就绪…"
  if ! wait_http "/api/health" 60; then
    log "启动失败，日志: $LOG_FILE" >&2
    tail -30 "$LOG_FILE" 2>/dev/null || true
    stop_shopmind
    exit 1
  fi

  log "就绪: $URL"
  log "局域网: $URL_LAN"
  open_browser
}

cmd_stop() {
  stop_shopmind
  log "已停止"
}

cmd_status() {
  if is_ready; then
    log "工作台: 运行中 $URL"
    [[ -f "$PID_FILE" ]] && log "  pid $(cat "$PID_FILE")"
  else
    log "工作台: 未运行"
  fi
}

case "${1:-start}" in
  start) cmd_start ;;
  stop) cmd_stop ;;
  status) cmd_status ;;
  *)
    echo "用法: $0 {start|stop|status}" >&2
    exit 1
    ;;
esac
