# 任务清单: 修复后端端口冲突导致的“闪退”

目录: `helloagents/history/2026-01/202601131921_fix-backend-port-conflict/`

---

## 1. 前端：避免重复拉起后端
- [√] 1.1 在 `frontend/main.js` 启用单实例锁，避免多实例重复启动后端导致 `:19080` 冲突
- [√] 1.2 在 `frontend/main.js` 启动后端前请求 `GET /health`，同 `userData` 则复用已运行服务；不同 `userData` 则提示端口占用并退出
- [√] 1.3 后端意外退出时弹窗提示 `app.log` 路径，提升可观测性

## 2. 后端：健康检查增强
- [√] 2.1 在 `backend/api/router.go` 的 `GET /health` 响应补充 `pid` 与 `userDataRoot`

## 3. 文档同步
- [√] 3.1 更新 `docs/api/openapi.yaml` 的 `/health` 响应字段
- [√] 3.2 更新 `helloagents/wiki/modules/frontend.md`
- [√] 3.3 更新 `helloagents/wiki/modules/backend.md`
- [√] 3.4 更新 `helloagents/CHANGELOG.md`
- [√] 3.5 更新 `helloagents/history/index.md`

## 4. 测试
- [√] 4.1 执行 `go test ./...`
