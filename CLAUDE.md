# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目简介

Biene 是一个本地优先的 AI 编程助手桌面应用，分为三层：

- **`core/`** — Go HTTP 服务，负责运行 AI 智能体循环和工具执行
- **`electron/`** — Electron 外壳，负责启动 core 进程并托管渲染层
- **`renderer/`** — Vue 3 前端，提供聊天界面

## 前置要求

- Node.js + npm（最新 LTS 即可）
- Go（版本见 `core/go.mod`）

## 常用命令

### 依赖安装

```bash
npm install                    # 根目录（Electron）
npm install --prefix renderer  # 渲染层
```

### 开发

```bash
npm run dev          # 启动一切：Vite 开发服务器 + Electron（core 使用 go run）
```

此命令等待 Vite 在 `http://127.0.0.1:5173` 就绪后启动 Electron，并设置 `BIENE_RENDERER_URL` 环境变量，使主进程用 `go run .` 代替编译好的二进制文件运行 core。

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
npm run build:core       # 将 Go 编译为二进制文件，输出到 core/dist/
npm run build:renderer   # 构建 Vue 应用，输出到 renderer/dist/
npm run build            # 同时执行上面两步
npm run package:desktop  # 使用 electron-builder 打包，产物在 release/
```

### Core（Go）

```bash
cd core
go build ./...           # 编译
go test ./...            # 运行所有测试
go test ./internal/...   # 运行指定包树下的测试
```

### Renderer（Vue）

```bash
cd renderer
npm run dev              # 仅启动 Vite 开发服务器（不含 Electron）
npm run build            # vue-tsc 类型检查 + vite build
```

### 配置

配置文件路径：`~/.biene/config.json`

首次启动 core 时，如果该文件不存在，会自动生成模板；之后直接编辑即可。支持多个命名模型配置，每个配置包含 `provider`（`anthropic` 或 `openai_compatible`）、`api_key`、`model` 和可选的 `base_url`。

## 架构说明

### Core（Go — `core/internal/`）

Core 是一个纯 `net/http` 的 Go HTTP 服务，没有额外框架。

**`internal/api/`** — 与 LLM 供应商无关的抽象层。`types.go` 定义了内部消息类型（`TextBlock`、`ToolUseBlock`、`ToolResultBlock`）以及 `Provider` 接口。`anthropic.go` 和 `openai.go` 分别实现该接口，负责与各 SDK 的协议格式互转。所有流式响应以 `<-chan StreamEvent` 返回。

**`internal/query/`** — 智能体循环的核心引擎。`loop.go` 中的 `Run()` 执行一轮完整的对话：调用模型（流式） → 收集工具调用 → 逐个权限检查 + 执行 → 将结果追加到消息历史 → 再次调模型，直到模型不再请求工具。所有中间事件通过 `<-chan Event` 返回给调用方。支持写操作权限的预检（`preparedPermission`），在模型还在流式输出时提前发起权限请求。

**`internal/server/`** — HTTP transport 层。
- `server.go`：注册所有路由并启动 `ListenAndServe`。
- `sse.go`：将 session 发出的事件帧写入 HTTP SSE 响应。
- `handler_*.go`：各处理器文件，逻辑很薄——解析请求、委托给 session、编码响应。

**`internal/session/`** — 会话生命周期与状态机。
- `session.go`：`Session` 结构体，持有单个智能体的核心状态（消息历史、工具注册表、权限管理器、提供者等）以及运行循环的驱动逻辑。
- `session_manager.go`：`SessionManager` 管理所有活跃会话的创建、查找、删除、持久化恢复和 agent-to-agent 交付。
- `session_persist.go`：序列化/反序列化 `display_messages` 和 `api_messages` 到 SQLite。
- `session_display.go`：维护 UI 展示用的 `DisplayMessage` 列表。
- `session_files.go`：处理附件上传、工作区文件路径校验和 agent 间文件复制。

**`internal/tools/`** — 每个工具实现 `Tool` 接口（`tool.go`）。`RegistryForWorkDir()` 预加载文件工具：`Read`、`Write`、`Edit`。`Bash` 工具也已实现（区分只读命令可跳过权限）。`ListAgents` 和 `SendToAgent` 由 `SessionManager` 在创建会话时单独注册（需要 `AgentDirectory` 接口）。工具通过 `PermissionKey`（`write`、`send_to_agent` 或只读为空）声明所需权限；写操作需要用户确认。

**`internal/prompt/`** — 构建发给 LLM 的系统提示词。`profile.go` 定义了 `AgentProfile`（`Domain`：`coding`/`general`；`Style`：`balanced`/`concise`/`thorough`/`skeptical`/`proactive`）。`system.go` 中的 `Build()` 将 profile、已注册工具、工作目录等组装成结构化的系统提示。

**`internal/store/`** — 会话持久化。每个会话在 `workspace/` 下有独立目录，内含 `meta.json`（会话元数据）和 `history.db`（SQLite）。数据库有两张表：`display_messages`（UI 渲染用）和 `api_messages`（发送给 LLM 的原始对话）。

**`internal/config/`** — 读写 `~/.biene/config.json`。

**`internal/permission/`** — 管理每个会话的权限状态。待审权限会阻塞智能体循环，直到用户通过 `POST /api/sessions/{id}/permission` 批准或拒绝。

### Renderer（Vue 3 — `renderer/src/`）

**状态管理**：`stores/sessions.ts`（Pinia）。`AgentSession` 保存会话元数据、展示消息、流式状态和待审权限。Store 在 session 附加时为每个会话建立 SSE 连接，并将收到的事件合并到本地状态。

**路由**（`router.ts`）：`/` → `GridView`（会话列表），`/agent/:id` → `AgentView`（完整聊天界面）。Electron 打开的智能体窗口直接加载 `/agent/:id`。

**API 层**（`api/http.ts`、`api/sse.ts`）：`http.ts` 对每个 REST 端点封装了带类型的调用函数；`sse.ts` 连接 `GET /api/sessions/{id}/events`，并将类型化事件分发给 store。

**关键组件**：`AgentChatView.vue`（主聊天面板）、`MessageItem.vue`（渲染单条消息）、`ToolCallCard.vue`（展示工具调用）、`PermissionDialog.vue`（批准/拒绝写操作）、`InputBar.vue`（消息输入框）。

### Electron（`electron/`）

`main.cjs` 先找一个空闲端口，然后启动 `biene-core`（开发环境用 `go run .`），轮询 `/api/health` 确认就绪后，创建主 `BrowserWindow`。智能体窗口是通过 IPC（`desktop:openAgentWindow`）打开的无边框次级窗口。core 的 URL 通过 `additionalArguments` 传递给渲染层窗口。

`preload.cjs` 以上下文隔离方式暴露一个小型 `window.desktop` 桥接，供渲染层调用 IPC 处理器。

开发环境中 workspace 在项目根目录下的 `workspace/`；打包后存储在 Electron `userData` 下。

## 关键数据流

**发送消息**：`InputBar` → `POST /api/sessions/{id}/send` → Go 处理器将消息入队 → `query.Run()` 启动智能体循环 → 流式获取 LLM 响应并通过 SSE 广播 → 渲染层 store 实时更新消息列表。

**需要权限的工具执行**：智能体循环触发写操作工具 → 发出 `permission_request` SSE 事件 → store 设置 `pendingPermission` → `PermissionDialog` 弹出 → 用户点击批准/拒绝 → `POST /api/sessions/{id}/permission` → 循环解除阻塞。

**会话持久化**：每轮对话后，session 将 `display_messages` 和 `api_messages` 序列化写入 SQLite。启动时，`SessionManager.Init()` 扫描 workspace 目录中已有的 `meta.json` 文件并重新加载会话。
