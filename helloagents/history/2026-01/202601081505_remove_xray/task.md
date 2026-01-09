# 任务清单: 移除 Xray 支持

目录: `helloagents/plan/202601081505_remove_xray/`

---

## 1. 后端域模型（domain）
- [√] 1.1 在 `backend/domain/entities.go` 中删除 Xray 相关类型/字段/枚举（`xray-json`、`XrayConfiguration`、`XrayConfig`、引擎/组件的 xray 值），并修正 JSON 结构与默认值，验证 why.md#需求-不再出现任何-xray-入口-场景-调用-apisdk-获取组件与运行状态
- [√] 1.2 在 `backend/domain/proxy_config_patch.go` 中移除 `XrayConfig` patch 合并逻辑，验证 why.md#需求-不再出现任何-xray-入口-场景-调用-apisdk-获取组件与运行状态，依赖任务 1.1

## 2. 组件管理（component/service）
- [√] 2.1 在 `backend/service/component/service.go` 中移除 Xray 组件幂等补齐、安装/卸载、清理与资产候选逻辑，验证 why.md#需求-运行时只可能启动-sing-boxclash-场景-引擎推荐选择
- [√] 2.2 删除/改写 `backend/service/component/service_test.go` 与 `backend/api/components_test.go` 中依赖 xray 的测试断言，验证 `go test ./...`，依赖任务 2.1

## 3. 引擎推荐/选择与运行路径（proxy/service）
- [√] 3.1 在 `backend/service/proxy/engine_select.go` 中删除 Xray 候选与兼容性计算，收敛推荐结果字段，仅在 sing-box/clash/auto 范围内工作，验证 why.md#需求-运行时只可能启动-sing-boxclash-场景-引擎推荐选择
- [√] 3.2 在 `backend/service/proxy/speed_measurer.go` 中移除 Xray adapter 绑定与组件 kind 映射，仅保留 sing-box/clash，验证 why.md#需求-运行时只可能启动-sing-boxclash-场景-测速探测，依赖任务 3.1
- [√] 3.3 更新 `backend/service/proxy/*_test.go`（含 `engine_select_test.go`、`service_test.go`、`speed_measurer_test.go`）删除 xray 依赖并补齐 sing-box/clash 覆盖，验证 `go test ./...`

## 4. 集成测试与遗留逻辑清理
- [√] 4.1 删除或改写 `backend/service/integration_test.go` 中 “本地 xray 服务端 → xray 客户端” 的集成测试，改为 sing-box 路径或直接移除，验证 `go test ./...`
- [√] 4.2 审查 `backend/service/shared/tun_linux.go` 中对 XRAY/XRAY_SELF 的清理逻辑：保留“清理遗留规则”的 best-effort 行为，但去除任何“项目仍支持 Xray”的误导性表述，验证 why.md#需求-运行时只可能启动-sing-boxclash-场景-引擎推荐选择

## 5. 前端 UI（theme/settings）
- [√] 5.1 在 `frontend/theme/light.html` 与 `frontend/theme/dark.html` 中移除 Xray 选项、Xray 状态面板与 `updateXrayUI/refreshXrayStatus` 相关逻辑，验证 why.md#需求-不再出现任何-xray-入口-场景-用户在-ui-中查看配置内核
- [√] 5.2 在 `frontend/settings-schema.js` 中移除 `xray.*` 设置项与相关说明文案（包括 mixed 端口 +1），验证 why.md#需求-不再出现任何-xray-入口-场景-用户在-ui-中查看配置内核

## 6. SDK 与 OpenAPI/文档同步
- [√] 6.1 在 `frontend/sdk/src/types.d.ts` 与 `frontend/sdk/src/vea-sdk.js` 中移除 xray 相关类型/注释/示例，并更新 `frontend/sdk/README.md`，验证 why.md#需求-不再出现任何-xray-入口-场景-调用-apisdk-获取组件与运行状态
- [√] 6.2 执行 `cd frontend/sdk && npm run build` 生成并提交 `frontend/sdk/dist/*`（确保 dist 不再出现 Xray 文案/类型）
- [√] 6.3 更新 `docs/api/openapi.yaml`、`README.md`、`docs/SING_BOX_INTEGRATION.md`、`docs/CHANGES.md` 等文档移除 Xray 描述与枚举，验证 why.md#需求-不再出现任何-xray-入口-场景-调用-apisdk-获取组件与运行状态

## 7. 兼容与迁移策略（旧 state/config）
- [√] 7.1 明确并实现旧 `state.json` 的处理策略：若读取到 `preferredEngine=xray` 或 `components.kind=xray` 等字段，采取“忽略并回落到 sing-box/auto”或“给出明确错误”中的一种，并在实现与文档中写清楚（避免 silent 失败）

## 8. 安全检查
- [√] 8.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）；重点确认不引入新的特权操作、不误删 TUN 清理中的安全兜底

## 9. 文档与知识库更新
- [√] 9.1 同步更新知识库：`helloagents/wiki/overview.md`、`helloagents/wiki/arch.md`、`helloagents/wiki/modules/backend.md`、`helloagents/wiki/modules/frontend.md` 移除 Xray 表述；更新 `helloagents/CHANGELOG.md` 记录“移除 Xray 支持”

## 10. 测试
- [√] 10.1 执行 `go test ./...`，确保通过；如存在与环境相关的跳过条件，确保跳过理由仍成立且不再依赖 xray 二进制
