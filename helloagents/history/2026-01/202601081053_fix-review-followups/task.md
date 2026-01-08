# 轻量迭代任务清单 - fix-review-followups

- [√] 去重 Shadowsocks 插件参数解析/归一化：提取到 `backend/service/shared`，供分享链接解析、Clash 订阅解析、sing-box 适配器复用
- [√] Clash YAML rules 合并后优先级连续化：避免优先级依赖原始行号导致不连续/过大
- [√] 订阅同步解析器选择优化：仅在 payload 命中特征时尝试 Clash YAML 解析，减少对非 YAML 内容的二次解析失败
- [√] `GET /app/logs?since=` 参数校验加强：拒绝负数，并补充单元测试
- [√] 节点仓库清空语义显式化：新增 `domain.ClearNodes` nil-slice sentinel，并在内存仓库测试中使用
- [√] TUN 清理脚本假设说明：补充注释，明确仅对 XRAY/XRAY_SELF + fwmark/table 默认值做 best-effort 清理
- [√] 主题页缩进一致性修复：修正日志面板与 `updateXrayUI` 附近的异常缩进
- [√] 运行测试：`go test ./...`
- [√] 同步知识库：`helloagents/CHANGELOG.md`、`helloagents/history/index.md`、`helloagents/wiki/api.md`、`helloagents/wiki/modules/backend.md`、`helloagents/wiki/modules/frontend.md`
