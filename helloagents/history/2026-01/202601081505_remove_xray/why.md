# 变更提案: 移除 Xray 支持

## 需求背景

当前项目同时支持 Xray 与 sing-box（以及 Clash）。多内核带来的主要问题：
- 代码与测试复杂度显著上升（引擎选择、配置渲染、组件管理、前端 UI/SDK/OpenAPI 均需要分支处理）
- 用户侧认知负担高（需要理解不同内核的能力差异与约束，且 Mixed 端口语义不一致）
- 维护成本高（升级、兼容、E2E/集成测试与文档都需要双份维护）

本变更目标：**删除整个项目对 Xray 的支持**，将 sing-box 作为唯一主内核（Clash 继续保留为独立内核），并确保对外接口不再暴露任何 Xray 相关选项或数据结构。

## 变更内容

1. 移除后端域模型中所有 Xray 相关枚举与配置结构（如 `EngineXray`、`ComponentXray`、`XrayConfiguration`、`xray-json` 等）。
2. 移除后端运行路径中所有 Xray 分支逻辑（引擎推荐/选择、测速启动、组件安装/卸载、mixed 端口 +1 规则等）。
3. 移除前端 UI 与设置项中的 Xray 面板、Xray 状态轮询与任何 Xray 文案/选项。
4. 移除 SDK 中的 Xray 类型与示例，更新生成产物（`frontend/sdk/dist`）。
5. 更新 API 文档/OpenAPI，删除 xray tag/endpoint/枚举值，保持接口语义一致。

## 影响范围

- **模块:**
  - 后端：domain / component / proxy / api / integration tests
  - 前端：theme / settings schema
  - SDK：types / README / dist
  - 文档：README / docs/* / OpenAPI / 知识库
- **文件:** 预计多文件联动（> 10），含测试与文档。
- **API:** 对外数据结构会发生破坏性变更（枚举值与字段删除）。
- **数据:** `state.json` 中若存在 `components.kind=xray` 或 `proxyConfig.preferredEngine=xray` 等历史数据，需要提供兼容处理（迁移/忽略/报错策略需在实现中明确）。

## 核心场景

### 需求: 不再出现任何 Xray 入口
**模块:** 前端 / SDK / OpenAPI / 后端 API

#### 场景: 用户在 UI 中查看/配置内核
删除所有 Xray 选项与面板。
- 预期结果: UI 中仅展示 sing-box/Clash（以及 auto，如仍保留）相关选项；不存在 “Xray” 文案或下拉选项。

#### 场景: 调用 API/SDK 获取组件与运行状态
- 预期结果: API/SDK 不再返回或接受 `xray` 类型值；OpenAPI 不再出现 xray tag/路径/枚举值。

### 需求: 运行时只可能启动 sing-box/clash
**模块:** 后端 proxy/service

#### 场景: 引擎推荐/选择
- 预期结果: 引擎选择逻辑不再考虑 Xray；推荐结果与原因仅在 sing-box/clash/auto 范围内。

#### 场景: 测速/探测
- 预期结果: 测速仅通过 sing-box 或 clash 启动测量代理；不存在任何 “寻找 xray 二进制/启动 xray” 的代码路径与测试。

## 风险评估

- **风险:** 破坏性变更（外部调用方/旧配置/旧状态文件仍可能包含 xray 相关字段与枚举值）。
  - **缓解:** 在实现阶段明确迁移策略：对旧字段进行兼容读取并忽略，或在加载时给出明确错误提示；同时更新文档说明迁移路径。
- **风险:** 前端与 SDK 产物需要同步更新，避免“代码删了但 dist/ 文档残留”。
  - **缓解:** 强制在任务清单中加入 `frontend/sdk` 构建与校验项，并把 `dist` 一并提交。
- **风险:** 测试用例大量依赖 xray（单测/集成测试）。
  - **缓解:** 删除/替换相关测试为 sing-box 路径验证，确保 `go test ./...` 稳定通过。

