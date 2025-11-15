# Vea 集成测试说明

## 概述

本目录包含了 Vea 项目的集成测试，用于验证与真实 xray 二进制文件的交互。

## 测试文件

- `integration_test.go` - 端到端集成测试

## 测试用例

### TestE2E_ProxyToCloudflare ✅

测试完整代理链路功能，验证：
- 本地启动 xray 服务端（VLESS 协议，端口 20086）
- 通过 Vea 创建指向本地服务端的节点
- 启动 Vea 管理的 xray 客户端（SOCKS5 端口 38087）
- 下载真实的 GeoIP/GeoSite 文件（确保路由规则正常）
- 子测试 1：通过 SOCKS5 测量延迟（连接 speed.cloudflare.com:443）
- 子测试 2：通过 SOCKS5 测试速度（从 speed.cloudflare.com 下载 100KB）

**状态**：✅ 通过

**运行方式**：
```bash
# 无需环境变量，自动查找 xray
go test -v -run TestE2E_ProxyToCloudflare ./internal/service/
```

## 前置条件

### 1. xray 二进制文件

测试需要 xray 二进制文件。可以通过以下方式提供：

**方法一：自动查找（推荐）**
```bash
# 自动从项目目录、环境变量或系统 PATH 查找 xray
go test -v ./internal/service/
```

**方法二：使用环境变量**
```bash
export XRAY_BINARY=/path/to/xray
go test -v ./internal/service/
```

**方法三：使用项目中已有的 xray**
```bash
XRAY_BINARY=$(pwd)/artifacts/core/xray/xray go test -v ./internal/service/
```

### 2. 跳过集成测试

如果要跳过所有集成测试：
```bash
go test -short ./internal/service/
```

## 测试架构

### 完整代理链路测试流程
```
测试代码
  ↓
下载 Geo 文件（GeoIP.dat + GeoSite.dat）
  ↓
启动 xray 服务端（VLESS，端口 20086）
  ↓
创建 Vea Service + 配置组件
  ↓
添加节点（指向本地服务端）
  ↓
调用 EnableXray 启动客户端（SOCKS5，端口 38087）
  ↓
子测试 1: 延迟测试
  └─ 通过 SOCKS5 连接 speed.cloudflare.com:443
  └─ 测量 TCP 连接延迟
  ↓
子测试 2: 速度测试
  └─ 通过 SOCKS5 下载 100KB 数据
  └─ 验证代理链路工作正常
  ↓
验证结果
```

## 运行所有集成测试

```bash
# 运行所有集成测试
go test -v ./internal/service/ -run "TestE2E"

# 只运行完整代理链路测试
go test -v ./internal/service/ -run "TestE2E_ProxyToCloudflare"

# 使用自定义 xray 路径
XRAY_BINARY=/path/to/xray go test -v ./internal/service/ -run "TestE2E"
```

## 故障排除

### 错误：xray binary not found

确保 xray 二进制文件存在且可执行：
```bash
ls -la artifacts/core/xray/xray
chmod +x artifacts/core/xray/xray
```

### 错误：port already in use

测试使用端口 20086（xray 服务端）和 38087（xray 客户端）。如果这些端口被占用，测试会失败。

### 速度测试失败

速度测试需要网络连接到 speed.cloudflare.com。在没有网络或被防火墙阻止的环境中，速度测试会被跳过，但不会导致整个测试失败。

## 贡献指南

添加新的集成测试时：
1. 确保测试使用 `testing.Short()` 检查，支持 `-short` 标志跳过
2. 使用临时目录（`t.TempDir()`）存放测试数据
3. 确保进程在测试结束后被正确清理（使用 `defer`）
4. 使用高端口号（> 20000）避免权限问题
