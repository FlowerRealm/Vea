# 任务清单: 修复 Linux 下 mihomo(Clash) TUN 断网

目录: `helloagents/history/2026-01/202601050639_fix-clash-tun-dns/`

## 1. 后端配置生成
- [√] 1.1 在 `backend/service/adapters/clash.go` 中对齐主流 TUN 默认值（MTU/stack/auto-redirect），验证 why.md#核心场景-需求-linux-下-tun-可用性-场景-mihomo-tun--dns-hijack
- [√] 1.2 在 `backend/service/adapters/clash.go` 中改进 DNS 配置逻辑（允许 scheme + bootstrap default-nameserver），验证 why.md#核心场景-需求-linux-下-tun-可用性-场景-mihomo-tun--dns-hijack
- [√] 1.3 在 `backend/service/adapters/clash.go` 中启用 `find-process-mode=strict`（保障 PROCESS-NAME 自保规则），验证 why.md#变更内容

## 2. 安全检查
- [√] 2.1 执行安全检查（无敏感信息、无破坏性命令、无外部密钥硬编码）

## 3. 文档更新
- [√] 3.1 更新知识库：记录本次默认值对齐与影响范围

## 4. 测试
- [√] 4.1 运行 `go test ./...`
