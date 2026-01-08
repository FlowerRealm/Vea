# 轻量迭代任务清单: fix-proxy-port-sync

目标: 让前端“系统代理端口（proxy.port）”真正联动到后端 `ProxyConfig.inboundPort`，并在 Mixed 模式下可用（HTTP/SOCKS）。

## 任务

- [√] 前端设置 `proxy.port` 变更时，同步更新后端 `/proxy/config` 的 `inboundPort`（非 TUN 时强制 `inboundMode=mixed`）
- [√] 内核运行中修改端口：自动重启内核使端口生效，并在系统代理启用时重新应用系统代理设置
- [√] 启动时从后端 `/proxy/config` 同步实际 `inboundPort` 到设置 UI，避免默认值与实际不一致造成误导
- [√] 更新知识库与 Changelog，并迁移方案包至 `history/`

## 备注

- sing-box / mihomo(clash) 的 Mixed 入站为同端口 HTTP+SOCKS；Xray 的 Mixed 通过 `HTTP(port) + SOCKS(port+1)` 组合实现。
