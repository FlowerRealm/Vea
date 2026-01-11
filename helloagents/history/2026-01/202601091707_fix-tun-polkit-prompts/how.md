# 技术设计: 减少 Linux TUN 模式提权弹窗次数

## 技术方案

### 核心思路

- 现有 `resolvectl-helper` 已实现“pkexec 启动一次 + Unix socket IPC + 白名单执行”的模式。
- 将其扩展为通用的 root helper：除 `resolvectl` 白名单外，新增 TUN 相关白名单 op。
- 后端侧优先通过 IPC 调用 helper 执行特权动作，避免同生命周期内多次 `pkexec`。

### 协议设计

- 请求/响应结构统一为 JSON（ExitCode/Stdout/Stderr/Error）。
- 新增字段 `op`：
  - `op` 为空 → 兼容旧逻辑，默认视为 `resolvectl`。
  - `op=tun-cleanup` → 清理冲突规则。
  - `op=tun-setup` + `binaryPath` → 执行 `SetupTUNForBinary(binaryPath)`。

### 生命周期与权限边界

- helper 通过 `pkexec vea resolvectl-helper --socket ... --uid ... --parent-pid ...` 启动。
- socket 文件:
  - `chown` 到调用用户 uid
  - `chmod 0600`
  - 限制同用户进程才能访问
- helper 监控 `parent-pid`，父进程退出后自动关闭 socket 并退出。

## 实现要点

- `backend/service/shared/root_helper_linux.go`
  - 实现 `EnsureRootHelper`（带 lock 文件防并发启动）与 `CallRootHelper`（Unix socket RPC）。
- `resolvectl_linux.go`
  - 扩展 helper server：根据 `op` 分发到 `resolvectl`/`tun-cleanup`/`tun-setup`。
  - resolvectl shim 侧复用 `shared.EnsureRootHelper/CallRootHelper`，减少重复实现。
- `backend/service/shared/tun_linux.go`
  - 将 `CleanConflictingIPTablesRules` 与 `EnsureTUNCapabilitiesForBinary` 的特权部分改为调用 helper（同生命周期只需授权一次）。

## 安全检查

- 不保存/缓存 sudo/管理员密码（仅触发系统 polkit 授权）。
- helper 不提供任意命令执行接口，仅支持白名单 op。
- `binaryPath` 做基本校验（非空、存在），避免空路径/错误路径导致的意外行为。

## 测试

- 运行 `go test ./...` 确认编译与单测通过。
- 手工验证（Linux 桌面环境）：
  - 首次启用 TUN：仅出现一次 polkit 授权弹窗（后续清理/配置走同一 helper）。
  - 重启内核/切换配置：同生命周期内不重复弹窗（除非 helper 已退出）。

