# 技术设计: 修复 Windows 下主题 UI 颜色对比度异常（Issue #39）

## 技术方案

### 核心技术
- 前端主题：`frontend/theme/*`（原生 HTML/CSS/JS）
- 运行环境：Electron（Chromium）

### 实现要点

1. **为主题声明 `color-scheme`**
   - 在 `frontend/theme/dark/css/main.css` 的 `:root` 增加 `color-scheme: dark;`
   - 在 `frontend/theme/light/css/main.css` 的 `:root` 增加 `color-scheme: light;`
   - 目标：让 Chromium 的原生表单控件（`<select>` 下拉/列表框、滚动条等）使用与主题一致的默认配色，避免 Windows 下弹出层仍使用浅色系统样式。

2. **补齐浅色主题缺失的 CSS 变量**
   - 现状：浅色主题存在多处 `var(--bg-secondary)` / `var(--border-color)` / `var(--card-bg)` / `var(--primary-color)` / `var(--shadow-md)` 等引用，但未在 `:root` 定义，导致样式回退到 UA 默认值，出现不一致甚至对比度问题。
   - 策略：在浅色主题 `:root` 定义这些变量，并与现有变量对齐（例如 `--border-color` 复用 `--border-subtle`；`--card-bg` 复用 `--bg-card`）。

3. **可选增强：增加 `meta name="color-scheme"`**
   - 若仅 CSS `color-scheme` 在部分环境不生效，可在 `frontend/theme/*/index.html` 的 `<head>` 增加：
     - dark：`<meta name="color-scheme" content="dark">`
     - light：`<meta name="color-scheme" content="light">`
   - 该增强不影响 API 与数据，变更范围可控。

## 安全与性能
- **安全:** 仅样式/主题变量调整，不引入新依赖、不触及敏感数据。
- **性能:** 仅新增少量 CSS 变量与声明，忽略不计。

## 测试与部署

- **手工回归（Windows 优先）**
  1. 打开“路由规则”面板，展开去向选择下拉，确认列表背景/文字对比正常。
  2. 打开模板/节点等列表选择弹窗（如有），确认列表可读、滚动正常。
  3. 切换 dark/light 两套主题分别验证上述场景。

- **自动化（基础）**
  - `go test ./...`（确保本次变更不影响后端编译与单测）

