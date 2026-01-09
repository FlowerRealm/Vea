# 轻量迭代任务清单 - fix-subscription-pull-refresh-duplicate

- [√] 订阅管理：去除“拉取/刷新”重复操作（配置行仅保留“拉取节点”）
- [X] 运行测试：`go test -short ./...`（失败：`TestSingBoxAdapter_WaitForReady_PortFreeTimesOut` 预期 error 但得到 nil）
- [√] 同步知识库：`helloagents/CHANGELOG.md`、`helloagents/history/index.md`、`helloagents/wiki/modules/frontend.md`
