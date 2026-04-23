# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

Biene 是一个本地优先的 AI 编程助手桌面应用，分为三层：

- **`core/`** — Go HTTP 服务，负责运行 AI 智能体循环和工具执行
- **`electron/`** — Electron 外壳，负责启动 core 进程并托管渲染层
- **`renderer/`** — Vue 3 前端，提供聊天界面

前置要求：Node.js（最新 LTS）+ Go（版本见 `core/go.mod`，当前为 1.26）。

## 常用命令

### 依赖安装

```bash
npm install                    # 根目录（Electron 外壳）
npm install --prefix renderer  # 渲染层
```

### 开发

```bash
npm run dev    # 启动 Vite 开发服务器 + Electron；主进程用 `go run .` 直接跑 core
```

`scripts/dev.cjs` 会等待 Vite 在 `http://127.0.0.1:5173` 就绪后启动 Electron，并通过 `BIENE_RENDERER_URL` 让主进程用 `go run .` 代替编译好的二进制文件运行 core。

#### 单独运行 Core 和 Renderer

```bash
cd core
go run . --host 127.0.0.1 --port 8080 --workspace ../workspace
```

```bash
VITE_CORE_URL=http://127.0.0.1:8080 npm --prefix renderer run dev
```

### 构建

```bash
npm run build:core       # 将 Go 编译为二进制，输出到 core/dist/
npm run build:renderer   # 类型检查 + vite build，输出到 renderer/dist/
npm run build            # 串行执行上面两步
npm run package:desktop  # electron-builder 打包，产物在 release/
```

### Core（Go）

```bash
cd core
go build ./...
go test ./...                            # 全量测试
go test ./internal/session/...           # 某个子树
go test -run TestXxx ./internal/session  # 单个用例
go vet ./...
```

### Renderer（Vue）

```bash
cd renderer
npm run dev              # 仅 Vite（不含 Electron）
npm run build            # vue-tsc 类型检查 + vite build
```

### 配置

配置文件位于 `~/.biene/config.json`。首次启动 core 时若不存在会自动生成模板；支持多个命名模型配置，每个配置包含 `provider`（`anthropic` 或 `openai_compatible`）、`api_key`、`model` 以及可选的 `base_url`。

## 架构总览

### Core（Go — `core/internal/`）

纯 `net/http` 实现，无框架。核心包分工如下：

- **`api/`** — 与 LLM 供应商无关的抽象层。`types.go` 定义内部消息块（`TextBlock` / `ToolUseBlock` / `ToolResultBlock`）和 `Provider` 接口；`anthropic.go` 与 `openai.go` 各自实现 SDK 协议互转。流式响应通过 `<-chan StreamEvent` 返回。

- **`agentloop/`** — 智能体循环引擎。`Run()` 执行一轮对话：调模型（流式）→ 收集工具调用 → 逐个权限检查 + 执行 → 追加结果 → 再次调模型，直到模型不再请求工具。写操作通过 `permission_prep.go` 在模型仍在流式输出时就提前发起权限请求；权限解析结果通过 `tools.WithPermissionResolution` 注入 `context.Context`，供工具 `Execute` 读取（见下文"权限解析通道"）。

- **`server/`** — HTTP transport。`server.go` 注册路由并启动 `ListenAndServe`；`handler_ws.go` 提供两个 WebSocket：`GET /api/sessions/ws`（会话列表级事件）和 `GET /api/sessions/{id}/ws`（单会话实时事件，如 reasoning delta、tool call、permission_request 等）；其他 `handler_*.go` 都是薄 REST 层——解析请求、委托给 session、编码响应。

- **`session/`** — 会话生命周期与状态机。
  - `session.go` / `session_runtime.go`：`Session` 持有消息历史、工具注册表、权限管理器、提供者等，驱动运行循环。
  - `session_manager.go` / `session_manager_*.go`：管理所有会话的创建、查找、删除、持久化恢复，以及 agent-to-agent 交付。
  - `session_persist.go`：序列化 `display_messages` / `api_messages` 到 SQLite。
  - `session_display.go`：维护 UI 展示用的 `DisplayMessage` 列表（reasoning / text / tool 分段状态机；前端需要与之对称）。
  - `session_files.go`：附件上传、工作区路径校验、agent 间文件复制、收件箱冲突检测。
  - `session_skills.go`：技能安装 / 卸载 / 激活（`use_skill` 通过 `ActivateSkill` 加载正文并标记为活跃）。
  - `session_process.go`：会话级的后台进程注册表，桥接 `processes.Controller`。
  - `session_realtime.go` / `session_manager_realtime.go`：WebSocket 订阅与事件广播。
  - `tool_mode.go`：在不同智能体 profile / 会话状态下决定启用哪些工具。

- **`tools/`** — 每个工具实现 `Tool` 接口（`tool.go`）。`RegistryForWorkDir()` 预注册文件类工具。`SessionManager` 在创建会话时还会单独注册需要外部依赖的工具（`ListAgents` / `SendToAgent` 需 `AgentDirectory`，skills 工具需 `SkillActivator`，process 工具需 controller）。
  - **权限模型**：工具通过 `PermissionKey()` 声明所需权限；`""` 表示只读，`"write"` 表示需用户确认，`"send_to_agent"` 等也进入同一流程。
  - **权限上下文**（`permission_ctx.go`）：工具可选实现 `PermissionContextProvider.PermissionContext(ctx, rawInput)`，在权限请求发出前计算要展示给用户的上下文（例如文件冲突列表）；用户在 UI 中作出的选择（`resolution`）会通过 `WithPermissionResolution` 注入 `context.Context`，工具 `Execute` 用 `PermissionResolutionFromContext(ctx)` 取回。`send_to_agent.go` 是这套机制的典型用法。
  - **内置工具**（`builtins/`）：`read_file` / `write_file` / `edit_file` / `list_files` / `run_command`（Bash，只读命令会跳过权限）；`start_process` / `stop_process` / `read_process_output`（长期运行的后台进程）；`list_agents` / `send_to_agent`（agent 间通信）；`list_skills` / `use_skill`（技能）。

- **`permission/`** — 每个会话一个权限管理器。待审权限会阻塞智能体循环，直到用户通过 `POST /api/sessions/{id}/permission` 批准或拒绝。`webperm/checker.go` 是实际挂在会话上的实现，负责把 PermissionContext 序列化进 `PermissionRequest`，并通过 decisionEnvelope 把 `resolution` 回传给循环。

- **`processes/`** — 通用的后台进程控制器。负责创建进程组（`pgroup_unix.go` / `pgroup_windows.go`）、收集 stdout/stderr、支持流式日志订阅，被 `start_process` / `stop_process` / `read_process_output` 工具调用。

- **`skills/`** — 技能加载、默认集合和用户技能仓库。`repository.go` 管理用户安装的技能，`loader.go` 读取单个技能正文。

- **`prompt/`** — 构建发给 LLM 的系统提示。`profile.go` 定义 `AgentProfile`（`Domain`：`coding`/`general`；`Style`：`balanced`/`concise`/`thorough`/`skeptical`/`proactive`）；`system.go` 的 `Build()` 将 profile、已注册工具、工作目录、收件箱规则等组装成结构化提示。

- **`store/`** — 会话持久化。每个会话在 `workspace/` 下有独立目录，内含 `meta.json`（元数据）和 `history.db`（SQLite，两张表：`display_messages` 和 `api_messages`）。

- **`config/`** / **`bienehome/`** — 读写 `~/.biene/config.json` 与 biene 主目录。

### Renderer（Vue 3 — `renderer/src/`）

- **状态管理**：`stores/sessions.ts`（Pinia）。`AgentSession` 保存会话元数据、展示消息、流式状态和待审权限。Store 在 session 附加时建立 WebSocket 连接，把事件合并进本地状态；前端分段状态机需要与后端 `session_display.go` 对称。

- **路由**（`router.ts`）：`/` → `GridView`（会话网格），`/agent/:id` → `AgentView`（完整聊天界面）。Electron 打开的二级智能体窗口直接加载 `/agent/:id`。

- **API 层**：`api/http.ts` 为每个 REST 端点封装带类型的调用函数；`api/ws.ts` 连接两个 WebSocket 端点并把类型化事件分发给 store。

- **关键组件**：`AgentChatView.vue`（主聊天面板）、`MessageItem.vue`、`ToolCallCard.vue`、`PermissionDialog.vue`（批准/拒绝写操作并附带 resolution）、`InputBar.vue`、`ProcessCapsule.vue` / `ProcessLogPanel.vue`（后台进程展示）、`SkillsBrowser.vue`。

### Electron（`electron/`）

`main.cjs` 先找一个空闲端口，再启动 `biene-core`（开发环境用 `go run .`），轮询 `/api/health` 确认就绪后创建主 `BrowserWindow`。智能体窗口是通过 IPC（`desktop:openAgentWindow`）打开的无边框次级窗口，core 的 URL 通过 `additionalArguments` 传给渲染层。`preload.cjs` 以上下文隔离暴露 `window.desktop` 桥接。

开发环境的 workspace 在项目根目录 `workspace/`，打包后落在 Electron `userData` 下。

## 关键数据流与设计约定

**发送消息**：`InputBar` → `POST /api/sessions/{id}/send` → 入队 → `agentloop.Run()` 启动循环 → WebSocket 广播流式响应 → store 实时更新。

**需要权限的工具执行**：循环触发写类工具 → webperm Checker 先调用工具的 `PermissionContext`（若实现）→ WebSocket 发出 `permission_request`（含 context）→ store 设置 `pendingPermission` → `PermissionDialog` 弹出 → 用户选择决策 + resolution → `POST /api/sessions/{id}/permission` → 循环解除阻塞，将 resolution 注入工具的 execute ctx。

**收件箱约定（Inbox）**：每个 agent 的工作区下都有 `inbox/` 目录；**用户上传**的文件进入 `inbox/user/`（`session_files.go` 中的 `UserUploadSubdir` 常量），**其他 agent 传来**的文件进入 `inbox/<sourceAgentID>/`（`AgentInboxSubdir()` 生成）。**这条规则由服务端强制执行**，`send_to_agent` 工具只能声明要发送的源文件路径，目标路径由 `copyFilesBetweenWorkspaces` 按公式推导——这是 capability isolation：agent 的工具调用只是"请求"，实际落盘由服务端代表它执行。

**文件名冲突**：`send_to_agent` 的 `PermissionContext` 先调用 `SessionManager.DetectFileCollisions` 把目标收件箱中已有的同名文件列出给用户；用户在 `PermissionDialog` 里选择 `rename` / `overwrite` / `skip` 中的一个作为整体策略；策略通过 resolution 通道传回工具 `Execute`，由服务端在复制时落实。

**会话持久化**：每轮对话后 `display_messages` 和 `api_messages` 都序列化写入 SQLite；启动时 `SessionManager.Init()` 扫描 workspace 中已有的 `meta.json` 并重建会话。
