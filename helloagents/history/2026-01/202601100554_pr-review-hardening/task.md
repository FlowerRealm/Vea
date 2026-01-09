# 轻量迭代任务清单: pr-review-hardening

目标: 跟进代码审查建议：加固 Linux root helper 路径校验，减少前端全局污染，并让订阅导入后的刷新更可靠。

## 任务

- [√] 后端：校验 socketPath 结构并安全推导 artifactsRoot，拒绝 "/" 根路径
- [√] 前端：移除 window.veaShowStatus 全局暴露，改为模块内共享 showStatus
- [√] 前端：订阅导入后改为轮询 lastSyncedAt，替代固定 setTimeout 刷新
- [√] 更新知识库与变更索引（backend/frontend 模块文档、Changelog、history 索引）
- [√] 运行基础测试（`go test ./...`）
