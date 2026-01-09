# 任务清单: 修复 Clash 安装权限与内核切换代理快失败

目录: `helloagents/plan/202601071306_fix-chmod-engine-switch-proxy-failfast/`

---

## 1. backend
- [√] 1.1 在 `backend/service/component/service.go` 中补齐 `normalizeClashInstall` 对 `os.Chmod` 的错误处理，避免安装后二进制缺少执行权限

## 2. frontend
- [√] 2.1 在 `frontend/theme/dark.html` 中，切换内核引擎时关闭系统代理失败改为快速失败（不继续重启内核）
- [√] 2.2 在 `frontend/theme/light.html` 中，切换内核引擎时关闭系统代理失败改为快速失败（不继续重启内核）

## 3. 测试
- [√] 3.1 运行 `go test -short ./...`

## 4. 文档更新
- [√] 4.1 更新 `helloagents/CHANGELOG.md`
- [√] 4.2 更新 `helloagents/wiki/modules/backend.md`
- [√] 4.3 更新 `helloagents/wiki/modules/frontend.md`
- [√] 4.4 更新 `helloagents/history/index.md`
