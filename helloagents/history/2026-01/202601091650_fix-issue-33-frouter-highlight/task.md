# 任务清单: 修复 issue #33（浅色主题 FRouter 选中高亮）

目录: `helloagents/plan/202601091650_fix-issue-33-frouter-highlight/`

---

## 1. 前端主题（浅色）
- [√] 1.1 在 `frontend/theme/light.html` 中补齐/映射缺失的 CSS 变量（用于兼容现有样式引用），验证 why.md#需求-浅色主题下突出当前选中的-frouter
- [√] 1.2 在 `frontend/theme/light.html` 中将 `.node-row.active` 选中态高亮调整为黑色边框，验证 why.md#场景-选中-frouter-后识别当前项

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9: 不引入敏感信息、无权限/外部服务变更）

## 3. 文档更新
- [√] 3.1 更新知识库：`helloagents/wiki/modules/frontend.md` 记录本次样式修复与影响范围
- [√] 3.2 更新知识库：`helloagents/CHANGELOG.md`、`helloagents/history/index.md`（开发实施阶段完成后同步）

## 4. 测试
- [√] 4.1 执行 `go test ./...` 并记录结果（开发实施阶段）
