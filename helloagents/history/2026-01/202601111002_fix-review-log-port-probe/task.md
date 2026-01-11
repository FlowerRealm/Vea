# 任务清单: 修复评审建议（日志/默认端口/端口探测去重）

目录: `helloagents/history/2026-01/202601111002_fix-review-log-port-probe/`

---

## 1. 后端（backend）
- [√] 1.1 调整 `backend/service/config/service.go` 的 ConfigCreate 后台首次同步日志：fallback payload 解析成功时不再误报“初始同步失败但最终仍失败”的语义。
- [√] 1.2 调整 `backend/service/facade.go` 的系统代理默认端口：抽取 `defaultProxyPort` 常量，避免 magic number。
- [√] 1.3 调整 `backend/service/proxy/service.go` 的 TUN readiness probe：抽取私有辅助函数，去除重复逻辑。

## 2. 安全检查
- [√] 2.1 自检：本次仅为日志与小型重构；不涉及权限变更、外部请求、敏感信息写入与破坏性操作。

## 3. 测试
- [√] 3.1 运行 `gofmt` 格式化变更文件。
- [√] 3.2 运行 `go test ./...`。
