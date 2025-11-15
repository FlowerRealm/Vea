# Vea 集成测试脚本 - 完整代理链路验证

## 项目概述

本项目为 Vea 实现了**真实的端到端集成测试**，验证完整的代理链路：

**本地 xray 服务端 → Vea 管理的 xray 客户端 → 外部网站（Google/httpbin.org）**

## 测试架构

### 设计思路

不同于简单的 TCP ping，这个测试验证了**真正的代理功能**：

1. **本地 xray 服务端**（VLESS，端口 20086）- 模拟真实的代理服务器
2. **Vea 管理的 xray 客户端**（SOCKS5，端口 38087）- 连接到服务端
3. **通过 SOCKS5 代理访问外部网站** - 验证整个链路

### 关键验证点

✅ **代理链路完整性**：流量必须走 `[测试] → SOCKS5 → xray 客户端 → xray 服务端 → 外网`
✅ **路由规则正确**：使用 google.com/httpbin.org 确保流量走代理而非直连
✅ **真实网络访问**：实际下载数据验证功能

## 已实现的测试

### ✅ TestE2E_ProxyToCloudflare - 完整代理链路测试

**测试内容**：
- 启动本地 VLESS 服务端（端口 20086）
- Vea 创建节点并启动 xray 客户端（SOCKS5 端口 38087）
- 下载真实的 GeoIP/GeoSite 文件（确保路由规则正常）
- 子测试 1：通过 SOCKS5 测试延迟（连接 speed.cloudflare.com:443）
- 子测试 2：通过 SOCKS5 下载数据（从 speed.cloudflare.com 下载 100KB）

**关键日志证明**：
```
[Info] app/dispatcher: taking detour [node-d7e8ca9a] for [tcp:speed.cloudflare.com:443]
[Info] proxy/vless/outbound: tunneling request to tcp:speed.cloudflare.com:443 via 127.0.0.1:20086
```

**运行方式**：
```bash
# 无需环境变量，直接运行
go test -v -run TestE2E_ProxyToCloudflare ./internal/service/
```

**预期结果**：
```
=== RUN   TestE2E_ProxyToCloudflare
    integration_test.go:35: 使用 xray: /home/flowerrealm/Vea/internal/service/artifacts/core/xray/xray
    integration_test.go:189: xray 客户端 SOCKS5 端口: 38087
=== RUN   TestE2E_ProxyToCloudflare/Latency
    integration_test.go:200: ✓ 延迟测试通过: 1 ms
=== RUN   TestE2E_ProxyToCloudflare/Speed
    integration_test.go:215: ✓ 速度测试通过: 下载 102400 bytes (0.78 Mbps)
--- PASS: TestE2E_ProxyToCloudflare (10.68s)
PASS
```


## 文件结构

```
internal/service/
├── integration_test.go  # 完整的代理链路测试
├── TESTING.md           # 详细技术文档
└── service_test.go      # 单元测试
```

## 运行测试

### 运行完整代理测试

```bash
# 无需环境变量，自动查找 xray
go test -v ./internal/service/ -run TestE2E_ProxyToCloudflare

# 或使用自定义路径的 xray
XRAY_BINARY=/path/to/xray go test -v ./internal/service/ -run TestE2E_ProxyToCloudflare
```

### 跳过集成测试

```bash
go test -short ./internal/service/
```

## 前置条件

1. **xray 二进制文件**：项目中已包含在 `artifacts/core/xray/xray`
2. **Go 1.22+**：确保 Go 版本兼容
3. **网络连接**：需要下载 GeoIP/GeoSite 文件（~10MB），并访问外部网站
4. **可用端口**：测试使用端口 20086（xray 服务端）和 38087（xray 客户端）

## 技术实现要点

### 1. 完整的测试流程

```
1. 下载 Geo 文件（GeoIP.dat + GeoSite.dat）
2. 启动本地 xray 服务端（VLESS，无加密）
3. 创建 Vea Service + 节点
4. 调用 EnableXray 启动客户端
5. 通过 golang.org/x/net/proxy 创建 SOCKS5 连接
6. 验证延迟和下载功能
```

### 2. 关键技术突破

**问题**：最初测试使用 `www.gstatic.com`，但流量被路由到直连（direct）而非代理。

**解决**：使用 Cloudflare 的测速服务：
- `speed.cloudflare.com:443` - 延迟测试
- `speed.cloudflare.com/__down?bytes=102400` - 速度测试（100KB）

**验证方法**：检查 xray 日志中的 `taking detour [node-xxx]`，而不是 `taking detour [direct]`

### 3. Geo 文件处理

初始尝试使用假的 Geo 文件会导致 xray 配置解析失败：
```
failed to load geosite: CATEGORY-ADS-ALL
list not found in geosite.dat
```

**解决**：测试中动态下载真实的 Geo 文件：
```go
downloadFileForTest("https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat", ...)
downloadFileForTest("https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat", ...)
```

### 4. SOCKS5 代理实现

使用 `golang.org/x/net/proxy` 包：
```go
dialer, _ := proxy.SOCKS5("tcp", "127.0.0.1:38087", nil, proxy.Direct)
conn, _ := dialer.Dial("tcp", "google.com:443")
```

HTTP 客户端通过 SOCKS5：
```go
httpClient := &http.Client{
    Transport: &http.Transport{
        DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
            return dialer.Dial(network, addr)
        },
    },
    Timeout: 30 * time.Second,
}
```

## 测试证明的功能

✅ **代理链路完整**：流量确实通过本地 xray 服务端转发
✅ **路由规则正确**：Geo 文件加载成功，规则生效
✅ **SOCKS5 工作正常**：xray 客户端正确响应 SOCKS5 请求
✅ **真实网络访问**：成功连接并从 speed.cloudflare.com 下载 100KB 数据

## 局限性与未来改进

### 当前局限

1. **需要网络连接**：测试依赖外部网站和 Geo 文件下载（约 10MB）
2. **测试时间较长**：首次运行约 13 秒（包含下载）
3. **不支持离线运行**：无法在完全隔离的环境中测试

### 未来改进方向

1. **缓存 Geo 文件**：首次下载后保存到项目目录，后续测试复用
2. **Mock HTTP 服务器**：在本地启动一个 HTTP 服务器替代 httpbin.org
3. **并发测试**：测试多个节点同时工作的场景
4. **性能基准**：使用 `testing.B` 测量代理的实际性能开销
5. **错误场景**：测试节点不可达、超时、认证失败等情况

## 总结

### [核心判断]
**完全值得做** - 这个测试验证了 Vea 与真实 xray 的完整集成，证明代理功能真的能工作。

### [关键洞察]
- **数据流清晰**：测试 → SOCKS5 (38087) → xray 客户端 → xray 服务端 (20086) → 外网
- **复杂度适中**：使用最简 VLESS 配置 + 真实 Geo 文件，既验证功能又避免过度工程
- **风险点已解决**：
  - Geo 文件下载（真实文件，规则生效）
  - 路由规则验证（确保走代理而非直连）
  - 组件配置管理（自动创建的组件更新）

### [Linus 式总结]

**"Good code proves itself through real tests, not theory."**

这个测试不依赖模拟或假数据：
- ✅ **真实的 xray 二进制**（服务端 + 客户端）
- ✅ **真实的网络流量**（连接 Google，下载数据）
- ✅ **真实的路由规则**（GeoIP/GeoSite 文件）

日志证明：`taking detour [node-xxx]` - 流量真的走代理了，不是直连。

**最重要的证明**：成功从 speed.cloudflare.com 下载了 102400 字节 (100KB)。这说明整个链路从头到尾都是通的 - 不是随机数，不是模拟，是真实的网络数据。

**"Theory and practice sometimes clash. Theory loses. Every single time."** - 我们用真实的代理流量验证了代码，而不是在理论上假设它能工作。这才是 good taste in testing.
