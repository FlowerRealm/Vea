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

### 4. **ProxyConfig（单例运行配置）**
- ✅ **以 FRouter 为一等单元**：启动/切换只需要指定 `frouterId`
- ✅ **配置收敛**：入站模式、引擎选择、TUN 配置统一归入 `ProxyConfig`
- ✅ **持久化**：配置自动保存到 `data/state.json`

### 5. **Node 独立实体（食材）**
- ✅ **节点全局列表**：Node 独立于 FRouter（工具），提供 `/nodes` 列表与测速/延迟测量 API
- ✅ **订阅同步节点**：`POST /configs/:id/pull-nodes` 从配置/订阅提取节点并写入全局节点集合
- ✅ **FRouter 图引用 NodeID**：图编辑入口收敛为 `/frouters/:id/graph`

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

// 代理运行配置（单例）
type ProxyConfig struct {
    InboundMode     InboundMode
    InboundPort     int
    TUNSettings     *TUNConfiguration
    PreferredEngine CoreEngineKind
    FRouterID       string
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
    BuildConfig(plan nodegroup.RuntimePlan, geo GeoFiles) ([]byte, error)
    RequiresPrivileges(config ProxyConfig) bool
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
backend/service/shared/tun_linux.go:
  - 创建 vea-tun 用户（禁止登录）
  - chown vea-tun:vea-tun <sing-box-binary>（注意：chown 会清除 capabilities）
  - setcap cap_net_admin,cap_net_bind_service,cap_net_raw+ep <sing-box-binary>
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

#### ProxyConfig（单例运行配置）
```
GET  /proxy/config  # 获取代理运行配置（单例）
PUT  /proxy/config  # 更新代理运行配置（单例）
POST /proxy/start   # 启动代理（以 FRouter 为中心）
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

无。该项目不承诺 API 向后兼容；API/状态 schema 可能发生破坏性变更。

---

## 文件结构（相关）

```
backend/
├── api/
│   ├── router.go
│   └── proxy.go
├── domain/
│   └── entities.go
├── repository/
│   ├── interfaces.go
│   ├── errors.go
│   ├── events/
│   └── memory/
│       └── node_repo.go
├── service/
│   ├── adapters/
│   ├── component/
│   ├── config/
│   ├── facade.go
│   ├── frouter/
│   ├── nodes/
│   ├── geo/
│   ├── node/
│   ├── nodegroup/
│   ├── proxy/
│   └── shared/
└── persist/
    ├── persist.go
    ├── snapshot_v2.go
    └── migrator.go

docs/
└── SING_BOX_INTEGRATION.md
```

---

## 测试清单

### 功能测试

- [ ] SOCKS 模式 + Xray
- [ ] Mixed 模式 + sing-box
- [ ] TUN 模式 + sing-box
- [ ] Hysteria2 节点自动选择 sing-box
- [ ] 权限检查 API
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
- [ ] 前端 UI 界面（ProxyConfig 配置）
- [ ] 节点测速（TUN 模式下）
- [ ] 流量统计（按 FRouter）
- [ ] Clash 内核支持
- [ ] 自动更新内核二进制

---

## 贡献者

感谢以下开源项目：
- [Xray-core](https://github.com/XTLS/Xray-core)
- [sing-box](https://github.com/SagerNet/sing-box)
- [v2ray-rules-dat](https://github.com/Loyalsoldier/v2ray-rules-dat)

---

**更新时间**: 2025-12-25
**版本**: v2.1.0 (arch v2 + sing-box integration)
