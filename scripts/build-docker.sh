#!/usr/bin/env bash
# make build-docker (Linux/macOS)：使用本机 Docker 打包镜像，并上传到远端服务器。
# 流程（与 scripts/build-docker.ps1 一致，Windows 使用 .ps1 版本）：
#   1. 提示输入 SSH 用户名（密码由 ssh/scp 在连接时提示输入，脚本不保存密码）
#   2. 用 Docker 构建镜像 sub2api:<版本号>（版本号取 backend/cmd/server/VERSION）
#   3. docker save 导出 tar 到本地 dist/ 目录（已被 gitignore）
#   4. 上传 tar 到远端服务器 /data/docker-img/ 目录
#      （ssh 建目录、scp 上传，各提示一次密码）
set -euo pipefail

# ===== 可按需修改的配置 =====
SERVER_HOST='154.51.40.64'      # 远端服务器地址
REMOTE_DIR='/data/docker-img'   # 远端镜像存放目录
IMAGE_NAME='sub2api'            # 镜像名
# 构建期代理：WSL 内容器直连网络不通（Windows 代理 TUN 不转发 WSL 直连流量），
# 构建以 --network=host 运行并通过此代理下载依赖（apk/go/npm）；置空则直连
BUILD_PROXY="${BUILD_PROXY:-http://127.0.0.1:7897}"
# Go 模块源：goproxy.cn 经本机代理会挂流（go 无超时会永久卡住），改用阿里云源
GO_PROXY="${GO_PROXY:-https://mirrors.aliyun.com/goproxy/,direct}"
# ============================

cd "$(dirname "$0")/.."
REPO_ROOT="$(pwd)"

# 1. 输入用户名；密码在 ssh/scp 连接时由其自己提示，不在脚本中出现
read -rp 'SSH 用户名: ' SSH_USER
if [ -z "${SSH_USER// /}" ]; then
    echo '错误：用户名不能为空'
    exit 1
fi

# 2. 版本号：backend/cmd/server/VERSION，读取失败回退 dev
VERSION='dev'
if [ -f backend/cmd/server/VERSION ]; then
    v="$(tr -d '[:space:]' < backend/cmd/server/VERSION)"
    [ -n "$v" ] && VERSION="$v"
fi
TIMESTAMP="$(date +%Y%m%d-%H%M)"
TAG="${IMAGE_NAME}:${VERSION}"
TAR_NAME="${IMAGE_NAME}-${VERSION}-${TIMESTAMP}.tar"
OUT_DIR="${REPO_ROOT}/dist"
TAR_PATH="${OUT_DIR}/${TAR_NAME}"
mkdir -p "$OUT_DIR"

echo ""
echo "==> 目标镜像：${TAG}"
echo "==> 本地产物：${TAR_PATH}"
echo ""

# 3. 检查 docker
if ! command -v docker >/dev/null 2>&1; then
    echo '错误：未找到 docker 命令，请先安装并启动 Docker'
    exit 1
fi
if ! docker version --format '{{.Server.Version}}' >/dev/null 2>&1; then
    echo '错误：Docker daemon 不可用（docker version 失败）。请确认 Docker 已启动'
    exit 1
fi

# 4. 构建镜像（构建上下文为整个仓库，根目录 Dockerfile）
echo "==> docker build -t ${TAG} ${REPO_ROOT}"
echo '    （首次构建需下载基础镜像与依赖，耗时较长）'
echo ""
if [ -n "${BUILD_PROXY}" ]; then
    docker build --network=host \
        --build-arg HTTP_PROXY="${BUILD_PROXY}" \
        --build-arg HTTPS_PROXY="${BUILD_PROXY}" \
        --build-arg NO_PROXY='127.0.0.1,localhost' \
        --build-arg GOPROXY="${GO_PROXY}" \
        -t "$TAG" "$REPO_ROOT"
else
    docker build --build-arg GOPROXY="${GO_PROXY}" -t "$TAG" "$REPO_ROOT"
fi

# 5. 导出 tar
echo ""
echo "==> docker save -o ${TAR_PATH} ${TAG}"
docker save -o "$TAR_PATH" "$TAG"
TAR_SIZE="$(du -h "$TAR_PATH" | cut -f1)"
echo "==> 镜像已导出：${TAR_PATH}（${TAR_SIZE}）"

# 6. 上传：ssh 建目录 + scp 上传（各提示一次密码）
echo ""
echo "==> 在服务器上创建目录 ${REMOTE_DIR}（会提示输入密码）"
ssh -o StrictHostKeyChecking=accept-new "${SSH_USER}@${SERVER_HOST}" "mkdir -p ${REMOTE_DIR}"

echo ""
echo "==> 上传 ${TAR_NAME} -> ${SSH_USER}@${SERVER_HOST}:${REMOTE_DIR}/（会提示输入密码）"
scp "$TAR_PATH" "${SSH_USER}@${SERVER_HOST}:${REMOTE_DIR}/"

echo ""
echo '==> 完成！可在服务器上加载镜像：'
echo "    docker load -i ${REMOTE_DIR}/${TAR_NAME}"
echo "    docker tag ${IMAGE_NAME}:${VERSION} ${IMAGE_NAME}:latest   # 如需 latest 标签"
