#!/usr/bin/env bash
# make dev：先通过 SSH 隧道把远程后端服务转发到本机，再启动前端开发服务器。
#
# 流程：
#   1. 提示输入 SSH 用户名（密码由 ssh 在连接时提示输入，脚本不保存密码）
#   2. 后台建立隧道 127.0.0.1:8090 -> 隧道服务器 154.51.40.64 上的后端服务
#   3. 以 VITE_DEV_PROXY_TARGET=http://127.0.0.1:8090 启动 vite 开发服务器
#   4. 前端退出（Ctrl+C）时自动关闭隧道
set -euo pipefail

# ===== 可按需修改的配置 =====
TUNNEL_HOST="154.51.40.64"     # 隧道服务器地址
LOCAL_PORT="8090"              # 本机访问后端的端口
REMOTE_HOST="127.0.0.1"        # 后端服务在隧道服务器上的地址
REMOTE_PORT="8090"             # 后端服务在隧道服务器上的端口
# ============================

cd "$(dirname "$0")/.."

BACKEND_URL="http://127.0.0.1:${LOCAL_PORT}"

# 1. 输入用户名；密码在 ssh 连接时由 ssh 自己提示，不在脚本中出现
read -rp "SSH 用户名: " SSH_USER
if [ -z "$SSH_USER" ]; then
  echo "错误：用户名不能为空"
  exit 1
fi

echo
echo "==> 建立隧道 ${BACKEND_URL} -> ${SSH_USER}@${TUNNEL_HOST}:${REMOTE_HOST}:${REMOTE_PORT}"
echo "==> 连接时会提示输入密码（输入时不回显，属正常现象）"
echo

# 2. 后台建立隧道；-N 只做端口转发不执行远程命令，绑定本机回环避免暴露到局域网
ssh -N \
  -o ExitOnForwardFailure=yes \
  -o StrictHostKeyChecking=accept-new \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  -L "127.0.0.1:${LOCAL_PORT}:${REMOTE_HOST}:${REMOTE_PORT}" \
  "${SSH_USER}@${TUNNEL_HOST}" &
TUNNEL_PID=$!

cleanup() {
  echo
  echo "==> 关闭 SSH 隧道（PID ${TUNNEL_PID}）"
  kill "${TUNNEL_PID}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# 3. 等待本地端口就绪（ssh 认证通过后才会开始监听；最多等 ~10 秒）
tunnel_ready=0
for _ in $(seq 1 50); do
  if ! kill -0 "${TUNNEL_PID}" 2>/dev/null; then
    echo "错误：隧道建立失败（请检查用户名/密码，或本机能否访问 ${TUNNEL_HOST}:22）"
    exit 1
  fi
  if (exec 3<>"/dev/tcp/127.0.0.1/${LOCAL_PORT}") 2>/dev/null; then
    tunnel_ready=1
    break
  fi
  sleep 0.2
done
if [ "${tunnel_ready}" != "1" ]; then
  echo "警告：10 秒内未能确认隧道就绪，仍继续启动前端（后端请求可能暂时失败）"
fi

# pnpm 不在 PATH 时回退到 corepack pnpm（Node 自带 corepack）
pnpm_cmd() {
  if command -v pnpm >/dev/null 2>&1; then
    pnpm "$@"
  elif command -v corepack >/dev/null 2>&1; then
    corepack pnpm "$@"
  else
    echo "错误：未找到 pnpm，请先安装（npm i -g pnpm 或 corepack enable）" >&2
    return 127
  fi
}

# 4. 启动前端开发服务器，API 代理指向隧道后的后端
echo "==> 隧道就绪，后端地址：${BACKEND_URL}"
echo "==> 启动前端开发服务器（Ctrl+C 退出并自动断开隧道）"
echo
export VITE_DEV_PROXY_TARGET="${BACKEND_URL}"
pnpm_cmd --dir frontend run dev
