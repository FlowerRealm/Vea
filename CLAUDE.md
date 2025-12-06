# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

Vea 是一个基于 Electron + Go 的桌面代理管理器，支持 Xray 和 sing-box 双内核。

**技术栈**:
- **前端**: Electron 28+ (Node.js 18+)
- **后端**: Go 1.22+ (Gin framework)
- **通信**: REST API (localhost:8080)
- **数据持久化**: JSON 文件 (`data/state.json`)

---

## 开发命令

### 基础命令

```bash
# 一键启动开发模式（编译 Go 后端 + 启动 Electron）
make dev

# 仅编译 Go 后端到 dist/vea
make build-backend

# 打包生产版本的 Electron 应用
make build

# 清理所有构建产物
make clean

# 安装 Electron 依赖
make deps
```

### Go 后端命令

```bash
# 编译后端（手动指定平台）
GOOS=linux GOARCH=amd64 make build-backend

# 运行后端服务（带开发日志）
./dist/vea --dev --addr :8080 --state data/state.json

# TUN 模式权限配置（仅 Linux）
sudo ./vea setup-tun
```

### Electron 前端命令

```bash
cd frontend

# 启动开发模式
npm run dev

# 打包应用（当前平台）
npm run build

# 打包指定平台
npm run build:linux
npm run build:mac
npm run build:win
```

### 测试命令

```bash
# 运行 Go 单元测试
go test -v ./backend/...

# 运行集成测试
go test -v -tags=integration ./backend/service/

# 测试特定包
go test -v ./backend/service -run TestXrayManager
```

---

## 架构设计

### 三层架构

```
前端 (Electron)
    ↓ HTTP API (localhost:8080)
路由层 (backend/api)
    ↓ 业务接口调用
服务层 (backend/service)
    ↓ 数据存取
存储层 (backend/store)
```

### 核心模块

#### 1. 领域模型 (`backend/domain/entities.go`)

定义所有核心实体：
- **Node**: 代理节点（支持 VLESS/VMess/Trojan/Shadowsocks/Hysteria2/TUIC）
- **Config**: 订阅配置
- **GeoResource**: GeoIP/GeoSite 资源
- **Component**: 核心组件（Xray/sing-box）
- **ProxyProfile**: 代理配置文件（入站模式 + 引擎选择）
- **RoutingRule**: 流量路由规则

#### 2. 服务层 (`backend/service/service.go`)

核心业务逻辑：
- 节点管理：CRUD、导入分享链接、测速、流量统计
- 配置管理：订阅导入、自动刷新、节点提取
- Geo 资源：定时同步、文件下载
- 组件管理：Xray/sing-box 安装、版本管理
- 代理控制：启动/停止、引擎选择、TUN 模式

#### 3. 适配器模式 (`backend/service/adapters/`)

核心引擎抽象：
- **CoreAdapter 接口**: 统一的核心引擎接口
- **XrayAdapter**: Xray-core 适配器（SOCKS/HTTP/Mixed）
- **SingBoxAdapter**: sing-box 适配器（TUN + 所有协议）

引擎选择逻辑 (`backend/service/engine_selector.go`):
1. TUN 模式 → 强制 sing-box
2. Hysteria2/TUIC 节点 → 强制 sing-box
3. 用户偏好（如果兼容）
4. 默认 Xray → 回退 sing-box

#### 4. 存储层 (`backend/store/memory.go`)

内存存储 + 快照持久化：
- 使用 `sync.RWMutex` 保护并发访问
- 写操作后触发 `AfterWrite` 回调（用于快照）
- 状态序列化到 `data/state.json`

#### 5. 后台任务 (`backend/tasks/`)

定时任务调度：
- **ConfigSync**: 订阅配置自动刷新（1分钟）
- **GeoSync**: GeoIP/GeoSite 同步（12小时）
- **ComponentUpdate**: 组件版本检查（6小时）
- **NodeProbe**: 节点延迟探测（按需）

---

## 关键特性

### 双内核支持

**Xray-core**:
- 协议：VLESS、VMess、Trojan、Shadowsocks
- 入站：SOCKS、HTTP、Mixed（需两个端口）

**sing-box**:
- 协议：所有 Xray 协议 + Hysteria2、TUIC
- 入站：SOCKS、HTTP、Mixed（原生单端口）、TUN
- 特性：系统级透明代理、更低内存占用

### TUN 模式

**权限要求**:
- **Linux**: CAP_NET_ADMIN capability（自动配置：`sudo ./vea setup-tun`）
- **Windows**: 管理员权限
- **macOS**: sudo 运行

**配置示例** (`TUNConfiguration`):
```json
{
  "interfaceName": "tun0",
  "mtu": 9000,
  "address": ["172.19.0.1/30"],
  "autoRoute": true,
  "strictRoute": true,
  "stack": "mixed",
  "dnsHijack": true
}
```

**权限检查 API**: `GET /tun/check`

### ProxyProfile 系统

代理配置与节点解耦，一个 Profile 包含：
- 入站模式（SOCKS/HTTP/Mixed/TUN）
- 引擎选择（Xray/sing-box/auto）
- 默认节点
- TUN 配置（如果适用）

**API 示例**:
```bash
# 创建 TUN Profile
POST /proxy-profiles
{
  "name": "系统级代理",
  "inboundMode": "tun",
  "preferredEngine": "singbox",
  "tunSettings": {...},
  "defaultNode": "<node-id>"
}

# 启动代理
POST /proxy-profiles/{id}/start

# 查看状态
GET /proxy/status

# 停止代理
POST /proxy/stop
```

---

## 代码风格约定

### Go 代码规范

1. **错误处理**: 优先使用哨兵错误（`var ErrXXX = errors.New(...)`）
2. **日志**: 使用标准库 `log`，生产模式只输出错误
3. **锁使用**: `store.MemoryStore` 已内置锁，service 层无需额外加锁
4. **常量**: 使用 `const` 块定义相关常量组
5. **接口**: 面向接口编程（如 `CoreAdapter`）

### 关键模式

#### 适配器注册

```go
// backend/service/adapters/adapter.go
var registry = map[CoreEngineKind]CoreAdapter{
    EngineXray:    &XrayAdapter{},
    EngineSingBox: &SingBoxAdapter{},
}

func GetAdapter(kind CoreEngineKind) CoreAdapter {
    return registry[kind]
}
```

#### 引擎选择

```go
// backend/service/engine_selector.go
func SelectEngine(
    profile ProxyProfile,
    node Node,
    components map[string]Component,
) CoreEngineKind {
    // 1. TUN 强制 sing-box
    // 2. 协议要求检查
    // 3. 用户偏好
    // 4. 默认 Xray
    // 5. 回退 sing-box
}
```

#### 服务依赖注入

```go
// main.go
memory := store.NewMemoryStore()
serviceInstance := service.NewService(memory)
router := api.NewRouter(serviceInstance)
```

---

## 文件路径约定

```
artifacts/
├── core/
│   ├── xray/xray-core         # Xray 可执行文件
│   └── singbox/sing-box       # sing-box 可执行文件
└── geo/
    ├── geoip.dat
    └── geosite.dat

data/
└── state.json                  # 状态持久化

frontend/
├── main.js                     # Electron 主进程（启动 Go 服务）
├── theme/
│   ├── dark.html              # 深色主题
│   └── light.html             # 浅色主题
└── sdk/                        # JavaScript SDK

backend/
├── api/                        # HTTP 路由和处理器
├── domain/                     # 领域模型（entities.go）
├── service/                    # 业务逻辑
│   ├── adapters/              # 核心引擎适配器
│   ├── service.go             # 主服务
│   ├── xray_manager.go        # Xray 管理
│   ├── proxy_profile.go       # ProxyProfile 管理
│   ├── engine_selector.go     # 引擎选择
│   ├── tun_setup.go           # TUN 权限配置
│   └── privilege_*.go         # 平台特定权限检查
├── store/                      # 数据存储（内存 + 持久化）
├── persist/                    # 快照管理
└── tasks/                      # 后台任务
```

---

## 常见开发任务

### 添加新的代理协议

1. 在 `backend/domain/entities.go` 添加协议常量
2. 在 `backend/service/adapters/` 实现协议支持
3. 更新 `SupportedProtocols()` 方法
4. 更新引擎选择逻辑（如有特殊要求）

### 添加新的 API 端点

1. 在 `backend/api/router.go` 的 `register()` 添加路由
2. 实现处理器方法（遵循现有命名：`listXxx`, `createXxx`, `updateXxx`, `deleteXxx`）
3. 在 `backend/service/service.go` 添加业务逻辑（如需要）

### 修改核心配置生成

1. 编辑 `backend/service/adapters/xray.go` 或 `singbox.go`
2. 修改 `BuildConfig()` 方法
3. 更新 `xray_templates.go` 或 `singbox_ruleset.go`（如有模板）

### 调试代理连接

```bash
# 启动开发模式查看完整日志
./dist/vea --dev

# 检查代理状态
curl http://localhost:8080/proxy/status

# 查看生成的配置（会在日志中打印）
# 或直接读取临时配置文件（service.go 中的 tmpConfig.json）
```

---

## 测试注意事项

- 项目要求：**编译通过即可，无需完整测试**（测试由维护者完成）
- 测试文件：仅用于开发参考，无需主动运行
- 编译后生成的测试二进制文件应删除

---

## 平台差异处理

### 权限管理

- **Linux**: `privilege_linux.go` - CAP_NET_ADMIN + vea-tun 用户
- **Windows**: `privilege_windows.go` - 管理员检查
- **macOS**: `privilege_darwin.go` - sudo 检查

### 系统代理配置

`backend/service/system_proxy.go` 实现了平台特定的系统代理设置：
- Linux: gsettings / KDE 配置
- Windows: 注册表
- macOS: networksetup

---

## 依赖管理

### Go 依赖

```bash
# 添加依赖
go get github.com/xxx/xxx

# 更新依赖
go get -u ./...

# 整理依赖
go mod tidy
```

### Electron 依赖

```bash
cd frontend
npm install --save <package>
npm install --save-dev <dev-package>
```

---

## 调试技巧

### Go 后端调试

```bash
# 使用 dlv 调试
dlv debug . -- --dev --addr :8080

# 或直接运行并查看日志
./dist/vea --dev 2>&1 | tee debug.log
```

### Electron 前端调试

运行 `make dev` 后：
1. 按 `F12` 打开 Chrome DevTools
2. 查看 Console、Network、Elements 面板
3. Go 后端日志同时输出到终端

### API 测试

```bash
# 健康检查
curl http://localhost:8080/health

# 获取完整状态快照
curl http://localhost:8080/snapshot | jq

# 列出所有节点
curl http://localhost:8080/nodes | jq

# 创建节点（导入分享链接）
curl -X POST http://localhost:8080/nodes \
  -H "Content-Type: application/json" \
  -d '{"shareLink": "vless://..."}'
```

---

## 参考文档

- [sing-box 集成说明](./docs/SING_BOX_INTEGRATION.md)
- [更新日志](./docs/CHANGES.md)
- [Electron 客户端](./frontend/README.md)
- [SDK 文档](./frontend/sdk/README.md)
- [API 文档](./docs/api/README.md)

---

## 常见问题

**Q: 为什么 Electron 需要 `--no-sandbox`？**
A: 部分 Linux 环境的沙箱与 TUN 模式权限冲突，已在 `frontend/package.json` 中配置。

**Q: 如何切换内核引擎？**
A: 通过 ProxyProfile API 的 `preferredEngine` 字段，或让系统自动选择（`auto`）。

**Q: TUN 模式在 Linux 下权限不足？**
A: 运行 `sudo ./vea setup-tun` 自动配置 capabilities 和用户。

**Q: 如何验证编译成功？**
A: 运行 `make build-backend && ls -lh dist/vea`，应看到可执行文件。
