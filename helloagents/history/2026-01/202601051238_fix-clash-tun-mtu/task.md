# 任务清单: 修复 Linux 下 mihomo(Clash) TUN 默认 MTU 导致断网

目录: `helloagents/plan/202601051238_fix-clash-tun-mtu/`

## 1. 后端修复
- [√] 1.1 在 `backend/service/proxy/service.go` 中按实际选中引擎应用 TUN 默认值（Linux + mihomo: 避免 MTU=9000 的“看起来全网断开”）
- [√] 1.2 增加/补齐 `backend/service/proxy/service_test.go` 覆盖：TUN + Clash 时默认 MTU 会被修正并持久化

## 2. 知识库同步
- [√] 2.1 更新 `helloagents/wiki/modules/backend.md`：记录本次修复点与关联历史
- [√] 2.2 更新 `helloagents/CHANGELOG.md`：补充 Unreleased 修复项

## 3. 验证
- [√] 3.1 运行 `go test ./backend/service/proxy -run TestService_Start_`（避免受全仓库既有失败用例影响）
