# 任务清单: 重构订阅节点 ID 体系并修复 open issues（#41/#55/#57）

目录: `helloagents/history/2026-01/202601162158_refactor-subscription-id-system/`

---

## 1. backend/service/config（订阅节点 ID 体系重构）
- [√] 1.1 在 `backend/service/config/node_id_reuse.go` 增强节点匹配索引：支持 identity 冲突时的 `identity+name` 唯一复用，验证 why.md#需求-订阅拉取节点不应导致-frouter-节点变为未知issue-55-场景-identity-冲突--订阅端参数变动后仍保持引用一致
- [√] 1.2 在 `backend/service/config/service.go` 的订阅同步流程中统一产出 `idMap`（节点 ID 映射），并为后续引用修复提供输入，验证 why.md#需求-订阅拉取节点不应导致-frouter-节点变为未知issue-55
- [√] 1.3 在 `backend/service/config/service.go` 实现“自定义 FRouter 引用修复”：同步后重写所有受影响 FRouter 的 `edges.to/edges.via/slots.boundNodeId`（仅对可确定映射），验证 why.md#需求-订阅拉取节点不应导致-frouter-节点变为未知issue-55-场景-自定义-frouter-引用订阅节点同步后仍可解析

## 2. backend/service/config（Clash YAML/分享链接一致性）
- [√] 2.1 在 `backend/service/config/clash_subscription.go` 清理/统一 `stableNodeIDForConfig` 实现来源，确保与订阅同步的 ID 规则一致，验证 why.md#需求-订阅拉取节点不应导致-frouter-节点变为未知issue-55

## 3. backend/service/proxy（Windows TUN 可用性补强）
- [√] 3.1 在 `backend/service/proxy/service.go` 强化 Windows/macOS TUN 就绪判定失败的错误信息（包含可操作提示与诊断线索），验证 why.md#需求-windows-下-tun-可正常使用issue-41-场景-未满足权限驱动条件时给出明确提示
- [√] 3.2 在 `backend/service/proxy/service.go` 调整非 Linux 的 TUN 识别兜底策略（避免误判失败），验证 why.md#需求-windows-下-tun-可正常使用issue-41-场景-满足条件时就绪判定不误报失败

## 4. frontend/theme（走向图全屏/大画布）
- [√] 4.1 在 `frontend/theme/_shared/js/app.js` 完善全屏/大画布走向图交互与兼容性（确保 marker id 不冲突、窗口尺寸足够），验证 why.md#需求-走向图支持全屏大画布查看issue-57

## 5. 安全检查
- [√] 5.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 6. 文档更新
- [√] 6.1 更新 `helloagents/wiki/modules/backend.md`：补充“订阅节点 NodeKey 与引用修复”的规则与边界
- [√] 6.2 更新 `helloagents/wiki/modules/frontend.md`：补充走向图全屏/大画布说明（如有变更）
- [√] 6.3 更新 `helloagents/CHANGELOG.md`：记录 Issue #41/#55/#57 的修复点

## 7. backend/service/shared（日志留存）
- [√] 7.1 应用/内核日志增加 7 天留存：启动会轮转 `app.log`/`kernel.log` 并清理过期文件，便于上传排障（Issue #66）
- [√] 7.2 补齐单元测试覆盖日志轮转与过期清理

## 8. 测试
- [√] 8.1 运行 `go test ./...`
