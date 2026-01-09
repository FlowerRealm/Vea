# backend 模块

## 职责
- 生成运行计划：将 `ProxyConfig + FRouter + Nodes + ChainProxySettings` 编译为可执行的 runtime plan
- 适配多内核：在 `backend/service/adapters/` 生成 sing-box / mihomo 的配置
- 进程管理：启动/停止内核，收集日志与状态
- 核心组件管理：安装/卸载 sing-box / mihomo 等核心组件
- 订阅解析：`backend/service/config` 解析分享链接与 Clash YAML（`proxies` + `proxy-groups` + `rules`），同步 Nodes 并自动生成订阅 FRouter（`sourceConfigId` 关联）

## 关键目录
- `backend/api/`：HTTP API
- `backend/service/`：核心业务逻辑
- `backend/service/adapters/`：内核适配器（本次变更涉及 `clash.go`）

## 变更历史
- [202601050639_fix-clash-tun-dns](../../history/2026-01/202601050639_fix-clash-tun-dns/) - 修复 Linux 下 mihomo TUN 断网（默认配置对齐主流客户端）
- [202601051238_fix-clash-tun-mtu](../../history/2026-01/202601051238_fix-clash-tun-mtu/) - 修复 Linux 下 mihomo TUN 默认 MTU=9000 导致“看起来全网断开”的问题（按选中引擎自动调整为 1500）
- [202601071130_fix-gz-extract-clash-install](../../history/2026-01/202601071130_fix-gz-extract-clash-install/) - 组件管理：新增核心组件卸载接口；修复 .gz 解压命名并清理 clash 安装归一化冗余逻辑
- [202601071248_refactor-tun-defaults-engine-ui](../../history/2026-01/202601071248_refactor-tun-defaults-engine-ui/) - 代理服务：提取 TUN 默认值常量并复用默认判定逻辑，降低重复与不一致风险
- [202601071306_fix-chmod-engine-switch-proxy-failfast](../../history/2026-01/202601071306_fix-chmod-engine-switch-proxy-failfast/) - 组件管理：clash 安装归一化补齐 chmod 错误处理，避免安装后二进制缺少执行权限
- [202601080729_compact-clash-rules-by-target](../../history/2026-01/202601080729_compact-clash-rules-by-target/) - 订阅解析：Clash YAML rules 按目标去向合并连续规则，避免生成海量路由边
- [202601080815_fix-singbox-ruleset-tag-case](../../history/2026-01/202601080815_fix-singbox-ruleset-tag-case/) - 代理服务：sing-box rule-set 下载对 geoip/geosite tag 做小写归一化，避免 `geoip-CN` 触发 404
- [202601081053_fix-review-followups](../../history/2026-01/202601081053_fix-review-followups/) - 代码审查跟进：去重 plugin-opts 解析/归一化、Clash 规则优先级连续化与订阅解析选择优化
- [202601081145_fix-review-report](../../history/2026-01/202601081145_fix-review-report/) - 代码审查跟进：Clash 解析单测补齐、重复 proxy 告警、keepalive 尊重用户 stop、TUN 清理日志增强与空订阅提示修正
- [202601081334_fix-singbox-tun-dns-doh](../../history/2026-01/202601081334_fix-singbox-tun-dns-doh/) - 代理服务：修复 sing-box TUN 模式下默认远程 DNS 使用 53 端口导致的“用一段时间后域名解析卡死”
- [202601082055_fix-clash-tun-sniffer-quic](../../history/2026-01/202601082055_fix-clash-tun-sniffer-quic/) - 代理服务：mihomo(Clash) TUN 默认开启 sniffer，并默认阻断 QUIC（UDP/443）提升可用性
- [202601091503_fix-speed-unit-mbs](../../history/2026-01/202601091503_fix-speed-unit-mbs/) - 测速：修正文档/注释的速度单位说明为 `MB/s`（与实际测速计算单位一致）
- [202601091512_fix-subscription-node-prune](../../history/2026-01/202601091512_fix-subscription-node-prune/) - 订阅节点：拉取成功后按快照清理旧节点，避免节点累积
