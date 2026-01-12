# 任务清单: 修复槽位功能不可用（Issue #40）

目录: `helloagents/plan/202601112042_fix-slot-ui/`

---

## 1. 主题页（light）
- [√] 1.1 在 `frontend/theme/light/index.html` 中为“路由规则”面板新增“槽位管理”入口与弹窗骨架，验证 why.md#需求-槽位可管理
- [√] 1.2 在 `frontend/theme/light/js/main.js` 中实现槽位新增/重命名/绑定/解绑逻辑，并确保规则编辑下拉可刷新，验证 why.md#需求-槽位可管理-场景-新增槽位 与 why.md#需求-槽位可管理-场景-重命名槽位 与 why.md#需求-槽位可管理-场景-绑定或解绑槽位
- [√] 1.3 在 `frontend/theme/light/js/main.js` 中补齐规则编辑弹窗中槽位展示与状态同步（含“未知绑定”兜底），验证 why.md#需求-规则可选择槽位-场景-编辑规则选择槽位并显示状态

## 2. 主题页（dark）
- [√] 2.1 在 `frontend/theme/dark/index.html` 同步新增“槽位管理”入口与弹窗骨架，验证 why.md#需求-槽位可管理
- [√] 2.2 在 `frontend/theme/dark/js/main.js` 同步实现槽位管理逻辑与规则编辑刷新，验证 why.md#需求-槽位可管理 与 why.md#需求-规则可选择槽位

## 3. 安全检查
- [√] 3.1 执行安全检查（按G9: 输入校验、渲染 escape、避免提交无效 slots、错误信息不泄露敏感数据）

## 4. 文档更新
- [√] 4.1 更新 `helloagents/wiki/modules/frontend.md` 记录“槽位管理”入口与交互说明
- [√] 4.2 更新 `helloagents/CHANGELOG.md` 记录修复 Issue #40（并核对/纠正与 Issue #29 相关的过时记录，确保与代码一致）

## 5. 测试
- [?] 5.1 手工回归：新增槽位→绑定节点→规则去向选择槽位→保存图配置→重开确认持久化，验证 why.md#需求-保存后生效-场景-保存图配置并应用到代理
  > 备注: 当前环境无法运行 Electron UI（需在 Windows 发布版或本地 GUI 环境验证）
- [√] 5.2 执行 `go test ./...` 确保后端未被影响
