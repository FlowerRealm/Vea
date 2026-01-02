# FRouter 路由与端到端测速重构计划（不做兼容 / 失败即失败）

> 状态：该计划已在 arch-v2 落地。文档保留为历史记录；实现细节以当前代码为准。

## 1. 问题（现状就是错的）

- 图编辑器能写 `routeRule`，但后端编译基本不使用：链路仍按“线性路径 + 最高优先级第一条边”走。
- Xray/sing-box 路由规则是硬编码（ads/cn/direct/default），不是 FRouter 规则。
- FRouter 测速/延迟当前是“选一个节点开临时 SOCKS 去测”，不是“按规则选出口 + 按 detour 链走完整路由”。
- “全局分流/TrafficProfile/TrafficRule”仍存在并参与 fallback，直接违背“FRouter 是唯一路由逻辑”。
- 延迟错误信息在内存仓储层被丢弃，排障看不见原因。

结论：要么把语义收敛成一个可编译、可验证的模型，要么永远在测/跑两套不同的逻辑。

---

## 2. 目标（必须满足）

1) **FRouter = 唯一的路由逻辑单元**
- 规则选择出口：`local -> {node|direct|block|slot}`（按 domains/ips 匹配，priority 越大越优先）
- detour 串联链路：`{node|slot} -> {node|slot}`（只表达上游关系，不写规则）

2) **端到端测速/延迟必须走同一套路由**
- 本机 →（按规则选出口/动作）→（按 detour 链建链）→ 默认 target
- `block` 必须**立即失败**（reject），不能“黑洞超时”。

3) **允许未绑定 slot**
- slot 未绑定时表现为“穿透”：对应边在编译时被跳过（不参与路由/链路）。

4) **不做兼容**
- 旧命名/旧模型/旧 API 不保留；编译/保存阶段发现问题直接报错。

---

## 3. 非目标（明确不做）

- 不支持在 detour 边（`A->B`）上写规则（你已确认不需要）。
- 不做“每一跳动态匹配再选下一跳”的路由语言（那是把自己送进复杂度地狱）。
- 不引入“全局分流”概念（TrafficProfile/TrafficRule 这套要被删除）。

---

## 4. 核心定义（数据模型与语义）

### 4.1 节点类型

- `local`：虚拟入口（唯一入口）
- `node`：真实代理节点（来自全局 Nodes；FRouter 仅在图里引用 NodeID）
- `direct`：动作节点（直连）
- `block`：动作节点（立即失败 / reject）
- `slot-*`：占位节点；未绑定 => 穿透（跳过对应边）

### 4.2 边的两类语义（必须互斥）

#### A) 选择边（Route Edge，可写规则）

`local -> X`，其中 `X ∈ {node|direct|block|slot}`

- `ruleType="route"` + `routeRule`：条件匹配边
- `ruleType=""`：无条件边（默认/兜底）
- `priority`：数值越大越优先
- **必须且只能有 1 条兜底边**

#### B) detour 边（Detour Edge，不写规则）

`A -> B`，其中 `A,B ∈ {node|slot}`

- 含义：**A 的上游是 B**（A 通过 B 建链）
- 约束：
  - 每个 `A` 最多 1 条 detour 上游
  - 禁止环
  - 不允许指向 `local/direct/block`

### 4.3 “匹配所有 => priority 自动归零”（新增规则）

只对“选择边（local->X）”生效：

判定为“匹配所有/兜底”的条件：
- `ruleType==""`（无条件边），或
- `ruleType=="route"` 且 `routeRule` 所有字段都为空（domains/ips/ports/protocols/processNames 都空）且 `invert=false`

行为：
- 自动把该边 `priority` 设为 **0**
- 同时强制全图最多只有 1 条“匹配所有/兜底边”；第二条直接拒绝保存（不是 warning）。

> 不去“猜测” `0.0.0.0/0`、`regexp:.*` 等是否是全匹配，避免不可预测。

---

## 5. 编译器（engine-agnostic 的唯一真相）

做一件事：把图编译成一个确定的中间表示（IR），并在编译阶段把垃圾拦住。

### 5.1 编译输入

- `Nodes`（全局节点集合；FRouter 仅在图里引用 NodeID）
- `FRouter.ChainProxy.Edges` + `Slots`

### 5.2 编译输出（IR 形态建议）

- `RouteRules[]`：按 priority 降序的规则列表（每条包含匹配条件 + 输出动作：node/direct/block）
  - 兜底边 priority 必为 0 且位于最后
- `DetourUpstreamMap[nodeId]=upstreamNodeId`：detour 映射（slot 解析后）
- `Outbounds` 的“动作 tag”映射：
  - `direct` => 引擎 direct outbound
  - `block` => 引擎 reject outbound
  - `node` => 引擎 node outbound（可带 detour chaining）

### 5.3 校验（编译失败即保存失败）

- 选择边：
  - `local` 出边为空 => error
  - “兜底边”数量 != 1 => error
  - detour 边写了 `ruleType/routeRule/condition` => error
  - 选择边的 `to` 不是 `{node|direct|block|slot}` => error
- detour 边：
  - `from/to` 不是 `{node|slot}` => error
  - 单节点多个上游 => error
  - 环 => error
- slot：
  - 未绑定 slot 的边编译时跳过（允许），但要能解释/提示（warning 级别即可）。
- 规则字段：
  - 第一阶段只需要 domains/ips（UI 目前只暴露这两项）；其余字段若出现建议先拒绝并报清晰错误（后续再扩展）。

---

## 6. 引擎生成（Xray / sing-box）

原则：adapters 只做“把 IR 翻译成配置”，不再做业务决策。

### 6.1 Xray

- 每个 node => 一个 outboundTag
- detour：`A` outbound 上设置 `proxySettings.tag = BTag`
- routing.rules：
  - 按 `RouteRules` 顺序生成 `field` 规则（domain/ip 列表保持 `geosite:/geoip:/keyword:/regexp:` 等语法）
  - 兜底规则最后：`network=tcp,udp` => defaultTag
- `block`：用 Xray 的 reject 响应（立即失败）

### 6.2 sing-box

- 每个 node => 一个 outbound tag
- detour：`A` outbound 上设置 `detour = BTag`
- route.rules：
  - `geosite:` / `geoip:` => rule_set
  - `keyword:` / `regexp:` / `domain:` 等 => 翻译到 sing-box 对应字段（需要写一个统一 parser）
  - 兜底用 `final = defaultTag`
- `block`：reject 语义（立即失败）

---

## 7. Proxy 运行与“当前 FRouter”

- `ProxyConfig.frouterId` 变为必填；为空直接报错。
- 删除所有 fallback：
  - 删除 “全局 chain-proxy” fallback（不允许测/跑用另一套图）
  - 删除 TrafficProfile.DefaultFRouterId fallback（TrafficProfile 整套最终会删除）

“切换”的概念：
- 只有当存在多个 FRouter 时才叫切换；否则就是“编辑当前 FRouter”。

---

## 8. 测速/延迟（端到端、复用同一份编译结果）

### 8.1 API 约束

- `/frouters/:id/ping`、`/frouters/:id/speedtest` 保持无参（默认 target）
- 默认 target 继续用当前候选列表（后续可再做可配置），但实现必须走同一 IR/同一引擎配置。

### 8.2 实现要点

- 临时启动本地入站（SOCKS/HTTP 任一，推荐 SOCKS），用代理发起连接/下载。
- 修复 SOCKS5 CONNECT 的 ATYP：如果 target 是 IP，必须用 IPv4/IPv6 ATYP，避免“永远走域名类型”导致 IP 规则测不准。
- `block` 命中时必须快速失败，并把错误写入 `LastLatencyError/LastSpeedError`。

---

## 9. UI/编辑器

- 图上显示：
  - `local`、`direct`、`block`、slot、nodes
  - 不再用 `destination` 这种误导节点
- 规则编辑只允许在 `local->*` 边上出现；detour 边不显示规则面板。
- “匹配所有 => priority=0”：
  - UI 保存时先归一化一次（即时反馈）
  - 后端再归一化一次（防止其他客户端写垃圾）

主页：
- 保留“打开主页自动测当前 FRouter”，但后端实现改正确后它才有意义。

---

## 10. 数据字段与错误可见性（必须补齐）

- Node/FRouter 增加 `LastLatencyError`，并确保仓储层不会再丢掉 latencyErr。
- 所有编译错误必须返回清晰信息（哪条边、什么问题、怎么改）。

---

## 11. 分阶段交付（建议顺序）

1) **模型收敛**：删 TargetNode 概念；定义两类边；补齐错误字段（含 latency error）。
2) **编译器落地**：实现 slot 解析、route/detour 分类、严格校验、priority 归一化（含“匹配所有 => 0”）。
3) **引擎生成替换**：Xray/sing-box routing 由 IR 生成，移除硬编码分流。
4) **Proxy 运行收敛**：只认 profile.frouterId，移除所有 fallback。
5) **测速/延迟重做**：复用 IR/配置，修复 SOCKS ATYP，block=reject 可验证。
6) **删除全局分流**：移除 TrafficProfile/TrafficRule 与相关 API/SDK/UI/OpenAPI。
7) **UI 改造**：编辑器只允许合法边/规则；兜底优先级自动归零；验证提示清晰。
8) **测试**：nodegroup 编译器单测、adapters 配置生成单测（至少覆盖：规则排序、default、detour、block reject、slot 穿透）。

---

## 12. 验收标准（做完就能一眼判定对错）

- 同一 FRouter：
  - 加一条 `local->block` + `domain:example.com`，测速/延迟应立即失败，UI 能看到明确错误原因。
  - 加一条“兜底边”（无条件或空规则），其 priority 自动归零，且只能存在一条。
  - 配置 detour `A->B`，再让规则命中 `local->A`，实际链路必须通过 B（两引擎一致）。
- 仓库中不再出现 `/traffic/*`、TrafficProfile/TrafficRule、全局 chain-proxy fallback、targetNodeId 这些概念。
