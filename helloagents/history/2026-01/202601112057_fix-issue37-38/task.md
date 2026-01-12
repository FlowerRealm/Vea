# 任务清单: 修复 Issue #37/#38（当前 IP 异常 + FRouter 配置界面无法打开）

目录: `helloagents/plan/202601112057_fix-issue37-38/`

---

## 1. IP Geo（Issue #37）
- [√] 1.1 在 `backend/service/facade.go` 的 `GetIPGeo` 增加 `busy` 场景的轻量重试/明确报错，避免误回落直连探测，验证 why.md#requirement-ip-geo-reflects-proxied-egress-when-proxy-is-enabled-scenario-home-panel-ip-geo-does-not-fall-back-to-direct-on-busy
- [√] 1.2 在 `frontend/theme/light/js/main.js` 调整 `loadHomePanel` 的执行顺序，避免 `loadIPGeo()` 与 `refreshCoreStatus()` 并发触发，验证 why.md#requirement-ip-geo-reflects-proxied-egress-when-proxy-is-enabled-scenario-home-panel-ip-geo-does-not-fall-back-to-direct-on-busy
- [√] 1.3 在 `frontend/theme/dark/js/main.js` 同步上述 `loadHomePanel` 调整，验证 why.md#requirement-ip-geo-reflects-proxied-egress-when-proxy-is-enabled-scenario-home-panel-ip-geo-does-not-fall-back-to-direct-on-busy

## 2. 链路编辑面板（Issue #38）
- [√] 2.1 在 `frontend/theme/light/js/main.js` 将 `initChainEditor` 调整为幂等的 `ensure` 语义（非首次进入也会刷新图数据），并修复 `openChainEditorPanel` 的重复打开问题，验证 why.md#requirement-chain-editor-can-be-opened-multiple-times-per-session-scenario-open-chain-editor-twice-without-restart
- [√] 2.2 在 `frontend/theme/dark/js/main.js` 同步上述修复，验证 why.md#requirement-chain-editor-can-be-opened-multiple-times-per-session-scenario-open-chain-editor-twice-without-restart

## 3. 安全检查
- [√] 3.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 4. 测试
- [√] 4.1 执行 `go test ./...`（后端回归）
- [?] 4.2 手测 UI：链路编辑可重复打开；切换/重启内核时“当前 IP”不误显示直连 IP
  > 备注: 当前环境无法运行 Electron UI（需在 Windows 发布版或本地 GUI 环境验证）
