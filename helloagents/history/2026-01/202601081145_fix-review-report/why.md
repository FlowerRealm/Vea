# 变更提案: 代码审查报告问题修复（后续跟进）

## 需求背景
本次变更针对代码审查报告中指出的可维护性/可观测性问题做补齐与收敛，目标是让行为更符合直觉、日志更可追踪、测试更易定位。

## 变更内容
1. 为 Clash YAML 解析核心逻辑补齐独立单元测试（解析与规则压缩）。
2. 统一 Clash 规则优先级归一化逻辑，减少维护歧义。
3. Clash 订阅中 proxy 名称重复时给出明确告警（避免静默覆盖）。
4. TUN 模式下 iptables 清理：规则不存在继续静默，但对“非预期错误”输出 warn 日志。
5. 内核 keepalive 尊重用户显式停止：用户调用 stop 后不再被自动拉起。
6. keepalive 回退选择默认 FRouter 时使用显式/稳定策略（按最早 CreatedAt）。
7. 订阅返回空内容的错误提示更准确（避免“已保留现有节点”在无节点时误导）。
8. 前端订阅表格同步错误字符串处理去冗余。

## 影响范围
- **模块:** backend、frontend、知识库（helloagents）
- **文件:** 见 task.md 清单
- **API:** `GET /proxy/status` 增加可选字段 `userStopped/userStoppedAt`（仅在用户显式停止后出现）
- **数据:** 无

## 核心场景

### 需求: 订阅解析可维护性
**模块:** backend

#### 场景: 单元测试快速定位解析问题
当 Clash YAML 订阅解析/规则压缩逻辑变更时：
- 单元测试可直接覆盖 `parseClashProxyToNode` / `compactClashSelectionEdges`
- 出现回归时能快速定位到解析/压缩层，而非依赖集成测试排查

### 需求: 代理生命周期行为符合直觉
**模块:** backend

#### 场景: 用户手动停止后不会被 keepalive 自动拉起
用户通过 `POST /proxy/stop` 停止代理后：
- keepalive 不应在下一个轮询周期自动 `StartProxy`

## 风险评估
- **风险:** `GET /proxy/status` 响应字段变化（新增可选字段）
- **缓解:** 仅新增字段、保持兼容；前端不依赖则无影响

