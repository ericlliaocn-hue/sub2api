#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="${ROOT_DIR}/backend"
FRONTEND_DIR="${ROOT_DIR}/frontend"
RUNTIME_DIR="${ROOT_DIR}/.dev/local"
BIN_DIR="${RUNTIME_DIR}/bin"
BACKEND_BIN="${BIN_DIR}/sub2api"
BACKEND_PID_FILE="${RUNTIME_DIR}/backend.pid"
FRONTEND_PID_FILE="${RUNTIME_DIR}/frontend.pid"
DOCS_PID_FILE="${RUNTIME_DIR}/docs.pid"
BACKEND_LOG="${RUNTIME_DIR}/backend.log"
FRONTEND_LOG="${RUNTIME_DIR}/frontend.log"
DOCS_LOG="${RUNTIME_DIR}/docs.log"
LOCAL_ENV_FILE="${SUB2API_LOCAL_ENV_FILE:-${BACKEND_DIR}/.env.local}"

BACKEND_HOST="127.0.0.1"
BACKEND_PORT="8080"
FRONTEND_HOST="127.0.0.1"
FRONTEND_PORT="3000"

log() {
  printf '[local-dev] %s\n' "$*"
}

fail() {
  printf '[local-dev] ERROR: %s\n' "$*" >&2
  exit 1
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || fail "缺少命令：$1"
}

ensure_pnpm() {
  local pnpm_version=""
  if command -v pnpm >/dev/null 2>&1; then
    pnpm_version="$(pnpm --version 2>/dev/null || true)"
  fi

  if [[ "${pnpm_version}" == 9.* ]]; then
    return
  fi

  require_command corepack
  log "激活项目要求的 pnpm 9.15.9..."
  COREPACK_NPM_REGISTRY=https://registry.npmjs.org \
    npm_config_registry=https://registry.npmjs.org \
    corepack prepare pnpm@9.15.9 --activate

  [[ "$(pnpm --version)" == 9.* ]] || fail "无法激活 pnpm 9.15.9"
}

resolve_go() {
  if [[ -n "${GO_BIN:-}" ]]; then
    [[ -x "${GO_BIN}" ]] || fail "GO_BIN 不可执行：${GO_BIN}"
    printf '%s\n' "${GO_BIN}"
    return
  fi

  if [[ -x /opt/homebrew/opt/go/bin/go ]]; then
    printf '%s\n' /opt/homebrew/opt/go/bin/go
    return
  fi

  command -v go || fail "缺少 Go；macOS 可执行：brew install go"
}

resolve_postgres_command() {
  local command_name="$1"
  local brew_formula="${POSTGRES_HOMEBREW_FORMULA:-postgresql@18}"
  local brew_command="/opt/homebrew/opt/${brew_formula}/bin/${command_name}"

  if [[ -x "${brew_command}" ]]; then
    printf '%s\n' "${brew_command}"
    return
  fi

  if command -v "${command_name}" >/dev/null 2>&1; then
    command -v "${command_name}"
    return
  fi

  fail "缺少 ${command_name}；macOS 可执行：brew install ${brew_formula}"
}

load_local_env() {
  [[ -f "${LOCAL_ENV_FILE}" ]] || fail "缺少 ${LOCAL_ENV_FILE}；先复制 backend/.env.local.example 并填写本地凭据"

  # shellcheck disable=SC1090
  source "${LOCAL_ENV_FILE}"

  DATABASE_HOST="${DATABASE_HOST:-127.0.0.1}"
  DATABASE_PORT="${DATABASE_PORT:-5432}"
  DATABASE_USER="${DATABASE_USER:-sub2api}"
  DATABASE_PASSWORD="${DATABASE_PASSWORD:-}"
  DATABASE_DBNAME="${DATABASE_DBNAME:-sub2api}"
  DATABASE_SSLMODE="${DATABASE_SSLMODE:-disable}"
  REDIS_HOST="${REDIS_HOST:-127.0.0.1}"
  REDIS_PORT="${REDIS_PORT:-6379}"
  REDIS_USERNAME="${REDIS_USERNAME:-}"
  REDIS_PASSWORD="${REDIS_PASSWORD:-}"
  REDIS_DB="${REDIS_DB:-0}"
  REDIS_ENABLE_TLS="${REDIS_ENABLE_TLS:-false}"
  ADMIN_EMAIL="${ADMIN_EMAIL:-admin@sub2api.local}"
  ADMIN_PASSWORD="${ADMIN_PASSWORD:-}"
  JWT_SECRET="${JWT_SECRET:-}"
  JWT_EXPIRE_HOUR="${JWT_EXPIRE_HOUR:-24}"
  TOTP_ENCRYPTION_KEY="${TOTP_ENCRYPTION_KEY:-}"
  AUTO_SETUP="${AUTO_SETUP:-true}"
  SKIP_SETUP="${SKIP_SETUP:-false}"
  POSTGRES_HOMEBREW_FORMULA="${POSTGRES_HOMEBREW_FORMULA:-postgresql@18}"
  TZ="${TZ:-Asia/Shanghai}"
  BACKEND_HOST="${SERVER_HOST:-${BACKEND_HOST}}"
  BACKEND_PORT="${SERVER_PORT:-${BACKEND_PORT}}"
  FRONTEND_HOST="${VITE_DEV_HOST:-${FRONTEND_HOST}}"
  FRONTEND_PORT="${VITE_DEV_PORT:-${FRONTEND_PORT}}"
}

pid_is_running() {
  local pid_file="$1"
  local pid

  [[ -f "${pid_file}" ]] || return 1
  pid="$(sed -n '1p' "${pid_file}")"
  [[ "${pid}" =~ ^[0-9]+$ ]] || return 1
  kill -0 "${pid}" >/dev/null 2>&1
}

cleanup_stale_pid() {
  local pid_file="$1"
  if [[ -f "${pid_file}" ]] && ! pid_is_running "${pid_file}"; then
    rm -f "${pid_file}"
  fi
}

wait_for_command() {
  local description="$1"
  local attempts="$2"
  shift 2

  local attempt
  for ((attempt = 1; attempt <= attempts; attempt++)); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done

  log "等待 ${description} 超时"
  return 1
}

wait_for_http() {
  local name="$1"
  local url="$2"
  local attempts="$3"
  wait_for_command "${name} (${url})" "${attempts}" curl -fsS "${url}"
}

ensure_postgres() {
  local pg_isready_bin
  pg_isready_bin="$(resolve_postgres_command pg_isready)"

  if "${pg_isready_bin}" -h "${DATABASE_HOST}" -p "${DATABASE_PORT}" >/dev/null 2>&1; then
    return
  fi

  require_command brew
  log "启动 ${POSTGRES_HOMEBREW_FORMULA}..."
  brew services start "${POSTGRES_HOMEBREW_FORMULA}" >/dev/null
  wait_for_command PostgreSQL 30 "${pg_isready_bin}" -h "${DATABASE_HOST}" -p "${DATABASE_PORT}" || \
    fail "PostgreSQL 启动失败"
}

ensure_redis() {
  require_command redis-cli

  local redis_args=(-h "${REDIS_HOST}" -p "${REDIS_PORT}" -n "${REDIS_DB}")
  if [[ -n "${REDIS_USERNAME}" ]]; then
    redis_args+=(--user "${REDIS_USERNAME}")
  fi

  if REDISCLI_AUTH="${REDIS_PASSWORD}" redis-cli "${redis_args[@]}" ping 2>/dev/null | grep -qx PONG; then
    return
  fi

  require_command brew
  log "启动 Redis..."
  brew services start redis >/dev/null

  local attempt
  for ((attempt = 1; attempt <= 30; attempt++)); do
    if REDISCLI_AUTH="${REDIS_PASSWORD}" redis-cli "${redis_args[@]}" ping 2>/dev/null | grep -qx PONG; then
      return
    fi
    sleep 1
  done

  fail "等待 Redis 超时"
}

verify_database() {
  local psql_bin
  psql_bin="$(resolve_postgres_command psql)"

  if ! PGPASSWORD="${DATABASE_PASSWORD}" "${psql_bin}" \
    -h "${DATABASE_HOST}" \
    -p "${DATABASE_PORT}" \
    -U "${DATABASE_USER}" \
    -d "${DATABASE_DBNAME}" \
    -v ON_ERROR_STOP=1 \
    -Atqc 'select 1' >/dev/null 2>&1; then
    fail "无法连接 PostgreSQL：${DATABASE_USER}@${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_DBNAME}"
  fi
}

verify_toolchain() {
  local go_bin="$1"
  local expected_go actual_go

  expected_go="$(awk '$1 == "go" { print "go" $2; exit }' "${BACKEND_DIR}/go.mod")"
  actual_go="$(cd "${BACKEND_DIR}" && "${go_bin}" env GOVERSION)"
  [[ "${actual_go}" == "${expected_go}" ]] || fail "Go 工具链不匹配：需要 ${expected_go}，实际 ${actual_go}"
}

install_frontend_dependencies() {
  if [[ -x "${FRONTEND_DIR}/node_modules/.bin/vite" ]]; then
    return
  fi

  require_command pnpm
  log "安装前端依赖..."
  (
    cd "${FRONTEND_DIR}"
    npm_config_registry=https://registry.npmjs.org pnpm install --frozen-lockfile
  )
}

bootstrap() {
  load_local_env
  require_command curl
  require_command node
  ensure_pnpm

  local go_bin
  go_bin="$(resolve_go)"
  verify_toolchain "${go_bin}"
  ensure_postgres
  ensure_redis
  verify_database
  install_frontend_dependencies

  mkdir -p "${BIN_DIR}"
  log "本地依赖检查通过"
}

build_backend() {
  local go_bin
  go_bin="$(resolve_go)"
  log "构建后端..."
  (cd "${BACKEND_DIR}" && "${go_bin}" build -o "${BACKEND_BIN}" ./cmd/server)
}

ensure_port_available() {
  local host="$1"
  local port="$2"
  local service_name="$3"

  if command -v lsof >/dev/null 2>&1 && lsof -nP -iTCP@"${host}":"${port}" -sTCP:LISTEN >/dev/null 2>&1; then
    fail "${service_name} 端口 ${host}:${port} 已被其他进程占用"
  fi
}

start_backend() {
  cleanup_stale_pid "${BACKEND_PID_FILE}"
  if pid_is_running "${BACKEND_PID_FILE}"; then
    log "后端已运行（PID $(sed -n '1p' "${BACKEND_PID_FILE}")）"
    return
  fi

  ensure_port_available "${BACKEND_HOST}" "${BACKEND_PORT}" 后端
  : >"${BACKEND_LOG}"

  log "启动后端：http://${BACKEND_HOST}:${BACKEND_PORT}"
  (
    cd "${BACKEND_DIR}"
    nohup env \
      AUTO_SETUP="${AUTO_SETUP}" \
      SKIP_SETUP="${SKIP_SETUP}" \
      DATA_DIR="${BACKEND_DIR}" \
      DATABASE_HOST="${DATABASE_HOST}" \
      DATABASE_PORT="${DATABASE_PORT}" \
      DATABASE_USER="${DATABASE_USER}" \
      DATABASE_PASSWORD="${DATABASE_PASSWORD}" \
      DATABASE_DBNAME="${DATABASE_DBNAME}" \
      DATABASE_SSLMODE="${DATABASE_SSLMODE}" \
      REDIS_HOST="${REDIS_HOST}" \
      REDIS_PORT="${REDIS_PORT}" \
      REDIS_USERNAME="${REDIS_USERNAME}" \
      REDIS_PASSWORD="${REDIS_PASSWORD}" \
      REDIS_DB="${REDIS_DB}" \
      REDIS_ENABLE_TLS="${REDIS_ENABLE_TLS}" \
      ADMIN_EMAIL="${ADMIN_EMAIL}" \
      ADMIN_PASSWORD="${ADMIN_PASSWORD}" \
      JWT_SECRET="${JWT_SECRET}" \
      JWT_EXPIRE_HOUR="${JWT_EXPIRE_HOUR}" \
      TOTP_ENCRYPTION_KEY="${TOTP_ENCRYPTION_KEY}" \
      TZ="${TZ}" \
      SERVER_HOST="${BACKEND_HOST}" \
      SERVER_PORT="${BACKEND_PORT}" \
      SERVER_MODE=debug \
      "${BACKEND_BIN}" >>"${BACKEND_LOG}" 2>&1 &
    printf '%s\n' "$!" >"${BACKEND_PID_FILE}"
  )

  if ! wait_for_http 后端 "http://${BACKEND_HOST}:${BACKEND_PORT}/health" 120; then
    tail -n 80 "${BACKEND_LOG}" >&2 || true
    return 1
  fi
}

start_frontend() {
  cleanup_stale_pid "${FRONTEND_PID_FILE}"
  if pid_is_running "${FRONTEND_PID_FILE}"; then
    log "前端已运行（PID $(sed -n '1p' "${FRONTEND_PID_FILE}")）"
    return
  fi

  ensure_port_available "${FRONTEND_HOST}" "${FRONTEND_PORT}" 前端
  : >"${FRONTEND_LOG}"

  log "启动前端：http://${FRONTEND_HOST}:${FRONTEND_PORT}"
  (
    cd "${FRONTEND_DIR}"
    nohup env \
      VITE_DEV_PROXY_TARGET="http://${BACKEND_HOST}:${BACKEND_PORT}" \
      VITE_DEV_PORT="${FRONTEND_PORT}" \
      "${FRONTEND_DIR}/node_modules/.bin/vite" \
      --host "${FRONTEND_HOST}" \
      --port "${FRONTEND_PORT}" >>"${FRONTEND_LOG}" 2>&1 &
    printf '%s\n' "$!" >"${FRONTEND_PID_FILE}"
  )

  if ! wait_for_http 前端 "http://${FRONTEND_HOST}:${FRONTEND_PORT}/" 60; then
    tail -n 80 "${FRONTEND_LOG}" >&2 || true
    return 1
  fi
}

start_docs() {
  cleanup_stale_pid "${DOCS_PID_FILE}"
  if pid_is_running "${DOCS_PID_FILE}"; then
    log "文档监听已运行（PID $(sed -n '1p' "${DOCS_PID_FILE}")）"
    return
  fi

  : >"${DOCS_LOG}"
  log "启动文档自动构建：http://doc.anytoken.localhost"
  (
    cd "${ROOT_DIR}/docs-site"
    nohup node watch.mjs </dev/null >>"${DOCS_LOG}" 2>&1 &
    printf '%s\n' "$!" >"${DOCS_PID_FILE}"
  )

  wait_for_http 文档 "http://doc.anytoken.localhost/" 30 || {
    tail -n 80 "${DOCS_LOG}" >&2 || true
    return 1
  }
}

start_all() {
  bootstrap
  build_backend
  start_backend
  start_frontend
  start_docs
  status_all
}

stop_process() {
  local name="$1"
  local pid_file="$2"

  cleanup_stale_pid "${pid_file}"
  if ! pid_is_running "${pid_file}"; then
    log "${name}未运行"
    return
  fi

  local pid
  pid="$(sed -n '1p' "${pid_file}")"
  log "停止${name}（PID ${pid}）..."
  kill "${pid}" >/dev/null 2>&1 || true

  local attempt
  for ((attempt = 1; attempt <= 10; attempt++)); do
    if ! kill -0 "${pid}" >/dev/null 2>&1; then
      rm -f "${pid_file}"
      return
    fi
    sleep 1
  done

  log "${name}未在 10 秒内退出，强制终止"
  kill -KILL "${pid}" >/dev/null 2>&1 || true
  rm -f "${pid_file}"
}

stop_all() {
  stop_process 文档监听 "${DOCS_PID_FILE}"
  stop_process 前端 "${FRONTEND_PID_FILE}"
  stop_process 后端 "${BACKEND_PID_FILE}"
}

print_process_status() {
  local name="$1"
  local pid_file="$2"
  local url="$3"

  cleanup_stale_pid "${pid_file}"
  if pid_is_running "${pid_file}"; then
    printf '%-10s running  PID=%s  %s\n' "${name}" "$(sed -n '1p' "${pid_file}")" "${url}"
  else
    printf '%-10s stopped  %s\n' "${name}" "${url}"
  fi
}

status_all() {
  load_local_env
  print_process_status backend "${BACKEND_PID_FILE}" "http://${BACKEND_HOST}:${BACKEND_PORT}"
  print_process_status frontend "${FRONTEND_PID_FILE}" "http://${FRONTEND_HOST}:${FRONTEND_PORT}"
  print_process_status docs "${DOCS_PID_FILE}" "http://doc.anytoken.localhost"

  local pg_isready_bin
  pg_isready_bin="$(resolve_postgres_command pg_isready)"
  if "${pg_isready_bin}" -h "${DATABASE_HOST}" -p "${DATABASE_PORT}" >/dev/null 2>&1; then
    printf '%-10s ready    %s:%s/%s\n' postgres "${DATABASE_HOST}" "${DATABASE_PORT}" "${DATABASE_DBNAME}"
  else
    printf '%-10s stopped  %s:%s\n' postgres "${DATABASE_HOST}" "${DATABASE_PORT}"
  fi

  if REDISCLI_AUTH="${REDIS_PASSWORD}" redis-cli -h "${REDIS_HOST}" -p "${REDIS_PORT}" ping 2>/dev/null | grep -qx PONG; then
    printf '%-10s ready    %s:%s\n' redis "${REDIS_HOST}" "${REDIS_PORT}"
  else
    printf '%-10s stopped  %s:%s\n' redis "${REDIS_HOST}" "${REDIS_PORT}"
  fi
}

doctor() {
  load_local_env

  local go_bin
  go_bin="$(resolve_go)"
  printf 'Go bootstrap: %s\n' "$("${go_bin}" version)"
  printf 'Go project:   %s\n' "$(cd "${BACKEND_DIR}" && "${go_bin}" env GOVERSION)"
  printf 'Node:         %s\n' "$(node --version)"
  printf 'pnpm:         %s\n' "$(pnpm --version)"
  printf 'Environment:  %s\n' "${LOCAL_ENV_FILE}"

  ensure_postgres
  ensure_redis
  verify_database
  log "数据库与 Redis 连接正常"
}

show_logs() {
  mkdir -p "${RUNTIME_DIR}"
  touch "${BACKEND_LOG}" "${FRONTEND_LOG}" "${DOCS_LOG}"
  tail -n 200 -f "${BACKEND_LOG}" "${FRONTEND_LOG}" "${DOCS_LOG}"
}

usage() {
  cat <<'EOF'
Usage: ./tools/local-dev.sh <command>

Commands:
  start       检查依赖、构建后端并启动前后端
  stop        停止前后端（保留 PostgreSQL/Redis）
  restart     重启前后端
  status      查看前后端及数据服务状态
  logs        持续查看前后端日志
  doctor      验证工具链、PostgreSQL 与 Redis
  bootstrap   启动数据服务并安装前端依赖，不启动应用
EOF
}

main() {
  mkdir -p "${RUNTIME_DIR}"

  case "${1:-}" in
    start)
      start_all
      ;;
    stop)
      stop_all
      ;;
    restart)
      stop_all
      start_all
      ;;
    status)
      status_all
      ;;
    logs)
      show_logs
      ;;
    doctor)
      doctor
      ;;
    bootstrap)
      bootstrap
      ;;
    -h|--help|help|"")
      usage
      ;;
    *)
      usage >&2
      exit 2
      ;;
  esac
}

main "$@"
