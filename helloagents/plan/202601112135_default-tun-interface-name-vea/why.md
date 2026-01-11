# 变更提案: 默认 TUN 网卡名改为 vea

## 需求背景
当前项目在 TUN 模式下使用 `tunSettings.interfaceName` 作为“默认网卡名”占位值，前端/后端/文档里默认均为 `tun0`。在 Linux 上该值会作为实际 TUN 设备名使用；在 Windows/macOS 上默认不强制写死名称，避免因设备名不可控导致就绪误判。

本次希望将默认网卡名从 `tun0` 调整为 `vea`，与产品命名保持一致，并在全代码库内统一默认值描述与实现。

## 变更内容
1. 将 `tunSettings.interfaceName` 的默认值从 `tun0` 改为 `vea`（前端 schema 与后端默认值一致）。
2. 兼容旧默认值 `tun0`：在非 Linux 平台仍将其视为“默认占位值”，保持不强制写死网卡名的策略，避免回归历史问题。
3. 同步更新相关文档与测试用例，确保默认值与行为一致。

## 影响范围
- **模块:**
  - `backend/service/proxy`（默认值与 TUN 就绪判定）
  - `backend/service/adapters`（sing-box / mihomo 配置生成）
  - `frontend`（设置 schema 默认值）
  - `docs`、`helloagents/wiki`（默认值说明与行为描述）
- **文件（预期）:**
  - `backend/service/proxy/service.go`
  - `backend/service/adapters/singbox.go`
  - `backend/service/adapters/clash.go`
  - `frontend/settings-schema.js`
  - `frontend/theme/light/js/settings-schema.js`
  - `frontend/theme/dark/js/settings-schema.js`
  - `docs/SING_BOX_INTEGRATION.md`
  - `helloagents/wiki/modules/backend.md`
- **API:** 无新增/删除字段，仅默认值与行为调整（`tunSettings.interfaceName` 字段仍保持不变）。
- **数据:** 可能存在历史配置仍为 `tun0`（旧默认值）。

## 核心场景

### 需求: 默认 TUN 网卡名为 vea
**模块:** backend/service/proxy + frontend

#### 场景: Linux 默认创建 vea
- 条件: Linux + `inboundMode=TUN` + `tunSettings.interfaceName` 未显式改动（为空或为默认值）
- 预期结果:
  - sing-box/mihomo 配置生成使用 `vea` 作为 TUN 设备名
  - 后端按名称等待 `vea` 网卡就绪

#### 场景: Windows/macOS 默认不强制名称
- 条件: Windows/macOS + `inboundMode=TUN` + `tunSettings.interfaceName` 为默认值 `vea`
- 预期结果:
  - 配置生成不写死网卡名（让内核/驱动自动选择实际名称）
  - 后端按 TUN 地址识别实际网卡并判定就绪

### 需求: 兼容旧配置 tun0
**模块:** backend/service/proxy + backend/service/adapters

#### 场景: Windows/macOS 旧配置仍可用
- 条件: Windows/macOS + 历史配置 `tunSettings.interfaceName=tun0`
- 预期结果:
  - 仍将 `tun0` 视为旧默认占位值，不强制依赖名称
  - 不回归 “等待 tun0 超时 → 误判未就绪 → 误杀内核” 的问题

#### 场景: Linux 旧配置仍保留原行为
- 条件: Linux + 历史配置 `tunSettings.interfaceName=tun0`
- 预期结果:
  - 仍按用户/历史配置使用 `tun0` 作为实际设备名

### 需求: 用户显式自定义 interfaceName
**模块:** backend/service/proxy + backend/service/adapters

#### 场景: Linux 自定义名称生效
- 条件: Linux + `tunSettings.interfaceName` 为非默认值（如 `mytun`）
- 预期结果: 配置与就绪判定按 `mytun` 执行

## 风险评估
- **风险:** 平台差异导致非 Linux 下网卡名不可控/不稳定，默认值变更可能让旧配置 `tun0` 被误当成“自定义名”，从而回归就绪误判。
  - **缓解:** 在非 Linux 路径将 `tun0` 视为 legacy 默认占位值，与 `vea` 一样不强制写死名称；就绪判定保持按地址识别。
- **风险:** Linux 上若存在同名网卡 `vea` 可能发生冲突。
  - **缓解:** 保留 `tunSettings.interfaceName` 覆盖能力；失败时错误信息应明确提示冲突与可通过设置更换名称。

