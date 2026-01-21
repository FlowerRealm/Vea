# 变更提案: 订阅节点引入 SourceKey 以稳定 FRouter 引用

## 需求背景
目前在「订阅 → 拉取节点」（分享链接订阅）后，FRouter 中原先引用的节点可能变为“未知: {uuid}”（或在槽位/规则去向处显示一串 UUID）。这意味着订阅同步过程中节点 `node.ID` 发生漂移/替换，而 FRouter 的 `slots.boundNodeId`、`edges.to`、`edges.via` 仍指向旧 ID，导致 UI 无法解析并显示为 UUID。

该问题长期存在且影响核心使用路径：用户仅仅同步订阅，就会造成既有 FRouter 配置“看起来坏了”。

## 变更内容
1. 为订阅节点引入 `sourceKey`（稳定身份键），用于跨拉取保持“同一节点”的稳定性。
2. 同步节点时优先按 `sourceKey` 复用历史节点 ID；对历史无 `sourceKey` 的节点进行逐步回填（不强制迁移）。
3. 当确实发生“旧 ID → 新 ID”切换时，自动重写 FRouter 的链路引用（仅在映射唯一且可确定时执行）。

## 影响范围
- **模块:** backend/service/config, backend/service/node, backend/domain, backend/repository/memory
- **文件(预估):**
  - backend/domain/entities.go
  - backend/domain/ids.go
  - backend/service/node/parser.go
  - backend/service/config/service.go
  - backend/service/config/node_id_reuse.go（可能）
  - backend/repository/memory/node_repo.go
  - backend/service/config/service_test.go
- **API:** 无（仅增加 Node JSON 字段，向后兼容）
- **数据:** state.json 中 Node 结构新增可选字段 `sourceKey`（无需 bump schemaVersion，旧数据可无感加载）

## 核心场景

### 需求: 拉取节点后不破坏 FRouter 引用
**模块:** backend/service/config

#### 场景: 分享链接订阅拉取后引用仍可解析
条件:
- 已有 FRouter 引用订阅节点（槽位绑定或规则去向/链路 via 引用 nodeId）
- 用户在订阅面板点击“拉取节点”

预期结果:
- 拉取后 FRouter 中引用仍能在节点列表中找到并展示节点名
- 不出现“未知: {uuid}”

#### 场景: 订阅节点参数变化但名称不变
条件:
- 同一订阅内某节点的 server/uuid/transport/tls 等参数发生变化
- 节点名称（用户认知的标识）保持不变

预期结果:
- 该节点在同步前后的 `node.ID` 保持稳定（通过 sourceKey 绑定）
- FRouter 引用不丢失

#### 场景: 同名节点存在时不误绑
条件:
- 同一订阅中存在多个同名节点

预期结果:
- 不进行不确定的自动重写
- 仅在映射唯一时复用/重写，避免把 FRouter 指向错误节点

## 风险评估
- **风险:** 以 name 为核心的 sourceKey 可能在“同名节点”场景下产生冲突，导致误复用/误重写。
- **缓解:** 对同名节点强制引入消歧规则（如 name + identity 摘要），并坚持“唯一才复用/唯一才重写”；无法确认时宁可保持 UUID 不动并给出可操作日志提示。

