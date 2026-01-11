# 变更历史索引

本文件记录所有已完成变更的索引，便于追溯和查询。

## 索引

| 时间戳 | 功能名称 | 类型 | 状态 | 方案包路径 |
|--------|----------|------|------|------------|
| 202601050639 | fix-clash-tun-dns | 修复 | ✅已完成 | [2026-01/202601050639_fix-clash-tun-dns](2026-01/202601050639_fix-clash-tun-dns/) |
| 202601051238 | fix-clash-tun-mtu | 修复 | ✅已完成 | [2026-01/202601051238_fix-clash-tun-mtu](2026-01/202601051238_fix-clash-tun-mtu/) |
| 202601071130 | fix-gz-extract-clash-install | 修复 | ✅已完成 | [2026-01/202601071130_fix-gz-extract-clash-install](2026-01/202601071130_fix-gz-extract-clash-install/) |
| 202601071248 | refactor-tun-defaults-engine-ui | 重构 | ✅已完成 | [2026-01/202601071248_refactor-tun-defaults-engine-ui](2026-01/202601071248_refactor-tun-defaults-engine-ui/) |
| 202601071306 | fix-chmod-engine-switch-proxy-failfast | 修复 | ✅已完成 | [2026-01/202601071306_fix-chmod-engine-switch-proxy-failfast](2026-01/202601071306_fix-chmod-engine-switch-proxy-failfast/) |
| 202601080702 | fix-frouter-rule-list-label | 修复 | ✅已完成 | [2026-01/202601080702_fix-frouter-rule-list-label](2026-01/202601080702_fix-frouter-rule-list-label/) |
| 202601080729 | compact-clash-rules-by-target | 修复 | ✅已完成 | [2026-01/202601080729_compact-clash-rules-by-target](2026-01/202601080729_compact-clash-rules-by-target/) |
| 202601080815 | fix-singbox-ruleset-tag-case | 修复 | ✅已完成 | [2026-01/202601080815_fix-singbox-ruleset-tag-case](2026-01/202601080815_fix-singbox-ruleset-tag-case/) |
| 202601080848 | fix-subscription-pull-refresh-duplicate | 修复 | ✅已完成 | [2026-01/202601080848_fix-subscription-pull-refresh-duplicate](2026-01/202601080848_fix-subscription-pull-refresh-duplicate/) |
| 202601080900 | fix-subscription-error-message-overflow | 修复 | ✅已完成 | [2026-01/202601080900_fix-subscription-error-message-overflow](2026-01/202601080900_fix-subscription-error-message-overflow/) |
| 202601081053 | fix-review-followups | 修复 | ✅已完成 | [2026-01/202601081053_fix-review-followups](2026-01/202601081053_fix-review-followups/) |
| 202601081145 | fix-review-report | 修复 | ✅已完成 | [2026-01/202601081145_fix-review-report](2026-01/202601081145_fix-review-report/) |
| 202601081334 | fix-singbox-tun-dns-doh | 修复 | ✅已完成 | [2026-01/202601081334_fix-singbox-tun-dns-doh](2026-01/202601081334_fix-singbox-tun-dns-doh/) |
| 202601081339 | fix-proxy-port-sync | 修复 | ✅已完成 | [2026-01/202601081339_fix-proxy-port-sync](2026-01/202601081339_fix-proxy-port-sync/) |
| 202601081553 | user-data-root | 重构 | ✅已完成 | [2026-01/202601081553_user_data_root](2026-01/202601081553_user_data_root/) |
| 202601081505 | remove-xray | 变更 | ✅已完成 | [2026-01/202601081505_remove_xray](2026-01/202601081505_remove_xray/) |
| 202601082055 | fix-clash-tun-sniffer-quic | 修复 | ✅已完成 | [2026-01/202601082055_fix-clash-tun-sniffer-quic](2026-01/202601082055_fix-clash-tun-sniffer-quic/) |
| 202601092026 | theme-package | 变更 | ✅已完成 | [2026-01/202601092026_theme-package](2026-01/202601092026_theme-package/) |
| 202601100601 | theme-pack-manifest | 变更 | ✅已完成 | [2026-01/202601100601_theme-pack-manifest](2026-01/202601100601_theme-pack-manifest/) |

## 按月归档

### 2026-01

- [202601050639_fix-clash-tun-dns](2026-01/202601050639_fix-clash-tun-dns/) - 修复 Linux 下 mihomo(Clash) TUN 默认配置导致的断网
- [202601051238_fix-clash-tun-mtu](2026-01/202601051238_fix-clash-tun-mtu/) - 修复 Linux 下 mihomo(Clash) TUN 默认 MTU=9000 导致“看起来全网断开”
- [202601071130_fix-gz-extract-clash-install](2026-01/202601071130_fix-gz-extract-clash-install/) - 组件管理：新增核心组件卸载接口；修复 .gz 安装文件命名并清理冗余逻辑
- [202601071248_refactor-tun-defaults-engine-ui](2026-01/202601071248_refactor-tun-defaults-engine-ui/) - 维护性：提取 TUN 默认值常量；主题页内核偏好切换公共刷新逻辑去重
- [202601071306_fix-chmod-engine-switch-proxy-failfast](2026-01/202601071306_fix-chmod-engine-switch-proxy-failfast/) - 修复 clash 安装归一化 chmod 错误处理；主题页切换内核关闭系统代理失败改为快失败
- [202601080702_fix-frouter-rule-list-label](2026-01/202601080702_fix-frouter-rule-list-label/) - 主题页：FRouter 路由规则列表优先展示模板/首条匹配项，并在次行显示去向
- [202601080729_compact-clash-rules-by-target](2026-01/202601080729_compact-clash-rules-by-target/) - 订阅解析：Clash YAML rules 按目标去向合并连续规则，避免生成海量路由边
- [202601080815_fix-singbox-ruleset-tag-case](2026-01/202601080815_fix-singbox-ruleset-tag-case/) - 代理服务：sing-box rule-set 下载对 geoip/geosite tag 做小写归一化，避免 `geoip-CN` 触发 404
- [202601080848_fix-subscription-pull-refresh-duplicate](2026-01/202601080848_fix-subscription-pull-refresh-duplicate/) - 订阅面板：配置行去除重复操作，移除“刷新”按钮，仅保留“拉取节点”
- [202601080900_fix-subscription-error-message-overflow](2026-01/202601080900_fix-subscription-error-message-overflow/) - 订阅面板：同步失败错误信息在状态列单行省略显示，避免表格行高度被撑爆
- [202601081053_fix-review-followups](2026-01/202601081053_fix-review-followups/) - 代码审查跟进：plugin-opts 去重归一化、Clash 规则优先级连续化、订阅解析选择优化、app logs since 负数校验、ClearNodes sentinel 与主题页缩进修复
- [202601081145_fix-review-report](2026-01/202601081145_fix-review-report/) - 代码审查跟进：Clash 解析单测补齐、重复 proxy 告警、keepalive 尊重用户 stop、TUN 清理日志增强与订阅/主题页小型修正
- [202601081334_fix-singbox-tun-dns-doh](2026-01/202601081334_fix-singbox-tun-dns-doh/) - 修复 sing-box TUN 模式下默认远程 DNS 使用 53 端口导致的“用一段时间后域名解析卡死”
- [202601081339_fix-proxy-port-sync](2026-01/202601081339_fix-proxy-port-sync/) - 主题页：系统代理端口设置联动后端 ProxyConfig，端口变更自动重启并重应用系统代理；启动时同步真实端口避免误导
- [202601081553_user_data_root](2026-01/202601081553_user_data_root/) - 运行期数据与 artifacts 统一写入 userData，并自动迁移清理遗留仓库目录
- [202601081505_remove_xray](2026-01/202601081505_remove_xray/) - 移除 Xray 支持，仅保留 sing-box/mihomo(Clash)
- [202601082055_fix-clash-tun-sniffer-quic](2026-01/202601082055_fix-clash-tun-sniffer-quic/) - 代理服务：mihomo(Clash) TUN 默认开启 sniffer，并默认阻断 QUIC（UDP/443）提升可用性
- [202601092026_theme-package](2026-01/202601092026_theme-package/) - 主题包：主题目录化 + ZIP 导入/导出；Electron 从 userData/themes 加载；主题页提供导入/导出与切换
- [202601100601_theme-pack-manifest](2026-01/202601100601_theme-pack-manifest/) - 主题包：支持 `manifest.json`（单包多子主题）；`GET /themes` 返回 `entry` 用于切换与启动加载
