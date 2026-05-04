# AGENTS.md

Canonical project guide for AI coding agents (Claude Code, Codex CLI,
Cursor, Aider, Jules, …) working in this repository. Human contributors
should also read this — it's the source of truth for "how things are
laid out and why."

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

本地只构建 mac，win 完全在 GitHub Actions 上跑。

```bash
npm run build:core             # 将 Go 编译为二进制，输出到 core/dist/
npm run build:renderer         # 类型检查 + vite build，输出到 renderer/dist/
npm run package:desktop        # electron-builder 打包，产物在 release/

# 端到端 mac 构建（含签名 + 公证 + staple + 校验）
npm run build:mac              # 调 scripts/release.cjs
npm run build:mac:sign-only    # 签名但不公证，本机快速测试
```

Win 产物：在 GitHub Actions 页面手动触发
`.github/workflows/build-win.yml`，跑完从 run 页面底部 Artifacts
区下载 `biene-win-x64.zip`（外层 GitHub 包装 + 内层是 electron-builder
产的 zip，解两次）。

### 发布到 GitHub

源码 + 二进制都发到 `shupianx/biene-ai`（当前是私有仓库 ——
release assets 仅 collaborator 能下载）。`scripts/publish-github.cjs`
通过 `gh release create` 在当前 origin 创建 draft release 并上传 mac
产物。Win 产物需要手动从 CI artifact 下载后用 `gh release upload`
追加到同一 draft release。

```bash
# 一次性安装 + 登录
brew install gh
gh auth login

# 发布流程（基于当前 package.json 的 version → tag v<version>）
git push origin main --follow-tags    # 把 tag 推到 origin
npm run build:mac                      # 本地构建 mac (.dmg + .zip + latest-mac.yml)
npm run release:mac                    # 创建 draft release + 上传 mac 产物

# 在 GitHub Actions 页面手动触发 Build Windows，跑完下载 artifact
gh release upload v<version> path/to/Biene-<version>-win.zip --clobber

# 然后浏览器打开 release，编辑 notes，点 "Publish release"
```

带 `-alpha` / `-beta` / `-rc` 后缀的 tag 会被自动标记为 GitHub
pre-release（不会成为仓库的 "latest"）。

`gh auth status` 要登录，且账号对仓库有写权限。

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

配置文件位于 `~/.biene/config.json`。首次启动 core 时若不存在会自动生成模板；支持多个命名模型配置，每个配置包含 `provider`、`api_key`、`model` 以及可选的 `base_url`。

支持的 `provider` 取值：

- **`anthropic`** — 走 Anthropic 官方 API（API key）
- **`openai_compatible`**（含历史别名 `openai`）— 走任何 OpenAI 兼容端点（API key + base_url）
- **`chatgpt_official`** — 通过 ChatGPT 账号 OAuth 调用 Codex 后端（`chatgpt.com/backend-api/codex`），无需 API key。**这种条目不写入 `config.json`**，而是由 `auth/` 包根据登录状态动态合成 model_id `chatgpt_official:<openai-model>`，凭据存于 `~/.biene/chatgpt_tokens.json`（0600）。

## 架构总览

### Core（Go — `core/internal/`）

纯 `net/http` 实现，无框架。核心包分工如下：

- **`api/`** — 与 LLM 供应商无关的抽象层。`types.go` 定义内部消息块（`TextBlock` / `ToolUseBlock` / `ToolResultBlock` / `ImageBlock` / `ReasoningBlock`）和 `Provider` 接口；`anthropic.go` / `openai.go` / `chatgpt_codex.go` 各自实现 SDK 协议互转。流式响应通过 `<-chan StreamEvent` 返回。`chatgpt_codex.go` 走 OpenAI Responses API（item-list 形式），用 `codexStreamTracker` 把事件流翻译成 Biene 的 block 模型；用户带图的 turn 走 `buildCodexUserMessageWithImages`（base64 data URI）。

- **`auth/`** — ChatGPT OAuth 流程（PKCE + 公开 Codex CLI client_id）。`chatgpt_oauth.go` 生成 authorize URL 与 PKCE 配对；`chatgpt_callback.go` 在 `localhost:1455` 上一次性监听回调；`chatgpt_manager.go` 持有刷新逻辑（带 5 分钟刷新 skew）+ `Authenticated()` 状态查询；`chatgpt_store.go` 序列化到 `~/.biene/chatgpt_tokens.json`（0600）。`auth.IsChatGPTOfficialModelID` / `ParseChatGPTOfficialModelID` 是合成 model_id 的解析点。

- **`agentloop/`** — 智能体循环引擎。`Run()` 执行一轮对话：调模型（流式）→ 收集工具调用 → 逐个权限检查 + 执行 → 追加结果 → 再次调模型，直到模型不再请求工具。写操作通过 `permission_prep.go` 在模型仍在流式输出时就提前发起权限请求；权限解析结果通过 `tools.WithPermissionResolution` 注入 `context.Context`，供工具 `Execute` 读取（见下文"权限解析通道"）。

- **`server/`** — HTTP transport。`server.go` 注册路由并启动 `ListenAndServe`；`handler_ws.go` 提供两个 WebSocket：`GET /api/sessions/ws`（会话列表级事件）和 `GET /api/sessions/{id}/ws`（单会话实时事件，如 reasoning delta、tool call、permission_request 等）；其他 `handler_*.go` 都是薄 REST 层——解析请求、委托给 session、编码响应。`handler_chatgpt_oauth.go` 协调登录流程的 `start` / `cancel` / `status` / `logout`；`handler_models.go` 暴露 `/api/models/available` 给 New Agent 选择器（用户配置 + 已登录的合成 OAuth 项）。

- **`session/`** — 会话生命周期与状态机。
  - `session.go` / `session_runtime.go`：`Session` 持有消息历史、工具注册表、权限管理器、提供者等，驱动运行循环。`SessionMeta.ModelAvailable` 由 `modelAvailableProvider` 闭包动态计算（chatgpt_official 取决于 OAuth 登录状态），登出时翻 false，handler_chat 据此拒绝 send。
  - `session_manager.go` / `session_manager_*.go`：管理所有会话的创建、查找、删除、持久化恢复，以及 agent-to-agent 交付。`session_manager_config.go` 负责 model_id → ModelEntry 解析（含合成的 chatgpt_official 路径）和 OAuth 状态变化的广播（`BroadcastChatGPTAuthChanged`）。
  - `session_persist.go`：序列化 `display_messages` / `api_messages` 到 SQLite。
  - `session_display.go`：维护 UI 展示用的 `DisplayMessage` 列表（reasoning / text / tool 分段状态机；前端需要与之对称）。
  - `session_files.go`：附件上传、工作区路径校验、agent 间文件复制、收件箱冲突检测。
  - `session_skills.go`：技能安装 / 卸载 / 激活（`use_skill` 通过 `ActivateSkill` 加载正文并标记为活跃）。
  - `session_process.go`：会话级的后台进程注册表，桥接 `processes.Controller`。
  - `session_realtime.go` / `session_manager_realtime.go`：WebSocket 订阅与事件广播。
  - `session_compaction.go`：压缩触发判定。**注意**：context window 缓存在 `Session.contextWindow` 字段（由 `resolveModelEntry` 时填入），不要回退到 `cfg.GetModel` —— 合成 provider（chatgpt_official）不在 `cfg.ModelList`，cfg 查询会落空并回退到 32K 默认值，导致每轮都误触压缩。
  - `tool_mode.go`：在不同智能体 profile / 会话状态下决定启用哪些工具。

- **`tools/`** — 每个工具实现 `Tool` 接口（`tool.go`）。`RegistryForWorkDir()` 预注册文件类工具。`SessionManager` 在创建会话时还会单独注册需要外部依赖的工具（`ListAgents` / `SendMessageToAgent` / `CoworkWithAgent` / `EndCoworkWithAgent` / `ListCoworks` 需 `AgentDirectory`，skills 工具需 `SkillActivator`，process 工具需 controller）。
  - **权限模型**：工具通过 `PermissionKey()` 声明所需权限；`""` 表示只读，`"write"` 表示需用户确认，`"send_message_to_agent"`、`"cowork"` 等也进入同一流程。
  - **权限上下文**（`permission_ctx.go`）：工具可选实现 `PermissionContextProvider.PermissionContext(ctx, rawInput)`，在权限请求发出前计算要展示给用户的上下文（例如文件冲突列表）；用户在 UI 中作出的选择（`resolution`）会通过 `WithPermissionResolution` 注入 `context.Context`，工具 `Execute` 用 `PermissionResolutionFromContext(ctx)` 取回。`send_message_to_agent.go` 是这套机制的典型用法。
  - **内置工具**（`builtins/`）：
    - 文件 / 命令：`read_file` / `write_file` / `edit_file` / `list_files` / `run_command`（Bash，只读命令会跳过权限）。
    - 后台进程：`start_process` / `stop_process` / `read_process_output`。
    - Agent 间通信——两种不同语义，别混为一谈：
      - **`send_message_to_agent`**（邮件式）：发消息为主，可附带文件**快照**，复制到目标 agent 的 `inbox/<selfID>/`（一次性快照，走收件箱冲突检测 + resolution）。对方改了不影响发送方。
      - **`cowork_with_agent`** / `end_cowork_with_agent` / `list_coworks`（协作邀请式）：在目标 agent 工作区里放**符号链接** `cowork/<selfID>/<basename>`，接收方可读写，改动直接落在发送方磁盘上（持续协作，不经过 inbox）。
    - 技能：`list_skills` / `use_skill`。

- **`permission/`** — 每个会话一个权限管理器。待审权限会阻塞智能体循环，直到用户通过 `POST /api/sessions/{id}/permission` 批准或拒绝。`webperm/checker.go` 是实际挂在会话上的实现，负责把 PermissionContext 序列化进 `PermissionRequest`，并通过 decisionEnvelope 把 `resolution` 回传给循环。

- **`processes/`** — 通用的后台进程控制器。负责创建进程组（`pgroup_unix.go` / `pgroup_windows.go`）、收集 stdout/stderr、支持流式日志订阅，被 `start_process` / `stop_process` / `read_process_output` 工具调用。

- **`skills/`** — 技能加载、默认集合和用户技能仓库。`repository.go` 管理用户安装的技能，`loader.go` 读取单个技能正文。

- **`prompt/`** — 构建发给 LLM 的系统提示。`profile.go` 定义 `AgentProfile`（`Domain`：`coding`/`general`；`Style`：`balanced`/`concise`/`thorough`/`skeptical`/`proactive`）；`system.go` 的 `Build()` 将 profile、已注册工具、工作目录、收件箱规则等组装成结构化提示。

- **`store/`** — 会话持久化。每个会话在 `workspace/` 下有独立目录，内含 `meta.json`（元数据）和 `history.db`（SQLite，两张表：`display_messages` 和 `api_messages`）。

- **`config/`** / **`bienehome/`** — 读写 `~/.biene/config.json` 与 biene 主目录。

- **`templates/`** — 内置的 provider/model 模板（vendor + base_url + context window）。`LookupContextWindow` 是合成 provider 也共用的窗口大小来源——同一个模型不管走 manual API key 还是 OAuth 都拿到同一个上限。

### Renderer（Vue 3 — `renderer/src/`）

- **状态管理**：`stores/sessions.ts`（Pinia）。`AgentSession` 保存会话元数据、展示消息、流式状态和待审权限。Store 在 session 附加时建立 WebSocket 连接，把事件合并进本地状态；前端分段状态机需要与后端 `session_display.go` 对称。

- **路由**（`router.ts`）：`/` → `GridView`（会话网格），`/agent/:id` → `AgentView`（完整聊天界面）。Electron 打开的二级智能体窗口直接加载 `/agent/:id`。

- **API 层**：`api/http.ts` 为每个 REST 端点封装带类型的调用函数；`api/ws.ts` 连接两个 WebSocket 端点并把类型化事件分发给 store。

- **关键组件**：`AgentChatView.vue`（主聊天面板，含 `model_available=false` 时的 composer 屏蔽 banner）、`MessageItem.vue`、`ToolCallCard.vue`、`PermissionDialog.vue`（批准/拒绝写操作并附带 resolution）、`InputBar.vue`、`ProcessCapsule.vue` / `ProcessLogPanel.vue`（后台进程展示）、`SkillsBrowser.vue`、`DesktopSettingsModal.vue`（含 ChatGPT OAuth 卡片）、`NewAgentModal.vue`（统一 picker：用户配置 + ChatGPT OAuth 合成项）。

### Electron（`electron/`）

`main.cjs` 先找一个空闲端口，再启动 `biene-core`（开发环境用 `go run .`），轮询 `/api/health` 确认就绪后创建主 `BrowserWindow`。智能体窗口是通过 IPC（`desktop:openAgentWindow`）打开的无边框次级窗口，core 的 URL 通过 `additionalArguments` 传给渲染层。`preload.cjs` 以上下文隔离暴露 `window.bieneDesktop` 桥接（含 `openExternal` 等）。

开发环境的 workspace 在项目根目录 `workspace/`，打包后落在 Electron `userData` 下。

## 关键数据流与设计约定

**发送消息**：`InputBar` → `POST /api/sessions/{id}/send` → 入队 → `agentloop.Run()` 启动循环 → WebSocket 广播流式响应 → store 实时更新。当 session 的 `model_available === false` 时（如 chatgpt_official 撤销 OAuth 后），handler_chat 直接返回 403 + `model_unavailable`，前端 banner 已经先一步把 composer 屏蔽掉，403 只是兜底。

**需要权限的工具执行**：循环触发写类工具 → webperm Checker 先调用工具的 `PermissionContext`（若实现）→ WebSocket 发出 `permission_request`（含 context）→ store 设置 `pendingPermission` → `PermissionDialog` 弹出 → 用户选择决策 + resolution → `POST /api/sessions/{id}/permission` → 循环解除阻塞，将 resolution 注入工具的 execute ctx。

**ChatGPT OAuth 状态同步**：登录/登出完成后，handler 调用 `SessionManager.BroadcastChatGPTAuthChanged()`，遍历每个 chatgpt_official 的 session 重发 meta，前端通过 WebSocket 收到 `session_updated`，grid 卡片透明度 + chat 面板 banner 即时更新，无需轮询。

**收件箱约定（Inbox）**：每个 agent 的工作区下都有 `inbox/` 目录；**用户上传**的文件进入 `inbox/user/`（`session_files.go` 中的 `UserUploadSubdir` 常量），**其他 agent 传来**的文件进入 `inbox/<sourceAgentID>/`（`AgentInboxSubdir()` 生成）。**这条规则由服务端强制执行**，`send_message_to_agent` 工具只能声明要发送的源文件路径，目标路径由 `copyFilesBetweenWorkspaces` 按公式推导——这是 capability isolation：agent 的工具调用只是"请求"，实际落盘由服务端代表它执行。

**文件名冲突**：`send_message_to_agent` 的 `PermissionContext` 先调用 `SessionManager.DetectFileCollisions` 把目标收件箱中已有的同名文件列出给用户；用户在 `PermissionDialog` 里选择 `rename` / `overwrite` / `skip` 中的一个作为整体策略；策略通过 resolution 通道传回工具 `Execute`，由服务端在复制时落实。

**会话持久化**：每轮对话后 `display_messages` 和 `api_messages` 都序列化写入 SQLite；启动时 `SessionManager.Init()` 扫描 workspace 中已有的 `meta.json` 并重建会话。

## Schema 设计准则（重要）

所有由后端拥有、磁盘持久化、且和前端共享的 JSON schema（`meta.json`、`config.json`、未来的模板/技能元数据等）遵循同一条规则：

> **后端定义"地板"，前端可以盖"天花板"——字段不能少，但可以多。**

具体含义：

- **后端必须输出**所有它认领的 typed 字段（`SessionMeta` 上声明的每一个 JSON tag）。这是和前端、其他客户端的契约——少一个字段就是 breaking change。
- **后端必须透传**它不认识的字段。前端写进文件的额外字段（典型例子：`avatar`，纯 UI 资源），后端读出 → 修改 → 写回时不能把它丢掉。
- **后端从不解析、不验证、不生成**这些"前端独占"字段。它们对后端就是不透明的字节，只负责搬运。

### 为什么需要这条准则

Go 的 `encoding/json` 默认把未知字段**静默丢弃**。如果不显式做透传，前端写进 meta.json 的任何 UI 状态都会在下一次后端写回时消失——这是个无声的 bug 源头。

把字段责任划分清楚（"后端拥有数据契约，前端拥有 UI 状态"）比让所有字段都进 Go struct 更扩展友好——前端加新 UI 字段不用改后端，未来其他客户端（CLI、API 用户）也清晰知道哪些字段是真理、哪些是装饰。

### 实现机制：Extras 透传

凡是受这条准则约束的 schema struct，都用同一种模式：

```go
type SessionMeta struct {
    ID   string `json:"id"`
    Name string `json:"name"`
    // ...后端拥有的字段

    // Extras 捕获所有非 typed 字段，原样在磁盘上保留。
    // 后端从不解析/修改这部分，只在序列化时合并回最终 JSON。
    Extras map[string]json.RawMessage `json:"-"`
}
```

配套自定义 `UnmarshalJSON` / `MarshalJSON`：读时 typed 字段走正常解码、剩下的塞进 Extras；写时 typed 字段先 marshal、再合并 Extras 中所有未冲突的 key。

具体实现见 `core/internal/store/extras.go`（待落地）。新增受准则约束的 struct 时，复用同一个 helper，不要每个 struct 自己实现一遍。

### 例外：哪些字段属于"前端独占"

| 字段 | 归属 | 说明 |
|---|---|---|
| `meta.json :: avatar` | 前端 | sprite 索引，纯 UI；后端不生成、不验证、原样透传 |

新加"前端独占"字段时**必须更新本表**，否则未来 maintainer 无从知道边界。

### 不受这条准则约束的范围

- SQLite 表（`history.db`）—— schema 由 Go 代码定义，迁移走 ALTER TABLE，不存在"前端独占字段"的概念
- 用户脚本/技能内部文件—— 那是用户内容，不是 schema
</content>
</invoke>