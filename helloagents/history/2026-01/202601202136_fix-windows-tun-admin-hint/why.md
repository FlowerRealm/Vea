# 变更提案: 修复 Windows TUN 状态误报管理员权限

## 需求背景

- 现象：Windows 下设置页的 TUN 状态会显示“需要管理员/未配置”，但实际 TUN 可以正常启用与使用。
- 现状代码：前端通过 `/tun/check` 的 `configured` 字段判断是否“已配置”；后端 Windows 实现将 `configured` 绑定到 `isAdmin()`（`IsUserAnAdmin`）。
- 根因：Windows 上 **TUN 能否工作并不等价于“当前进程是否提升为管理员”**；在开发模式/某些发行方式下后端可能以普通用户态运行，但内核仍可成功创建/使用 TUN（或由驱动/系统策略允许），导致 `/tun/check` 误报。

## 变更内容

1. 调整 Windows 的 TUN 能力检查语义：`/tun/check.configured` 表示“无需额外一次性配置即可尝试运行 TUN”，而不是“当前进程具备管理员权限”。
2. 更新 `/tun/check` 在 Windows 下的提示文案：默认提示“无需额外配置；若启动失败再尝试以管理员运行”，避免误导。
3. （可选增强）当 Windows 上 TUN 启动失败且错误明显与权限相关时，返回更直接的提示（例如引导用户以管理员运行），但不在“能力检查”阶段提前误判。

## 影响范围

- **模块:** backend / frontend / docs
- **文件:**
  - backend/service/shared/tun_windows.go
  - backend/api/proxy.go
  - backend/service/proxy/service.go（可选）
  - frontend/theme/_shared/js/app.js（可选兜底）
  - docs/api/openapi.yaml（描述补充）
  - helloagents/wiki/modules/backend.md
  - helloagents/CHANGELOG.md
- **API:** 不新增接口；`/tun/check` 字段保持兼容，仅调整 Windows 下 `configured` 的语义与文案。
- **数据:** 无

## 核心场景

### 需求: tun-status-windows

**模块:** backend / frontend

Windows 下 TUN 状态应与实际运行一致，不再因“管理员检测”误报未配置。

#### 场景: tun-status-windows-no-false-admin-hint

前置条件:
- Windows 10/11
- Vea 后端进程可能以普通用户态运行（开发模式常见）
- TUN 实际可正常启用并工作

- 预期结果1: `/tun/check` 返回 `configured=true`，不再提示“需要管理员/未配置”。
- 预期结果2: 前端设置页显示 `已配置 (windows)` 或在启用后显示 `运行中/已启用`，不再出现误导性“需要管理员”提示。

#### 场景: tun-status-windows-permission-fail-hint

前置条件:
- Windows 环境中 TUN 启动确因权限失败（例如驱动/策略限制）

- 预期结果: 启动失败信息优先反映真实原因，并包含“可尝试以管理员身份运行”的可操作提示。

### 需求: tun-status-nonregression

**模块:** backend

Linux/macOS 的现有行为不回退（Linux 仍以 capabilities 配置为准；macOS 仍以 sudo/权限要求为准）。

#### 场景: tun-status-linux-unchanged

- 预期结果: Linux 下 `/tun/check` 的 `configured` 判定逻辑保持不变，仍能正确反映是否完成 setcap 配置。

## 风险评估

- **风险:** Windows 上将 `configured` 视作“无需额外配置”后，用户可能在“确实需要管理员”的环境里少了提前提示。
- **缓解:** 把“管理员提示”移动到**启动失败**场景（真实错误）中呈现；同时在 `/tun/check` 的 Windows 文案中提供温和提示（“如启动失败再尝试管理员运行”）。

