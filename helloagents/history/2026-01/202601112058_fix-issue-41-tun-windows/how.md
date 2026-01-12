# 技术设计: 修复 Issue #41 - Windows TUN 启动失败

## 技术方案

### 核心技术

- Go（`net.Interfaces` / `net.InterfaceAddrs`）
- 现有内核启动流程（`backend/service/proxy/service.go`）
- sing-box TUN inbound 配置生成（`backend/service/adapters/singbox.go`）

### 实现要点

- 在 `startProcess()` 的 TUN 路径中调整就绪判定：
  - **Linux**：继续使用 `waitForTUNReadyWithIndex(interfaceName, prevIndex, timeout)`（保留按名称重建的语义）。
  - **Windows/macOS**：
    - 使用启动前快照 `existingIfaces`（已存在）识别“新出现的网卡”。
    - 循环探测期间监听 `handle.Done`；若内核进程提前退出，立即失败并给出“进程已退出”的明确错误提示（并提示查看 kernel log 路径）。
    - 对新网卡做识别：
      - 优先匹配网卡地址是否包含 `tunSettings.address` 中任一 CIDR 的网关 IP（例如默认 `172.19.0.1/30` 中的 `172.19.0.1`）。
      - 若地址尚未配置，回退到轻量特征匹配（如 `utun*` / `*wintun*` / `*tun*`），避免完全依赖固定名称。
    - 返回实际网卡名并写入 `s.tunIface`，供 stop/restart 的释放等待使用。
- sing-box 配置生成：
  - 非 Linux 下，如果 `tunSettings.interfaceName` 仍为默认值 `tun0`（且用户未显式配置），则不写 `interface_name` 字段（留空让 sing-box 自动选择），与官方文档一致。

## API设计

- 不改现有后端接口；继续使用 `/proxy/start`、`/proxy/status`、`/tun/check`。

## 安全与性能

- **安全:** 不引入新的提权流程；仅调整就绪检测与错误信息，不触碰敏感数据。
- **性能:** 就绪探测保持 10s 超时与低频轮询，不新增高频 API 请求。

## 测试与部署

- **测试:**
  - 新增/调整纯函数单测覆盖“地址匹配/网卡识别”的核心逻辑。
  - 回归：`go test ./...`
- **部署:** 随后端发布；Windows 用户升级后，TUN 不再因网卡名误判而失败。

