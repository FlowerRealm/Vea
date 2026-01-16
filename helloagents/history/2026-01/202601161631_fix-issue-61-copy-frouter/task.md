# 任务清单（轻量迭代）

目标: 允许在前端复制（创建副本）FRouter（Issue #61）。

## Tasks

- [√] 前端：FRouter 列表右键菜单新增“复制 FRouter”，基于当前 FRouter 的 `chainProxy` 与 `tags` 创建副本，命名冲突时自动递增序号。
- [√] 后端：修复 `backend/service/config/node_id_reuse.go` 变量重复声明导致编译失败的问题，恢复 `go test ./...` 可运行。
- [√] 知识库：更新 `helloagents/CHANGELOG.md` 与 `helloagents/wiki/modules/frontend.md` 同步本次变更。
- [√] 验证：运行 `go test ./...`。

