# 任务清单: 订阅 fallback 清理同步错误

目录: `helloagents/history/2026-01/202601110913_fix-subscription-fallback-syncstatus/`

---

## 1. 后端: 订阅创建 fallback 状态一致性
- [√] 1.1 调整 `backend/service/config/service.go`：创建订阅后台首次 Sync 失败且 fallback payload 解析成功时，清空 `LastSyncError` 并更新 payload/checksum
- [√] 1.2 更新 `backend/service/config/service_test.go`：补充“fallback 解析成功后清理同步错误”单测

## 2. 文档更新
- [√] 2.1 更新 `helloagents/wiki/modules/backend.md`：补充订阅创建 fallback 行为说明

## 3. 测试
- [√] 3.1 执行 `go test ./...`

