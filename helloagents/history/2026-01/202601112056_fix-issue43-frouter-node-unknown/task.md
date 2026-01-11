# 任务清单: 修复 Issue #43（订阅拉取后 FRouter 节点变为未知）

目录: `helloagents/history/2026-01/202601112056_fix-issue43-frouter-node-unknown/`

---

## 1. 订阅同步（节点 ID 复用）
- [√] 1.1 在 `backend/service/nodes/service.go` 增加 `ListByConfigID` 能力（如当前 Service 未暴露），用于订阅同步读取历史节点，验证 why.md#requirement-preserve-node-references-across-subscription-sync-scenario-pull-nodes-does-not-break-frouter-bindings
- [√] 1.2 在 `backend/service/config/service.go` 的订阅解析路径中实现“指纹 → 历史 ID”复用，并在 ReplaceNodesForConfig 前对解析节点赋予复用后的 ID，验证 why.md#requirement-preserve-node-references-across-subscription-sync-scenario-pull-nodes-does-not-break-frouter-bindings

## 2. Clash YAML 订阅链路一致性
- [√] 2.1 在 `backend/service/config/clash_subscription.go` 或 `backend/service/config/service.go` 增加 ChainProxySettings 的节点引用重写（edges/via/slots/positions），确保与最终 node.ID 一致，验证 why.md#requirement-preserve-node-references-across-subscription-sync-scenario-pull-nodes-does-not-break-frouter-bindings，依赖任务1.2

## 3. 安全检查
- [√] 3.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 4. 测试
- [√] 4.1 在 `backend/service/config/service_test.go` 增加回归测试覆盖 Issue #43 的核心场景（订阅同步不破坏 FRouter 引用）
