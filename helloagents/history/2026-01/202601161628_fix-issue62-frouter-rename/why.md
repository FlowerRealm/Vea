# 变更提案: fix-issue62-frouter-rename

## 需求背景
当前前端主题页仅支持创建 FRouter 与编辑链路图，但缺少修改 FRouter 名称的入口。
这会导致用户在创建后无法对 FRouter 进行更名整理，影响可用性与长期维护。

## 变更内容
1. 新增后端接口 `PUT /frouters/:id/meta`：仅更新 FRouter 的 `name/tags`，不触发链路编译校验。
2. 前端主题页在 FRouter 列表右键菜单中增加“重命名”入口，并调用上述接口完成修改。
3. 同步更新 SDK 与 OpenAPI，保证对外接口一致。

## 影响范围
- **模块:** backend/api、frontend/theme、frontend/sdk、docs/api
- **文件:** `backend/api/router.go`、`frontend/theme/_shared/js/app.js`、`docs/api/openapi.yaml`、`frontend/sdk/*`
- **API:** 新增 `PUT /frouters/:id/meta`
- **数据:** 不新增字段；仅更新现有 `FRouter.name/tags`

## 核心场景

### 需求: 允许修改 FRouter 名称
**模块:** frontend/theme、backend/api
用户可以对已有 FRouter 进行重命名。

#### 场景: 在主题页重命名 FRouter
在 FRouter 列表中对某个 FRouter 执行重命名操作：
- 预期结果：FRouter 名称被更新并立即在列表/选择器中可见（Issue #62）

## 风险评估
- **风险:** 入口可能导致误操作/空名称写入。
- **缓解:** 后端对空名称做校验；前端对输入做 trim 与空值提示。
