# 技术设计: 修复 Issue #43（订阅拉取后 FRouter 节点变为未知）

## 技术方案

### 核心技术
- Go（后端）
- 订阅同步：`backend/service/config/service.go`
- 节点仓储：`backend/repository/memory/node_repo.go`
- ChainProxy 图：`backend/domain`（`ChainProxySettings` / `ProxyEdge` / `SlotNode`）

### 实现要点
1. **在订阅同步前构建“节点指纹 → 现有节点 ID”映射**
   - 读取当前 `configID` 下的历史节点列表
   - 为每个历史节点计算“稳定指纹 key”（建议复用现有 stableNodeIDForConfig 的计算逻辑，或抽出共享函数）
   - 记录 `fingerprintKey -> existingNode.ID`

2. **对新解析出来的节点做 ID 复用**
   - 对每个解析节点计算 fingerprintKey
   - 若命中 `existingID`，则将解析节点的 `ID` 设为 `existingID`
   - 同时收集 `oldID -> newID` 的映射（用于 Clash ChainProxy 图重写）

3. **Clash YAML 订阅：重写 ChainProxySettings 中的节点引用**
   - 遍历并重写：
     - `edges[].from / to / via[]`（跳过 `local/direct/block` 与 slot 节点）
     - `slots[].boundNodeId`
     - `positions` 的 key（如有）
   - 使用 `oldID -> newID` 映射进行替换，确保图中引用与最终写入仓储的 node.ID 一致

4. **保持快照替换语义**
   - 仍使用 `ReplaceNodesForConfig` 执行“以最新快照替换并删除缺失节点”
   - 通过 ID 复用使“同一节点”在快照替换中被识别为同一实体，从而不会被误删

## 安全与性能
- **安全:** 不引入外部输入执行；仅做内存映射与数据结构重写
- **性能:** 指纹映射与重写为 O(N) 级别（N 为订阅节点数 + 边数），对桌面端场景可接受

## 测试与验证
- 在 `backend/service/config/service_test.go` 增加回归用例：
  - 构造“已有 FRouter 引用订阅节点”的状态
  - 执行一次订阅 Sync（payload 不变）
  - 断言：同步后被引用的节点仍存在且引用仍可解析（不出现 NodeNotFound 语义）

