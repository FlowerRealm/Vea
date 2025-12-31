# NodeGroup 黑盒设计

## 目标

- 对外提供稳定的“节点群黑盒”接口：外部只关心输入与输出，不关心内部链式路由细节。
- 主代理与测速统一走同一编译管线，避免配置语义分叉。
- adapters 只负责 schema 渲染，业务决策收敛到 nodegroup。

## 黑盒边界

**输入**
- ProxyConfig / FRouter / Nodes / ChainProxySettings
- Purpose：`proxy` / `measurement`
- Engine：`xray` / `sing-box`

**输出**
- RuntimePlan（引擎无关的中间表示）
- 诊断信息（Explain）

## RuntimePlan 关键字段

- `Purpose`：用途区分（主代理 / 测速）
- `Engine`：内核类型
- `ProxyConfig`：运行配置（单例）
- `FRouterID` / `FRouterName`：当前 FRouter
- `Nodes`：参与计划的节点集合
- `Compiled`：FRouter 编译产物（含规则/链路/默认目标等）
- `InboundMode` / `InboundPort`：入站需求

## 编译管线

1. NormalizeChainProxySettings：统一虚拟节点 ID（local/destination → client/target）
2. CompileChainPlan：解析 slot，得到线性链路与节点集
3. ActiveNodeIDs：合并 target 节点
4. Render：adapter 根据 RuntimePlan 输出各引擎配置

## 对外接口建议

- `CompileProxyPlan(engine, config, frouter, nodes) -> RuntimePlan`
- `CompileMeasurementPlan(engine, inboundPort, frouter, nodes) -> RuntimePlan`
- `BuildConfig(plan, geo) -> config bytes`
- `Explain(plan) -> 可读诊断`
