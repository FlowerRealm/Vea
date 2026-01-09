# 任务清单: 代码审查报告问题修复（后续跟进）

目录: `helloagents/history/2026-01/202601081145_fix-review-report/`

---

## 1. 后端修复
- [√] 1.1 在 `backend/service/config/clash_subscription.go` 中补齐重复 proxy name 告警，并提取 `assignPriorities` 统一优先级归一化逻辑
- [√] 1.2 新增 `backend/service/config/clash_subscription_test.go`，覆盖 `parseClashProxyToNode` / `compactClashSelectionEdges` 的单元测试
- [√] 1.3 在 `backend/service/shared/tun_linux.go` 中改进 iptables 清理脚本：仅对“规则不存在”静默，其他错误输出 warn
- [√] 1.4 在 `backend/service/config/service.go` 中修正空订阅错误提示文案，避免在无现有节点时产生误导
- [√] 1.5 在 `backend/service/proxy/service.go` / `backend/service/facade.go` / `backend/api/proxy.go` / `main.go` 中加入“用户停止”状态并让 keepalive 尊重该状态
- [√] 1.6 在 `backend/api/logs_test.go` 补充注释，说明 nil Facade 的测试意图

## 2. 前端修复
- [√] 2.1 在 `frontend/theme/dark.html` 与 `frontend/theme/light.html` 中去除多余 `String()` 转换

## 3. 测试
- [√] 3.1 执行 `go test -short ./...`

## 4. 文档与归档
- [√] 4.1 更新 `helloagents/CHANGELOG.md`、`helloagents/wiki/modules/backend.md`、`helloagents/wiki/modules/frontend.md`、`helloagents/wiki/api.md`
- [√] 4.2 迁移方案包至 `helloagents/history/2026-01/202601081145_fix-review-report/` 并更新 `helloagents/history/index.md`
