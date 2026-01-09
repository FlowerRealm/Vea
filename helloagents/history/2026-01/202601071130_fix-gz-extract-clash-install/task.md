# 轻量迭代任务清单 - fix-gz-extract-clash-install

- [√] 新增核心组件卸载能力（后端接口 + 仓库支持 + 运行中保护）
- [√] 修复 ExtractArchive 的 .gz 解压命名（使用 gzip header 原始文件名，并做路径安全处理）
- [√] 清理 normalizeClashInstall 中针对 `shared.ComponentFile` 的特殊兼容逻辑（保留基于文件名前缀的归一化）
- [√] 重命名 Clash 适配器中 `applyTransport` 的局部变量 `http` → `httpOpts`
- [√] dark 主题：`.chain-btn.primary:hover` 使用 `--accent-hover` 变量（保留 fallback）
- [√] 前端/SDK/OpenAPI 同步组件卸载入口
- [√] 运行格式化与测试：`gofmt` + `go test ./...`
- [√] 同步知识库：`helloagents/CHANGELOG.md`、`helloagents/history/index.md`、模块文档
