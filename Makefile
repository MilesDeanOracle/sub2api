.PHONY: help dev dev-local build build-backend build-frontend build-docker test test-backend test-frontend test-frontend-critical

# 裸 make 仍保持编译行为（help 现在是第一个目标，需显式声明默认目标）
.DEFAULT_GOAL := build

ifeq ($(OS),Windows_NT)
# Windows（PowerShell/cmd 下 make 用 cmd 执行命令）：PATH 里没有 bash，
# $(shell) 探测也不可用，统一直接用 corepack pnpm（Node 自带 corepack）
PNPM := corepack pnpm
else
# Linux/macOS：优先用 PATH 里的 pnpm，找不到回退 corepack pnpm
PNPM := $(shell command -v pnpm >/dev/null 2>&1 && echo pnpm || echo corepack pnpm)
endif

FRONTEND_CRITICAL_VITEST := \
	src/i18n/__tests__/localeKeyCompleteness.spec.ts \
	src/api/__tests__/client.spec.ts \
	src/api/__tests__/tokenRefresh.spec.ts \
	src/api/__tests__/keys.bulkUpdate.spec.ts \
	src/composables/__tests__/useModelWhitelist.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.spec.ts \
	src/components/account/__tests__/OpenAIReferralCell.transport.spec.ts \
	src/components/account/__tests__/OpenAIQuotaResetCell.spark_shadow.spec.ts \
	src/constants/__tests__/platforms.spec.ts \
	src/components/account/__tests__/credentialsBuilder.platformCatalog.spec.ts \
	src/components/account/__tests__/CreateAccountModal.spec.ts \
	src/components/account/__tests__/EditAccountModal.spec.ts \
	src/components/account/__tests__/credentialsBuilder.spec.ts \
	src/components/account/__tests__/OpenCodeGoProtocolRulesEditor.spec.ts \
	src/components/keys/__tests__/BulkEditKeysModal.spec.ts \
	src/components/admin/user/__tests__/UserPlatformQuotaModal.spec.ts \
	src/views/user/__tests__/KeysView.spec.ts \
	src/api/__tests__/channelMonitorV2.spec.ts \
	src/views/auth/__tests__/LinuxDoCallbackView.spec.ts \
	src/views/auth/__tests__/WechatCallbackView.spec.ts \
	src/views/user/__tests__/PaymentView.spec.ts \
	src/views/user/__tests__/PaymentResultView.spec.ts \
	src/views/user/__tests__/ChannelStatusView.mode.spec.ts \
	src/components/user/profile/__tests__/ProfileInfoCard.spec.ts \
	src/components/user/profile/__tests__/ProfileIdentityBindingsSection.spec.ts \
	src/views/admin/__tests__/SettingsView.spec.ts \
	src/features/channel-monitor-v2/__tests__/designSystem.structure.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorFormat.spec.ts \
	src/features/channel-monitor-v2/__tests__/monitorZoom.spec.ts \
	src/components/admin/channel/__tests__/PricingEntryCard.modelDefaultPrice.spec.ts

## 用法: make <target>
##
## 可用命令:
ifeq ($(OS),Windows_NT)
help: ## 显示所有可用命令
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/make-help.ps1
else
help: ## 显示所有可用命令
	@awk -F ':.*##' '/^##/ { sub(/^## */, ""); print $$0; next } /^[a-zA-Z0-9_-]+:.*##/ { if (!seen[$$1]++) { sub(/^ +/, "", $$2); printf "  \033[36m%-24s\033[0m %s\n", $$1, $$2 } }' $(MAKEFILE_LIST)
endif

ifeq ($(OS),Windows_NT)
dev: ## 通过 SSH 隧道连接远程后端并启动前端开发服务器（输入用户名密码后连接）
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev.ps1
else
dev: ## 通过 SSH 隧道连接远程后端并启动前端开发服务器（输入用户名密码后连接）
	@bash scripts/dev.sh
endif

ifeq ($(OS),Windows_NT)
dev-local: ## 启动本地前后端开发服务（连接本地 PostgreSQL 和 Redis）
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/dev-local.ps1
else
dev-local: ## 启动本地前后端开发服务（连接本地 PostgreSQL 和 Redis）
	@bash scripts/dev-local.sh
endif

ifeq ($(OS),Windows_NT)
build-docker: ## 用 WSL Docker 打包镜像并上传到服务器 /data/docker-img（输入用户名密码后上传）
	@powershell -NoProfile -ExecutionPolicy Bypass -File scripts/build-docker.ps1
else
build-docker: ## 用 Docker 打包镜像并上传到服务器 /data/docker-img（输入用户名密码后上传）
	@bash scripts/build-docker.sh
endif

build: build-backend build-frontend ## 编译前后端（后端 + 前端）

build-backend: ## 编译后端（复用 backend/Makefile）
	@$(MAKE) -C backend build

build-frontend: ## 编译前端（需要已安装依赖）
	@$(PNPM) --dir frontend run build

test: test-backend test-frontend ## 运行全部测试（后端 + 前端）

test-backend: ## 运行后端测试
	@$(MAKE) -C backend test

test-frontend: ## 运行前端检查（lint + 类型检查 + 关键单测）
	@$(PNPM) --dir frontend run lint:check
	@$(PNPM) --dir frontend run typecheck
	@$(MAKE) test-frontend-critical

test-frontend-critical: ## 运行前端关键单测
	@$(PNPM) --dir frontend exec vitest run $(FRONTEND_CRITICAL_VITEST)
