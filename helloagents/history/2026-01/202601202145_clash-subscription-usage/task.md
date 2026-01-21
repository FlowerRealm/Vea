# 任务清单: Clash 订阅用量显示（已用/总量）

目录: `helloagents/plan/202601202145_clash-subscription-usage/`

---

## 1. 后端（订阅同步 + 数据模型）
- [√] 1.1 在 `backend/domain/entities.go` 为 `Config` 增加用量字段（`usageUsedBytes/usageTotalBytes`），验证 why.md#需求-订阅用量展示-场景-同步成功并展示用量
- [√] 1.2 在 `backend/service/config/service.go` 的订阅下载/同步路径中解析 `subscription-userinfo` 并更新配置用量字段，验证 why.md#需求-订阅用量展示-场景-同步成功并展示用量，依赖任务1.1
- [√] 1.3 在 `backend/repository/interfaces.go` 与 `backend/repository/memory/config_repo.go` 增强同步状态更新能力（同步时间/错误 + 可选用量字段），验证 why.md#需求-订阅用量展示-场景-响应头缺失或字段不完整，依赖任务1.2

## 2. 前端（订阅列表展示）
- [√] 2.1 在 `frontend/theme/_shared/js/app.js` 的 `renderConfigs()` 中展示“已用/总量”（无数据时显示 `—`），验证 why.md#需求-订阅用量展示-场景-同步成功并展示用量

## 3. SDK 类型同步
- [√] 3.1 在 `frontend/sdk/src/types.d.ts` 为 `Config` 增加对应可选字段，并按项目流程更新 `frontend/sdk/dist/` 构建产物，验证 why.md#需求-订阅用量展示-场景-同步成功并展示用量

## 4. 安全检查
- [√] 4.1 执行安全检查（输入解析健壮性、避免日志泄漏订阅敏感信息）

## 5. 文档更新（知识库）
- [√] 5.1 更新 `helloagents/wiki/modules/backend.md`，补充订阅用量字段与同步逻辑说明
- [√] 5.2 更新 `helloagents/wiki/modules/frontend.md`，补充订阅列表用量展示说明

## 6. 测试
- [√] 6.1 在 `backend/service/config/` 增加/补充单元测试：header 解析与 Sync 流程集成校验
