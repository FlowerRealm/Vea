# 技术设计: 重构订阅节点 ID 体系并修复 open issues（#41/#55/#57）

## 技术方案

### 核心技术
- Go（后端）
- Electron/原生 JS（主题页）
- 现有订阅同步/节点仓储/链式代理图结构（不新增新框架）

### 实现要点

#### 1) 引入可解释的订阅节点稳定标识（NodeKey）
目标是让“同一个节点”在订阅端小幅变化时仍能被可靠识别，从而复用历史 `node.ID`，避免 FRouter 引用断裂。

建议将节点匹配拆为“由强到弱”的多级 key（均基于 canonicalization）：
1. **fingerprintKey（强匹配）**：协议/地址/端口/安全/传输/TLS 的规范化指纹
2. **identityKey（弱匹配）**：忽略部分易变字段，仅保留“身份核心”（如 uuid/password/method 等）
3. **identity+name（歧义消解）**：当 identity 冲突时，允许用“名称”做唯一定位（仍坚持“唯一才复用”）

> 原则：宁可少复用，也不要错复用。只有在唯一匹配时才复用/才做引用重写。

#### 2) 订阅同步：在“替换节点集合”前先完成映射与引用修复
现有同步路径大致为：下载 payload → 解析 nodes → `ReplaceNodesForConfig` 替换 → （Clash YAML 时）重写订阅 FRouter 的 chainProxy 引用。

本次重构将“引用修复”升级为通用能力：
- 同步时对 **该 configID 的历史节点集合** 建索引（按 NodeKey）
- 对新解析出的 nodes 做匹配与 ID 复用，并生成 `idMap`（oldID -> newID 或 parsedID -> reusedID）
- 在持久化节点替换前后，统一对以下引用做修复：
  - 订阅生成的 FRouter（`sourceConfigId = configID`）
  - 用户自定义 FRouter 中引用到该订阅节点的图（edges.to / edges.via / slots.boundNodeId）

修复策略：
- 仅对可确定映射的 nodeID 做替换
- 对无法映射/歧义映射的部分，不做 destructive rewrite（避免错路由）

#### 3) 统一稳定 ID 生成函数，减少重复与分叉
当前代码中存在两处 `stableNodeIDForConfig`（config 与 repo 各一份），容易产生差异与回归风险。

重构方向：
- 提取为单一实现（建议放在 `backend/domain` 或 `backend/service/shared`），统一 canonicalization 逻辑
- 所有解析/生成节点 ID 的路径复用该实现

#### 4) Windows TUN 可用性补强（Issue #41）
目标不是“猜测所有失败原因”，而是：
- **不误判**（减少“内核已运行但判定失败”的情况）
- **可诊断**（失败时给出下一步指引）

改进点（按收益优先）：
- 就绪判定失败时，错误信息应包含：
  - 期望的 TUN address/CIDR（脱敏）
  - 建议操作（管理员权限/查看内核日志）
- 适当放宽非 Linux 的接口识别兜底策略（在地址尚未出现时用“新网卡 + 更像 TUN 的特征”作为短暂兜底）

#### 5) 走向图全屏/大画布（Issue #57）
原则：不引入重型图形库；复用现有 dagre + SVG。

改进点：
- 确保全屏窗口尺寸足够大（100vw/100vh 或接近全屏）
- 避免 SVG marker/defs 在多实例图中发生 ID 冲突（确保 marker id 全局唯一）
- 交互一致：拖拽平移、滚轮缩放、双击 fit-to-view

## 架构决策 ADR

### ADR-20260116-01: 订阅节点“稳定标识 + 引用修复”策略
**上下文:** 订阅同步会替换节点集合；node.ID 变化会导致 FRouter 图引用断裂并显示“未知节点”。仅靠单点修复（仅订阅 FRouter）无法覆盖用户自定义 FRouter、identity 冲突等边界条件。  
**决策:** 引入分级 NodeKey 作为匹配依据，先做 ID 复用并生成映射，再对所有受影响 FRouter 引用做“仅确定部分”的修复。  
**理由:** 兼顾安全性（避免错绑）与可用性（减少未知节点）；可写单测覆盖关键边界。  
**替代方案:** 直接更换 node.ID 生成算法并对 state.json 做全量迁移 → 拒绝原因: 风险高、迁移复杂且容易引入不可逆数据问题。  
**影响:** 同步逻辑更复杂但集中在 config/service 层；通过单测与日志降低回归成本。

## API设计
本次不新增 API。必要时仅增强错误信息（`error` 文案）以提升可诊断性。

## 数据模型
不强制新增持久化字段；NodeKey 作为派生信息在运行时计算即可。若后续排障需要，可考虑仅新增只读字段用于诊断，但不作为逻辑必需项。

## 安全与性能
- **安全:** 仅在内存中计算 key 与映射；不输出订阅 URL/token；引用修复坚持“唯一才改”
- **性能:** 映射构建与应用为 O(n)；对常见订阅规模可接受

## 测试与部署
- **测试:**
  - 单元测试覆盖：identity 冲突 + 名称消解；自定义 FRouter 引用修复；Clash YAML 订阅链路重写
  - 回归：`go test ./...`
- **部署:** 无额外部署步骤；随版本发布生效
