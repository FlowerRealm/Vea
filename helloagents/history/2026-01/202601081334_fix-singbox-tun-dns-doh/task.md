# 任务清单: 修复 sing-box TUN 模式 DNS 断网（默认改用 DoH）

目录: `helloagents/history/2026-01/202601081334_fix-singbox-tun-dns-doh/`

---

## 1. 后端修复
- [√] 1.1 调整 `backend/service/adapters/singbox.go`：将默认 `dns-remote` 从 `tcp 8.8.8.8:53` 改为 DoH(HTTPS, 443)

## 2. 测试
- [√] 2.1 新增单元测试 `backend/service/adapters/singbox_dns_detour_test.go`：断言 `dns-remote` 默认使用 DoH 且关键字段完整
- [√] 2.2 执行 `go test ./...`

## 3. 文档与归档
- [√] 3.1 更新 `helloagents/CHANGELOG.md`、`helloagents/wiki/modules/backend.md`
- [√] 3.2 更新 `helloagents/history/index.md`

