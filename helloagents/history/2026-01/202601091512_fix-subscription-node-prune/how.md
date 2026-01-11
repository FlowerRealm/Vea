# 技术设计: 修复订阅节点自动清理

## 技术方案

### 核心技术
- Go（内存仓储 + 事件总线）

### 实现要点
- 在 `backend/repository/memory/node_repo.go` 的 `ReplaceNodesForConfig`：
  - 先构建 `nextIDs`（本次快照节点 ID 集合）
  - 在持锁状态下：
    - `nodes == nil`：删除该 config 的全部节点
    - `len(nodes) > 0`：删除该 config 中不在 `nextIDs` 的节点
    - `len(nodes) == 0`：不删除（保持历史节点），用于调用方表达“本次不更新节点集合”
  - 保持原有 upsert 行为：相同 ID 的节点保留 `CreatedAt`、测速指标、用户自定义 `Name/Tags`
  - 继续通过事件总线发布 `NodeCreated/NodeUpdated/NodeDeleted`

## API 设计
- 无 API 变更

## 安全与性能
- **安全:** 删除逻辑受 `configID` 约束，仅作用于 `SourceConfigID == configID` 的订阅节点
- **性能:** 以 config 维度线性扫描节点 map；节点规模通常较小，可接受

## 测试与部署
- **测试:** 更新 `backend/repository/memory/node_repo_test.go`，验证“替换会删除缺失节点”与“显式清空仅影响该 config”
- **验证:** 执行 `go test ./...`
