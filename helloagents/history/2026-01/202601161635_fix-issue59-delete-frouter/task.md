# 任务清单（轻量迭代）

> 目标: 修复 Issue #59「允许删除 FRouter」。

- [√] 前端：在 FRouter 列表右键菜单新增“删除 FRouter”，调用 `DELETE /frouters/:id` 并刷新列表（删除最后一个时给出提示）。
- [√] 后端：删除 FRouter 后兜底修复 `ProxyConfig.frouterId`（指向不存在时自动切换可用 FRouter；删到空集合自动创建默认 FRouter）。
- [√] 测试：补齐 `Facade.DeleteFRouter` 单测并确保 `go test ./...` 通过。
- [√] 知识库：更新 `helloagents/wiki/modules/frontend.md`、`helloagents/wiki/modules/backend.md` 与 `helloagents/CHANGELOG.md`，并将方案包迁移至 `helloagents/history/`。
