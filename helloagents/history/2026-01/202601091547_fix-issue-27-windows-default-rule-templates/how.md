# 技术设计: 修复 Issue #27 Windows 默认规则模板缺失

## 技术方案

### 核心技术
- Electron / electron-builder（`frontend/electron-builder.yml`）
- 主题页静态资源加载（`frontend/theme/*.html`）

### 实现要点
- 在 `frontend/electron-builder.yml` 的 `files:` 中新增 `chain-editor/**/*`（或精确到 `chain-editor/rule-templates.js`），确保模板脚本进入发布包。
- 保持主题页引用 `../chain-editor/rule-templates.js` 不变，避免引入额外回归。
- （可选）在主题页初始化模板列表时，若 `RULE_TEMPLATES` 未加载，向用户提示“默认模板加载失败（可能为发布包缺失资源）”，避免静默失败。

## 架构决策 ADR
（无）本变更不引入新模块/新依赖，仅修复资源打包遗漏。

## API设计
无

## 数据模型
无

## 安全与性能
- **安全:** 仅打包静态脚本，不涉及外部输入与权限变更。
- **性能:** 模板脚本约 10KB，加载开销可忽略。

## 测试与部署
- **开发环境验证:** `cd frontend && npm run dev`，确认模板选择器正常展示。
- **发布包验证（Windows 优先）:**
  - Windows 上执行 `cd frontend && npm run build:win`（或使用 CI 打包产物）。
  - 检查资源路径存在：`resources/app.asar/chain-editor/rule-templates.js`（以实际 asar 结构为准）。
  - 运行应用，打开模板选择器，确认分类/模板均非空且可应用。

