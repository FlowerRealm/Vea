# backend 模块

## 职责
- 生成运行计划：将 `ProxyConfig + FRouter + Nodes + ChainProxySettings` 编译为可执行的 runtime plan
- 适配多内核：在 `backend/service/adapters/` 生成 sing-box / mihomo 的配置
- 进程管理：启动/停止内核，收集日志与状态
- TUN 就绪判定：Linux 以 `interface_name` 为准等待网卡就绪；Windows/macOS 以 TUN 地址识别实际网卡，默认不强制依赖网卡名称（`vea`/legacy `tun0`）
- 核心组件管理：安装/卸载 sing-box / mihomo 等核心组件
- 提供健康检查：`GET /health` 返回 `pid` 与 `userDataRoot`，便于前端判定“端口占用者是否为同一实例”，避免多实例导致后端启动即退出
- 配置/订阅解析：`backend/service/config` 解析分享链接与 Clash YAML（`proxies` + `proxy-groups` + `rules`）；创建时会从 payload 解析 Nodes（即使 `sourceUrl` 为空），订阅型配置可自动生成订阅 FRouter（`sourceConfigId` 关联）；创建订阅的后台首次同步失败时会尝试用创建时的 payload 作为 fallback 解析，成功后会清空同步错误并更新 checksum，避免“节点已生成但订阅仍标红失败”的状态不一致
- 主题包管理：`backend/service/theme` 提供主题目录扫描与 ZIP 导入/导出（支持主题包 `manifest.json` 展开子主题），并在 manifest 校验异常时输出告警日志便于排障

## 关键目录
- `backend/api/`：HTTP API
- `backend/service/`：核心业务逻辑
- `backend/service/adapters/`：内核适配器（本次变更涉及 `clash.go`）
- `backend/service/theme/`：主题包管理（`/themes`：list/import/export/delete；支持 `manifest.json` 主题包）

## 规范

### 需求: 订阅同步不破坏 FRouter 引用
**模块:** backend/service/config

订阅“拉取节点”/自动同步在更新节点集合时，应尽量复用“同一节点”的历史 `node.ID`（基于协议/地址/端口/安全/传输/TLS 指纹），避免节点 ID 变化导致 FRouter 的 `slots.boundNodeId`、`edges.to`、`edges.via` 引用断裂并在 UI 显示为“未知: {id}”。

#### 场景: 拉取节点后重启仍正常显示
- 条件: 已有 FRouter 引用订阅节点（自定义 FRouter 或订阅 FRouter）
- 预期结果: 同步后 FRouter 引用仍可解析到节点；Clash YAML 订阅生成的 `chainProxy` 会同步重写节点引用以保持一致

### 需求: 默认 TUN 网卡名为 vea
**模块:** backend/service/proxy, backend/service/adapters

将默认 TUN 网卡名从 `tun0` 调整为 `vea`，并确保后端默认值与配置生成策略一致。

#### 场景: Linux 默认创建 vea
- 条件: Linux + InboundMode=TUN，且用户未显式配置 `tun.interfaceName`
- 预期结果: 默认 `interfaceName=vea`，生成配置显式写入 `interface_name/device=vea` 并按名称等待就绪

#### 场景: Windows/macOS 默认不强制名称
- 条件: Windows/macOS + InboundMode=TUN，且 `tun.interfaceName` 为默认 `vea`
- 预期结果: 配置生成不写死设备名；TUN 就绪判定按地址识别实际网卡

### 需求: 兼容旧配置 tun0
**模块:** backend/service/proxy, backend/service/adapters

历史配置 `tun.interfaceName=tun0` 在 Windows/macOS 视为 legacy 默认占位值：默认不强制写死名称，并继续按地址判定就绪。

#### 场景: Windows/macOS 旧配置仍可用
- 条件: Windows/macOS + InboundMode=TUN，且 `tun.interfaceName=tun0`
- 预期结果: 配置生成不写死设备名；TUN 就绪判定按地址识别实际网卡

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
- [202601091540_fix-subscription-create-async](../../history/2026-01/202601091540_fix-subscription-create-async/) - 订阅：创建订阅接口立即返回，后台拉取解析并同步节点/FRouter，避免 UI 等待卡顿感
- [202601091553_fix-ip-geo-proxy](../../history/2026-01/202601091553_fix-ip-geo-proxy/) - 修复首页“当前 IP”在代理运行时仍显示真实出口 IP（Issue #26）
- [202601091707_fix-tun-polkit-prompts](../../history/2026-01/202601091707_fix-tun-polkit-prompts/) - 代理服务：减少 Linux TUN 模式提权弹窗次数，复用 root helper 避免多次 pkexec
- [202601092026_theme-package](../../history/2026-01/202601092026_theme-package/) - 主题包：主题目录化 + ZIP 导入/导出；Electron 从 userData/themes 加载；主题页提供导入/导出与切换
- [202601092132_fix-ip-geo-context](../../history/2026-01/202601092132_fix-ip-geo-context/) - 修复 IP Geo 探测未贯穿请求 context，支持取消/超时
- [202601100554_pr-review-hardening](../../history/2026-01/202601100554_pr-review-hardening/) - root helper：校验 socketPath 结构并拒绝将 artifactsRoot 解析为 `/`，避免 capabilities 操作范围扩大
- [202601100601_theme-pack-manifest](../../history/2026-01/202601100601_theme-pack-manifest/) - 主题包：支持 `manifest.json`（单包多子主题）；`GET /themes` 返回 `entry` 用于切换与启动加载
- [202601110913_fix-subscription-fallback-syncstatus](../../history/2026-01/202601110913_fix-subscription-fallback-syncstatus/) - 订阅：创建订阅后台首次同步失败但 fallback 解析成功时清理同步错误，避免 UI 误标红
- [202601111002_fix-review-log-port-probe](../../history/2026-01/202601111002_fix-review-log-port-probe/) - 代码审查跟进：ConfigCreate fallback 日志语义修正；系统代理默认端口常量；TUN readiness probe 去重
- [202601111339_theme-review-followups](../../history/2026-01/202601111339_theme-review-followups/) - 主题包：导入/导出维护性补强（常量复用、临时文件关闭简化、manifest 校验告警日志）
- [202601112056_fix-issue43-frouter-node-unknown](../../history/2026-01/202601112056_fix-issue43-frouter-node-unknown/) - 订阅：拉取节点时复用历史节点 ID，避免重启后 FRouter 节点显示未知（Issue #43 / #18）
- [202601112057_fix-issue37-38](../../history/2026-01/202601112057_fix-issue37-38/) - IP Geo：避免 busy 误判回落直连导致“当前 IP”显示真实出口；TUN 模式在存在入站端口时优先走本地入站探测（Issue #37）
- [202601112058_fix-issue-41-tun-windows](../../history/2026-01/202601112058_fix-issue-41-tun-windows/) - 代理服务：修复 Windows 下 sing-box TUN 启动因固定 `tun0` 就绪判定失败（Issue #41）
- [202601121916_default-tun-interface-name-vea](../../history/2026-01/202601121916_default-tun-interface-name-vea/) - 代理服务：默认 TUN 网卡名从 `tun0` 调整为 `vea`，并兼容 Windows/macOS 默认不强制设备名与 legacy `tun0`
- [202601131921_fix-backend-port-conflict](../../history/2026-01/202601131921_fix-backend-port-conflict/) - 健康检查：`/health` 增加 `pid/userDataRoot`；前端可据此避免端口冲突导致后端启动即退出
