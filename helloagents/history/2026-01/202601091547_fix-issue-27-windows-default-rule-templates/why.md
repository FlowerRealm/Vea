# 变更提案: 修复 Issue #27 Windows 默认规则模板缺失

## 需求背景
Issue #27 反馈：在 Windows 10 x64 的 Vea 2.2.0 Release 中，“默认规则模板/可用模板”列表为空，无法选择默认规则模板。该能力在 Issue #13 中宣称已完成，但发布包中仍不可用。

初步定位：前端主题页通过 `<script src="../chain-editor/rule-templates.js">` 加载模板数据，但 Electron 打包配置 `frontend/electron-builder.yml` 的 `files` 列表未包含 `chain-editor/` 目录，导致发布包缺失该脚本，从而模板列表为空。

## 变更内容
1. 调整 Electron 打包配置，确保 `frontend/chain-editor/rule-templates.js` 被包含在发布产物中。
2. （可选）在 UI 初始化模板时增加缺失提示，避免静默失败与误解为“无默认模板”。

## 影响范围
- **模块:** frontend（Electron 打包 + 主题页面）
- **文件:**
  - `frontend/electron-builder.yml`（必改）
  - `frontend/theme/dark.html` / `frontend/theme/light.html`（可选：缺失提示）
  - `frontend/chain-editor/rule-templates.js`（不改，仅确保被打包）
- **API:** 无
- **数据:** 无

## 核心场景

### 需求: Windows 发布版可用默认规则模板
**模块:** frontend

在 Windows 发布版中，用户打开 FRouter 规则编辑器/模板选择器时，应能看到默认模板分类与模板列表，并可一键应用为规则。

#### 场景: 打开模板选择器
条件：运行 Windows 发布包，进入 FRouter 规则编辑页面并打开模板选择器。
- 预期结果：分类下拉可选；模板下拉存在默认模板条目；选择模板后能生成对应规则（或填充表单）。

## 风险评估
- **风险:** 打包体积微增；打包配置变更可能影响其它资源的包含/排除规则。
- **缓解:** 仅新增 `chain-editor/**/*` 进入 `files` 列表，不改现有排除规则；打包后检查 asar 内资源路径与主题脚本引用一致。

