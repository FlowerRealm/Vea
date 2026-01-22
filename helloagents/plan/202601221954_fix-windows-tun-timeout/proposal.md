# 变更提案：修复 Windows 下 TUN 启动 10s 超时（sing-box / clash）

## 背景与现象

- 用户反馈：Windows 上启用 TUN（设置页开关）后，等待约 10 秒即报错并回退，TUN 无法正常使用。
- 影响范围：`sing-box` 与 `clash(mihomo)` 两种内核均出现。
- 当前仓库已存在相关已关闭 Issue（例如 `#30/#32/#41`），其中一部分属于“内核已运行但 UI/后端就绪判定误报失败”的问题；但仍有用户在新版本上复现“10s 超时并失败”。

## 目标（Goals）

1. Windows 上启用 TUN 时：只要内核实际创建/接管成功，应稳定判定为“已就绪”，不再因识别失败导致 10s 超时。
2. 若 TUN 实际启动失败：错误应可操作（包含 `kernel.log` 路径、关键提示、必要的诊断信息），避免“只看到超时”。
3. 不回退 Linux 行为：Linux 继续依赖明确的 `interface_name` 与释放等待逻辑（避免 EBUSY/残留）。

## 非目标（Non-Goals）

- 不在本次直接实现“自动安装/修复 Wintun 驱动”的完整链路（可能涉及签名/权限/分发策略）。
- 不新增复杂的跨平台网络诊断 UI（优先用日志与最小必要的诊断输出）。

## 初步根因假设（基于现有实现与 Issue 归纳）

> 需要通过日志验证；以下是用于规划的候选根因。

### 假设 A：TUN 实际已起来，但被后端“就绪判定”误杀/误判

- 当前后端在 `StartProxy(TUN)` 后，会等待 TUN 网卡在 10 秒内“就绪”，否则主动停止内核并返回失败。
- Windows 下 TUN 网卡可能：
  - 复用历史已存在的虚拟网卡（非“新网卡”语义）；
  - 网卡名是本地化的通用名称（如“以太网 X”），不包含 `wintun/tun` 关键字；
  - 地址绑定/可见性滞后（短时间内 `net.Interface.Addrs()` 不可靠）。

### 假设 B：Wintun 不可用（缺失/被策略阻止/被安全软件拦截）

- 两种内核均依赖 Windows 的 TUN 驱动能力（常见为 Wintun）；当驱动不可用时，内核可能启动但无法创建设备或无法配置路由，表现为“等待就绪直到超时”。

### 假设 C：启动链路缺少足够的诊断输出，导致用户只能感知“超时”

- 即便已有 `kernel.log`，用户未必能在 UI 中快速定位路径、理解下一步（导致反馈信息不足，问题难以闭环）。

## 拟定方案概览

### 1) 后端：增强 Windows/macOS 的 TUN 就绪识别与错误可观测性

- 调整 `backend/service/proxy/service.go` 的 `waitForTUNReadyByAddress()`：
  - 继续优先：**新网卡 + 地址匹配**。
  - 新增兜底：当地址匹配成立但网卡不是“新网卡”时，也允许在短暂等待后判定为就绪（并优先选择 `Up` 的接口）。
  - 保持 Linux 路径不变（仍使用 `waitForTUNReadyWithIndex`）。
- 仅 Windows：将 TUN 就绪等待上限从 10s 调整为更宽松的值（建议 20–30s），降低慢设备/首次创建的误判概率。
- 当超时失败：错误信息中追加（控制长度）：
  - 期待的 CIDR（例如 `172.19.0.1/30` / `198.18.0.1/30`）；
  - 观测到的候选网卡名（不输出完整本机网络信息，避免隐私泄露）；
  - `kernel.log` 路径（已存在则强化展示）。

### 2) 前端：把“失败”变成可执行动作

- 启用 TUN 失败时，提示用户：
  - 立即打开“日志”面板；
  - 复制 `kernel.log` 路径/内容后再反馈（Issue/群/私信）。
- （可选）对常见关键词（`wintun`, `access denied`, `requires elevation`）给出提示：管理员/Wintun/安全软件。

### 3) 组件与文档：补齐 Windows 的“Wintun 可用性”路径

- 文档 `docs/SING_BOX_INTEGRATION.md` 补充：
  - “出现 `TUN interface not ready after 10s` 时首先查看 kernel.log”的步骤；
  - Windows 下 Wintun 常见问题与自查项。
- （可选）后端对 Windows 组件安装目录做轻量自检：当用户启用 TUN 时若检测到 `wintun.dll` 明显缺失，提前给出明确提示（不做自动安装）。

## 验收标准（Acceptance Criteria）

- Windows 10/11 x64（管理员运行）：
  - 启用 TUN：不再稳定复现“等待 10s 超时失败”；
  - `/proxy/status` 显示 `running=true` 且 `inboundMode=tun`；
  - 失败时错误信息包含 `kernel.log` 路径与下一步建议。
- 回归：
  - `go test ./...` 通过；
  - Linux 侧 TUN 快速重启不回退（避免 EBUSY/残留）。

