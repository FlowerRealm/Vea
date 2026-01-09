# 任务清单: 主题包（目录化 + ZIP 导入/导出）

目录: `helloagents/history/2026-01/202601092026_theme-package/`

---

## 1. 后端：主题管理 API + ZIP
- [√] 1.1 在 `backend/service/` 新增主题服务：扫描 `<userData>/themes` 列表、校验主题入口 `index.html`，验证 why.md#核心场景
- [√] 1.2 在 `backend/api/router.go` 增加 `/themes` 路由（list/import/export/delete），验证 why.md#需求-主题-zip-导入导出
- [√] 1.3 增加 Go 单测覆盖主题 ZIP 导入的安全校验（路径穿越、缺失 index.html），确保 `go test ./...` 通过

## 2. Electron：从 userData 加载主题
- [√] 2.1 在 `frontend/main.js` 增加 `<userData>/themes` 初始化：缺少内置主题则从 app resources 复制，验证 why.md#主题以目录形式加载-启动后加载用户选择的主题
- [√] 2.2 在 `frontend/main.js` 启动创建窗口前读取后端前端设置 `theme`（默认 `dark`），加载 `<userData>/themes/<id>/index.html`
- [√] 2.3 更新 `frontend/electron-builder.yml`：确保内置主题目录 `frontend/theme/**` 被打包进 app resources

## 3. 前端主题：拆分为目录 + css/（修正单文件）
- [√] 3.1 将 `frontend/theme/dark.html` 拆分为 `frontend/theme/dark/index.html` + `frontend/theme/dark/css/*` + `frontend/theme/dark/js/*`，保证功能一致
- [√] 3.2 将 `frontend/theme/light.html` 拆分为 `frontend/theme/light/index.html` + `frontend/theme/light/css/*` + `frontend/theme/light/js/*`，保证功能一致
- [√] 3.3 主题内实现主题管理 UI：从 `/themes` 动态加载主题列表；提供“导入主题(.zip)”与“导出当前主题(.zip)”按钮，验证 why.md#需求-主题-zip-导入导出
- [√] 3.4 更新主题切换逻辑：从“跳转 dark.html/light.html”改为“跳转 ../<themeId>/index.html”

## 4. 安全检查
- [√] 4.1 执行导入/导出安全检查（按 G9：输入验证、路径穿越、大小限制、错误处理），并在 how.md#安全与性能 中落地实现点

## 5. 文档与知识库同步
- [√] 5.1 更新 `helloagents/wiki/modules/frontend.md`（主题目录化与运行时加载）
- [√] 5.2 更新 `helloagents/wiki/api.md`（新增 `/themes` 接口）
- [√] 5.3 更新 `frontend/README.md`（主题目录结构与导入导出说明）
- [√] 5.4 更新 `helloagents/CHANGELOG.md`（记录新增主题包能力）

## 6. 测试
- [√] 6.1 执行 `go test ./...` 并修复与本次变更相关的失败项
