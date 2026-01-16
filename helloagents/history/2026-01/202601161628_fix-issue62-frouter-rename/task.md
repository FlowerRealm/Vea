# 任务清单: fix-issue62-frouter-rename

目录: `helloagents/plan/202601161628_fix-issue62-frouter-rename/`

---

## 1. 后端 API（FRouter 元信息）
- [√] 1.1 在 `backend/api/router.go` 增加 `PUT /frouters/:id/meta`（name/tags），验证 why.md#需求-允许修改-frouter-名称-场景-在主题页重命名-frouter
- [√] 1.2 在 `backend/api/router.go` 修复 `PUT /frouters/:id` 未传 `tags` 时清空 tags 的问题，验证 why.md#需求-允许修改-frouter-名称-场景-在主题页重命名-frouter
- [√] 1.3 在 `backend/api/` 新增单元测试覆盖 `PUT /frouters/:id/meta`：重命名成功、tags 保持、空请求返回 400

## 2. 前端主题页（入口）
- [√] 2.1 在 `frontend/theme/_shared/js/app.js` 为 FRouter 右键菜单增加“重命名”，调用 `PUT /frouters/:id/meta` 并刷新列表，验证 why.md#需求-允许修改-frouter-名称-场景-在主题页重命名-frouter

## 3. 文档与 SDK
- [√] 3.1 更新 `docs/api/openapi.yaml`：新增 `/frouters/{id}/meta` 与 `FRouterMetaRequest`
- [√] 3.2 更新 `frontend/sdk/src/vea-sdk.js`：为 `FRoutersAPI` 增加 `updateMeta` 并重建 `frontend/sdk/dist/vea-sdk.esm.js`
- [√] 3.3 更新主题内置 SDK（`frontend/theme/*/js/vea-sdk.esm.js`）：为 `FRoutersAPI` 增加 `updateMeta`（与后端一致）

## 4. 安全检查
- [√] 4.1 执行安全检查（输入验证、无敏感信息引入、无越权路径）

## 5. 知识库更新
- [√] 5.1 更新 `helloagents/wiki/api.md` 补齐接口索引
- [√] 5.2 更新 `helloagents/CHANGELOG.md` 记录 Issue #62

## 6. 测试
- [√] 6.1 运行 `go test ./...` 并确保通过
