# 技术设计: 移除 Xray 支持

## 技术方案

### 核心技术
- **后端:** Go（保持现有架构与包结构）
- **前端:** Electron + 主题页 HTML/JS（保持现有主题结构）
- **SDK:** `frontend/sdk`（现有 Rollup 构建产物 `dist/` 需要同步更新）

### 实现要点

1. **域模型收敛**
   - 删除 `domain` 层中所有 Xray 专用概念（引擎、组件 kind、配置结构、配置格式）。
   - 若存在“为了兼容旧 state.json”需要保留的字段，应在加载/迁移层做兼容，而不是在新模型里继续暴露 Xray。

2. **组件管理收敛**
   - `component/service` 中移除 Xray 的默认组件补齐、安装目录、release repo、清理逻辑与资产选择。
   - 删除与 Xray 相关的安装检测与单测。

3. **代理服务收敛**
   - 引擎推荐/选择只在 sing-box/clash/auto 范围内工作。
   - 测速模块的 adapters 映射移除 Xray；相关 switch 分支移除。
   - 清理 mixed 端口语义中的 “Xray: port+1” 规则与前端提示。

4. **API/OpenAPI/SDK/前端同步删除**
   - OpenAPI：移除 xray tag、xray 相关描述/示例/枚举值。
   - SDK：移除 `xray` 类型值与 `xray-json` 格式；更新 README 与 `dist/`。
   - 前端 theme：移除 Xray UI、状态轮询、配置项与文案。

5. **测试策略**
   - 删除所有 xray 依赖测试（组件测试、引擎选择测试、集成测试）。
   - 保留并增强 sing-box/clash 路径的单测覆盖，目标是 `go test ./...` 通过。

## 架构决策 ADR

### ADR-001: 移除 Xray 支持，仅保留 sing-box/clash
**上下文:** 当前项目存在双内核（Xray + sing-box）与多处分支逻辑，维护成本高且用户理解成本高。用户明确要求“删除整个项目对 Xray 的支持”。

**决策:** 删除所有 Xray 相关的对外能力与内部实现：域模型、组件管理、引擎选择、测速与集成测试、前端 UI、SDK 与文档。

**理由:**
- 目标明确：彻底移除意味着后续不再需要为 Xray 兼容做长期维护。
- 代码更简单：减少分支与边界条件（尤其是 mixed 端口差异与协议支持差异）。
- 用户体验更一致：统一以 sing-box 为主路径，避免“同功能不同内核行为不同”。

**替代方案:** 保留 `xray-json` 作为导入格式但不再运行 Xray → 拒绝原因: 仍然保留 Xray 相关数据结构与文档/SDK 负担，不符合“整个项目移除”。

**影响:**
- 破坏性变更：API/SDK/配置字段与枚举值删除，需要提供迁移说明。
- 旧数据处理：旧 `state.json` 可能包含 xray 数据，需要明确加载/迁移策略（见任务清单）。

## API 设计

本变更的 API 目标不是新增接口，而是**删除 xray 相关表述与枚举值**，并保持剩余接口语义不变：
- `/components` 仅允许 `singbox/clash/geo/generic`
- `/proxy/*` 与 `/tun/*` 的引擎推荐/选择不再出现 xray
- 配置格式 `ConfigFormat` 不再包含 `xray-json`（若后续仍需要“导入配置/订阅”，应以更通用的格式与命名重新定义，而不是挂在 xray 名下）

## 安全与性能

- **安全:** 不引入新权限；不变更 TUN 权限模型。注意不要删除“清理 XRAY/XRAY_SELF iptables 链”的 best-effort 清理逻辑（它是在清理历史遗留规则，不等同于支持 Xray）。
- **性能:** 移除分支逻辑后运行时开销更低；重点在于避免在热路径增加额外 JSON 解析/迁移成本。

## 测试与部署

- **测试:** `go test ./...` 必须通过；如前端/SDK 有构建流程，执行 `cd frontend/sdk && npm run build` 并提交 `dist/`。
- **部署:** 无新增部署步骤；但需要在发行说明/README 中明确迁移与 breaking changes。

