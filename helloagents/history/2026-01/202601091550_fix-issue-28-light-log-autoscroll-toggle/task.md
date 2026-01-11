# 任务清单: 修复 Issue #28 浅色主题日志页自动滚动开关不可见

目录: `helloagents/plan/202601091550_fix-issue-28-light-log-autoscroll-toggle/`

---

## 1. UI 样式修复
- [√] 1.1 在 `frontend/theme/light.html` 的 `:root` 中补充 `--border-color` 定义（映射到 `--border-highlight`），验证 why.md#需求-浅色主题日志页自动滚动开关关闭态可见-场景-在日志页切换自动滚动开关
- [-] 1.2（可选）如关闭态仍偏淡，为 `.chain-toggle-slider` 增加 `border: 1px solid var(--border-subtle)` 或调整关闭态底色，验证 why.md#需求-浅色主题日志页自动滚动开关关闭态可见-场景-在日志页切换自动滚动开关
  > 备注: 当前已通过补齐 `--border-color` 修复“轨道透明导致不可见”的根因，先不额外叠加边框样式，避免引入不必要的视觉差异。

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9：输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 3. 文档更新
- [√] 3.1 更新 `helloagents/wiki/modules/frontend.md` 记录主题变量/开关样式修复点
- [√] 3.2 更新 `helloagents/CHANGELOG.md` 增加 Issue #28 修复记录

## 4. 测试
- [?] 4.1 手工验证：Windows 浅色主题日志页关闭/开启“自动滚动”，控件始终可见；同时检查复用 `.chain-toggle` 的其它位置无回归
  > 备注: 当前环境未运行 Electron UI；请在 Windows 浅色主题下确认观感与可用性。
