# sing-box 内核集成指南

本文档说明如何在 Vea 中使用 sing-box 内核，包括 TUN 模式和 Hysteria2/TUIC 协议支持。

## 核心特性

### ✅ 已实现
- **多内核支持**：sing-box + Clash(mihomo) 自动选择
- **TUN 模式**：系统级透明代理（Linux/Windows/macOS）
- **新协议支持**：Hysteria2、TUIC
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

#### 配置 SOCKS 模式并启动

```bash
curl -X PUT http://localhost:19080/proxy/config \
  -H "Content-Type: application/json" \
  -d '{
    "inboundMode": "socks",
    "inboundPort": 38087,
    "preferredEngine": "auto"
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
	      "interfaceName": "vea",
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

**注意**：实际运行的 `engine` 会根据可用内核与节点协议自动选择；如需固定可设置 `preferredEngine` 为 `singbox` 或 `clash`。

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

1. **用户偏好**（`preferredEngine`）→ 支持入站模式且能覆盖全部节点时优先（未安装会触发安装后重试）
2. **前端默认引擎**（`engine.defaultEngine`）→ 作为候选
3. **协议推荐结果** → 作为候选（优先 sing-box，无法满足时回退 clash）
4. **兜底候选** → sing-box → clash

最终会在候选列表里选择第一个“已安装且支持全部节点”的引擎，否则报错。

### 示例场景

| 节点协议 | 入站模式 | 用户偏好 | 实际使用 | 原因 |
|---------|---------|---------|---------|------|
| VLESS | SOCKS | auto | sing-box | 默认推荐 |
| Hysteria2 | SOCKS | clash | clash | 用户偏好 + 协议支持 |
| VMess | TUN | auto | sing-box | 默认推荐 |
| Trojan | Mixed | singbox | sing-box | 用户偏好 + 协议支持 |

---

## TUN 模式配置

### Linux

```json
{
  "tunSettings": {
    "interfaceName": "vea",
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
- `interfaceName`: 虚拟网卡名称（默认 `vea`；Windows/macOS 默认不强制写死名称；旧默认 `tun0` 仍兼容）
- `mtu`: 最大传输单元（推荐 `9000` 用于性能优化）
- `address`: TUN 设备 IP 地址
- `autoRoute`: 自动配置系统路由表
- `strictRoute`: 严格路由模式（sing-box 特有）
- `stack`: 网络栈（`system`/`gvisor`/`mixed`）
- `dnsHijack`: DNS 劫持（防止 DNS 泄漏）

### Windows

Windows 下 TUN 通常无需额外“一次性配置”；若启动失败，请尝试以**管理员身份**运行 Vea，并确认 Wintun 驱动可用。

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
  "configured": true,
  "platform": "windows",
  "setupCommand": "无需额外配置",
  "description": "Windows 下 TUN 通常无需一次性配置；若启动失败请尝试以管理员身份运行 Vea，并确认 Wintun 驱动可用"
}
```

---

## 节点协议支持

### 支持的协议
- VLESS
- VMess
- Trojan
- Shadowsocks
- Hysteria2
- TUIC

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

### Windows：启用 TUN 超时 / `TUN interface not ready`

**现象**：打开 TUN 后等待约 10~30 秒提示失败（sing-box / mihomo 均可能出现）。

**排查步骤**：
1. 打开应用“日志”面板，复制 `kernel.log` 路径，并截取当次启动日志（从 `----- kernel start ... -----` 到报错位置）。
2. 在 `kernel.log` 中搜索关键词：`wintun`、`access is denied`、`requires elevation`、`failed`，优先以日志为准定位真实失败原因。
3. 确认以管理员身份运行；若系统安装了其它 VPN/加速器/安全软件网络防护，可能与 TUN/路由表冲突，建议先暂时关闭或卸载后复测。

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
    "interfaceName": "vea",
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
（已移除对其他内核的对比；以实际运行体验为准）

---

## 参考资料

- [sing-box 官方文档](https://sing-box.sagernet.org/)
- [mihomo(Clash.Meta) 项目](https://github.com/MetaCubeX/mihomo)
- [Linux Capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html)
