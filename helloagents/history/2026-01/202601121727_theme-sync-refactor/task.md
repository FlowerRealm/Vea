# 任务清单: 主题同步重构与样式一致性

目录: `helloagents/plan/202601121727_theme-sync-refactor/`

---

## 1. 主题同步代码重构
- [√] 1.1 将 `frontend/main.js` 中的 `ensureBundledThemes` 抽离为独立模块（如 `frontend/theme_manager.js`），保持行为一致
- [√] 1.2 调整 `frontend/main.js` 引用方式，确保启动流程仍会执行主题同步

## 2. 前端主题 HTML 格式一致性
- [√] 2.1 修复 `frontend/theme/dark/index.html` 中 Home 面板相关新增区域的缩进（与 `frontend/theme/light/index.html` 对齐，统一空格缩进）
- [√] 2.2 修复 `frontend/theme/dark/index.html` 中 FRouter 面板头部区域的缩进（与 `frontend/theme/light/index.html` 对齐，统一空格缩进）

## 3. 安全检查
- [√] 3.1 执行安全检查（文件复制/删除逻辑无路径注入风险；无敏感信息硬编码）

## 4. 文档更新
- [√] 4.1 更新 `helloagents/wiki/modules/frontend.md`（记录主题同步模块位置与职责）

## 5. 测试
- [√] 5.1 执行 `node --check frontend/main.js` 与 `node --check frontend/theme_manager.js`
