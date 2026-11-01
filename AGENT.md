# AGENT.md — 开发守则

> 本文件是对 AI Agent 的硬性约束。环境配置、本地服务、已知坑点见 `DEV_GUIDE.md`，本文不重复。

## 项目概览

- **后端**：Go 1.27 + Gin + Ent ORM，PostgreSQL 16 + Redis，代码在 `backend/`（分层：`internal/handler` → `internal/service` → `internal/repository`）。
- **前端**：Vue 3 + TypeScript + Vite + Tailwind，包管理只用 **pnpm**（不是 npm），代码在 `frontend/`。
- **变更流程**：较大的功能先在 `openspec/changes/` 下写提案（proposal / design / tasks），再动手实现。
- **提交信息**：Conventional Commits + 中文描述，如 `feat(frontend): xxx`、`fix(backend): xxx`。

## 一、前端规范（重点）

### 1. UI 框架选型：新页面一律 shadcn，禁用 Element Plus

- **新增的菜单、页面、功能模块**，一律使用 **shadcn-vue** 风格实现，**不要引入 element-plus 的新组件**。
  - shadcn 基础组件在 `frontend/src/components/ui/`（button、badge、input、separator 等），由 `frontend/components.json` 约定别名（`@/components/ui`），缺什么用 shadcn CLI 按 `components.json` 添加，不要手写重复轮子。
  - 类名合并统一用 `cn()`（`frontend/src/lib/utils.ts`），样式用 Tailwind 工具类 + CVA variants 模式，参照现有 `ui/button` 的写法。

### 2. 已有同功能的弹窗、抽屉，直接搬过来，不要用 shadcn 再做一套

- **动手前先搜全库**：要做的弹窗 / 抽屉如果库里已经有实现了同功能的（例：建账号、指派分组、批量编辑等），**直接把那个组件搬过来用**——import 现有组件即可，位置不合适的就平移到公共目录再引用。**即使它不是 shadcn 风格（如 element-plus 存量组件），也优先搬，不要按新页面的风格用 shadcn 重新实现一遍。**
- 两条规则的关系：第 1 条约束"新建"——只有全库确实没有同功能实现时才新建，新建的必须按 shadcn 风格写；本条约束"复用"——功能组件以"找得到、能直接用"为先，风格统一让位于不重复建设。
- 已有的功能组件先查这些地方：

| 功能 | 已有实现 |
|------|----------|
| 通用对话框 / 确认框 | `src/components/common/`：`BaseDialog`、`ConfirmDialog`、`Toast` 等（该目录有 `README.md` 和 `index.ts`，先读再用） |
| 页面骨架 | `src/components/layout/`：`AppLayout`、`TablePageLayout` |
| 列表页 | `DataTable` + `Pagination` + `SearchInput` + `EmptyState` + `StatusBadge`（均在 `components/common/`） |
| 业务弹窗（账号、密钥、用户、分组等） | `src/components/account/`（如 `CreateAccountModal`、`EditAccountModal`）、`src/components/keys/`、`src/components/admin/`、`src/components/element/` 下的存量 Modal/Drawer |
| 图标、徽章、选择器 | `PlatformIcon`、`ModelIcon`、`GroupBadge`、`GroupSelector`、`Select` 等 |

- 复用方式优先级：**直接使用 > 加 props/slot 扩展 > 抽取公共部分**。新建前先试试给现有组件加 props/slot 能否满足。
- 确实需要新建时才建，并在提交说明 / PR 描述里写清楚"库里为什么没有可复用的"；新组件放对位置：shadcn 基础件进 `components/ui/`，通用业务件进 `components/common/` 或对应业务目录（如 `components/account/`），不要散落在 views 里。

### 3. 其他前端约定

- **i18n**：所有文案走 `src/i18n/`，zh / en 两个 locale 必须同步补全，`pnpm build` 内置 `check:i18n` 校验，缺 key 会挂。
- **包管理**：只用 pnpm；新增依赖后必须同步提交 `pnpm-lock.yaml`（CI 用 `--frozen-lockfile`）。
- **校验命令**（提交前跑）：

```bash
pnpm --dir frontend run lint:check
pnpm --dir frontend run typecheck
pnpm --dir frontend run test:run
```

## 二、总体要求

### 1. 遵循现有设计，不要自创

- 改动前先看同目录、同类功能是怎么写的，**照着现有模式来**：命名、目录归属、分层职责、错误处理、状态管理（Pinia stores）、API 封装（`src/api/`）都保持一致。
- **禁止**未经确认做以下事情：
  - 调整代码结构或目录结构（如移动、重命名、合并现有模块）；
  - 引入新的中间件、新依赖、新的状态管理 / 请求库 / UI 框架；
  - 改动 CI、构建脚本、Dockerfile 的既有约定（Go 版本联动规则见 `DEV_GUIDE.md`）。
- 如果确实认为某处结构不合理需要重构，先单独说明理由并征得同意，不要夹带在功能提交里。

### 2. 复用优先，新增须说明理由

- **优先使用已有方法、函数、组合式（`src/composables/`）、工具（`src/utils/`、后端 `internal/util` 等）**。写新代码前先搜一遍现有实现，避免重复建设。
- 现有实现确实无法满足时才新增，并且要：
  1. 新增代码放在项目约定的对应分层 / 目录，不另起炉灶；
  2. 在提交说明或代码注释中写清"现有方案为什么不够、新增了什么"。
- 后端同理：沿用 Ent + Gin 的既有分层与写法，改 Ent schema 后必须重新生成代码（见 `DEV_GUIDE.md` 坑 9）。

## 三、常用命令

```bash
make dev-local        # 本地起前后端开发服务（连本地 PostgreSQL / Redis）
make dev              # SSH 隧道连远程后端 + 本地前端
make build            # 编译前后端
make test             # 全部测试
make test-backend     # 后端测试（也可 cd backend && go test -tags=unit ./...）
make test-frontend    # 前端 lint + typecheck + 关键单测
```

提交前请确保对应测试与 lint 通过；Windows 环境的已知坑（psql、PowerShell 转义、node_modules 冲突等）一律查 `DEV_GUIDE.md`。
