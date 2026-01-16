# 技术设计: fix-issue62-frouter-rename

## 技术方案
### 核心技术
- Go（Gin）后端 API
- 主题页共享脚本 `frontend/theme/_shared/js/app.js`
- JS SDK（`frontend/sdk`）与主题内置 SDK
- OpenAPI（`docs/api/openapi.yaml`）

### 实现要点
- 后端新增 `PUT /frouters/:id/meta`：
  - 请求体支持 `name`/`tags` 任意组合（至少一个非空）。
  - 仅更新字段，不触发链路图编译校验，避免“链路不合法导致无法改名”的耦合。
- 兼容性修复：`PUT /frouters/:id` 在未携带 `tags` 时不应清空已有 tags（按 nil=不更新语义处理）。
- 前端：为 FRouter 列表右键菜单增加“重命名”，通过 `prompt` 获取新名称并调用 `PUT /frouters/:id/meta`，成功后刷新 FRouter 列表。
- SDK/OpenAPI：补齐接口定义与 SDK 方法，避免前后端/文档不一致。

## API设计
### PUT /frouters/{id}/meta
- **请求:** `{ "name"?: string, "tags"?: string[] }`
- **响应:** `FRouter`

## 安全与性能
- **安全:** 输入校验（trim、空值拒绝）；不引入新权限点；不记录敏感信息。
- **性能:** 该接口不触发链路编译，降低不必要的计算与失败概率。

## 测试与部署
- **测试:** 新增/补齐后端 API 测试覆盖 rename 与 tags 保持逻辑；运行 `go test ./...`。
- **部署:** 无迁移步骤；接口新增向后兼容。
