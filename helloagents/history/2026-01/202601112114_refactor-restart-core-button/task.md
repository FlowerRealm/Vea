# 轻量迭代任务清单 - refactor-restart-core-button

- [√] 主题页（dark/light）：抽取首页核心状态/按钮区域内联样式到 CSS
- [√] 主题页（dark/light）：重构 `handleCoreRestart`（拆分辅助函数，保持行为不变）
- [√] 运行自检：`node --check frontend/theme/*/js/main.js` 与 `go test ./...`
- [√] 同步知识库：`helloagents/CHANGELOG.md`、`helloagents/history/index.md`、`helloagents/wiki/modules/frontend.md`

## 备注
- `updateCoreUI` 会重写 `#core-state` 的 `className`，样式通过 `#core-state` 的 CSS selector 落到主题 CSS，避免类名丢失导致样式失效。

