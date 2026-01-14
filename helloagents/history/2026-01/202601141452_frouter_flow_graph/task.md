# 任务清单: FRouter 静态走向流程图

目录: `helloagents/plan/202601141452_frouter_flow_graph/`

---

## 1. 走向图渲染（frontend/theme/_shared）
- [√] 1.1 在 `frontend/theme/_shared/js/app.js` 中实现“走向模型构建”（规则节点、slot 解析、detour/via 链路追踪），验证 why.md#需求-frouter-静态走向可视化-场景-查看默认去向与-detour-链路
- [√] 1.2 在 `frontend/theme/_shared/js/app.js` 中实现 dagre 自动布局 + SVG 渲染（节点/边/tooltip），验证 why.md#需求-frouter-静态走向可视化-场景-查看路由规则匹配条件
- [√] 1.3 在 `frontend/theme/_shared/js/app.js` 中实现拖拽平移 + 滚轮缩放交互，并挂载到“FRouter 详情卡片”区域，验证 why.md#需求-frouter-静态走向可视化-场景-交互浏览缩放拖拽

## 2. 主题资源与样式（frontend/theme/dark|light）
- [√] 2.1 新增 `frontend/theme/_shared/vendor/dagre.min.js` 与许可证文件，并在 how.md 记录版本/来源
  > 备注: 实际文件为 `frontend/theme/_shared/vendor/dagre-0.8.5.min.js` + `frontend/theme/_shared/vendor/dagre-0.8.5.LICENSE`。
- [√] 2.2 在 `frontend/theme/dark/index.html` 与 `frontend/theme/light/index.html` 引入 dagre vendor 脚本
- [√] 2.3 在 `frontend/theme/dark/css/main.css` 与 `frontend/theme/light/css/main.css` 增加走向图区域样式（容器、节点颜色、边样式、tooltip可读性）

## 3. 安全检查
- [√] 3.1 走向图所有动态文本（规则/节点名/ID）统一做转义与截断，避免 XSS；确认不展示敏感字段（地址/端口）

## 4. 文档更新
- [√] 4.1 更新 `helloagents/wiki/modules/frontend.md`：补充“FRouter 详情卡片走向图”规范与说明
- [√] 4.2 更新 `helloagents/CHANGELOG.md`：新增条目记录该功能

## 5. 测试
- [?] 5.1 本地手工回归：两套主题（dark/light）均可正常显示走向图，并保持现有 FRouter 操作行为不变
  > 备注: 需要在具备 GUI 的环境下通过 `make dev` 手工验证（本环境仅完成后端单测：`go test ./...`）。
