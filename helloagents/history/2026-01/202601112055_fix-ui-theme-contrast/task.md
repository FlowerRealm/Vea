# 任务清单: 修复 Windows 下主题 UI 颜色对比度异常（Issue #39）

目录: `helloagents/plan/202601112055_fix-ui-theme-contrast/`

---

## 1. 主题样式（dark/light）
- [√] 1.1 在 `frontend/theme/dark/css/main.css` 中为 `:root` 增加 `color-scheme: dark`，并补齐必要的变量（如 `--primary`），验证 why.md#需求-下拉列表可读且可用-场景-路由规则面板的去向选择下拉
- [√] 1.2 在 `frontend/theme/light/css/main.css` 中为 `:root` 增加 `color-scheme: light`，并补齐缺失变量（`--bg-secondary`、`--border-color`、`--card-bg`、`--primary/--primary-color`、`--shadow-md` 等），验证 why.md#需求-下拉列表可读且可用-场景-模板节点等列表选择控件
- [√] 1.3 （可选）在 `frontend/theme/dark/index.html` 与 `frontend/theme/light/index.html` 增加 `meta name=\"color-scheme\"`，验证 why.md#需求-主题一致性浅色深色-场景-切换到浅色主题后控件仍可读

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9：确认无敏感信息写入、无危险命令/依赖变更）

## 3. 文档更新
- [√] 3.1 更新 `helloagents/wiki/modules/frontend.md`，记录主题对原生表单控件的 `color-scheme` 约束与原因（Windows 兼容）
- [√] 3.2 更新 `helloagents/CHANGELOG.md`，记录修复 Issue #39

## 4. 测试
- [√] 4.1 执行 `go test ./...` 确保无回归
- [?] 4.2 手工回归：Windows 下验证路由规则去向下拉/模板选择列表的可读性（见 why.md#核心场景）
  > 备注: 需要在 Windows 10 22H2 的 Electron GUI 环境下验证；当前环境无法直接复现截图所示的原生控件配色问题。
