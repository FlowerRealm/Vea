# 轻量迭代任务清单 - pr-review-theme-shared-async

- [√] 主题页（dark/light）：抽取重复应用逻辑到共享模块 `frontend/theme/_shared/js/app.js`
- [√] 主题页（dark/light）：入口 `js/main.js` 改为动态加载共享模块（兼容 userData 注入与 app resources fallback）
- [√] Electron 主进程：主题同步改为异步实现（`fs.promises`），避免启动期阻塞
- [√] 主题同步：启动时将共享模块注入到 `userData/themes/<builtin>/_shared/`，确保导出/导入主题自包含
- [√] 运行自检：`node --check frontend/main.js` 与 `go test ./...`
- [√] 同步知识库：`helloagents/CHANGELOG.md`、`helloagents/history/index.md`、`helloagents/wiki/modules/frontend.md`

## 备注
- 主题目录 hash 判定会忽略注入目录 `_shared`，避免把运行期注入误判为“用户修改”从而阻断后续内置主题更新。

