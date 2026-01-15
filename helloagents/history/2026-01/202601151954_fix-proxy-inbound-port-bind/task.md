# 任务清单: 修复 Windows 下 sing-box mixed 入站端口占用导致启动失败

目录: `helloagents/history/2026-01/202601151954_fix-proxy-inbound-port-bind/`

---

## 1. backend（默认端口与启动失败提示）
- [√] 1.1 在 `backend/repository/memory/store.go` 中将默认 `proxyConfig.inboundPort` 从 `1080` 调整为 `31346`，并同步加载 state 时 `inboundPort=0` 的兜底值，验证 why.md#需求-默认端口调整为-31346-新安装默认值
- [√] 1.2 在 `backend/service/proxy/service.go` 中将 `applyConfigDefaults` 的非 TUN 默认端口从 `1080` 调整为 `31346`，验证 why.md#需求-默认端口调整为-31346-新安装默认值
- [√] 1.3 在 `backend/service/facade.go` 中将用于状态兜底的默认端口常量从 `1080` 调整为 `31346`，并确保系统代理/探测逻辑一致，验证 why.md#需求-默认端口调整为-31346-新安装默认值
- [√] 1.4 在 `backend/service/proxy/service.go` 的启动流程中增加入站端口占用检查（停止旧内核后执行），端口占用时返回 `repository.ErrInvalidData` 包装的明确错误信息（包含 host:port 与操作建议），验证 why.md#需求-端口被占用时明确失败-场景-windows--mixed--端口占用

## 2. 测试
- [√] 2.1 补齐/新增单测：验证默认端口变更（31346）与端口占用返回错误（ErrInvalidData），覆盖 `backend/service/proxy` 与 `backend/repository/memory` 的关键路径
- [√] 2.2 执行 `go test ./...`，确保全量测试通过

## 3. 安全检查
- [√] 3.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 4. 文档更新（知识库）
- [√] 4.1 更新 `helloagents/wiki/modules/backend.md`：记录“默认入站端口=31346”与“端口占用 fail-fast”的规范/注意事项
- [√] 4.2 更新 `helloagents/CHANGELOG.md`：记录本次修复（Windows mixed 端口占用导致启动失败 + 默认端口调整）
