# 任务清单: fix-issue60-edit-frouter-tags

目录: `helloagents/plan/202601161631_fix-issue60-edit-frouter-tags/`
类型: 轻量迭代

---

## 1. 前端主题页（FRouter 标签编辑）
- [√] 1.1 在 `frontend/theme/_shared/js/app.js` 为 FRouter 列表增加“标签”操作入口：弹窗编辑并调用 `PUT /frouters/:id` 更新 tags
- [√] 1.2 在 FRouter 右键菜单增加“编辑标签”入口（复用同一更新逻辑）

## 2. 测试
- [√] 2.1 运行 `go test ./...`

## 3. 知识库与归档
- [√] 3.1 更新 `helloagents/CHANGELOG.md` 记录 Issue #60
- [√] 3.2 迁移方案包至 `helloagents/history/2026-01/` 并更新 `helloagents/history/index.md`
