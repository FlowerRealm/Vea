# 技术设计: 代码审查报告问题修复（后续跟进）

## 技术方案

### 核心技术
- Go 单元测试（`go test`）
- 现有后端服务结构（Facade / proxy.Service / config service）

### 实现要点
- Clash 解析：
  - 在 `clash_subscription.go` 增加 `assignPriorities(edges)` 统一优先级归一化入口
  - 在 proxy 名称重复时向 `Warnings` 追加告警，避免静默覆盖
  - 新增 `clash_subscription_test.go`，对解析与规则压缩做单元测试覆盖
- keepalive / Stop 行为：
  - 在 `proxy.Service` 增加 `userStopped` 状态与 `StopUser` 方法
  - `POST /proxy/stop` 调用 `StopProxyUser`，标记用户停止
  - keepalive 轮询时若 `userStopped=true` 则跳过自动拉起
- TUN 清理：
  - shell 脚本不再全量 `2>/dev/null`，改为识别“规则不存在”类错误并静默
  - 对其他错误输出 `[TUN-Cleanup][WARN] ...`，便于诊断
- 前端：
  - `syncErr` 赋值改为 `cfg.lastSyncError || ""`，去除多余 `String()` 转换

## API设计
- `GET /proxy/status`：
  - 当用户通过 `POST /proxy/stop` 停止代理后，可选返回 `userStopped=true` 与 `userStoppedAt`
  - 仅新增字段，不改变既有字段语义

## 安全与性能
- 不引入新的外部依赖；日志输出受控，仅在异常场景输出 warn

## 测试与部署
- 测试：执行 `go test -short ./...`
- 部署：无额外步骤

