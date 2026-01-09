# 变更提案: 修复 Issue #32 - TUN 状态显示错误

## 需求背景
- 现象：用户在 Windows 10 上启用 TUN 后，界面提示“开启失败/未配置”，但实际代理流量仍可用（Issue #32，关联 #30）。
- 根因（基于当前代码）：首页的 “TUN 模式” 卡片主要展示 `GET /tun/check` 的结果，该接口语义是 **能力/权限检查**，并不等同于 **是否已启用/是否在运行**；从而造成状态误读与误报。

## 变更内容
1. 前端将 “TUN 模式” 卡片的主状态改为 **运行状态**（来自 `GET /proxy/status` 的 `running + inboundMode`），避免把能力检查当作运行状态。
2. 保留 `GET /tun/check` 的能力检查信息，仅用于 “详情/配置指引/权限提示”。
3. 启用/禁用 TUN 的交互文案与提示，改为与运行状态一致（成功/失败只反映启动/重启结果，不被能力文案干扰）。

## 影响范围
- 模块:
  - frontend/theme (dark/light)
- 文件:
  - frontend/theme/dark.html
  - frontend/theme/light.html
- API:
  - 不新增/不修改接口；继续使用 `/proxy/status`、`/tun/check`
- 数据:
  - 无

## 核心场景

### 需求: tun-status-display
**模块:** frontend
TUN 的主状态应表达“是否在 TUN 模式运行”，而不是“是否具备配置能力”。

#### 场景: tun-status-display-toggle
前置条件:
- 已选择一个 FRouter
- 核心可启动（已安装内核）

- 预期结果1: 启用 TUN 并启动内核后，卡片显示 `运行中`（running=true 且 inboundMode=tun）。
- 预期结果2: 禁用 TUN 后，卡片显示 `未启用`（inboundMode!=tun）或核心停止时显示相应状态。
- 预期结果3: 若启动失败，卡片不显示误导性成功；错误提示明确且不掩盖真实状态。

### 需求: tun-capability-info
**模块:** frontend
能力/权限检查仍可查看，但不作为主状态。

#### 场景: tun-capability-info-dialog
前置条件:
- 任意状态

- 预期结果1: 点击 TUN 卡片/详情，可查看 `platform/configured/setupCommand/description` 等能力信息（来自 `/tun/check`）。
- 预期结果2: 详情中同时展示当前运行状态摘要（来自 `/proxy/status`），便于定位问题。

## 风险评估
- **风险:** 仅前端展示语义调整，可能影响用户对“已配置/未配置”的理解。
- **缓解:** 将能力信息保留在详情与设置页，主状态仅展示运行态，文案明确“能力/权限” vs “运行”。

