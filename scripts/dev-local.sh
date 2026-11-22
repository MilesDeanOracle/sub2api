#!/usr/bin/env bash
# make dev-local (Linux/macOS): start the local Go backend and Vite frontend.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "${REPO_ROOT}"

import_local_env() {
  local env_file="$1" line name value
  [ -f "${env_file}" ] || return 0

  while IFS= read -r line || [ -n "${line}" ]; do
    line="${line%$'\r'}"
    [[ "${line}" =~ ^[[:space:]]*$ || "${line}" =~ ^[[:space:]]*# ]] && continue
    line="${line#export }"
    [[ "${line}" == *=* ]] || continue
    name="${line%%=*}"
    value="${line#*=}"
    name="${name//[[:space:]]/}"
    [[ "${name}" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    [ -z "${!name-}" ] || continue

    value="${value#${value%%[![:space:]]*}}"
    value="${value%${value##*[![:space:]]}}"
    if [[ "${value}" =~ ^\".*\"$ || "${value}" =~ ^\'.*\'$ ]]; then
      value="${value:1:${#value}-2}"
    fi
    printf -v "${name}" '%s' "${value}"
    export "${name}"
  done < "${env_file}"
}

tcp_ready() {
  local host="$1" port="$2"
  (exec 3<>"/dev/tcp/${host}/${port}") 2>/dev/null
}

stop_tracked_process() {
  local state_file="$1" expected_marker="$2" label="$3" pid command
  [ -f "${state_file}" ] || return 0
  pid="$(tr -dc '0-9' < "${state_file}")"
  rm -f "${state_file}"
  [[ "${pid}" =~ ^[0-9]+$ ]] || return 0
  kill -0 "${pid}" 2>/dev/null || return 0
  command="$(ps -p "${pid}" -o command= 2>/dev/null || true)"
  [[ "${command}" == *"${expected_marker}"* ]] || return 0

  echo "==> 关闭上一次 dev-local ${label}（PID ${pid}）"
  pkill -TERM -P "${pid}" 2>/dev/null || true
  kill "${pid}" 2>/dev/null || true
  for _ in $(seq 1 20); do
    kill -0 "${pid}" 2>/dev/null || break
    sleep 0.1
  done
  pkill -KILL -P "${pid}" 2>/dev/null || true
  kill -KILL "${pid}" 2>/dev/null || true
}

import_local_env "${REPO_ROOT}/.env.local"

: "${DATA_DIR:=${REPO_ROOT}/.dev/local}"
: "${AUTO_SETUP:=true}"
: "${SERVER_HOST:=127.0.0.1}"
: "${SERVER_PORT:=8080}"
: "${SERVER_MODE:=debug}"
: "${DATABASE_HOST:=127.0.0.1}"
: "${DATABASE_PORT:=5432}"
: "${DATABASE_USER:=postgres}"
: "${DATABASE_PASSWORD:=}"
: "${DATABASE_DBNAME:=sub2api}"
: "${DATABASE_SSLMODE:=disable}"
: "${REDIS_HOST:=127.0.0.1}"
: "${REDIS_PORT:=6379}"
: "${REDIS_USERNAME:=}"
: "${REDIS_PASSWORD:=}"
: "${REDIS_DB:=0}"
: "${REDIS_ENABLE_TLS:=false}"
: "${TZ:=Asia/Shanghai}"
: "${VITE_DEV_PORT:=3000}"

export DATA_DIR AUTO_SETUP SERVER_HOST SERVER_PORT SERVER_MODE
export DATABASE_HOST DATABASE_PORT DATABASE_USER DATABASE_PASSWORD DATABASE_DBNAME DATABASE_SSLMODE
export REDIS_HOST REDIS_PORT REDIS_USERNAME REDIS_PASSWORD REDIS_DB REDIS_ENABLE_TLS
export TZ VITE_DEV_PORT

BACKEND_URL="http://127.0.0.1:${SERVER_PORT}"
export VITE_DEV_PROXY_TARGET="${BACKEND_URL}"
LOG_DIR="${DATA_DIR}/logs"
BACKEND_LOG="${LOG_DIR}/backend.log"
BACKEND_PID_FILE="${DATA_DIR}/backend.pid"
FRONTEND_PID_FILE="${DATA_DIR}/frontend.pid"
mkdir -p "${LOG_DIR}"

stop_tracked_process "${BACKEND_PID_FILE}" 'go run ./cmd/server' '后端'
stop_tracked_process "${FRONTEND_PID_FILE}" 'frontend run dev' '前端'

command -v go >/dev/null 2>&1 || { echo '错误：未找到 Go，请先安装 Go 并加入 PATH' >&2; exit 127; }
command -v curl >/dev/null 2>&1 || { echo '错误：未找到 curl，请先安装 curl' >&2; exit 127; }

if command -v pnpm >/dev/null 2>&1; then
  PNPM=(pnpm)
elif command -v corepack >/dev/null 2>&1; then
  PNPM=(corepack pnpm)
else
  echo '错误：未找到 pnpm 或 corepack，请先安装 Node.js/pnpm' >&2
  exit 127
fi

tcp_ready "${DATABASE_HOST}" "${DATABASE_PORT}" || {
  echo "错误：无法连接 PostgreSQL ${DATABASE_HOST}:${DATABASE_PORT}" >&2
  exit 1
}
tcp_ready "${REDIS_HOST}" "${REDIS_PORT}" || {
  echo "错误：无法连接 Redis ${REDIS_HOST}:${REDIS_PORT}" >&2
  exit 1
}
if tcp_ready 127.0.0.1 "${SERVER_PORT}"; then
  echo "错误：后端端口 ${SERVER_PORT} 已被占用" >&2
  exit 1
fi
if tcp_ready 127.0.0.1 "${VITE_DEV_PORT}"; then
  echo "错误：前端端口 ${VITE_DEV_PORT} 已被占用" >&2
  exit 1
fi

echo "==> PostgreSQL：${DATABASE_HOST}:${DATABASE_PORT}/${DATABASE_DBNAME}"
echo "==> Redis：${REDIS_HOST}:${REDIS_PORT} (DB ${REDIS_DB})"
echo "==> 启动本地后端：${BACKEND_URL}"
echo "==> 后端日志：${BACKEND_LOG}"

(
  cd "${REPO_ROOT}/backend"
  exec go run ./cmd/server
) >"${BACKEND_LOG}" 2>&1 &
BACKEND_PID=$!
printf '%s\n' "${BACKEND_PID}" > "${BACKEND_PID_FILE}"

FRONTEND_PID=''

cleanup() {
	if [ -n "${FRONTEND_PID}" ] && kill -0 "${FRONTEND_PID}" 2>/dev/null; then
	  echo
	  echo "==> 关闭本地前端（PID ${FRONTEND_PID}）"
	  pkill -TERM -P "${FRONTEND_PID}" 2>/dev/null || true
	  kill "${FRONTEND_PID}" 2>/dev/null || true
	  wait "${FRONTEND_PID}" 2>/dev/null || true
	fi
  if kill -0 "${BACKEND_PID}" 2>/dev/null; then
    echo
    echo "==> 关闭本地后端（PID ${BACKEND_PID}）"
    pkill -TERM -P "${BACKEND_PID}" 2>/dev/null || true
    kill "${BACKEND_PID}" 2>/dev/null || true
    wait "${BACKEND_PID}" 2>/dev/null || true
  fi
	rm -f "${BACKEND_PID_FILE}" "${FRONTEND_PID_FILE}"
}
trap cleanup EXIT INT TERM

backend_ready=0
for _ in $(seq 1 240); do
  if ! kill -0 "${BACKEND_PID}" 2>/dev/null; then
    break
  fi
  if curl --silent --fail --max-time 1 "${BACKEND_URL}/health" >/dev/null; then
    backend_ready=1
    break
  fi
  sleep 0.5
done

if [ "${backend_ready}" != '1' ]; then
  echo '错误：本地后端启动失败，最近的日志如下：' >&2
  tail -n 120 "${BACKEND_LOG}" 2>/dev/null || true
  exit 1
fi

echo "==> 后端已就绪，启动前端：http://127.0.0.1:${VITE_DEV_PORT}"
echo '==> 按 Ctrl+C 同时停止前后端'
echo
"${PNPM[@]}" --dir frontend run dev --host 127.0.0.1 --strictPort &
FRONTEND_PID=$!
printf '%s\n' "${FRONTEND_PID}" > "${FRONTEND_PID_FILE}"
wait "${FRONTEND_PID}"
