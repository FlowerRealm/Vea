# 变更提案：系统代理端口支持局域网连接（Allow LAN）

## 背景

当前 Vea 的“系统代理端口”（`ProxyConfig.inboundPort`，通常为 `mixed`）主要面向本机使用（默认监听 `127.0.0.1`）。
用户希望在同一局域网内的其他设备也能连接该端口，将本机作为代理网关使用（例如 `192.168.x.x:1080`）。

## 目标（Goals）

1. 提供一个“允许局域网连接”的开关（默认关闭）。
2. 开关开启后：`mixed` 入站监听全网卡（`0.0.0.0`），允许整个 LAN 访问。
3. 开关关闭后：回到仅本机可连（默认 `127.0.0.1`）。
4. 兼容 sing-box 与 mihomo(clash) 两种引擎；内核运行时变更会自动重启生效。

## 非目标（Non-Goals）

- 不增加白名单/鉴权等安全能力（用户明确“允许整个 LAN”）。
- 不处理系统防火墙/路由器策略导致的 LAN 访问失败（仅保证应用侧监听与配置正确）。

## 方案概览

### 1) 后端

- 使用 `ProxyConfig.InboundConfig.AllowLAN` 作为开关语义。
- sing-box：在生成入站时，当 `allowLan=true` 且 `listen` 为空或为 loopback（`127.0.0.1/localhost/::1`）时，将 `listen` 写为全网卡（`0.0.0.0` 或 `::`）。
- 端口占用检查：与最终监听地址保持一致（避免只检查 `127.0.0.1` 导致漏检）。

### 2) 前端

- 设置项 `inbound.allowLan` 变更时，联动调用 `PUT /proxy/config` 更新 `inboundConfig.allowLan`。
- 若内核运行中：自动触发 `POST /proxy/start` 重启应用配置。
- 启动时从后端同步 `inboundConfig.allowLan`，确保 UI 状态与实际一致。

## 验收标准（Acceptance Criteria）

- 开关开启 + 端口设置为 `1080`：
  - 其他局域网设备可使用 `本机局域网 IP:1080`（HTTP/SOCKS）正常连通并代理。
- 开关关闭：
  - 其他局域网设备无法通过 `本机局域网 IP:1080` 连接（或连接失败）。
- `go test ./...` 通过。

