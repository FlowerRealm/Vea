# 数据模型

本项目核心概念（权威以代码为准，主要在 `backend/domain/entities.go`）：

## FRouter
- 对外的一等操作单元（工具）
- 通过 `ChainProxySettings` 引用 NodeID 构建链式代理图

## Node
- 独立资源（食材）
- 通常由订阅/配置同步生成

## ProxyConfig
- 运行配置（单例）
- 关键字段包括：`InboundMode`、`TUNSettings`、`DNSConfig`、`PreferredEngine`、`FRouterID`

## ChainProxySettings
- 图结构描述 `local/direct/block/slot` 与节点的连接关系
