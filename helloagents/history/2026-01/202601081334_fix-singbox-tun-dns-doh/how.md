# 技术设计: 修复 sing-box TUN 模式 DNS 断网（默认改用 DoH）

## 技术方案

### 核心思路
将 sing-box 的默认 `dns-remote` 从 `tcp + 8.8.8.8:53` 调整为 **DoH(HTTPS, 443)**，以规避 53 端口在真实网络中的常见限制。

### 配置细节
- `dns-local`：保持 `udp + 223.5.5.5`（用于节点域名解析自举，避免 bootstrap 依赖代理）。
- `dns-remote`：
  - `type: "https"`
  - `server: "1.1.1.1"`, `server_port: 443`, `path: "/dns-query"`
  - `tls.server_name: "cloudflare-dns.com"`（确保证书校验与 SNI 正确）
  - `detour`：沿用既有逻辑，仅当 `defaultTag != "direct"` 时才设置，避免指向“空 direct outbound”触发 sing-box 运行期 fatal。

## 测试策略
- 增加单元测试断言 `dns-remote` 默认使用 DoH，并检查关键字段（type/server/port/path/tls）。
- 执行 `go test ./...` 确保全量通过。

## 安全与兼容性
- 不引入新依赖；
- 仅调整默认 DNS 出站的协议/端口，语义保持“远程 DNS 走 detour”不变；
- DoH 使用标准 TLS 校验（未开启 insecure）。

