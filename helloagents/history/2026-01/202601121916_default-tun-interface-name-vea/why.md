# 变更提案: 默认 TUN 网卡名改为 vea

## 需求背景

当前默认 TUN 网卡名为 `tun0`，存在几个实际问题：
1. **Linux 上命名冲突/可读性差**：`tun0` 过于通用，容易与其它软件/容器环境产生混淆；排障时也很难一眼识别“这是 Vea 的 TUN”。
2. **Windows/macOS 上名称不可控**：TUN 设备名在非 Linux 平台往往不稳定或不可控；“默认写死名称”会造成不必要的失败与误判。

因此希望将“默认配置值”统一为 `vea`，并在 **非 Linux** 平台把该字段视为“逻辑名称/占位默认值”，**默认不强制写入内核配置**，改用地址/网段等更稳定的条件做就绪判定。

## 变更内容

1. **后端默认值**：`defaultTunInterfaceName` 从 `tun0` 调整为 `vea`。
2. **非 Linux 默认策略**：当 `interfaceName` 为默认值 `vea`（或 legacy 默认 `tun0`）时，生成 sing-box/mihomo 配置时不写死设备名；后端 TUN 就绪判定按地址优先，避免依赖名称。
3. **兼容旧配置**：保持旧配置 `interfaceName=tun0` 在 Windows/macOS 仍可用（不强制写死名称、按地址就绪）。
4. **前端与文档同步**：前端 settings schema 与文档默认值同步为 `vea`，并说明非 Linux 默认行为。

## 影响范围

- **模块:**
  - `backend/service/proxy`（默认值与 TUN 就绪判定）
  - `backend/service/adapters`（sing-box / mihomo 配置生成）
  - `frontend/`（settings schema 默认值展示）
  - `docs/`（默认值与平台差异说明）
- **文件:**
  - `backend/service/proxy/service.go`
  - `backend/service/adapters/singbox.go`
  - `backend/service/adapters/clash.go`
  - `backend/service/adapters/*_test.go`、`backend/service/proxy/*_test.go`
  - `frontend/settings-schema.js`
  - `frontend/theme/dark/js/settings-schema.js`
  - `frontend/theme/light/js/settings-schema.js`
  - `docs/SING_BOX_INTEGRATION.md`
  - `helloagents/wiki/modules/backend.md`（如有相关描述）
- **API:** 无（字段不变，仅默认值与生成策略调整）
- **数据:** 无迁移；仅影响“新建/重置默认值”场景。既有配置如显式写了 `tun0` 会走兼容逻辑。

## 核心场景

### 需求: 默认 TUN 网卡名为 vea
**模块:** `backend/service/proxy`、`backend/service/adapters`、`frontend`

将默认 TUN 网卡名从 `tun0` 调整为 `vea`，并确保各处默认值一致（后端、前端 schema、文档）。

#### 场景: Linux 默认创建 vea
前置条件：Linux + InboundMode=TUN，且用户未显式配置 `tun.interfaceName`。
- 预期结果: 后端默认值为 `vea`。
- 预期结果: 生成 sing-box/mihomo 配置时显式写入 `interface_name/device=vea`。
- 预期结果: 后端 TUN 就绪判定按名称 `vea` 等待（避免误判）。

#### 场景: Windows/macOS 默认不强制名称
前置条件：Windows/macOS + InboundMode=TUN，且使用默认值 `tun.interfaceName=vea`（或为空被补齐）。
- 预期结果: 生成 sing-box/mihomo 配置时默认不写死设备名（不依赖 `vea`）。
- 预期结果: 后端 TUN 就绪判定优先按地址/网段确认实际创建的 TUN 已就绪。
- 预期结果: UI 仍展示默认值 `vea`（作为逻辑默认值），但不暗示“系统一定会出现名为 vea 的网卡”。

### 需求: 兼容旧配置 tun0
**模块:** `backend/service/proxy`、`backend/service/adapters`

对历史配置（`tun.interfaceName=tun0`）保持兼容，避免升级后出现“默认写死 tun0 导致失败”或“就绪判定误判”。

#### 场景: Windows/macOS 旧配置仍可用
前置条件：Windows/macOS + InboundMode=TUN，且既有配置 `tun.interfaceName=tun0`。
- 预期结果: 生成 sing-box/mihomo 配置时不写死设备名（将 `tun0` 视为 legacy 默认）。
- 预期结果: 后端 TUN 就绪判定按地址/网段确认就绪，不依赖名称 `tun0`。

## 风险评估

- **风险:** 默认值变更导致“新建配置”与“旧配置”行为差异，引发用户疑惑（尤其非 Linux 平台上名称不一定出现）。
  - **缓解:** 文档与 UI 文案明确“非 Linux 默认不强制名称”；并保留显式配置自定义名称的能力（仅在用户明确指定时尝试写入）。
- **风险:** 适配器对 device/interface_name 的写入逻辑调整可能引入回归。
  - **缓解:** 补齐/调整单元测试覆盖默认值与 legacy 行为；合并后运行 `go test ./...`。
