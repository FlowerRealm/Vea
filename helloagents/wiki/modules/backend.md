# backend 模块

## 职责
- 生成运行计划：将 `ProxyConfig + FRouter + Nodes + ChainProxySettings` 编译为可执行的 runtime plan
- 节点组解析：支持全局 `NodeGroup`（节点组）资源；`ChainProxySettings` 可引用 `NodeGroupID`，在编译/启动/测速前通过 resolver 将其解析为具体 `NodeID`（策略：最低延迟/最快速度/轮询/失败切换；执行路径会推进 cursor）
- 适配多内核：在 `backend/service/adapters/` 生成 sing-box / mihomo 的配置
- 进程管理：启动/停止内核，收集日志与状态
- 日志留存：应用日志 `app.log` 与内核日志 `kernel.log` 会轮转并保留最近 7 天，便于上传排障
- TUN 就绪判定：Linux 以 `interface_name` 为准等待网卡就绪；Windows/macOS 以 TUN 地址识别实际网卡（优先精确 IP 命中；地址无 mask 的 *net.IPAddr 仍视为精确；必要时用“新网卡 + MTU”兜底），默认不强制依赖网卡名称（`vea`/legacy `tun0`），且 Windows 下默认等待更久（25s）降低慢创建误判
- 代理状态：`GET /proxy/status` 会在 TUN 模式下返回 `tunIface`（实际创建/识别到的网卡名），用于 UI 展示与排障
- 核心组件管理：安装/卸载 sing-box / mihomo 等核心组件；当检测到已安装但 `lastVersion` 为空时，会从二进制路径/版本输出探测并回填用于 UI 展示
- 提供健康检查：`GET /health` 返回 `pid` 与 `userDataRoot`，便于前端判定“端口占用者是否为同一实例”，避免多实例导致后端启动即退出
- 配置/订阅解析：`backend/service/config` 解析分享链接与 Clash YAML（`proxies` + `proxy-groups` + `rules`）；创建时会从 payload 解析 Nodes（即使 `sourceUrl` 为空），订阅型配置可自动生成订阅 FRouter（`sourceConfigId` 关联）；创建订阅的后台首次同步失败时会尝试用创建时的 payload 作为 fallback 解析，成功后会清空同步错误并更新 checksum，避免“节点已生成但订阅仍标红失败”的状态不一致
- 主题包管理：`backend/service/theme` 提供主题目录扫描与 ZIP 导入/导出（支持主题包 `manifest.json` 展开子主题），并在 manifest 校验异常时输出告警日志便于排障

## 关键目录
- `backend/api/`：HTTP API
- `backend/service/`：核心业务逻辑
- `backend/service/adapters/`：内核适配器（本次变更涉及 `clash.go`）
- `backend/service/theme/`：主题包管理（`/themes`：list/import/export/delete；支持 `manifest.json` 主题包）

## 节点组（NodeGroup）
- **领域模型**：`backend/domain/entities.go`（`NodeGroupStrategy` / `NodeGroup`；`ServiceState.nodeGroups`）
- **仓储**：`backend/repository/interfaces.go`（`NodeGroupRepository`）；内存实现 `backend/repository/memory/nodegroup_repo.go`
- **业务服务**：`backend/service/nodegroups/service.go`（CRUD + `UpdateCursor`）
- **解析器**：`backend/service/nodegroup/nodegroup_resolver.go`（`ResolveFRouterNodeGroups`）
  - 规则：同名冲突时 `NodeID` 优先于 `NodeGroupID`
  - failover 可用性：`lastLatencyError==""` 视为可用（“连不上都算失败”）
  - cursor：round-robin / failover 在真实执行路径（启动/测速）会推进 cursor
- **API**：`backend/api/router.go`（`/node-groups` CRUD）；`docs/api/openapi.yaml` 维护接口定义

## 规范

### 需求: 订阅同步不破坏 FRouter 引用
**模块:** backend/service/config

订阅“拉取节点”/自动同步在更新节点集合时，应尽量复用“同一节点”的历史 `node.ID`（基于协议/地址/端口/安全/传输/TLS 指纹），避免节点 ID 变化导致 FRouter 的 `slots.boundNodeId`、`edges.to`、`edges.via` 引用断裂并在 UI 显示为“未知: {id}”。

**匹配/复用规则（由强到弱，且坚持“唯一才复用/唯一才重写”）：**
- `fingerprintKey`：协议/地址/端口/安全/传输/TLS 的规范化指纹（强匹配）
- `identityKey`：忽略部分易变字段，仅保留“身份核心”（弱匹配）
- `identity+name`：当 `identityKey` 发生冲突（同 identity 对应多个历史节点）时，用 `name` 做唯一消解；若 name 仍冲突则不复用

**引用修复：**
- 同步过程中若产生 `idMap`（`parsedID -> reusedID`），会统一重写订阅生成的 FRouter 以及所有 FRouter 的 `ChainProxySettings` 引用（`edges.from/to/via`、`slots.boundNodeId`、`positions` key），仅替换可确定映射

**稳定 ID 生成：**
- 统一使用 `domain.StableNodeIDForConfig`，避免 Clash YAML 解析与节点仓储在 ID 规则上分叉导致的回归风险

**分享链接订阅（share links）的稳定身份：**
- 分享链接订阅的节点参数（如 uuid/password/path/fp 等）可能发生滚动更新，单纯依赖指纹/identity 复用会在边界场景失效并导致 FRouter 引用断裂（UI 显示“未知: {uuid}”）。
- 为此订阅节点引入 `node.sourceKey`（订阅语义稳定键），用于跨拉取稳定复用同一节点的 `node.ID`。
  - `sourceKey` 与 `name` 解耦：即使用户在 UI 中改名订阅节点（仅改 `name/tags`），也不会影响后续同步对同一节点的匹配与复用。
  - 当入参节点未携带 `id` 且存在 `sourceKey` 时，仓储会优先用 `domain.StableNodeIDForSourceKey` 生成稳定 ID；同时同步逻辑会尽量复用历史 ID 并在必要时按 `sourceKey` 生成 `oldID -> newID` 映射修复 FRouter 引用。

#### 场景: 拉取节点后重启仍正常显示
- 条件: 已有 FRouter 引用订阅节点（自定义 FRouter 或订阅 FRouter）
- 预期结果: 同步后 FRouter 引用仍可解析到节点；Clash YAML 订阅生成的 `chainProxy` 会同步重写节点引用以保持一致

### 需求: 订阅用量字段（subscription-userinfo）
**模块:** backend/service/config, backend/domain

订阅同步时，从 HTTP 响应头 `subscription-userinfo` 解析 `upload/download/total` 字段并计算用量：
- `usageUsedBytes = upload + download`
- `usageTotalBytes = total`

解析失败/字段缺失不应影响订阅同步；用量字段保持不变（避免把已有值覆盖成 0）。即使订阅内容 checksum 未变化，也应允许用量随同步刷新。

#### 场景: 同步后返回用量字段
- 条件: 订阅响应头包含 `subscription-userinfo` 且字段可解析
- 预期结果: `GET /configs` 返回 `usageUsedBytes/usageTotalBytes`，供前端展示“已用/总量”

### 需求: 本地日志保留 7 天
**模块:** main.go, backend/service/proxy, backend/service/shared

- 后端启动时 `app.log` 会轮转为 `app-YYYYMMDD-HHMMSS.log` 并清理 7 天前的轮转文件
- 内核启动时 `kernel.log` 同样轮转，便于用户上传排障

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

### 需求: Windows /tun/check 不误报管理员权限
**模块:** backend/service/shared, backend/api

Windows 下 `/tun/check.configured` 的语义为“是否需要一次性配置动作”。
- Linux 需要 setcap/capabilities，因此 `configured=false` 表示“尚未完成一次性配置”
- Windows 通常无需一次性配置，因此 `configured` 不应绑定为“当前进程是否管理员”，避免出现“实际 TUN 可用但 UI 显示未配置/需要管理员”的误报

#### 场景: TUN 可用但状态不误报
- 条件: Windows 下 TUN 可正常启用与使用
- 预期结果: `/tun/check` 返回 `configured=true`；前端不再提示“需要管理员/未配置”

### 需求: 默认入站端口为 31346 + 端口占用 fail-fast
**模块:** backend/service/proxy, backend/repository/memory, backend/service/facade

Windows 下常见代理软件默认占用 `127.0.0.1:1080`，如果后端默认也使用 1080，会导致 sing-box `mixed` 入站启动失败并产生“mixed 不可用”的误判。
因此需要：
1) 默认端口与前端一致（降低冲突概率）；2) 启动前对端口占用做 fail-fast 并返回可操作提示（不自动换端口）。

#### 场景: 新安装/空配置默认端口
- 条件: `InboundMode != tun` 且未显式设置 `inboundPort`
- 预期结果: 默认 `inboundPort=31346`（与前端「系统代理 → 代理端口」默认值一致）

#### 场景: 端口被占用时启动失败
- 条件: 目标监听端口已被其他进程监听（例如 `127.0.0.1:1080`）
- 预期结果: `POST /proxy/start` 返回 400（`ErrInvalidData`），错误信息包含端口与处理建议（改端口/关闭占用者），且不误判为“内核已就绪”

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
- [202601151954_fix-proxy-inbound-port-bind](../../history/2026-01/202601151954_fix-proxy-inbound-port-bind/) - 修复 Windows 下 sing-box mixed 入站端口占用启动失败：默认端口调整为 31346，并在端口占用时 fail-fast 返回明确错误提示
- [202601161626_fix-issue55-subscription-node-id-reuse](../../history/2026-01/202601161626_fix-issue55-subscription-node-id-reuse/) - 订阅：拉取节点时复用历史节点 ID 并重写订阅链路引用，避免 FRouter 节点变为未知（Issue #55 / #18）
- [202601161628_fix-issue62-frouter-rename](../../history/2026-01/202601161628_fix-issue62-frouter-rename/) - FRouter：新增 `PUT /frouters/:id/meta`，并修复更新 FRouter 时未携带 tags 会意外清空 tags（Issue #62）
- [202601161631_fix-issue-61-copy-frouter](../../history/2026-01/202601161631_fix-issue-61-copy-frouter/) - FRouter：新增复制接口 `POST /frouters/:id/copy`（Issue #61）
- [202601161635_fix-issue59-delete-frouter](../../history/2026-01/202601161635_fix-issue59-delete-frouter/) - FRouter：删除后自动修复 `ProxyConfig.frouterId`（删到空集合自动创建默认 FRouter）（Issue #59）
- [202601162158_refactor-subscription-id-system](../../history/2026-01/202601162158_refactor-subscription-id-system/) - 订阅/代理：订阅节点复用补强（identity 冲突支持 identity+name）；TUN 就绪判定兜底与错误提示增强；应用/内核日志 7 天留存（Issue #32/#41/#57/#66）
- [202601202132_fix-issue68-component-version](../../history/2026-01/202601202132_fix-issue68-component-version/) - 组件管理：修复 sing-box/mihomo 版本号不显示（Issue #68）
