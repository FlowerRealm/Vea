# 任务清单: 修复订阅节点自动清理

目录: `helloagents/history/2026-01/202601091512_fix-subscription-node-prune/`

---

## 1. 后端: 订阅节点替换语义
- [√] 1.1 调整 `backend/repository/memory/node_repo.go` 的 `ReplaceNodesForConfig`：当 `len(nodes) > 0` 时按快照删除旧节点，避免节点累积
- [√] 1.2 更新 `backend/repository/memory/node_repo_test.go`：验证替换会删除缺失节点，且不影响其他 config

## 2. 安全检查
- [√] 2.1 确认删除逻辑仅作用于 `SourceConfigID == configID` 的节点，且不会在解析失败/空 payload 场景误触发

## 3. 测试
- [√] 3.1 执行 `go test ./...`

## 4. 文档更新
- [√] 4.1 更新 `helloagents/CHANGELOG.md`、`helloagents/wiki/modules/backend.md`、`helloagents/history/index.md`
