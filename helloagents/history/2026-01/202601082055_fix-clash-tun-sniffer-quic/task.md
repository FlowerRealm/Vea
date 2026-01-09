# 轻量迭代任务清单

目标: 提升 mihomo(Clash) 在 TUN 模式下的可用性与一致性（与 sing-box 行为对齐），避免“可启动但访问异常/分流失效”。

- [√] 在 Clash 配置生成中默认开启 sniffer（TUN 模式）
- [√] 在 Clash 配置生成中默认阻断 QUIC（UDP/443），强制回落 TCP/HTTPS（TUN 模式）
- [√] 补齐单元测试覆盖 sniffer 与 QUIC 规则
- [√] 本地验证：`go test ./...`
- [√] 同步更新知识库与变更历史索引
