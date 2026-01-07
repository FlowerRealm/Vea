# Vea 架构重构文档 (v2.1)

> 版本: 2.1.0
> 更新日期: 2025-12-25

---

## 概述

本次重构将原有的单体服务架构改造为分层架构，实现了数据存储与业务逻辑的解耦，提升了代码可维护性和可测试性。

### 设计目标

1. **关注点分离**: 数据访问、业务逻辑、API 路由各司其职
2. **事件驱动持久化**: 写操作触发异步快照，避免阻塞业务
3. **接口抽象**: 面向接口编程，便于单元测试和未来扩展
4. **破坏性重命名**: 以 `FRouter` / `/frouters` / `frouterId` 为唯一对外口径，不保留旧命名/旧字段

### 术语与对外模型（重要口径）

1. **FRouter 优先**：对外一等概念是 `FRouter` / `/frouters`，其语义为“转发路由定义（FRouter）”。
2. **节点独立**：`Node` 为独立资源（`/nodes`）；通常由配置/订阅同步生成；`FRouter` 仅通过链式代理图引用 `NodeID`。
3. **不做兼容**：不保留旧字段（如 `defaultNodeId` / `nodeId`）与旧 API（如 `/routes` / `routeId`）。

---

## 架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                         API Layer                                │
│                      api/router.go                               │
│                           │                                      │
│                           ▼                                      │
│                    service.Facade                                │
│                    (API 聚合层)                                   │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                       Service Layer                              │
│  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐       │
│  │FRouterSvc │ │ NodeSvc   │ │ ConfigSvc │ │ ProxySvc  │       │
│  │FRouter管理 │ │ 节点管理   │ │ 订阅管理   │ │ 代理控制   │       │
│  └───────────┘ └───────────┘ └───────────┘ └───────────┘       │
│  ┌───────────────┐ ┌───────────┐                                │
│  │ ComponentSvc  │ │ GeoSvc    │                                │
│  │ 组件安装       │ │ Geo资源   │                                │
│  └───────────────┘ └───────────┘                                │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Repository Layer                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐                          │
│  │FRouterRepo││ NodeRepo ││ConfigRepo│                          │
│  └──────────┘ └──────────┘ └──────────┘                          │
│  ┌──────────┐ ┌──────────────────┐ ┌────────────┐               │
│  │ GeoRepo  │ │ ComponentRepo    │ │SettingsRepo│               │
│  └──────────┘ └──────────────────┘ └────────────┘               │
└─────────────────────────────────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                      Storage Layer                               │
│                                                                  │
│   memory.Store ◄──── events.Bus ────► SnapshotterV2             │
│   (内存存储)          (事件总线)         (防抖持久化)              │
│                                              │                   │
│                                              ▼                   │
│                                        state.json                │
└─────────────────────────────────────────────────────────────────┘
```

---

## 目录结构

```
backend/
├── api/                          # API 路由层
│   ├── router.go                 # 主路由 (使用 Facade)
│   └── proxy.go                  # Proxy API（start/status/config/stop、tun/check/setup）
│
├── domain/                       # 领域模型
│   └── entities.go               # 实体定义 (FRouter, Node, Config, etc.)
│
├── repository/                   # 仓储层 (NEW)
│   ├── interfaces.go             # 仓储接口定义
│   ├── errors.go                 # 统一错误定义
│   ├── events/                   # 事件系统
│   │   ├── events.go             # 事件类型定义
│   │   └── bus.go                # 事件总线实现
│   └── memory/                   # 内存存储实现
│       ├── store.go              # 核心存储引擎
│       ├── frouter_repo.go       # FRouterRepository 实现（FRouter）
│       ├── node_repo.go          # NodeRepository 实现（节点独立实体）
│       ├── config_repo.go        # ConfigRepository 实现
│       ├── geo_repo.go           # GeoRepository 实现
│       ├── component_repo.go     # ComponentRepository 实现
│       └── settings_repo.go      # SettingsRepository 实现
│
├── service/                      # 服务层
│   ├── facade.go                 # 门面服务 (API 聚合层)
│   ├── shared/                   # 共享工具（artifacts root / 下载 / system proxy / TUN 等）
│   ├── frouter/                  # FRouter 服务
│   │   └── service.go            # FRouter CRUD、测速/延迟队列
│   ├── nodes/                    # 节点服务
│   │   └── service.go            # 节点列表、测速/延迟队列
│   ├── node/                     # 节点解析
│   │   └── parser.go             # 分享链接解析
│   ├── nodegroup/                # 运行计划编译（NodeGroup）
│   │   ├── plan_compile.go       # 计划编译入口（CompileProxyPlan / CompileMeasurementPlan）
│   │   ├── frouter_compiler.go   # FRouter 图编译（CompileFRouter）
│   │   └── runtime_plan.go       # 运行计划结构
│   ├── config/                   # 配置服务 (NEW)
│   │   └── service.go            # 订阅同步
│   ├── proxy/                    # 代理服务 (NEW)
│   │   └── service.go            # 启停控制、引擎选择
│   ├── component/                # 组件服务 (NEW)
│   │   └── service.go            # 组件安装
│   ├── geo/                      # Geo服务 (NEW)
│   │   └── service.go            # GeoIP/GeoSite 管理
│   ├── adapters/                 # 核心引擎适配器
│   │   ├── adapter.go            # CoreAdapter 接口
│   │   ├── xray.go               # Xray 适配器
│   │   └── singbox.go            # sing-box 适配器
│
├── persist/                      # 持久化层
│   ├── snapshot_v2.go            # 快照读写 + 防抖保存
│   └── migrator.go               # schemaVersion 迁移/校验
│
```

---

## 核心组件详解

### 1. Repository Layer (仓储层)

仓储层抽象了数据访问逻辑，所有数据操作通过接口进行。

#### 接口定义 (`repository/interfaces.go`)

```go
// FRouterRepository FRouter 仓储接口（主要对外操作单元）
type FRouterRepository interface {
    Get(ctx context.Context, id string) (domain.FRouter, error)
    List(ctx context.Context) ([]domain.FRouter, error)
    Create(ctx context.Context, frouter domain.FRouter) (domain.FRouter, error)
    Update(ctx context.Context, id string, frouter domain.FRouter) (domain.FRouter, error)
    Delete(ctx context.Context, id string) error
    // ...
}

// Repositories 仓储容器接口
type Repositories interface {
    Node() NodeRepository
    FRouter() FRouterRepository
    Config() ConfigRepository
    Geo() GeoRepository
    Component() ComponentRepository
    Settings() SettingsRepository
}
```

#### 错误定义 (`repository/errors.go`)

```go
var (
    ErrNotFound          = errors.New("entity not found")
    ErrConfigNotFound    = errors.New("config not found")
    ErrGeoNotFound       = errors.New("geo resource not found")
    ErrComponentNotFound = errors.New("component not found")
)
```

### 2. Event System (事件系统)

事件系统解耦了数据变更和持久化，实现了异步快照。

#### 事件类型 (`repository/events/events.go`)

```go
const (
    // FRouter 事件
    EventFRouterCreated EventType = "frouter.created"
    EventFRouterUpdated EventType = "frouter.updated"
    EventFRouterDeleted EventType = "frouter.deleted"

    // 配置事件
    EventConfigCreated EventType = "config.created"
    // ...

    // 通配符（订阅所有事件）
    EventAll EventType = "*"
)
```

#### 事件总线 (`repository/events/bus.go`)

```go
// 订阅特定事件
bus.Subscribe(events.EventFRouterCreated, func(e events.Event) {
    frouterEvent := e.(events.FRouterEvent)
    log.Printf("FRouter created: %s", frouterEvent.FRouterID)
})

// 订阅所有事件（用于持久化）
bus.SubscribeAll(func(e events.Event) {
    snapshotter.Schedule()
})
```

### 3. Memory Store (内存存储)

内存存储是所有仓储的底层数据源，提供线程安全的数据访问。

#### 关键设计

```go
type Store struct {
    mu sync.RWMutex

    // 数据映射
    frouters   map[string]domain.FRouter
    configs    map[string]domain.Config
    // ...

    // 事件总线
    eventBus *events.Bus
}

// 事件在锁外发布，避免死锁
func (r *FRouterRepo) Create(ctx context.Context, frouter domain.FRouter) (domain.FRouter, error) {
    r.store.Lock()
    r.store.FRouters()[frouter.ID] = frouter
    r.store.Unlock()  // 先释放锁

    // 然后发布事件
    r.store.PublishEvent(events.FRouterEvent{
        EventType: events.EventFRouterCreated,
        FRouterID: frouter.ID,
        FRouter:   frouter,
    })
    return frouter, nil
}
```

### 4. Service Layer (服务层)

服务层包含业务逻辑，每个服务负责特定领域。

#### FRouterService (`service/frouter/service.go`)

```go
type Service struct {
    repo repository.FRouterRepository

    // 异步探测队列（FRouter 级别）
    latencyQueue chan string
    speedQueue   chan string
}

// 异步延迟探测
func (s *Service) ProbeLatencyAsync(id string) {
    select {
    case s.latencyQueue <- id:
    default:
        // 队列满，跳过
    }
}
```

> 说明：节点解析保留在 `service/node/parser.go`（分享链接解析）；节点列表为全局资源（`/nodes`），`FRouter` 通过图引用 `NodeID`。

#### ProxyService (`service/proxy/service.go`)

```go
type Service struct {
    frouters   repository.FRouterRepository
    components repository.ComponentRepository
    settings   repository.SettingsRepository

    adapters map[domain.CoreEngineKind]adapters.CoreAdapter

    mu      sync.Mutex
    mainCmd *exec.Cmd
}

// 启动代理
func (s *Service) Start(ctx context.Context, cfg domain.ProxyConfig) error {
    // 1. 读取/合并 ProxyConfig（单例运行配置）
    // 2. 获取 FRouter + Nodes 并编译 NodeGroup
    // 3. 选择引擎 (Xray/sing-box)
    // 4. 构建配置
    // 5. 启动进程
}
```

### 5. Facade (门面服务)

Facade 作为 API 聚合层，内部委托给各个服务。

```go
type Facade struct {
    frouter   *frouter.Service
    nodes     *nodes.Service
    config    *configsvc.Service
    proxy     *proxy.Service
    component *component.Service
    geo       *geo.Service
    repos     repository.Repositories
}

// UpdateFRouter 使用 UpdateFn 模式更新 FRouter
func (f *Facade) UpdateFRouter(id string, updateFn func(domain.FRouter) (domain.FRouter, error)) (domain.FRouter, error) {
    frouter, err := f.frouter.Get(context.Background(), id)
    if err != nil {
        return domain.FRouter{}, err
    }
    updated, err := updateFn(frouter)
    if err != nil {
        return domain.FRouter{}, err
    }
    return f.frouter.Update(context.Background(), id, updated)
}
```

### 6. Persistence (持久化)

#### 版本校验 (`persist/migrator.go`)

```go
func (m *Migrator) Migrate(data []byte) (domain.ServiceState, error) {
    // 先只解析 schemaVersion，避免直接丢字段导致不可逆数据丢失
    var meta struct {
        SchemaVersion string `json:"schemaVersion,omitempty"`
    }
    if err := json.Unmarshal(data, &meta); err != nil {
        return domain.ServiceState{}, err
    }

    // 兼容旧 state.json：历史版本未写入 schemaVersion
    if meta.SchemaVersion == "" {
        var state domain.ServiceState
        if err := json.Unmarshal(data, &state); err != nil {
            return domain.ServiceState{}, err
        }
        state.SchemaVersion = SchemaVersion
        return state, nil
    }

    switch meta.SchemaVersion {
    case SchemaVersion:
        var state domain.ServiceState
        if err := json.Unmarshal(data, &state); err != nil {
            return domain.ServiceState{}, err
        }
        return state, nil
    default:
        return domain.ServiceState{}, fmt.Errorf("unsupported schemaVersion %s (expected %s)", meta.SchemaVersion, SchemaVersion)
    }
}
```

#### 防抖快照 (`persist/snapshot_v2.go`)

```go
func (s *SnapshotterV2) Schedule() {
    s.mu.Lock()
    if s.pending {
        s.dirty = true
        s.mu.Unlock()
        return
    }
    s.pending = true
    s.dirty = false
    s.mu.Unlock()

    go func() {
        for {
            time.Sleep(s.debounce)  // 默认 200ms
            _ = s.save()

            s.mu.Lock()
            if s.dirty {
                s.dirty = false
                s.mu.Unlock()
                continue
            }
            s.pending = false
            s.mu.Unlock()
            return
        }
    }()
}
```

---

## 初始化流程

```go
// main.go

// 1. 创建事件总线
eventBus := events.NewBus()

// 2. 创建内存存储
memStore := memory.NewStore(eventBus)

// 3. 加载状态
if state, err := persist.LoadV2(statePath); err == nil {
    memStore.LoadState(state)
}

// 4. 创建仓储层
nodeRepo := memory.NewNodeRepo(memStore)
frouterRepo := memory.NewFRouterRepo(memStore) // FRouter 仓储
configRepo := memory.NewConfigRepo(memStore)
// ...

repos := repository.NewRepositories(memStore, nodeRepo, frouterRepo, configRepo, geoRepo, componentRepo, settingsRepo)

// 5. 创建服务层
nodeSvc := nodes.NewService(nodeRepo)
frouterSvc := frouter.NewService(frouterRepo, nodeRepo)
configSvc := configsvc.NewService(configRepo, nodeSvc)
proxySvc := proxy.NewService(frouterRepo, nodeRepo, componentRepo, settingsRepo)
// ...

// 6. 创建 Facade
facade := service.NewFacade(nodeSvc, frouterSvc, configSvc, proxySvc, componentSvc, geoSvc, repos)

// 7. 设置持久化（事件驱动）
snapshotter := persist.NewSnapshotterV2(statePath, memStore)
snapshotter.SubscribeEvents(eventBus)

// 8. 创建路由
router := api.NewRouter(facade)
```

---

## 数据流示例

### 创建 FRouter

```
1. POST /frouters
       │
       ▼
2. router.createFRouter()
       │
       ▼
3. nodegroup.CompileFRouter(frouter, nodes)  // 校验图语义
       │
       ▼
4. facade.CreateFRouter(frouter)
       │
       ▼
5. frouterSvc.Create()
       │
       ▼
6. frouterRepo.Create()
       │
       ├──► store.Lock()
       ├──► store.frouters[id] = frouter
       ├──► store.Unlock()
       │
       ▼
7. store.PublishEvent(FRouterCreated)
       │
       ▼
8. eventBus.Publish()
       │
       ▼
9. snapshotter.Schedule()  // 所有订阅者收到事件
       │
       ▼
10. [200ms debounce]
       │
       ▼
11. state.json 写入
```

---

## 旧代码清理状态

旧单体服务/旧存储/旧快照器已移除，当前只保留新架构实现。

---

## 测试

```bash
# 编译检查
go build ./...

# 运行后端
./dist/vea --dev --addr :19080

# 验证 API
curl http://localhost:19080/health
curl http://localhost:19080/snapshot | jq '.schemaVersion'
# 应返回 "2.1.0"
```

---

## 附录: 接口速查表

### FRouterRepository

| 方法 | 说明 |
|------|------|
| `Get(id)` | 获取单个 FRouter |
| `List()` | 列出所有 FRouter |
| `Create(frouter)` | 创建 FRouter |
| `Update(id, frouter)` | 更新 FRouter |
| `Delete(id)` | 删除 FRouter |
| `UpdateLatency(id, ms, err)` | 更新 FRouter 延迟 |
| `UpdateSpeed(id, mbps, err)` | 更新 FRouter 速度 |

### NodeRepository

| 方法 | 说明 |
|------|------|
| `Get(id)` | 获取单个 Node |
| `List()` | 列出所有 Node |
| `ListByConfigID(configID)` | 按配置ID列出 Node |
| `ReplaceNodesForConfig(configID, nodes)` | 同步订阅：替换配置下节点列表 |
| `UpdateLatency(id, ms, err)` | 更新 Node 延迟 |
| `UpdateSpeed(id, mbps, err)` | 更新 Node 速度 |

### SettingsRepository

| 方法 | 说明 |
|------|------|
| `GetSystemProxy()` | 获取系统代理设置 |
| `UpdateSystemProxy(settings)` | 更新系统代理 |
| `GetTUN()` | 获取 TUN 设置 |
| `UpdateTUN(settings)` | 更新 TUN 设置 |
| `GetFrontend()` | 获取前端设置 |
| `UpdateFrontend(settings)` | 更新前端设置 |

---

*文档版本: 2.1.0 | 生成时间: 2025-12-25*
