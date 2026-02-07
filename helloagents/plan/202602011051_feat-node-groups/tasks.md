# 任务清单: feat-node-groups

目录: `helloagents/plan/202602011051_feat-node-groups/`

---

## 任务状态符号说明

| 符号 | 状态 | 说明 |
|------|------|------|
| `[ ]` | pending | 待执行 |
| `[√]` | completed | 已完成 |
| `[X]` | failed | 执行失败 |
| `[-]` | skipped | 已跳过 |
| `[?]` | uncertain | 待确认 |

---

## 执行状态
```yaml
总任务: 26
已完成: 0
完成率: 0%
```

---

## 任务列表

### 1. Domain & Schema

- [ ] 1.1 在 `backend/domain/entities.go` 新增 `NodeGroupStrategy` + `NodeGroup`，并扩展 `ServiceState` 增加 `NodeGroups`
  - 验证: `go test ./...` 编译通过

- [ ] 1.2 在 `backend/persist/migrator.go` 升级 `SchemaVersion` 并新增 2.1.0 → 新版本迁移（`NodeGroups` 默认为空）
  - 依赖: 1.1
  - 验证: `go test ./...`；旧 schemaVersion 的 state.json 可被加载

### 2. Repository & Memory Store

- [ ] 2.1 在 `backend/repository/interfaces.go` 增加 `NodeGroupRepository` 与 `Repositories.NodeGroup()`
  - 验证: `go test ./...` 编译通过

- [ ] 2.2 在 `backend/repository/errors.go` 增加 `ErrNodeGroupNotFound`（以及必要的 InvalidData 复用策略）
  - 依赖: 2.1

- [ ] 2.3 在 `backend/repository/memory/store.go` 增加 nodeGroups 存储，并在 `Snapshot/LoadState` 支持 `NodeGroups`
  - 依赖: 1.1
  - 验证: 快照保存/加载不丢失 NodeGroups

- [ ] 2.4 新增 `backend/repository/memory/nodegroup_repo.go`（CRUD + 事件发布）
  - 依赖: 2.1, 2.3

- [ ] 2.5 新增测试 `backend/repository/memory/nodegroup_repo_test.go`
  - 依赖: 2.4
  - 验证: `go test ./...`

### 3. NodeGroupResolver（四策略）

- [ ] 3.1 新增 `NodeGroupResolver`（建议位置：`backend/service/nodegroup/nodegroup_resolver.go`），实现：
  - 解析规则：NodeID 优先，其次 NodeGroupID；支持 `edges.from/edges.to/edges.via[*]`
  - 策略：lowest-latency / fastest-speed / round-robin / failover
  - 错误：组不存在、组内无可用节点、引用非法 ID 时返回可定位的错误

- [ ] 3.2 新增测试 `backend/service/nodegroup/nodegroup_resolver_test.go`（覆盖四策略 + cursor 行为 + 缺失节点/空组）
  - 依赖: 3.1
  - 验证: `go test ./...`

### 4. 编译/代理/测速接入

- [ ] 4.1 更新 `backend/service/proxy/service.go`：`Start()` 链路在编译前对目标 `FRouter` 做 resolver 解析
  - 依赖: 3.1
  - 验证: 启动代理成功，生成的 config.explain.txt 中 default 为 `node:...`（不出现 NodeGroup）

- [ ] 4.2 更新 `backend/service/proxy/engine_select.go`：引擎选择前解析 NodeGroup（避免协议/节点集合判断失真）
  - 依赖: 3.1

- [ ] 4.3 更新 `backend/service/proxy/speed_measurer.go`：测延迟/测速与 `startMeasurement()` 在编译前解析 NodeGroup
  - 依赖: 3.1

- [ ] 4.4 更新 `backend/api/router.go`：所有 `CompileFRouter` 校验点改为“对 resolved 副本编译”，但持久化仍保存原始图（包含 NodeGroupID）
  - 依赖: 3.1
  - 验证: 允许保存包含 NodeGroupID 的图；`POST /frouters/:id/graph/validate` 返回 OK

### 5. NodeGroup Service & API

- [ ] 5.1 新增 `backend/service/nodegroups/service.go`（NodeGroup CRUD）
  - 依赖: 2.1, 2.4

- [ ] 5.2 扩展 `backend/service/facade.go`：注入 nodegroups service；`Snapshot()` 增加 `NodeGroups`
  - 依赖: 5.1
  - 验证: `GET /snapshot` 含 `nodeGroups`

- [ ] 5.3 扩展 `backend/api/router.go`：新增 `/node-groups` 路由（list/create/update/delete）
  - 依赖: 5.2
  - 验证: HTTP CRUD 可用

- [ ] 5.4 更新 `docs/api/openapi.yaml`：补充 node-groups endpoints 与 schemas
  - 依赖: 5.3

- [ ] 5.5 更新 `helloagents/wiki/api.md`：补充 node-groups 接口索引
  - 依赖: 5.4

### 6. Frontend SDK & UI

- [ ] 6.1 更新 `frontend/sdk/src/vea-sdk.js`：新增 NodeGroup API 方法；重建 `frontend/sdk/dist/`
  - 依赖: 5.3
  - 验证: `cd frontend/sdk && npm run build`

- [ ] 6.2 更新 `frontend/theme/_shared/js/app.js`：节点页新增“节点组”视图（列表/创建/编辑/删除）
  - 依赖: 6.1
  - 验证: UI 可创建/编辑/删除节点组

- [ ] 6.3 更新 `frontend/theme/_shared/js/app.js`：图编辑器边目标选择支持 NodeGroup（显示“组:xxx”，保存时写入 groupId）
  - 依赖: 6.1
  - 验证: 保存/重新加载图后仍保留 NodeGroupID

### 7. 测试与验收

- [ ] 7.1 运行后端单测：`go test ./...`
  - 依赖: 1.1-6.3

- [ ] 7.2 后端冒烟（可选）：`go run . --dev --addr :19080`，通过 HTTP/SDK 创建 NodeGroup 并启动代理

### 8. 知识库同步 & 归档

- [ ] 8.1 更新 `helloagents/wiki/modules/backend.md`：补充 NodeGroup 领域/仓储/API/解析器说明
  - 依赖: 7.1

- [ ] 8.2 更新 `helloagents/CHANGELOG.md`（新增：NodeGroup）
  - 依赖: 7.1

- [ ] 8.3 完成后迁移方案包到 `helloagents/archive/2026-02/202602011051_feat-node-groups/`
  - 依赖: 8.1, 8.2

---

## 执行备注

> 执行过程中的重要记录

| 任务 | 状态 | 备注 |
|------|------|------|
