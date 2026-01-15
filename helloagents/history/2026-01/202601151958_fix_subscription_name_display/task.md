# 任务清单: 修复订阅名显示错误（Issue #53/#42）

目录: `helloagents/plan/202601151958_fix_subscription_name_display/`

---

## 1. 前端（主题共享逻辑）
- [√] 1.1 在 `frontend/theme/_shared/js/app.js` 中为 configs 引入“已加载”状态，并调整 `formatSubscriptionLabel()` 的回退策略，验证 why.md#核心场景
- [√] 1.2 在 `frontend/theme/_shared/js/app.js` 的「节点」面板加载流程中触发 `loadConfigs()`（必要时带缓存/去重），确保首次进入节点面板可显示订阅名，验证 why.md#核心场景

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9：输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 3. 文档更新
- [√] 3.1 更新 `helloagents/CHANGELOG.md`（记录修复）
- [√] 3.2 更新 `helloagents/history/index.md`（记录本次方案包）
- [√] 3.3 更新 `helloagents/wiki/modules/frontend.md`（补齐规范与变更历史）

## 4. 测试
- [√] 4.1 执行 `go test ./...`（确保后端与整体构建不被影响）
