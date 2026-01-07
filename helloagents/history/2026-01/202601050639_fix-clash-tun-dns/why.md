# 变更提案: 修复 Linux 下 mihomo(Clash) TUN 断网

## 需求背景
用户反馈在 Linux 下使用 mihomo(Clash) 内核开启 TUN 后出现“全网断网”，Vea 侧无明显报错，内核日志持续出现 DNS hijack 但连接不可用。问题高度怀疑与 **TUN/DNS 默认配置** 不合理有关，且用户明确要求参考主流项目的配置逻辑，而不是拍脑袋调参。

## 变更内容
1. 对齐主流客户端的 TUN 默认值（尤其是 MTU、stack 与 auto-redirect 行为）
2. 对齐主流客户端的 DNS 配置逻辑（支持带 scheme 的 nameserver，并提供自举用的 default-nameserver）
3. 为 PROCESS-NAME 规则启用 find-process-mode（保证自保规则可生效）

## 影响范围
- **模块:** backend
- **文件:** `backend/service/adapters/clash.go`、`backend/service/adapters/clash_test.go`
- **API:** 无
- **数据:** 无

## 核心场景

### 需求: Linux 下 TUN 可用性
**模块:** backend
在 Linux 开启 TUN 后，系统应保持可上网（至少不应因默认配置导致“全网断网”）。

#### 场景: mihomo TUN + DNS hijack
当用户启用 `InboundMode=tun` 且启用 `dns-hijack` 时：
- 内核能正常创建 TUN 设备
- DNS 能正常解析（包括代理节点域名解析自举）
- 默认配置不应引入高风险参数（如强制 auto-redirect、过高 MTU 等）

## 风险评估
- **风险:** 改动默认 DNS/TUN 行为可能影响少数用户习惯配置
- **缓解:** 保持用户显式配置优先；默认值向主流客户端对齐；增加单测覆盖默认行为
