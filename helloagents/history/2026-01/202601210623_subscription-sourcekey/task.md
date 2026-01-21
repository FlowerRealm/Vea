# 任务清单: 订阅节点引入 SourceKey 以稳定 FRouter 引用

目录: `helloagents/plan/202601210623_subscription-sourcekey/`

---

## 1. 数据模型与稳定 ID
- [√] 1.1 在 `backend/domain/entities.go` 为 `Node` 增加 `sourceKey` 字段，验证 why.md#需求-拉取节点后不破坏-frouter-引用-场景-分享链接订阅拉取后引用仍可解析
- [√] 1.2 在 `backend/domain/ids.go` 增加 `StableNodeIDForSourceKey(configID, sourceKey)` 并在订阅同步链路中使用，验证 why.md#需求-拉取节点后不破坏-frouter-引用-场景-订阅节点参数变化但名称不变

## 2. 分享链接解析补齐 SourceKey
- [√] 2.1 在 `backend/service/node/parser.go` 补齐分享链接解析后的 `node.SourceKey`（按 how.md 规则），验证 why.md#需求-拉取节点后不破坏-frouter-引用-场景-订阅节点参数变化但名称不变

## 3. 订阅同步与引用重写
- [√] 3.1 在 `backend/service/config/service.go` 引入 `sourceKey` 优先复用策略，并在必要时生成 `oldID->newID` 映射调用 `rewriteFRoutersNodeIDs`，验证 why.md#需求-拉取节点后不破坏-frouter-引用-场景-分享链接订阅拉取后引用仍可解析
- [-] 3.2 如需复用现有索引结构，在 `backend/service/config/node_id_reuse.go` 扩展索引以支持 `sourceKey`（保持“唯一才复用”），验证 why.md#需求-拉取节点后不破坏-frouter-引用-场景-同名节点存在时不误绑
  > 备注: 已通过新增 `backend/service/config/source_key.go` 独立实现 sourceKey 复用与引用重写，无需改动 `node_id_reuse.go`。

## 4. 仓储合并规则
- [√] 4.1 在 `backend/repository/memory/node_repo.go` 的 Upsert 合并逻辑中保留/回填 `sourceKey`（与 name/tags 一致），验证 why.md#需求-拉取节点后不破坏-frouter-引用-场景-分享链接订阅拉取后引用仍可解析

## 5. 测试
- [√] 5.1 在 `backend/service/config/service_test.go` 增加回归测试：分享链接订阅同步后 FRouter 引用不变、不会出现未知 UUID，验证点: slots/edges/via 解析成功
- [√] 5.2 在 `backend/service/config/service_test.go` 或 `backend/service/config/node_id_reuse_test.go` 增加测试：同名节点不会被错误复用/重写（仅唯一才处理）

## 6. 安全检查
- [√] 6.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 7. 文档更新
- [√] 7.1 更新 `helloagents/wiki/modules/backend.md`：补充“订阅节点 sourceKey 稳定身份/引用修复”规范
