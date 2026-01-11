# 变更提案: 修复 Issue #43（拉取订阅后 FRouter 节点变为未知）

## 需求背景
Issue #43 反馈：在“订阅”中拉取节点后（常见复现为：拉取→启用一次代理→退出并重进），FRouter 配置界面中原本已选择的节点会显示为“未知”，并以一串 ID 形式展示（用户感知为“乱码”）。

该问题会直接破坏用户已配置的 FRouter（slot 绑定、路由规则去向、链路 hop 等），属于高频的可用性问题。

## 变更内容
1. 订阅同步（拉取节点）时，尽可能复用“同一节点”的历史 ID，避免节点 ID 在同步过程中发生变化导致 FRouter 引用断裂。
2. Clash YAML 订阅自动生成的 ChainProxy 图在复用节点 ID 后，同步重写图中的节点引用（edges / via / slots.boundNodeId / positions）。
3. 保持现有“快照替换/清理旧节点”的语义不变：当订阅快照中确实不再包含某节点时，仍会删除该节点（此时 FRouter 引用该节点显示未知属于合理结果）。

## 影响范围
- **模块:** backend
- **文件（预期）:**
  - `backend/service/config/service.go`（订阅同步：在 ReplaceNodesForConfig 前做 ID 复用/映射）
  - `backend/service/config/clash_subscription.go`（Clash ChainProxy 引用重写/复用映射）
  - `backend/service/nodes/service.go`（如需：补齐 ListByConfigID 能力）
  - `backend/service/config/service_test.go`（回归测试：复现并锁定 Issue #43）
- **API:** 无变化
- **数据:** 订阅同步过程将尽量保持节点 ID 不变；用户既有 FRouter 配置应不再被同步破坏

## 核心场景

### Requirement: Preserve node references across subscription sync
**模块:** backend  
订阅拉取成功后，既有 FRouter 中引用的订阅节点（slot 绑定、规则去向、via hop）仍能被正确解析为节点名称，而不是“未知: {id}”。

#### Scenario: Pull nodes does not break FRouter bindings
- 条件:
  - 已存在订阅节点，并被某个 FRouter 引用（slot.boundNodeId / edge.to / edge.via 等）
  - 再次执行“拉取节点”（订阅内容不变或仅节点名称变更）
- 预期结果:
  - FRouter 的节点引用仍指向有效节点（UI 不再显示“未知: {id}”）
  - 订阅节点仍按快照语义清理旧节点（仅当节点确实不在订阅中才删除）

## 风险评估
- **风险:** 若“同一节点”的判定过宽，可能把不同节点错误复用到同一 ID 上，导致用户引用错乱。
- **缓解:**
  - 使用严格的节点指纹（protocol/address/port/security/transport/tls）来做复用判定
  - 对指纹冲突/多对一匹配进行告警或保守处理（必要时放弃复用，保持现状）

