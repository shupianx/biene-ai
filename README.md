# Biene

Biene is a desktop AI coding assistant built as an Electron app with a local Go core service and a Vue 3 renderer.

Each agent runs in its own workspace, has its own prompt profile and tool permissions, and can exchange messages or files with other agents. The current codebase is oriented around local, multi-agent coding workflows rather than a generic chat UI.

## What It Does

- Create multiple agents, each with its own working directory and persisted history
- Configure per-agent profiles:
  - domain: `coding` or `general`
  - style: `balanced`, `concise`, `thorough`, `skeptical`, or `proactive`
  - custom instructions
- Control tool permissions per agent:
  - file changes
  - agent-to-agent transfer
- Let agents read, write, and edit files inside their own workspace
- Let agents discover and message other agents, with optional file delivery and reply requests
- Stream tool activity and assistant output live in the UI
- Interrupt an in-flight agent run from the chat input
- Render assistant output as Markdown, including code blocks, tables, lists, and task lists

## Architecture

The repository is split into three main parts:

- [`electron/`](./electron): desktop shell, window lifecycle, and process orchestration
- [`renderer/`](./renderer): Vue 3 + TypeScript frontend
- [`core/`](./core): local Go HTTP service that manages sessions, tool execution, permissions, persistence, and model providers

At runtime:

- Electron starts the local core service
- The renderer talks to that core over HTTP + SSE
- Agent state is persisted on disk
- In development, agent workspaces live under [`workspace/`](./workspace)
- In packaged builds, workspaces are stored under Electron `userData`

## Configuration

Global config lives at `~/.biene/config.json`.

You can create it interactively with:

```bash
cd core
go run . config init
```

You can also edit it directly. A minimal example:

```json
{
  "default_model": "main",
  "model_list": [
    {
      "name": "main",
      "provider": "anthropic",
      "api_key": "YOUR_API_KEY",
      "model": "claude-opus-4-6",
      "base_url": ""
    }
  ],
  "settings": {
    "max_tokens": 8192
  }
}
```

Notes:

- `provider` supports `anthropic` and OpenAI-compatible endpoints
- `base_url` is mainly used for OpenAI-compatible providers
- If the config file does not exist, the core will generate a template automatically

## Development

### Requirements

- A recent Node.js + npm installation
- Go matching [`core/go.mod`](./core/go.mod)

### Install dependencies

```bash
npm install
npm install --prefix renderer
```

### Start the desktop app in development

```bash
npm run dev
```

This does the following:

- starts the Vite dev server for the renderer
- launches Electron
- lets Electron start the Go core service with a local workspace directory

### Run pieces separately

Start the core manually:

```bash
cd core
go run . --host 127.0.0.1 --port 8080 --workspace ../workspace
```

Then start the renderer against that core:

```bash
VITE_CORE_URL=http://127.0.0.1:8080 npm --prefix renderer run dev
```

## Build and Packaging

Build the packaged desktop app:

```bash
npm run build
```

Useful subcommands:

- `npm run build:renderer`
- `npm run build:core`
- `npm run package:desktop`

Packaging output is written to [`release/`](./release).

## Repository Layout

```text
.
├── core/       # Go core service
├── electron/   # Electron main/preload processes
├── renderer/   # Vue frontend
├── scripts/    # development and build helpers
└── workspace/  # development-time agent workspaces
```

## Current Tool Surface

The current registered agent tools are:

- `Read`
- `Write`
- `Edit`
- `ListAgents`
- `SendToAgent`

The permission model currently groups tool access into:

- `write`
- `send_to_agent`

## Persistence

Per-agent state is stored in each session workspace under `.biene/`, including:

- chat history
- session metadata
- permission state
- uploaded or transferred file references

## Status

This repository is already usable as a local multi-agent desktop assistant, but it is still early-stage:

- the root app flow is implemented
- the core/renderer/Electron split is in place
- packaging is wired up
- developer docs and contributor-facing polish are still catching up
