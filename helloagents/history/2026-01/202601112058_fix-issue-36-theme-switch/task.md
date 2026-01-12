# 任务清单: 修复 Issue #36 主题切换失败（Windows）

目录: `helloagents/plan/202601112058_fix-issue-36-theme-switch/`

---

## 1. 主题页入口解析（dark/light）
- [√] 1.1 在 `frontend/theme/dark/js/main.js` 中重写 `getThemesBaseHref()`/`getCurrentThemeEntry()`：基于 `URL.pathname` 解码并归一化分隔符，兼容 Windows `file://` URL，验证 why.md#需求-默认主题可切换（issue-36）-场景-在-windows-下从深色切到浅色（或相反）
- [√] 1.2 在 `frontend/theme/light/js/main.js` 中同步实现相同逻辑，验证 why.md#需求-默认主题可切换（issue-36）-场景-在-windows-下从深色切到浅色（或相反）
- [√] 1.3 为无法解析主题根路径的情况增加最小兜底（优先保证 `dark`/`light` 可切换），验证 why.md#需求-默认主题可切换（issue-36）-场景-从主题包返回默认主题

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 3. 文档更新
- [√] 3.1 更新 `helloagents/wiki/modules/frontend.md` 记录本次修复点与 Windows 兼容性注意事项
- [√] 3.2 更新 `helloagents/CHANGELOG.md` 记录 Issue #36 修复条目

## 4. 测试
- [√] 4.1 执行 `go test ./...`（回归后端不受影响）
- [?] 4.2 Windows 手动回归：切换 `dark`/`light`、主题包→默认主题回退（记录验证结论）
  > 备注: 当前环境无 Windows GUI，需在 Windows 发布版实际验证。
