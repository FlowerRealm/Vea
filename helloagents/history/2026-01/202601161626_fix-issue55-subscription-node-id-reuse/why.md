# 变更提案: 修复订阅同步节点 ID 复用（Issue #55）

## 需求背景
用户在订阅处执行“拉取节点/同步”后，已有 FRouter 引用的节点可能在 UI 中显示为“未知”，本质原因通常是订阅同步过程中节点集合被替换，而节点 `node.ID` 未能稳定复用，导致 FRouter 的 `edges.to/edges.via`、`slots.boundNodeId` 引用断裂。

Issue #55 反馈该问题在 Windows 发布版仍可复现（与 Issue #18 / #51 复现方式相同）。

## 变更内容
1. 增强订阅同步时的节点 ID 复用策略：在“同一 identity 下存在多个节点”的情况下，引入 `identity + name` 的唯一映射以辅助匹配。
2. 补齐单元测试：覆盖 identity 冲突 + 传输细节变化（例如 ws path）仍应保持节点 ID 不变的场景。

## 影响范围
- **模块:** backend/service/config
- **文件:**
  - backend/service/config/node_id_reuse.go
  - backend/service/config/service_test.go
- **API:** 无
- **数据:** 无（仅影响同步时的节点 ID 复用策略）

## 核心场景

### 需求: 订阅同步不破坏 FRouter 引用
**模块:** backend/service/config

#### 场景: identity 冲突时仍能按名称复用节点 ID
条件:
- 同一订阅配置下存在多个“identity 相同”的节点（例如同协议/同地址/同端口/同凭据，但传输细节不同）
- 订阅同步时传输细节发生变化（例如 ws path 更新）

预期结果:
- 同步后节点 `node.ID` 尽量复用历史值
- 既有 FRouter 对节点的引用不应断裂（避免 UI 显示“未知”）

## 风险评估
- **风险:** name 参与匹配可能在少数订阅中发生误复用
- **缓解:** 仅在 `identity+name` 映射唯一且未被占用时复用；并用单测覆盖典型冲突场景

