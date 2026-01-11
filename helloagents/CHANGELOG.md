# Changelog

本文件记录项目所有重要变更。
格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/),
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 移除
- 移除 Xray 支持：对外接口/SDK/前端 UI 不再暴露任何 Xray 相关选项；引擎选择仅在 sing-box/clash/auto 范围内工作；旧 `state.json` 中的历史 Xray 字段会在加载时被清理并回落到可用引擎。

### 新增
- 增加核心组件卸载能力：新增 `POST /components/:id/uninstall`，并在前端组件面板提供“卸载”按钮（代理运行中会拒绝卸载正在使用的引擎）。
- 支持 Clash YAML 订阅解析：解析 `proxies` 并结合 `proxy-groups`/`rules` 自动生成订阅 FRouter（用于将订阅路由语义落到 Vea 的 `ChainProxySettings`）。
- 增加应用内“检查更新”能力：支持 Windows/macOS 从 GitHub Releases 获取最新稳定版并自动下载、安装与重启（Issue #24）。

### 变更
- 运行期数据与 artifacts 统一写入 userData（开发模式同样）；启动时会将仓库/可执行目录旁遗留的 `data/` 与 `artifacts/` 迁移到 userData 并清理源目录。

### 修复
- 修复速度单位显示不一致的问题：前端主题/SDK/OpenAPI 将速度单位从 `Mbps` 修正为 `MB/s`（与实际测速计算单位一致）。
- 修复前端“系统代理端口（proxy.port）”不生效的问题：端口变更会联动更新 `ProxyConfig.inboundPort`；内核运行中修改端口会自动重启并在系统代理启用时重新应用系统代理设置；启动时从后端同步实际端口避免 UI 默认值误导。
- 修复 Linux 下 mihomo(Clash) TUN 在默认 MTU=9000 时可能出现“看起来全网断开”的问题：当检测到未自定义的默认 TUN 组合时，自动将 MTU 调整为 1500。
- 修复 mihomo(Clash) TUN 模式在部分环境下因 DoH/QUIC 导致“可启动但访问异常/分流失效”的问题：默认开启 sniffer，并默认拒绝 QUIC（UDP/443）以强制回落到 TCP/HTTPS。
- 修复 Linux 下 sing-box TUN 模式可能出现“IP 通但域名解析卡死”的问题：默认远程 DNS 从 `8.8.8.8:53(TCP)` 改为 DoH（Cloudflare `1.1.1.1:443`）。
- 修复组件安装流程中 `.gz` 解压结果固定写入 `artifact.bin` 导致 mihomo 等单文件发行包安装不可靠的问题：改为使用 gzip header 中的原始文件名，并清理冗余归一化分支。
- 修复 clash 安装归一化过程中 `os.Chmod` 错误被忽略的问题：当无法设置可执行权限时，直接返回错误，避免后续运行时失败。
- 提取代理服务 TUN 默认值常量，避免 `applyConfigDefaults` 与默认判定逻辑重复导致的不一致风险。
- 前端主题抽取 `updateEngineSetting` 的公共刷新逻辑，减少重复代码并降低后续维护成本。
- 修复主题页切换内核引擎时禁用系统代理失败仍继续重启的问题：改为快速失败，避免旧代理被停止后系统代理仍指向旧进程导致网络中断。
- 修复 FRouter 路由规则列表的规则摘要显示：优先展示模板/首条匹配项，并在次行展示去向，避免仅显示节点名造成困惑。
- 修复 Clash YAML 订阅导入生成海量路由边的问题：按目标去向合并连续规则，显著减少边数量并保持规则顺序语义。
- 修复 sing-box rule-set 下载对 geoip/geosite tag 大小写敏感导致的 404：URL 构造做小写归一化（例如 `geoip-CN` → `geoip-cn`）。
- 修复 `GET /app/logs?since=` 参数校验：当 `since` 非非负整数时返回 400，避免静默回退到默认值造成误解。
- 修复 sing-box 启动 Shadowsocks+obfs 节点时报错 `plugin not found: obfs`：兼容 Clash/Mihomo 订阅的 `plugin: obfs` 写法并归一化为 `obfs-local`（simple-obfs）。
- 修复浅色主题下 FRouter 选中态高亮不明显/异常的问题：选中态改为黑色边框（Issue #33）。
- 修复 TUN 状态显示错误（Issue #32）：主题页 TUN 卡片主状态改为运行态展示，能力检查仅用于详情/指引。
- 修复订阅节点无法自动清理导致节点无限增长的问题：当订阅成功解析出节点时，按最新快照删除旧节点，避免节点越积越多。
- 修复订阅拉取节点的异常保护：订阅返回空内容时返回错误并保留现有节点与旧 payload（避免数据丢失）。
- 修复订阅面板配置行操作重复的问题：移除“刷新”按钮，仅保留“拉取节点”。
- 修复订阅面板同步失败时错误信息过长导致表格行高度被撑爆的问题：错误信息在状态列单行省略显示，完整信息通过悬浮提示查看。
- 修复 keepalive 在用户手动停止代理后仍可能自动拉起的问题：`POST /proxy/stop` 标记 userStopped 状态，keepalive 轮询尊重该状态。
- 修复 TUN 模式下 iptables 清理脚本完全静默错误的问题：规则不存在继续忽略，其他异常输出 `[TUN-Cleanup][WARN]` 便于排障。
- 减少 Linux TUN 模式提权弹窗次数：复用 pkexec 启动的 root helper，让 TUN 权限配置与冲突规则清理等特权操作共享一次授权。
- 修复 Clash YAML 订阅解析的可维护性问题：补齐核心解析/压缩单元测试，并在 proxy 名称重复时输出告警避免静默覆盖。
- 修复订阅返回空内容时错误提示可能误导的问题：文案改为“未更新节点（如有现有节点将保持不变）”。
- 修复订阅面板同步错误字段处理的冗余：去除多余 `String()` 转换。
- 修复创建订阅时等待拉取导致 UI 易误判卡死的问题：导入接口改为立即返回并后台拉取解析；主题页提示“后台拉取中…”并显示“未同步”状态；SDK 时间格式化对零值显示 `-`。
- 修复 Windows 发布版默认规则模板不可用的问题：electron-builder 打包文件清单补齐 `chain-editor/rule-templates.js`（Issue #27）。
- 修复首页“当前 IP”在代理运行时仍显示真实出口 IP（Issue #26）：`GET /ip/geo` 在代理运行且非 TUN 时通过本地入站代理探测出口 IP。
- 修复 IP Geo 探测未贯穿请求 context 的问题：API 请求取消/超时后可及时中断外部探测请求，避免无意义等待。
- 修复浅色主题日志面板“自动滚动”开关关闭态几乎不可见的问题：补齐 `--border-color` 变量（Issue #28）。
- 修复槽位功能不可用（Issue #29）：主题页在 FRouter 路由规则面板新增“槽位管理”，支持新增/重命名/绑定节点；保存图配置时保留 `positions`，避免意外清空布局数据。
- 修复主题页“检查应用更新”点击无响应的问题：修复 `showStatus` 作用域导致的静默异常，确保可触发 IPC 并在状态栏给出反馈。
- 修复主题页订阅导入后依赖固定 `setTimeout` 刷新的竞态问题：改为轮询配置 `lastSyncedAt` 变化，并在超时/失败时给出提示。
- 加固 Linux root helper 对 `artifactsRoot` 的推导与校验：`socketPath` 必须符合 `<ArtifactsRoot>/runtime/resolvectl-helper.sock`，并拒绝将根路径解析为 `/`，避免 capabilities 操作范围扩大。
- 清理订阅创建后台同步协程的冗余 bgCtx nil check：构造器已保证 bgCtx 非 nil。
- 修复订阅创建后台首次同步失败但 fallback payload 解析成功时状态不一致的问题：成功 fallback 会清空 `lastSyncError` 并更新 checksum，避免 UI 误标红。

## [0.0.1] - 2026-01-05

### 修复
- 修复 Linux 下 mihomo(Clash) TUN 模式可能出现“全网断网”的默认配置问题：对齐主流客户端的 TUN/DNS 默认值，并改进 DNS server 解析自举策略。
