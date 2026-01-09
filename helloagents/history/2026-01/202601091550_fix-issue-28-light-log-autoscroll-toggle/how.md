# 技术设计: 修复 Issue #28 浅色主题日志页自动滚动开关不可见

## 技术方案

### 核心技术
- Electron 主题页（`frontend/theme/light.html`）内联 CSS
- 开关组件：`.chain-toggle` / `.chain-toggle-slider`

### 实现要点
- 在浅色主题 `:root` 中新增 `--border-color`，推荐映射到 `--border-highlight`（更易在白底上识别）。
- 保持 `.chain-toggle` DOM 与 checked 态样式不变，仅通过变量补齐修复关闭态“透明轨道”的问题。
- （可选）如关闭态仍偏淡：为 `.chain-toggle-slider` 增加 `border: 1px solid var(--border-subtle)`，确保在极浅背景下也能看到轮廓。

## 架构决策 ADR
（无）仅样式与变量补齐，不引入新模块/依赖。

## API设计
无

## 数据模型
无

## 安全与性能
- **安全:** 纯前端样式调整，不涉及外部输入与权限变更。
- **性能:** CSS 变量新增对性能无可见影响。

## 测试与部署
- 手工验证（重点 Windows + 浅色主题）：
  - 进入“日志”页，关闭“自动滚动”，确认开关仍可见且可再次开启。
  - 检查同样复用 `.chain-toggle` 的其它区域（如规则编辑对话框）关闭态是否正常显示。

