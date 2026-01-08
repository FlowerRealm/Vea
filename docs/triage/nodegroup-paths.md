# 节点群黑盒路径对照

## 现有路径（已收敛）

- 主代理启动：`proxy/service.go` → `nodegroup.CompileProxyPlan(engine, config, frouter, nodes)` → `adapter.BuildConfig`
- 测速执行：`proxy/speed_measurer.go` → `nodegroup.CompileMeasurementPlan(engine, inboundPort, frouter, nodes)` → `adapter.BuildConfig`

## 历史差异（已修复）

- 测速路径曾直接构建引擎配置，绕过 nodegroup 计划，导致主代理/测速配置不一致。
- 曾有引擎分支在链式代理边上未解析插槽节点，导致 slot 绑定在不同内核之间表现不一致。

## 仍需关注

- `RuntimePlan.Nodes` 已收敛为“实际会用到的节点集”（default/rules 命中的节点 + detour 上游闭包），避免无关节点导致配置失败/误选引擎。
- adapters 仍只负责 schema 渲染，业务决策（链式代理路径、节点选择）放在 nodegroup 编译层。
