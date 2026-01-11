# 任务清单: 减少 Linux TUN 模式提权弹窗次数

目录: `helloagents/plan/202601091707_fix-tun-polkit-prompts/`

---

## 1. root helper 扩展
- [√] 1.1 扩展 `resolvectl-helper` IPC 协议，支持 `op` 字段并兼容旧请求（默认 resolvectl）
- [√] 1.2 新增 `tun-cleanup` 与 `tun-setup` 两个白名单 op（仅允许固定行为，不支持任意命令）
- [√] 1.3 在 `backend/service/shared` 提供 `EnsureRootHelper` / `CallRootHelper` 供后端复用

## 2. backend: TUN 特权调用复用 helper
- [√] 2.1 `CleanConflictingIPTablesRules` 从直接 pkexec 改为调用 helper（同生命周期只授权一次）
- [√] 2.2 `EnsureTUNCapabilitiesForBinary` 从 pkexec 子命令改为调用 helper 执行 `SetupTUNForBinary`

## 3. 文档更新
- [√] 3.1 更新 `helloagents/wiki/modules/backend.md` 增加本次变更条目
- [√] 3.2 更新 `helloagents/CHANGELOG.md` 记录修复
- [√] 3.3 更新 `helloagents/history/index.md` 增加索引与月归档条目

## 4. 测试
- [√] 4.1 运行 `go test ./...`

