# FRouter 术语与口径（设计说明）

## 结论（唯一口径）

- **对外概念**：使用 **FRouter** 作为一等概念与主要操作单元。
- **对外契约**：HTTP API 使用 `/frouters`，JSON 字段使用 `frouterId`，领域模型使用 `FRouter`。
- **节点（Node）独立**：Node 为独立资源（`/nodes`）；通常由配置/订阅同步生成；`FRouter` 仅通过链式代理图引用 `NodeID`。
- **不做兼容**：不保留旧命名（`Route` / `/routes` / `routeId`）与旧字段（如 `defaultNodeId` / `nodeId`）。

## 命名约定

- UI/文档：统一写作 **FRouter**（必要时可附注中文解释）。
- 代码/接口：统一使用 `FRouter` / `/frouters` / `frouterId`。

## 参考

- `backend/domain/entities.go`
- `docs/api/openapi.yaml`
