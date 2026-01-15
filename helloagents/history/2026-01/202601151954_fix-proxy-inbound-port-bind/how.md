# 技术设计: 修复 Windows 下 sing-box mixed 入站端口占用导致启动失败

## 技术方案

### 核心技术
- Go（后端进程管理 + 端口检查）
- 现有启动流程：`backend/service/proxy/service.go` → `startProcess()` → `adapter.WaitForReady()`

### 实现要点
- **默认端口统一：**
  - 将后端默认 `inboundPort` 统一调整为 `31346`（与前端 schema 的 `proxy.port` 默认一致）
  - 涉及：内存默认值、配置补默认值、facade 中用于 fallback 的默认端口常量

- **端口占用 fail-fast：**
  - 在停止旧内核（若有）之后、启动新内核之前，检查目标监听地址+端口是否可用
  - 若被占用：返回 `repository.ErrInvalidData` 包装的错误，HTTP 层会返回 400，并在 UI 直接展示可操作提示
  - 不做自动换端口（按用户选择 1A）

- **readiness probe 可选增强（看实现复杂度决定是否纳入本次变更）：**
  - Windows 当前用 Dial 探测端口是否可连通，若端口已被其他程序占用可能产生“假就绪”
  - 如仅做 fail-fast 端口检查，可降低该误判概率；若仍存在边界 race，可在 `WaitForReady` 中增加“进程已退出”检测/二次确认

## 安全与性能
- **安全:** 不涉及凭证/权限变更；仅增加本地端口检查与更清晰错误提示
- **性能:** 端口检查为启动时短暂操作（毫秒级），不影响运行期性能

## 测试与部署
- **测试:**
  - `go test ./...`
  - 新增/补齐单测：验证默认端口为 `31346`；验证端口占用时 `Start()` 返回 `ErrInvalidData` 且错误信息包含端口
- **部署:**
  - 无额外部署步骤；仅升级应用版本

