# FRouter 全仓改动映射（可追踪清单）

> 目的：做**破坏性重命名**，把 `FRouter` 作为主要对外操作单元（同时 Node 独立），统一后端/前端/SDK/文档口径，并**删除**旧命名与 legacy 字段（不做兼容）。

## 快速检索关键词

- 对外契约：`/frouters` / `frouterId` / `/proxy/start` / `/proxy/config`
- 旧命名（应为 0）：`/routes` / `routeId` / `defaultRouteId` / `Route`

## 重点落点（按模块）

### Backend

- Domain：`backend/domain/entities.go`（`FRouter`/`ProxyConfig`/`ChainProxySettings`）
- API：`backend/api/router.go`（`/frouters`、`/frouters/:id/graph`、`/nodes`、`/proxy/*`、`/tun/*` 等）
- Repo：`backend/repository/interfaces.go`（`FRouterRepository`/`NodeRepository`）+ `backend/repository/memory/*_repo.go`

### Frontend

- 主题 UI：`frontend/theme/light.html`、`frontend/theme/dark.html`（主操作单元为 FRouter；主页自动测速/测延迟当前 FRouter）
- 图编辑器：`frontend/chain-editor/chain-editor.js`（接口：`/frouters/:id/graph`；图语义：`local`/`direct`/`block`/slot；只允许 `local->*` 写规则）

### SDK

- 源码：`frontend/sdk/src/vea-sdk.js`（资源 API 改 `frouters`，删除旧别名/状态管理兼容层）
- 类型与文档：`frontend/sdk/src/types.d.ts`、`frontend/sdk/README.md`
- 产物：`frontend/sdk/dist/*`（构建产物与源码一致）

### Docs

- OpenAPI：`docs/api/openapi.yaml`（`/frouters` + `FRouter` schema + 新字段）
- 架构：`docs/ARCHITECTURE_V2.md`（所有 Route 口径改 FRouter）

## 验收标准（每次提交自检）

1. `go test ./...` 通过
2. `cd frontend/sdk && npm run build` 成功，且 `frontend/sdk/dist/` 更新与源码一致
3. UI 冒烟：能创建/刷新/测速/测延迟 FRouter；能启动/停止代理（绑定 `frouterId`）
