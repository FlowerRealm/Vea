# 技术设计: 修复首页“当前 IP”未走代理（Issue #26）

## 技术方案

### 核心技术
- Go `net/http`（自定义 `http.Transport`）
- 最小 SOCKS5 Dialer（noauth / username+password）

### 实现要点

- 入口在 `GET /ip/geo`：由 `backend/service/facade.go:GetIPGeo()` 决定“直连 or 走本地入站代理”。
- 当代理运行且 `inboundMode != tun` 时：
  - `socks`：使用 SOCKS5 Dialer 构造 `http.Client`（不依赖系统代理/环境变量）。
  - `http`：使用 HTTP Proxy（`http.Transport.Proxy`）构造 `http.Client`。
  - `mixed`：优先尝试 SOCKS5（更通用），失败后回退到 HTTP Proxy（兼容部分实现差异）。
- 当代理未运行时：沿用现有直连逻辑（避免“没有代理但强行走 127.0.0.1:1080”导致误报）。
- 不做“走代理失败→静默回退直连”：避免再次出现 Issue#26 的用户误判。

## API设计

### [GET] /ip/geo
- **请求:** 无
- **响应:** 字段不变（`ip/location/asn/isp`）；失败时返回 `error`
- **行为修复:** 代理运行中时返回代理出口 IP（而非真实出口 IP）

## 安全与性能

- **安全:**
  - 不写入/输出任何订阅、Token 等敏感信息。
  - 不依赖系统代理环境变量，避免被外部环境劫持。
- **性能:**
  - 复用现有 6s 超时、64KB 响应上限与多 provider 回退策略。
  - 仅在用户查看首页或点击刷新时触发（前端已有按钮）。

## 测试与部署

- **测试:**
  - 增加离线单测：本地起一个假 IP 服务 + 假 SOCKS5/HTTP proxy，验证 `GetIPGeo` 在不同 `inboundMode` 下是否确实经由代理发起请求。
  - 运行 `go test ./...`
- **部署:**
  - 无额外部署步骤；随应用更新发布。

