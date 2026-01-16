# 任务清单: 修复 Issue #54（TUN 端口设置错误）

目录: `helloagents/history/2026-01/202601161625_fix_issue54_tun_port/`

---

## 1. 前端联动修复
- [√] 1.1 在 `frontend/theme/_shared/js/app.js` 修复 `proxy.port` 与后端 `ProxyConfig.inboundPort` 联动：TUN 模式下也应更新端口，并在内核运行时触发重启使其生效（Issue #54）

## 2. 文档更新
- [√] 2.1 更新 `helloagents/CHANGELOG.md`，补充 Issue #54 修复说明
- [√] 2.2 更新 `helloagents/wiki/modules/frontend.md`，补充本次变更历史条目

## 3. 测试
- [√] 3.1 执行 `go test ./...`，确保后端单测不受影响
