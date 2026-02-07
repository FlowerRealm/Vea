# 变更提案: feat-node-groups

## 元信息
```yaml
类型: 新功能
方案类型: implementation
优先级: P1
状态: 待执行
创建: 2026-02-01
```

---

## 1. 需求

### 背景
目前 Vea 的一等资源为 `FRouter` 与 `Node`：`FRouter.ChainProxy` 的图只能引用 `NodeID`，导致：
- 当节点数量较多时，图编辑与维护成本高（需要逐个选择节点）。
- 无法表达“这条边用一组节点，按最低延迟/最快速度/轮询/失败切换来选”的意图。

需要引入“节点组（NodeGroup）”作为全局资源，用户可以把多个节点聚合为一个可引用目标，并绑定内部选择策略。

### 目标
- 新增全局资源 `NodeGroup`（增删改查）。
- `ChainProxySettings` 中原本引用 `NodeID` 的位置允许引用 `NodeGroupID`（包括 `edges.from/edges.to/edges.via[*]`）。
- 代理启动/测速/引擎选择/图校验等所有编译链路：在编译前将 `NodeGroupID` 解析为具体 `NodeID`，保持后续编译与引擎适配逻辑不变。
- 节点组策略（绑定在 NodeGroup 上）：
  - 延迟最低（lowest-latency）
  - 速度最快（fastest-speed）
  - 轮询（round-robin）
  - 失败切换（failover；“连不上”都算失败，基于现有测试结果与错误字段判定）

### 约束条件
```yaml
时间约束: 无
性能约束: 解析应为纯内存计算；默认不在“编译/校验”链路做网络探测
兼容性约束: state schema 需要升级并提供从 2.1.0 的迁移
业务约束: 策略数据来源沿用现有测速/测延迟逻辑（Node 的 lastLatency/lastSpeed 字段）
```

### 验收标准
- [ ] 支持 `NodeGroup` 全局 CRUD，并出现在 `GET /snapshot` 中
- [ ] `FRouter.ChainProxy` 引用 `NodeGroupID` 时：
  - [ ] API 侧保存/校验通过（不会被当成“node not found”）
  - [ ] 启动代理成功，生成配置仅包含具体 `Node`（不出现 NodeGroup）
- [ ] 策略行为符合定义：
  - [ ] 延迟最低：选择 `LastLatencyMS` 最小且 `LastLatencyError` 为空的节点（无可用测量时回退到组内第一个存在的节点）
  - [ ] 速度最快：选择 `LastSpeedMbps` 最大且 `LastSpeedError` 为空的节点（无可用测量时回退）
  - [ ] 轮询：按组内顺序轮转选择（可跨重启保留游标，或至少在同一运行期内稳定轮询）
  - [ ] 失败切换：当前节点“不可用”时按顺序切到下一个可用节点
- [ ] `go test ./...` 通过

---

## 2. 方案

### 技术方案
采用“应用层解析 NodeGroup → Node”的方式（方案 1）：
1) 新增领域模型 `NodeGroup`（全局资源）与持久化/仓储层。
2) 新增 `NodeGroupResolver`：输入 `FRouter + Nodes + NodeGroups`，输出“只包含 NodeID 的等价 FRouter 副本”。
3) 在所有 `CompileFRouter` 调用之前接入 resolver（代理启动/测速/引擎选择/API 校验等），后续编译与引擎适配器保持不变。

策略定义（解析规则）：
- lowest-latency: 在组内可用节点中选最小 `LastLatencyMS>0` 且 `LastLatencyError==""`；否则按 `NodeIDs` 顺序选第一个存在的节点。
- fastest-speed: 在组内可用节点中选最大 `LastSpeedMbps>0` 且 `LastSpeedError==""`；否则回退同上。
- round-robin: 维护组内游标（cursor），每次解析选择下一个存在的节点并更新游标。
- failover: 维护组内游标；优先选择当前游标指向的节点（若可用），否则向后扫描第一个可用节点并更新游标。

可用性判定（failover 使用）：
- 节点存在（ID 在当前 `Nodes` 列表中）
- 且（优先）`LastLatencyError=="" && LastLatencyMS>0`（沿用现有“连不上会写 error”的逻辑）

### 影响范围
```yaml
涉及模块:
  - backend/domain: 新增 NodeGroup 数据结构，扩展 ServiceState
  - backend/repository: 新增 NodeGroupRepository + 内存实现 + Store 快照/恢复
  - backend/persist: schemaVersion 升级与迁移
  - backend/service/nodegroup: 增加 NodeGroupResolver（编译前解析）
  - backend/service/proxy: 启动/引擎选择/测速链路接入 resolver
  - backend/api: NodeGroup API + FRouter 图校验链路接入 resolver
  - docs/api: OpenAPI 增补 node-groups
  - frontend/sdk + frontend/theme: SDK 方法与 UI 支持（节点组管理 + 图编辑选择）
预计变更文件: 20-35
```

### 风险评估
| 风险 | 等级 | 应对 |
|------|------|------|
| 失败切换不等价于“内核运行期自动切换” | 中 | 在文档与 UI 文案中明确：本次为“编译前解析”；运行期切换需要后续引擎层方案 |
| latency/speed 运行态数据未持久化导致重启后选择退化 | 中 | 明确依赖“现有测试逻辑结果”；必要时引导用户先测量；后续可评估持久化或自动探测 |
| NodeGroup 引用包含已被订阅更新删除的节点 | 低 | resolver 跳过不存在节点；组内无可用节点时返回明确错误 |
| NodeID 与 NodeGroupID 极小概率冲突 | 低 | 解析优先匹配 Node；文档注明 ID 命名空间与冲突处理 |
| schemaVersion 升级导致旧 state 无法加载 | 低 | 提供 2.1.0 → 新版本的迁移逻辑与测试覆盖 |

---

## 3. 技术设计（可选）

> 涉及架构变更、API设计、数据模型变更时填写

### 架构设计
```mermaid
flowchart TD
    UI[Frontend UI] --> SDK[frontend/sdk]
    SDK --> API[backend/api]
    API --> Facade[backend/service/facade]
    Facade --> NodeGroupsSvc[backend/service/nodegroups]
    Facade --> ProxySvc[backend/service/proxy]
    ProxySvc --> Resolver[NodeGroupResolver]
    Resolver --> Compiler[nodegroup.CompileFRouter]
    Compiler --> Adapters[adapters (sing-box/clash)]
```

### API设计
#### GET /node-groups
- **响应**: `{ "nodeGroups": NodeGroup[] }`

#### POST /node-groups
- **请求**: `{ "name": string, "nodeIds": string[], "strategy": NodeGroupStrategy, "tags"?: string[] }`
- **响应**: `NodeGroup`

#### PUT /node-groups/:id
- **请求**: 同 POST（全量更新）
- **响应**: `NodeGroup`

#### DELETE /node-groups/:id
- **响应**: 204

### 数据模型
| 字段 | 类型 | 说明 |
|------|------|------|
| id | string | NodeGroupID |
| name | string | 显示名称 |
| nodeIds | string[] | 成员节点 ID 列表 |
| strategy | string | lowest-latency / fastest-speed / round-robin / failover |
| tags | string[] | 标签（可选） |
| cursor | int | 轮询/失败切换游标（实现细节，可选字段） |
| createdAt | time | 创建时间 |
| updatedAt | time | 更新时间 |

---

## 4. 核心场景

> 执行完成后同步到对应模块文档

### 场景: 创建并使用节点组
**模块**: backend/api, backend/service/nodegroups
**条件**: 用户已存在若干 `Node`
**行为**:
1) 创建 `NodeGroup(strategy=lowest-latency, nodeIds=[...])`
2) 在 `FRouter.ChainProxy` 的 `local -> <groupId>` 边引用该组
3) 启动代理
**结果**: 代理启动时 resolver 将 `<groupId>` 解析为具体 `<nodeId>`，并生成仅包含 node 的内核配置

### 场景: 轮询策略
**模块**: NodeGroupResolver
**条件**: `NodeGroup(strategy=round-robin)` 且组内存在 3 个节点
**行为**: 多次启动/编译解析同一 `FRouter`
**结果**: 每次解析按顺序选择不同节点（按 cursor 推进）

### 场景: 失败切换
**模块**: NodeGroupResolver
**条件**: `NodeGroup(strategy=failover)`；当前 cursor 指向节点最近一次探测有错误（LastLatencyError 非空）
**行为**: 解析该组
**结果**: 跳过不可用节点，选择下一个可用节点并更新 cursor

---

## 5. 技术决策

> 本方案涉及的技术决策，归档后成为决策的唯一完整记录

### feat-node-groups#D001: 采用“应用层解析 NodeGroup → Node”的落地方式
**日期**: 2026-02-01
**状态**: ✅采纳
**背景**: 需要在不重写两套引擎适配器的前提下，为图编辑提供“可引用的一组节点 + 策略选择”的能力。
**选项分析**:
| 选项 | 优点 | 缺点 |
|------|------|------|
| A: 应用层解析（本方案） | 引擎无关、复用现有编译/适配器、改动集中可控 | 运行期无法自动切换，需要重启/重新编译才能生效 |
| B: 引擎层 NodeGroup（Clash proxy-groups / sing-box outbounds） | 更接近运行期自动切换 | 同时改两套适配器且“最快速度”难复用现有逻辑，风险与复杂度高 |
**决策**: 选择方案 A
**理由**: 以最小代价快速引入 NodeGroup 作为产品能力；将“运行期自动切换”留作后续迭代（可在不破坏 API 的前提下升级实现）。
**影响**: backend/domain、repository、persist、api、proxy、nodegroup（编译链路）、frontend/sdk、frontend/theme
