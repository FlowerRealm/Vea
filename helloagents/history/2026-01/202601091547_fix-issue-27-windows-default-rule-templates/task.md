# 任务清单: 修复 Issue #27 Windows 默认规则模板缺失

目录: `helloagents/plan/202601091547_fix-issue-27-windows-default-rule-templates/`

---

## 1. 前端打包修复
- [√] 1.1 在 `frontend/electron-builder.yml` 中将 `chain-editor/**/*` 加入 `files` 列表，验证 why.md#需求-windows-发布版可用默认规则模板-场景-打开模板选择器
- [√] 1.2 构建发布包并确认 `chain-editor/rule-templates.js` 出现在产物中（asar 或等效路径），验证 why.md#需求-windows-发布版可用默认规则模板-场景-打开模板选择器

## 2. UI 兜底提示（可选）
- [√] 2.1 在 `frontend/theme/light.html` 与 `frontend/theme/dark.html` 的模板初始化处加入缺失提示（当模板脚本未加载时提示用户），验证 why.md#需求-windows-发布版可用默认规则模板-场景-打开模板选择器

## 3. 安全检查
- [√] 3.1 执行安全检查（按G9：输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 4. 文档更新
- [√] 4.1 更新 `helloagents/wiki/modules/frontend.md` 记录打包资源包含项变更
- [√] 4.2 更新 `helloagents/CHANGELOG.md` 增加 Issue #27 修复记录

## 5. 测试
- [-] 5.1 运行 `cd frontend && npm run build:win`（建议在 Windows 或 CI 环境），并手工验证模板选择器展示与应用行为
  > 备注: 当前环境仅完成 Linux `electron-builder --dir` 打包验证，仍建议在 Windows/CI 复验发布包与模板选择器行为。
