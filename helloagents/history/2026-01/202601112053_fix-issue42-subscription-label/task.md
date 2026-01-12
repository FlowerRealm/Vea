# 任务清单: 修复订阅名首次进入显示乱码（Issue #42）

目录: `helloagents/plan/202601112053_fix-issue42-subscription-label/`

---

## 1. 前端主题（light）
- [√] 1.1 在 `frontend/theme/light/js/main.js` 中将 `configsCache` 初始值设为 `null`，并在 `loaders["panel-nodes"]` 中首次进入节点面板时先 `loadConfigs()` 再 `loadNodes()`，验证 why.md#核心场景-需求-订阅名展示稳定-场景-首次启动重启后打开节点界面时

## 2. 前端主题（dark）
- [√] 2.1 在 `frontend/theme/dark/js/main.js` 同步上述逻辑，确保两套主题行为一致，验证 why.md#核心场景-需求-订阅名展示稳定-场景-首次启动重启后打开节点界面时

## 3. 安全检查
- [√] 3.1 执行安全检查（输入验证、敏感信息处理、权限控制、EHRB 风险规避）

## 4. 文档更新
- [√] 4.1 更新 `helloagents/CHANGELOG.md`（记录本次修复）
- [√] 4.2 执行完成后将方案包迁移至 `helloagents/history/YYYY-MM/` 并更新 `helloagents/history/index.md`

## 5. 测试
- [?] 5.1 手工验证 Issue #42 复现步骤（Windows 10）：启动后首次进入节点界面订阅名不再显示为 `configId`，切换选项卡不再影响订阅名显示
  > 备注: 当前环境无法直接运行 Windows 版 GUI，请在 Windows 10 上按 issue 复现步骤确认。
- [?] 5.2 回归：配置刷新、节点测速/测延迟与分组展示仍正常
  > 备注: 已执行 `node --check` 与 `go test ./...`；UI 回归需在实际运行环境验证。
