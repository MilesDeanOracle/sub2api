#!/usr/bin/env bash
# make up-file <tar路径> (Linux/macOS)：跳过构建，直接上传已打包的 docker 镜像 tar 到远端服务器。
# 与 scripts/build-docker.sh 的第 6 步相同，只是复用本地已有产物（如 dist/*.tar），
# 适合构建完成后重传、或 build-docker 只跑到 save 就中断的场景。
# 流程（与 scripts/up-file.ps1 一致，Windows 使用 .ps1 版本）：
#   1. 校验 tar 文件存在
#   2. 提示输入 SSH 用户名（密码由 ssh/scp 在连接时提示输入，脚本不保存密码）
#   3. 上传 tar 到远端服务器 /data/docker-img/ 目录
#      （ssh 建目录、scp 上传，各提示一次密码）
set -euo pipefail

# ===== 可按需修改的配置（与 scripts/build-docker.sh 保持一致） =====
SERVER_HOST='154.51.40.64'      # 远端服务器地址
SSH_PORT='9922'                 # 远端 SSH 端口
REMOTE_DIR='/data/docker-img'   # 远端镜像存放目录
# ============================

# 1. 参数：tar 文件路径；缺省或传入目录时，自动选取该目录（默认 dist/）下最新的 *.tar
TAR_PATH="${1:-}"
if [ -z "${TAR_PATH// /}" ] || [ "$TAR_PATH" = 'latest' ]; then
    TAR_PATH='dist'
fi
if [ -d "$TAR_PATH" ]; then
    picked="$(ls -t "$TAR_PATH"/*.tar 2>/dev/null | head -n 1 || true)"
    if [ -z "$picked" ]; then
        echo "错误：目录 ${TAR_PATH} 下没有 .tar 文件（可先运行 make build-docker 生成）"
        exit 1
    fi
    echo "==> 未指定文件，自动选择目录内最新 tar"
    TAR_PATH="$picked"
fi
if [ ! -f "$TAR_PATH" ]; then
    echo "错误：找不到 tar 文件：$TAR_PATH"
    echo '用法：make up-file [tar文件路径]（不带参数自动上传 dist/ 下最新的 tar）'
    echo '      例如：make up-file dist/sub2api-0.2.19-20260101-1200.tar'
    exit 1
fi
TAR_NAME="$(basename "$TAR_PATH")"
TAR_SIZE="$(du -h "$TAR_PATH" | cut -f1)"

echo ""
echo "==> 上传文件：${TAR_PATH}（${TAR_SIZE}）"

# 2. 输入用户名；密码在 ssh/scp 连接时由其自己提示，不在脚本中出现
read -rp 'SSH 用户名: ' SSH_USER
if [ -z "${SSH_USER// /}" ]; then
    echo '错误：用户名不能为空'
    exit 1
fi

echo "==> 目标位置：${SSH_USER}@${SERVER_HOST}:${REMOTE_DIR}/${TAR_NAME}"
echo ""

# 3. 上传：ssh 建目录 + scp 上传（各提示一次密码）
echo "==> 在服务器上创建目录 ${REMOTE_DIR}（会提示输入密码）"
ssh -p "${SSH_PORT}" -o StrictHostKeyChecking=accept-new "${SSH_USER}@${SERVER_HOST}" "mkdir -p ${REMOTE_DIR}"

echo ""
echo "==> 上传 ${TAR_NAME} -> ${SSH_USER}@${SERVER_HOST}:${REMOTE_DIR}/（会提示输入密码）"
scp -P "${SSH_PORT}" "$TAR_PATH" "${SSH_USER}@${SERVER_HOST}:${REMOTE_DIR}/"

echo ""
echo '==> 完成！可在服务器上加载镜像：'
echo "    docker load -i ${REMOTE_DIR}/${TAR_NAME}"
