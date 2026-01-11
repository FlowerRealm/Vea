# 轻量迭代任务清单: fix-speed-unit-mbs

目标: 修复测速结果的速度单位显示不一致问题（将 UI/SDK/OpenAPI 的 `Mbps` 统一修正为 `MB/s`，与实际测速计算单位一致）。

## 任务

- [√] 前端主题页：节点与 FRouter 的速度展示单位从 `Mbps` 改为 `MB/s`
- [√] SDK：`utils.formatSpeed` 输出单位从 `Mbps` 改为 `MB/s`（同步更新构建产物与 README 示例）
- [√] OpenAPI：`lastSpeedMbps` 字段与 speedtest 接口描述补齐/修正单位为 `MB/s`
- [√] 后端：修正测速相关注释的单位说明为 `MB/s`
- [√] 更新知识库与变更索引

## 备注

- 字段名 `lastSpeedMbps` 属历史命名；本次仅修正“显示/文档”单位，不改字段名以避免不必要的破坏性变更。
