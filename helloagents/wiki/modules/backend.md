# backend 模块

## 职责
- 生成运行计划：将 `ProxyConfig + FRouter + Nodes + ChainProxySettings` 编译为可执行的 runtime plan
- 适配多内核：在 `backend/service/adapters/` 生成 Xray / sing-box / mihomo 的配置
- 进程管理：启动/停止内核，收集日志与状态
- 核心组件管理：安装/卸载 Xray / sing-box / mihomo 等核心组件

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
