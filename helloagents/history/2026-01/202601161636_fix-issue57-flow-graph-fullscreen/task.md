# 任务清单: 修复 Issue #57（走向图全屏查看）

目录: `helloagents/plan/202601161636_fix-issue57-flow-graph-fullscreen/`

---

## 1. UI 功能
- [√] 1.1 FRouter 详情卡片：新增“全屏”入口（走向图）
- [√] 1.2 路由规则面板：新增“走向图”入口（全屏查看）
- [√] 1.3 新增走向图全屏 Modal（dark/light）
- [√] 1.4 修复多图同时存在时 SVG marker id 冲突

## 2. 文档更新
- [√] 2.1 更新 `helloagents/CHANGELOG.md`，补充 Issue #57 说明
- [√] 2.2 更新 `helloagents/wiki/modules/frontend.md`，补充本次变更历史条目与交互说明

## 3. 测试
- [√] 3.1 执行 `go test ./...`，确保后端单测不受影响
