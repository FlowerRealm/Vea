# 更新日志：sing-box 内核集成

## 🎉 重大更新

Vea 现已支持 **sing-box 内核**，实现了以下核心功能：

---

## 新增功能

### 1. **双内核架构**
- ✅ **Xray-core**：传统代理协议（VLESS/VMess/Trojan/Shadowsocks）
- ✅ **sing-box**：现代代理协议 + TUN 模式
- ✅ **自动选择**：根据节点协议和入站模式智能选择最佳内核

### 2. **TUN 模式支持**
- ✅ **系统级透明代理**：无需配置应用代理
- ✅ **跨平台**：Linux（CAP_NET_ADMIN）、Windows（管理员）、macOS（sudo）
- ✅ **安全隔离**：Linux 使用专用用户 `vea-tun`，最小权限原则

### 3. **新协议支持**
- ✅ **Hysteria2**：基于 QUIC 的高速代理协议
- ✅ **TUIC**：基于 QUIC 的代理协议

### 4. **ProxyProfile 管理**
- ✅ **配置分离**：入站模式、引擎选择、TUN 配置独立管理
- ✅ **一键切换**：SOCKS/HTTP/Mixed/TUN 模式快速切换
- ✅ **持久化**：配置自动保存到 `data/state.json`

---

## 架构变更

### 数据模型

#### 新增领域模型
```go
// backend/domain/entities.go

// 入站模式
type InboundMode string
const (
    InboundSOCKS InboundMode = "socks"
    InboundHTTP  InboundMode = "http"
    InboundMixed InboundMode = "mixed"
    InboundTUN   InboundMode = "tun"
)

// 内核引擎类型
type CoreEngineKind string
const (
    EngineXray    CoreEngineKind = "xray"
    EngineSingBox CoreEngineKind = "singbox"
    EngineAuto    CoreEngineKind = "auto"
)

// 代理配置文件
type ProxyProfile struct {
    ID              string
    Name            string
    InboundMode     InboundMode
    InboundPort     int
    TUNSettings     *TUNConfiguration
    PreferredEngine CoreEngineKind
    ActualEngine    CoreEngineKind
    DefaultNode     string
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

// TUN 配置
type TUNConfiguration struct {
    InterfaceName string
    MTU           int
    Address       []string
    AutoRoute     bool
    StrictRoute   bool
    Stack         string
    DNSHijack     bool
    Platform      *PlatformTUNConfig
}
```

### 适配器模式

#### CoreAdapter 接口
```go
// backend/service/adapters/adapter.go

type CoreAdapter interface {
    Kind() CoreEngineKind
    BinaryNames() []string
    SupportedProtocols() []NodeProtocol
    SupportsInbound(mode InboundMode) bool
    BuildConfig(profile ProxyProfile, nodes []Node, geo GeoFiles) ([]byte, error)
    RequiresPrivileges(profile ProxyProfile) bool
}
```

#### 实现
- ✅ **XrayAdapter**：`backend/service/adapters/xray.go`
- ✅ **SingBoxAdapter**：`backend/service/adapters/singbox.go`

### 权限管理

#### Linux（推荐方案）
```bash
# 一键设置
sudo ./vea setup-tun

# 实现细节
backend/service/privilege_linux.go:
  - 创建 vea-tun 用户（禁止登录）
  - setcap cap_net_admin+ep <sing-box-binary>
  - chown vea-tun:vea-tun <sing-box-binary>
```

#### Windows
```powershell
# 以管理员身份运行
backend/service/privilege_windows.go:
  - 检查 IsUserAnAdmin()
```

#### macOS
```bash
# 使用 sudo
sudo ./vea
```

---

## API 变更

### 新增端点

#### ProxyProfile CRUD
```
GET    /proxy-profiles          # 列出所有 Profile
POST   /proxy-profiles          # 创建 Profile
GET    /proxy-profiles/:id      # 获取 Profile
PUT    /proxy-profiles/:id      # 更新 Profile
DELETE /proxy-profiles/:id      # 删除 Profile
POST   /proxy-profiles/:id/start # 启动 Profile
```

#### 代理控制
```
GET  /proxy/status  # 获取代理状态
POST /proxy/stop    # 停止代理
```

#### TUN 权限检查
```
GET  /tun/check     # 检查 TUN 权限配置
```

### 兼容性

**旧的 Xray API 保持不变**：
```
GET  /xray/status
POST /xray/start
POST /xray/stop
```

---

## 文件结构

### 新增文件

```
backend/
├── domain/
│   └── entities.go                    # 新增 ProxyProfile, TUNConfiguration
├── service/
│   ├── adapters/
│   │   ├── adapter.go                 # CoreAdapter 接口
│   │   ├── xray.go                    # Xray 适配器
│   │   └── singbox.go                 # SingBox 适配器
│   ├── engine_selector.go             # 自动引擎选择
│   ├── proxy_profile.go               # ProxyProfile Service 方法
│   ├── privilege_linux.go             # Linux 权限管理
│   ├── privilege_windows.go           # Windows 权限管理
│   └── privilege_darwin.go            # macOS 权限管理
├── store/
│   └── memory.go                      # 新增 ProxyProfile CRUD
└── api/
    └── proxy_profile.go               # ProxyProfile API 处理器

main.go                                # 新增 setup-tun 子命令

docs/
└── SING_BOX_INTEGRATION.md            # 使用文档
```

---

## 迁移指南

### 从旧版本升级

**无需手动迁移**！

1. 编译新版本：
   ```bash
   make build
   ```

2. 启动应用：
   ```bash
   ./vea
   ```

3. 旧的 Xray 配置会自动保留

4. 新增 sing-box 组件：
   ```bash
   curl -X POST http://localhost:8080/components \
     -H "Content-Type: application/json" \
     -d '{
       "name": "sing-box",
       "kind": "singbox",
       "sourceUrl": "https://github.com/SagerNet/sing-box/releases/latest"
     }'
   ```

5. 配置 TUN 权限（可选）：
   ```bash
   sudo ./vea setup-tun
   ```

### 数据持久化

新增字段会自动添加到 `data/state.json`：

```json
{
  "nodes": [...],
  "configs": [...],
  "geoResources": [...],
  "components": [
    {
      "kind": "xray",    // 保留
      ...
    },
    {
      "kind": "singbox", // 新增
      ...
    }
  ],
  "proxyProfiles": [     // 新增
    {
      "id": "...",
      "name": "默认 SOCKS",
      "inboundMode": "socks",
      "inboundPort": 38087,
      "preferredEngine": "xray",
      "actualEngine": "xray",
      "defaultNode": "..."
    }
  ],
  "activeProfile": "...", // 新增
  "trafficProfile": {...},
  "systemProxy": {...}
}
```

---

## 测试清单

### 功能测试

- [ ] SOCKS 模式 + Xray
- [ ] Mixed 模式 + sing-box
- [ ] TUN 模式 + sing-box
- [ ] Hysteria2 节点自动选择 sing-box
- [ ] 权限检查 API
- [ ] Profile 切换
- [ ] 代理启动/停止

### 平台测试

- [ ] Linux（Ubuntu 22.04）
- [ ] Linux（Arch Linux）
- [ ] Windows 11
- [ ] macOS（M1/M2）

---

## 已知限制

1. **Xray TUN 模式**：已移除（使用 sing-box 替代）
2. **自动协议转换**：不支持 Xray ↔ sing-box 配置互转
3. **TUN DNS 劫持**：需要 `autoRoute: true`

---

## 后续计划

### Phase 2（未实施）
- [ ] 前端 UI 界面（Profile 管理）
- [ ] 节点测速（TUN 模式下）
- [ ] 流量统计（按 Profile）
- [ ] Clash 内核支持
- [ ] 自动更新内核二进制

---

## 贡献者

感谢以下开源项目：
- [Xray-core](https://github.com/XTLS/Xray-core)
- [sing-box](https://github.com/SagerNet/sing-box)
- [v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)

---

**更新时间**: 2025-01-20
**版本**: v2.0.0 (sing-box integration)
