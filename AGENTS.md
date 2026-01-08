# Repository Guidelines

## Project Structure & Module Organization
- `main.go`: backend entrypoint (default `--addr :19080`, state snapshot defaults to userData: `--state <userData>/data/state.json`).
- `backend/`: HTTP API (`backend/api/`), entities (`backend/domain/`), services/adapters (`backend/service/`), repositories (`backend/repository/`), persistence (`backend/persist/`), schedulers (`backend/tasks/`).
- `backend/service/proxy/engine_select.go`: engine selection (sing-box vs Clash) based on inbound mode / protocol constraints.
- `backend/domain/entities.go`: domain model of `FRouter`, `Node` (节点), `ProxyConfig`, `ChainProxySettings` and engines.
- `backend/api/router.go`: HTTP endpoints, including `/frouters`, `/frouters/:id/graph`, `/nodes`, `/proxy/*`, `/tun/*`.
- `backend/service/nodegroup/`: compiles runtime plans from `ProxyConfig` + `FRouter` + `Nodes` + `ChainProxySettings` (proxy + measurement).
- `frontend/`: Electron app (`frontend/main.js`) and UI themes (`frontend/theme/*.html`).
- `frontend/theme/*.html`: UI uses FRouter as the primary unit.
- `frontend/sdk/`: zero-dependency JS SDK (Rollup) — `frontend/sdk/dist/` is committed.
- `frontend/sdk/src/vea-sdk.js`: SDK client for nodes / frouters / proxy / settings endpoints.
- `docs/`: design notes and API documentation.
- `docs/api/openapi.yaml`: HTTP API spec (frouters, proxy, tun, settings, etc.).

Legacy runtime dirs (gitignored): `data/`, `artifacts/` (startup migrates them into userData); build outputs: `dist/`, `release/`, root `vea`/`vea.exe`.

## Domain Concepts & Terminology
- `FRouter`：对外的一等操作单元（工具）；通过 `ChainProxySettings` 图引用 `NodeID`；API 入口为 `/frouters`。
- `Node`（节点）：独立资源（食材）；通常由配置/订阅同步生成；对外提供 `/nodes` 列表与测量能力。
- `ProxyConfig`：运行配置（单例；入站模式、frouterId、引擎偏好、TUN 配置等）。
- `ChainProxySettings`（链式代理）：用图结构定义 `local`/`direct`/`block`/slot 与节点之间的连接；`local->*` 是“选择边（可写规则）”，`{node|slot}->{node|slot}` 是 detour 边（不写规则）。

## Build, Test, and Development Commands
Prereqs: Go 1.22+, Node.js 18+. Electron requires a GUI environment.

- `make dev`: build backend, install deps, run Electron.
- `make build`: package desktop app (see `frontend/electron-builder.yml`, output `release/`).
- `make build-backend`: compile backend to `dist/vea` (override `GOOS`/`GOARCH`).
- `cd frontend/sdk && npm run build`: rebuild SDK bundles in `frontend/sdk/dist/`.

Backend-only (useful on headless machines):
```bash
go run . --dev --addr :19080
```
Ports: fixed to `:19080`.

## Coding Style & Naming Conventions
- Go: `gofmt`; prefer sentinel errors (`ErrXxx`) and standard `log`.
- API handlers: follow existing verb naming (`listXxx`, `createXxx`, `updateXxx`, `deleteXxx`).
- API/domain design: prefer FRouter as the primary unit (`frouters`, `frouterId`); avoid adding new standalone Node CRUD surface unless explicitly required.
- JS/HTML: follow existing 2-space indentation and the no-semicolon style used in `frontend/`.
- UI copy: prefer “FRouter” to represent the active unit; avoid re-introducing “当前节点” as a primary concept.
- Do not commit build outputs (`dist/`, `release/`, `data/`, `artifacts/`), except `frontend/sdk/dist/`.

## Engine & Routing Notes
- Engine selection is constrained by inbound mode and protocol support (e.g. TUN and some protocols may force sing-box); see `backend/service/proxy/engine_select.go`.
- Routing rules are generated from FRouter compile output (`backend/service/nodegroup/`) and translated by adapters (`backend/service/adapters/`).

## Testing Guidelines
- Default: `go test ./...`
- Quick run: `go test -short ./...` (skips networked `TestE2E_*`).

## Compatibility & Migration Notes
- This repo does not keep legacy naming/fields for backward compatibility. Breaking changes are allowed and should fail fast at compile time.
- Chain proxy graph uses virtual endpoints; front-end IDs `local`/`direct`/`block` 与 slot 会被后端按常量语义处理（见 `backend/domain/entities.go` 与 `backend/service/nodegroup/`）。

## Commit & Pull Request Guidelines
- Prefer Conventional Commits (common in history): `feat:`, `fix:`, `docs:`, `chore:`, `ci:`, `build:` (Chinese subject/body is OK).
- PRs should include: what/why, steps to test, and screenshots/GIFs for UI changes (`frontend/theme/`). Ensure CI passes on Linux/macOS/Windows.

## Security & Privilege Notes
- Never commit credentials, tokens, or real subscription URLs; use `.env`/local config (gitignored).
- TUN/system-proxy features may require elevated privileges; be careful when changing Linux `pkexec`/polkit assets under `scripts/`.
