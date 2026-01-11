# 任务清单: 修复 Issue #32 - TUN 状态显示错误

目录: `helloagents/plan/202601091657_fix-issue-32-tun-status/`

---

## 1. Frontend（TUN 状态展示）
- [√] 1.1 在 `frontend/theme/dark.html` 中将 “TUN 模式” 卡片主状态改为运行态显示，验证 why.md#需求-tun-status-display 与 why.md#场景-tun-status-display-toggle
- [√] 1.2 在 `frontend/theme/light.html` 同步上述改动，验证 why.md#需求-tun-status-display 与 why.md#场景-tun-status-display-toggle
- [√] 1.3 在 `frontend/theme/dark.html`/`frontend/theme/light.html` 的 TUN 详情弹窗中补充能力检查信息展示（/tun/check），验证 why.md#需求-tun-capability-info 与 why.md#场景-tun-capability-info-dialog

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 3. 文档更新
- [√] 3.1 如 UI 文案语义变更较大，更新 `docs/CHANGES.md` 或 `helloagents/wiki/modules/frontend.md` 记录变更

## 4. 验证
- [?] 4.1 本地手动验证：启用/禁用 TUN 后状态一致（Windows 优先），验证 why.md#场景-tun-status-display-toggle
  > 备注: 需要在 Windows 环境启动 Electron 应用验证；当前工作区无法执行 GUI 验证。
- [?] 4.2 回归：普通 mixed 模式启动/停止不受影响
  > 备注: 已运行 `go test ./...` 通过；仍建议在应用内做一次 mixed 启停回归。
