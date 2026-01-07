# 变更历史索引

本文件记录所有已完成变更的索引，便于追溯和查询。

## 索引

| 时间戳 | 功能名称 | 类型 | 状态 | 方案包路径 |
|--------|----------|------|------|------------|
| 202601050639 | fix-clash-tun-dns | 修复 | ✅已完成 | [2026-01/202601050639_fix-clash-tun-dns](2026-01/202601050639_fix-clash-tun-dns/) |
| 202601051238 | fix-clash-tun-mtu | 修复 | ✅已完成 | [2026-01/202601051238_fix-clash-tun-mtu](2026-01/202601051238_fix-clash-tun-mtu/) |
| 202601071130 | fix-gz-extract-clash-install | 修复 | ✅已完成 | [2026-01/202601071130_fix-gz-extract-clash-install](2026-01/202601071130_fix-gz-extract-clash-install/) |
| 202601071248 | refactor-tun-defaults-engine-ui | 重构 | ✅已完成 | [2026-01/202601071248_refactor-tun-defaults-engine-ui](2026-01/202601071248_refactor-tun-defaults-engine-ui/) |

## 按月归档

### 2026-01

- [202601050639_fix-clash-tun-dns](2026-01/202601050639_fix-clash-tun-dns/) - 修复 Linux 下 mihomo(Clash) TUN 默认配置导致的断网
- [202601051238_fix-clash-tun-mtu](2026-01/202601051238_fix-clash-tun-mtu/) - 修复 Linux 下 mihomo(Clash) TUN 默认 MTU=9000 导致“看起来全网断开”
- [202601071130_fix-gz-extract-clash-install](2026-01/202601071130_fix-gz-extract-clash-install/) - 组件管理：新增核心组件卸载接口；修复 .gz 安装文件命名并清理冗余逻辑
- [202601071248_refactor-tun-defaults-engine-ui](2026-01/202601071248_refactor-tun-defaults-engine-ui/) - 维护性：提取 TUN 默认值常量；主题页内核偏好切换公共刷新逻辑去重
