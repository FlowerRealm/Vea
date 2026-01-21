# 技术设计: 订阅节点引入 SourceKey 以稳定 FRouter 引用

## 技术方案

### 核心技术
- Go（后端）
- 现有订阅同步链路：`backend/service/config` → `backend/service/node`（分享链接解析）→ `NodeRepository.ReplaceNodesForConfig` → `rewriteFRoutersNodeIDs`

### 实现要点
- 为 `domain.Node` 新增字段：`SourceKey string \`json:"sourceKey,omitempty"\``
- 为分享链接订阅解析补齐 `sourceKey`（不依赖 `node.ID` 推导）
- 同步时用 `sourceKey` 做“稳定身份”：
  - 优先按 `sourceKey` 找历史节点并复用其 `node.ID`
  - 对旧节点逐步回填 `sourceKey`（通过本次同步匹配到的节点更新写回）
- 当发生 ID 切换时生成 `oldID -> newID` 映射，并重写所有 FRouter 链路引用

## 架构决策 ADR

### ADR-20260121: 订阅节点引入 SourceKey 作为稳定身份
**上下文:** 目前订阅拉取会替换节点集合，节点 ID 的生成/复用在部分边界场景失败，导致 FRouter 引用断裂并在 UI 显示“未知: uuid”。单纯依赖“协议/地址/端口/安全/传输/TLS 指纹”作为 identity，在订阅提供方“参数滚动更新”的场景下并不稳定。

**决策:** 引入 `Node.sourceKey`，作为订阅侧的稳定身份键；后端在订阅同步时优先以 `sourceKey` 复用历史节点 ID，并在必要时用 `oldID -> newID` 映射修复 FRouter 引用。

**理由:**
- 贴近用户认知：用户配置 FRouter 时主要依赖“节点名称/订阅语义”，而不是 UUID/指纹字段
- 可渐进落地：旧 state.json 无需迁移即可运行；后续同步会逐步回填 sourceKey
- 风险可控：坚持“唯一才复用/唯一才重写”，避免误绑

**替代方案:** 继续强化 fingerprint/identity 复用 → 拒绝原因: 订阅参数变化较大时仍会漂移；且无法表达“以订阅语义为准”的稳定身份。

**影响:**
- `domain.Node` 增加可选字段（向后兼容）
- 订阅同步逻辑变更（对 share links 生效）
- 需要补齐单测，锁定“参数变化但名称不变仍保持引用”的回归

## 实现细节

### 1) SourceKey 生成规则（分享链接订阅）
目标：在“名称不变但参数变化”的场景保持稳定；在“同名节点”场景不误绑。

建议规则（可落地且可调优）：
1. 规范化 name：`nameNorm = strings.ToLower(strings.TrimSpace(node.Name))`
2. 若 `nameNorm` 为空：回退到 `identityKey(node)`（现有逻辑），并标记为“弱稳定”（仅保证同 identity 不漂移）
3. 同名消歧：当同一订阅内存在多个相同 `nameNorm`，则使用 `nameNorm + "|" + shortHash(identityKey(node))` 作为 `sourceKey`（identityKey 为空则不自动消歧，避免不确定行为）

### 2) NodeID 生成/复用策略
- 新增 `domain.StableNodeIDForSourceKey(configID, sourceKey)`：
  - 只依赖 `configID + sourceKey` 生成稳定 UUID（SHA1 namespace）
  - 避免把易变字段（server/uuid/path/fingerprint 等）编码进 ID
- 同步时的 ID 选择顺序（由强到弱）：
  1. `existingBySourceKey[sourceKey]` → 复用 existing ID
  2. 现有 `reuseNodeIDs`（fingerprint/identity/identity+name）→ 复用 existing ID，并回填 `sourceKey`
  3. 生成新 ID：`StableNodeIDForSourceKey(configID, sourceKey)`（保证后续同步稳定）

### 3) FRouter 引用修复（oldID -> newID）
当本次同步产生 “旧节点未被复用且会被清理/替换” 时，需要生成 `oldID -> newID`：
- 映射来源（由强到弱）：
  1. 旧节点已有 `sourceKey` 且本次同步存在同 `sourceKey` 的新节点（唯一）
  2. 旧节点 name（规范化）与新节点 name 匹配且唯一（仅在同名不冲突时）
- 对映射应用 `rewriteFRoutersNodeIDs`（edges.from/to/via、slots.boundNodeId、positions key）
- 仅当映射唯一且不含歧义时执行；否则保守不改并输出可操作日志（提示用户重新绑定/检查同名）

## 安全与性能
- **安全:** 不引入外部网络/敏感数据处理；避免在日志中输出完整订阅 payload（仅输出 configID + 计数 + 诊断信息）
- **性能:** 仅在同步时构建索引与映射（O(n)），节点规模通常较小

## 测试与部署
- **测试:** Go 单测补齐（服务层同步场景 + 同名节点消歧 + 引用重写）
- **部署:** 无额外部署步骤；升级后首次同步会逐步回填 `sourceKey`

