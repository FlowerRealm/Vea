# 任务清单: fix-ip-geo-context

目录: `helloagents/plan/202601092132_fix-ip-geo-context/`

> 说明: 轻量迭代（仅 task.md），用于跟进代码审查建议：IPGeo 请求贯穿 context、清理冗余 bgCtx nil check。

---

## 1. 后端：bgCtx 冗余逻辑清理
- [√] 1.1 移除 `backend/service/config/service.go` 中 `s.bgCtx` 的冗余 nil check（`NewService` 已保证非 nil）

## 2. 后端：IPGeo context 贯穿
- [√] 2.1 调整 `backend/service/facade.go` 的 `GetIPGeo` 签名为 `GetIPGeo(ctx context.Context)`，并贯穿传递 ctx
- [√] 2.2 调整 `backend/service/shared/tun.go` 的 `GetIPGeo`/`GetIPGeoWithHTTPClient` 支持 ctx，并使用 `http.NewRequestWithContext`
- [√] 2.3 更新 API handler：`backend/api/router.go` 通过 `c.Request.Context()` 调用 `GetIPGeo`

## 3. 测试
- [√] 3.1 更新 `backend/service/shared/tun_test.go` 适配 helper 新签名
- [√] 3.2 运行 `go test ./...`

## 4. 文档更新
- [√] 4.1 更新 `helloagents/wiki/modules/backend.md` 变更历史索引
- [√] 4.2 更新 `helloagents/CHANGELOG.md`
- [√] 4.3 更新 `helloagents/history/index.md`

