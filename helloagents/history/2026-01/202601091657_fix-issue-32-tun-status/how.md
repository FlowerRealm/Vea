# 技术设计: 修复 Issue #32 - TUN 状态显示错误

## 技术方案

### 核心技术
- 前端：Electron + HTML（theme）直接改 UI 逻辑
- 复用现有 API：`GET /proxy/status`（运行态）、`GET /tun/check`（能力态）

### 实现要点
- 将现有 `tunStatusCache` 语义拆分：
  - `tunCapabilityCache`：保存 `/tun/check` 返回，用于权限提示/配置指引。
  - 运行态直接取 `coreStatus`（`/proxy/status` 已轮询/刷新）。
- `updateTUNUI()` 以运行态为主：
  - `coreStatus.running && coreStatus.inboundMode === 'tun'` → `运行中`（success）
  - `!coreStatus.running && coreStatus.inboundMode === 'tun'` → `已启用（未运行）`（warning）
  - `coreStatus.inboundMode !== 'tun'` → `未启用`（secondary）
  - 能力态异常（`tunCapabilityCache.error`）仅在详情里显示，不覆盖主状态。
- `showTUNStatusDialog()` 在现有弹窗/状态展示上补充两块信息：
  - 运行状态：engine/running/inboundMode
  - 能力检查：platform/configured/setupCommand/description/error

## API 设计
- 不改现有后端接口；不新增端点。

## 安全与性能
- **安全:** 不涉及权限提升与敏感信息；仅 UI 展示调整。
- **性能:** 复用现有轮询结果，不新增高频 API 请求。

## 测试与部署
- **手动验证:** Windows 10 + sing-box：
  1. 选择 FRouter，启用 TUN，观察卡片显示 `运行中`；
  2. 禁用 TUN，观察卡片显示 `未启用`；
  3. 故意制造启动失败（例如不选择 FRouter），确认提示与卡片状态一致；
  4. 点击详情，确认能力态与运行态都能展示。
- **发布风险:** 仅前端 UI 改动，风险低。

