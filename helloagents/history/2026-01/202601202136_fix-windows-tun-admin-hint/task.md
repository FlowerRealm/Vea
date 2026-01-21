# 任务清单: 修复 Windows TUN 状态误报管理员权限

目录: `helloagents/plan/202601202136_fix-windows-tun-admin-hint/`

---

## 1. 后端（TUN 能力检查语义）
- [√] 1.1 在 `backend/service/shared/tun_windows.go` 调整 `CheckTUNCapabilities/GetTUNCapabilityStatus`：Windows 下 `configured` 表达“无需额外配置”而非管理员判定，验证 why.md#需求-tun-status-windows-场景-tun-status-windows-no-false-admin-hint
- [√] 1.2 在 `backend/api/proxy.go` 更新 `/tun/check` 的 Windows 文案（setupCommand/description），验证 why.md#需求-tun-status-windows-场景-tun-status-windows-no-false-admin-hint
- [-] 1.3（可选）在 `backend/service/proxy/service.go` 针对 Windows 权限类启动失败追加“可尝试管理员运行”的提示，验证 why.md#需求-tun-status-windows-场景-tun-status-windows-permission-fail-hint
  > 备注: 现有 `wrapTUNNotReady()` 已包含 Windows 下的“可能需要管理员/Wintun”提示，本次不重复添加

## 2. 前端（显示一致性兜底 - 可选）
- [-] 2.1 如仍存在“已运行但显示未配置”的边界情况，在 `frontend/theme/_shared/js/app.js` 让设置页配置状态在 `tunRunning=true` 时优先显示为已配置，验证 why.md#需求-tun-status-windows-场景-tun-status-windows-no-false-admin-hint
  > 备注: `/tun/check` 语义修复后该边界不再出现，先不增加前端兜底逻辑（避免重复状态来源）

## 3. 安全检查
- [√] 3.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 4. 文档更新
- [√] 4.1 更新 `docs/api/openapi.yaml`：补充 `/tun/check` 在 Windows 下的语义说明
- [√] 4.2 更新 `helloagents/wiki/modules/backend.md` 记录 Windows `/tun/check` 语义调整
- [√] 4.3 更新 `helloagents/CHANGELOG.md` 记录本次修复
- [√] 4.4 更新 `docs/SING_BOX_INTEGRATION.md`：同步 Windows `/tun/check` 示例与说明

## 5. 测试
- [√] 5.1 运行 `gofmt` 格式化变更文件
- [√] 5.2 运行 `go test ./...`
- [?] 5.3 手动验证（Windows）：设置页不再误报“需要管理员/未配置”，验证 why.md#需求-tun-status-windows-场景-tun-status-windows-no-false-admin-hint
  > 备注: 需要在 Windows 环境下启动应用验证
