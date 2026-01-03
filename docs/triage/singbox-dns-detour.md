---
title: sing-box 启动失败：dns/udp[dns-local] detour to an empty direct outbound
created_at: 2025-12-19
---

# 现象

启动 sing-box 进程时立刻崩溃：

```
FATAL[0000] start service: start dns/udp[dns-local]: detour to an empty direct outbound makes no sense
```

注意：`sing-box check -c <config>` 可能通过，但 `sing-box run -c <config>` 会在运行期初始化 DNS service 时失败。

# 复现（最小）

环境：仓库内置 `artifacts/core/sing-box/sing-box-1.12.13-linux-amd64/sing-box`。

1) 生成一个包含 `dns-local` 且 `detour: "direct"` 的配置（示例取自当前代码生成逻辑）：

```json
{
  "outbounds": [
    { "type": "direct", "tag": "direct" }
  ],
  "dns": {
    "servers": [
      { "tag": "dns-local", "type": "udp", "server": "223.5.5.5", "detour": "direct" }
    ],
    "final": "dns-local"
  }
}
```

2) 运行：

```
timeout 2 artifacts/core/sing-box/sing-box-1.12.13-linux-amd64/sing-box run -c <config.json>
```

3) 预期输出上述 FATAL 并退出。

同时对比：

```
artifacts/core/sing-box/sing-box-1.12.13-linux-amd64/sing-box check -c <config.json>
```

可能不会报错（因为它不启动 service）。

# 根因（推断已验证）

sing-box 在运行期会拒绝「DNS server detour 指向一个 *空* 的 direct outbound」。

这里的“空 direct outbound”指：`type: "direct"` 但没有任何 dialer/出站细化参数（例如未设置 `bind_interface` / `routing_mark` / `inet4_bind_address` 等），此时 `dns.server.detour = "direct"` 被认为是无意义且会触发 fatal。

# 影响范围（当前仓库）

以下位置会生成 `dns-local detour: "direct"`，并且 direct outbound 在某些场景下没有 dialer 参数（会触发本问题）：

- 主配置（TUN layer）：`backend/service/adapters/singbox.go:186`
- 主配置（常规 DNS）：`backend/service/adapters/singbox.go:690`
- 测速配置（measurement）：`backend/service/adapters/singbox.go:882`

# 解决思路

推荐做法：对 “直连 DNS” 不显式填写 `detour: "direct"`，让其使用默认直连路径（或仅在 direct outbound *确实* 有 dialer 参数时才写 detour）。

验收建议：

- 用 `sing-box run -c <generated-config>` 进行最小启动验证（可用 `timeout` 杀掉进程即可）。
- 覆盖主代理配置与测速配置两个入口。

# 现状（2025-12-19）

- 主代理与测速均通过 `nodegroup` 编译计划入口生成配置，避免“测速旁路”带来的配置不一致问题。
- `dns-local` 默认不设置 `detour: "direct"`，避免 sing-box 运行期拒绝空 direct outbound。
