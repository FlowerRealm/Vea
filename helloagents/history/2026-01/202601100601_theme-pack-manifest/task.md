# 任务清单: 主题包支持 manifest（单包多子主题、隔离无冲突）

目录: `helloagents/plan/202601100601_theme-pack-manifest/`

---

## 1. 后端：主题列表支持主题包展开
- [√] 1.1 在 `backend/service/theme/service.go` 增加 `manifest.json` 解析与校验，并在 `List` 中将主题包展开为“可切换主题列表”，验证 why.md#核心场景
- [√] 1.2 在 `docs/api/openapi.yaml` 扩展 `ThemeInfo` schema（新增 `entry/name/packId/packName` 可选字段），保持兼容，验证 why.md#变更内容

## 2. 后端：主题包 ZIP 导入
- [√] 2.1 在 `backend/service/theme/service.go` 扩展 `ImportZip`：支持顶层目录包含 `manifest.json` 的主题包导入；校验 manifest entry 安全与入口存在，验证 why.md#需求-导入主题包-zip
- [√] 2.2 在 `backend/service/theme/service_test.go` 增加主题包导入成功用例与路径穿越拒绝用例，验证 why.md#风险评估

## 3. Electron：启动入口解析支持虚拟主题ID
- [√] 3.1 在 `frontend/main.js` 调整启动时主题入口解析：读取 `/settings/frontend` 的 `theme` 后，通过 `GET /themes` 找到匹配项并加载其 `entry`，验证 why.md#需求-启动时加载已保存子主题

## 4. 主题页：切换与导出按 entry/pack 逻辑工作
- [√] 4.1 在 `frontend/theme/light/js/main.js` 重构主题管理：使用 `/themes` 返回的 `entry` 跳转；导出时若存在 `packId` 则导出 pack，验证 why.md#需求-切换到包内子主题
- [√] 4.2 在 `frontend/theme/dark/js/main.js` 同步实现与 light 一致的主题管理逻辑，验证 why.md#需求-切换到包内子主题

## 5. 安全检查
- [√] 5.1 执行安全检查（按G9: 输入验证、路径穿越、zip bomb、符号链接、错误信息不泄露敏感路径），并对 manifest 解析失败/非法字段进行 fail-fast 处理

## 6. 文档与知识库
- [√] 6.1 更新 `frontend/README.md` 增补主题包（manifest + 多子主题）目录结构与运行机制说明
- [√] 6.2 更新 `helloagents/wiki/modules/frontend.md` 记录新主题包形态与相关行为变更
- [√] 6.3 更新 `helloagents/CHANGELOG.md` 记录新增“主题包 manifest（单包多子主题）”能力

## 7. 测试
- [√] 7.1 运行 `go test ./...`，确保通过；如有必要补充/修复与本变更直接相关的测试
