# sing-box 内核集成指南

本文档说明如何在 Vea 中使用 sing-box 内核，包括 TUN 模式和 Hysteria2/TUIC 协议支持。

## 核心特性

### ✅ 已实现
- **双内核支持**：Xray + sing-box 自动选择
- **TUN 模式**：系统级透明代理（Linux/Windows/macOS）
- **新协议支持**：Hysteria2、TUIC（仅 sing-box）
- **自动引擎选择**：根据节点协议和入站模式自动选择最佳内核
- **权限管理**：Linux CAP_NET_ADMIN、Windows 管理员、macOS sudo

---

## 快速开始

### 1. 安装 sing-box 内核

创建 sing-box 组件：

```bash
curl -X POST http://localhost:19080/components \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sing-box",
    "kind": "singbox",
    "sourceUrl": "https://github.com/SagerNet/sing-box/releases/latest"
  }'
```

安装组件：

```bash
curl -X POST http://localhost:19080/components/{component-id}/install
```

### 2. 配置 TUN 模式权限（Linux 专用）

```bash
# 一键设置 TUN 权限
sudo ./vea setup-tun
```

**执行内容**：
1. 创建系统用户 `vea-tun`（禁止登录）
2. 设置 sing-box 二进制文件的 `CAP_NET_ADMIN` capability
3. 配置文件所有者为 `vea-tun`

**验证**：
```bash
# sing-box 可能位于版本子目录（例如 artifacts/core/sing-box/sing-box-*/sing-box）
# 推荐优先从应用日志中查看实际路径：`[TUN-Check] sing-box 路径: ...`
find artifacts/core/sing-box -maxdepth 2 -type f -name 'sing-box' -print

# 对实际路径执行 getcap（将 <path> 替换为上面 find 的输出之一）
getcap <path>
# 输出示例：<path> = cap_net_admin,cap_net_bind_service,cap_net_raw+ep
```

---

## API 使用

### ProxyConfig API（单例运行配置）

#### 配置 SOCKS 模式（Xray）并启动

```bash
curl -X PUT http://localhost:19080/proxy/config \
  -H "Content-Type: application/json" \
  -d '{
    "inboundMode": "socks",
    "inboundPort": 38087,
    "preferredEngine": "xray"
  }'

curl -X POST http://localhost:19080/proxy/start \
  -H "Content-Type: application/json" \
  -d '{ "frouterId": "<frouter-id>" }'
```

#### 配置 TUN 模式（sing-box 强制）并启动

```bash
curl -X PUT http://localhost:19080/proxy/config \
  -H "Content-Type: application/json" \
  -d '{
    "inboundMode": "tun",
    "preferredEngine": "singbox",
    "tunSettings": {
      "interfaceName": "tun0",
      "mtu": 9000,
      "address": ["172.19.0.1/30"],
      "autoRoute": true,
      "strictRoute": true,
      "stack": "mixed",
      "dnsHijack": true
    }
  }'

curl -X POST http://localhost:19080/proxy/start \
  -H "Content-Type: application/json" \
  -d '{ "frouterId": "<frouter-id>" }'
```

**注意**：`inboundMode: "tun"` 会强制选择 `engine: "singbox"`。

#### 查看代理状态

```bash
curl http://localhost:19080/proxy/status
```

响应示例：
```json
{
  "running": true,
  "frouterId": "frouter-uuid",
  "inboundMode": "tun",
  "engine": "singbox",
  "pid": 12345
}
```

#### 停止代理

```bash
curl -X POST http://localhost:19080/proxy/stop
```

---

## 自动引擎选择逻辑

### 规则优先级

1. **TUN 模式** → 强制 sing-box
2. **节点要求 sing-box**（Hysteria2/TUIC 或 Shadowsocks 插件）→ 优先 sing-box
3. **用户偏好**（`preferredEngine`）→ 已安装且支持全部节点时优先
4. **前端默认引擎**（`engine.defaultEngine`）→ 作为候选
5. **协议推荐结果** → 作为候选
6. **兜底候选** → sing-box → xray

最终会在候选列表里选择第一个“已安装且支持全部节点”的引擎，否则报错。

### 示例场景

| 节点协议 | 入站模式 | 用户偏好 | 实际使用 | 原因 |
|---------|---------|---------|---------|------|
| VLESS | SOCKS | xray | Xray | 用户偏好 + 协议支持 |
| Hysteria2 | SOCKS | xray | sing-box | 协议强制 |
| VMess | TUN | auto | sing-box | TUN 强制 |
| Trojan | Mixed | singbox | sing-box | 用户偏好 + 协议支持 |

---

## TUN 模式配置

### Linux

```json
{
  "tunSettings": {
    "interfaceName": "tun0",
    "mtu": 9000,
    "address": ["172.19.0.1/30"],
    "autoRoute": true,
    "strictRoute": true,
    "stack": "mixed",
    "dnsHijack": true
  }
}
```

**字段说明**：
- `interfaceName`: 虚拟网卡名称（默认 `tun0`）
- `mtu`: 最大传输单元（推荐 `9000` 用于性能优化）
- `address`: TUN 设备 IP 地址
- `autoRoute`: 自动配置系统路由表
- `strictRoute`: 严格路由模式（sing-box 特有）
- `stack`: 网络栈（`system`/`gvisor`/`mixed`）
- `dnsHijack`: DNS 劫持（防止 DNS 泄漏）

### Windows

需要以**管理员身份**运行 Vea。无需额外配置。

### macOS

需要使用 `sudo` 运行 Vea：

```bash
sudo ./vea
```

---

## 权限检查 API

### 检查 TUN 权限

```bash
curl http://localhost:19080/tun/check
```

**Linux 响应**（已配置）：
```json
{
  "configured": true,
  "platform": "linux",
  "setupCommand": "sudo ./vea setup-tun",
  "description": "Creates vea-tun user and sets capabilities for sing-box (cap_net_admin,cap_net_bind_service,cap_net_raw)"
}
```

**Windows 响应**：
```json
{
  "configured": false,
  "platform": "windows",
  "setupCommand": "Run Vea as Administrator",
  "description": "TUN mode requires administrator privileges on Windows"
}
```

---

## 节点协议支持

### Xray 支持的协议
- VLESS
- VMess
- Trojan
- Shadowsocks

### sing-box 额外支持
- **Hysteria2** ✅
- **TUIC** ✅
- 以及所有 Xray 协议

---

## 故障排除

### TUN 模式启动失败

**错误**：`TUN mode not configured`

**解决**：
```bash
# Linux
sudo ./vea setup-tun

# Windows
# 以管理员身份运行 Vea

# macOS
sudo ./vea
```

### Hysteria2 节点无法连接

**检查**：
1. 确认 sing-box 组件已安装
2. 查看代理状态 `engine` 是否为 `singbox`

```bash
curl http://localhost:19080/proxy/status
curl http://localhost:19080/proxy/config
```

### 权限被拒绝

**错误**：`permission denied`

**检查**：
```bash
# 验证 capabilities
find artifacts/core/sing-box -maxdepth 2 -type f -name 'sing-box' -print
getcap <path>

# 验证用户
id vea-tun

# 验证二进制文件所有者
ls -l <path>
```

---

## 配置文件示例

### 完整的 ProxyConfig（TUN）

```json
{
  "inboundMode": "tun",
  "tunSettings": {
    "interfaceName": "tun0",
    "mtu": 9000,
    "address": ["172.19.0.1/30"],
    "autoRoute": true,
    "strictRoute": true,
    "stack": "mixed",
    "dnsHijack": true
  },
  "preferredEngine": "singbox",
  "frouterId": "frouter-id",
  "updatedAt": "2025-12-25T00:00:00Z"
}
```

---

## 高级用法

### 混合模式（HTTP + SOCKS）

```bash
curl -X PUT http://localhost:19080/proxy/config \
  -H "Content-Type: application/json" \
  -d '{
    "inboundMode": "mixed",
    "inboundPort": 38087,
    "preferredEngine": "singbox"
  }'

curl -X POST http://localhost:19080/proxy/start \
  -H "Content-Type: application/json" \
  -d '{ "frouterId": "<frouter-id>" }'
```

**sing-box 优势**：原生 `mixed` 类型，无需两个端口。

---

## 性能对比

| 特性 | Xray | sing-box |
|------|------|----------|
| 内存占用 | ~50 MB | ~30 MB |
| TUN 模式 | ❌ | ✅ |
| Hysteria2 | ❌ | ✅ |
| TUIC | ❌ | ✅ |
| 配置复杂度 | 中 | 低 |
| 社区活跃度 | 高 | 高 |

---

## 参考资料

- [sing-box 官方文档](https://sing-box.sagernet.org/)
- [Xray 文档](https://xtls.github.io/)
- [Linux Capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html)
