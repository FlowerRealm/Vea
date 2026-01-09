# 变更提案: 减少 Linux TUN 模式提权弹窗次数

## 需求背景

- Linux 下 TUN 模式涉及少量需要特权的操作（例如清理历史遗留的 iptables 规则、首次配置 setcap 等）。
- 现状：这些操作可能分别通过 `pkexec` 触发，导致用户在一次启动/切换过程中多次看到 polkit 授权弹窗。
- 目标：**不在应用内缓存 sudo/管理员密码**，但尽量做到“同一应用生命周期内只授权一次”。

## 变更内容

1. 复用现有 `resolvectl-helper`（pkexec 启动、socket IPC）的“单次授权”能力，扩展其能力范围：
   - 支持 `tun-cleanup`：清理可能与 TUN 冲突的历史 iptables/ip rule。
   - 支持 `tun-setup`：在 root helper 内完成 `vea-tun` 用户创建与内核二进制 `setcap` 配置。
2. 将 Linux TUN 相关的特权调用从“多次 pkexec”改为“同一 helper IPC”，避免重复弹窗。
3. 保持权限边界：root helper **只暴露白名单操作**，不接受任意命令执行。

## 影响范围

- **模块:** backend/shared + Linux root helper（主程序子命令）
- **文件:** `backend/service/shared/tun_linux.go`、`resolvectl_linux.go`、`backend/service/shared/root_helper_linux.go`
- **API/数据:** 无对外接口变更

## 核心场景

### 场景: tun-enable-single-auth
前置条件：Linux、启用 TUN（sing-box/mihomo 任一）
- 预期结果：需要特权时，**尽量只出现一次** polkit 授权弹窗；后续同生命周期内的特权动作复用同一 helper。

## 风险评估

- **风险:** root helper 复用授权会扩大“已授权窗口”，同一用户态进程可在窗口内调用白名单特权操作。
- **缓解:** socket 采用 0600 且 chown 到调用用户；helper 仅支持固定白名单 op，不提供任意命令执行能力；helper 监控父进程退出自动结束。

