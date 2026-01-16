# 任务清单: 修复订阅同步节点 ID 复用（Issue #55）

目录: `helloagents/plan/202601161626_fix-issue55-subscription-node-id-reuse/`

---

## 1. backend/service/config
- [√] 1.1 在 `backend/service/config/node_id_reuse.go` 增强节点 ID 复用：支持 `identity+name` 唯一匹配，验证 why.md#需求-订阅同步不破坏-frouter-引用-identity-冲突时仍能按名称复用节点-id
- [√] 1.2 在 `backend/service/config/service_test.go` 增加单测：identity 冲突 + 传输细节变化仍复用历史节点 ID，验证 why.md#需求-订阅同步不破坏-frouter-引用-identity-冲突时仍能按名称复用节点-id

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9：仅做确定性映射，不引入敏感信息与危险操作）

## 3. 文档更新
- [√] 3.1 更新 `helloagents/wiki/modules/backend.md`：补充订阅同步 ID 复用规则在 identity 冲突场景下的处理
- [√] 3.2 更新 `helloagents/CHANGELOG.md`：补充 Issue #55 修复说明

## 4. 测试
- [√] 4.1 运行 `go test ./...`
