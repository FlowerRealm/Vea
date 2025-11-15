# API Changelog

遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/) 格式。

所有版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/) 规范。

## [Unreleased]

### 计划中
- WebSocket 实时通知支持
- 批量操作 API 优化
- GraphQL 端点（可选）

---

## [1.0.0] - 2024-11-15

### 初始版本

#### 新增

**节点管理**
- `GET /nodes` - 列出所有节点
- `POST /nodes` - 创建节点（支持分享链接和手动输入）
- `PUT /nodes/:id` - 更新节点信息
- `DELETE /nodes/:id` - 删除节点
- `POST /nodes/:id/reset-traffic` - 重置节点流量
- `POST /nodes/:id/traffic` - 增量更新节点流量
- `POST /nodes/:id/ping` - 异步测试节点延迟
- `POST /nodes/:id/speedtest` - 异步测试节点速度
- `POST /nodes/:id/select` - 切换 Xray 使用的节点
- `POST /nodes/bulk/ping` - 批量测试节点延迟
- `POST /nodes/reset-speed` - 重置节点速度测试结果

**配置管理**
- `GET /configs` - 列出所有配置
- `POST /configs/import` - 导入配置（支持 Xray JSON 和订阅链接）
- `PUT /configs/:id` - 更新配置信息
- `DELETE /configs/:id` - 删除配置
- `POST /configs/:id/refresh` - 从 sourceUrl 刷新配置内容
- `POST /configs/:id/pull-nodes` - 从配置中提取并创建节点
- `POST /configs/:id/traffic` - 增量更新配置流量

**Geo 资源管理**
- `GET /geo` - 列出所有 Geo 资源
- `POST /geo` - 创建 Geo 资源
- `PUT /geo/:id` - 更新 Geo 资源（upsert）
- `DELETE /geo/:id` - 删除 Geo 资源
- `POST /geo/:id/refresh` - 从 sourceUrl 刷新 Geo 资源

**核心组件管理**
- `GET /components` - 列出所有核心组件
- `POST /components` - 创建核心组件记录
- `PUT /components/:id` - 更新核心组件信息
- `DELETE /components/:id` - 删除核心组件
- `POST /components/:id/install` - 下载并安装核心组件

**Xray 控制**
- `GET /xray/status` - 获取 Xray 运行状态
- `POST /xray/start` - 启动 Xray 服务
- `POST /xray/stop` - 停止 Xray 服务

**流量策略**
- `GET /traffic/profile` - 获取流量策略配置
- `PUT /traffic/profile` - 更新流量策略（默认出口、DNS）
- `GET /traffic/rules` - 列出所有分流规则
- `POST /traffic/rules` - 创建分流规则
- `PUT /traffic/rules/:id` - 更新分流规则
- `DELETE /traffic/rules/:id` - 删除分流规则

**系统设置**
- `GET /settings/system-proxy` - 获取系统代理设置
- `PUT /settings/system-proxy` - 更新系统代理设置

**其他**
- `GET /health` - 健康检查
- `GET /snapshot` - 获取全量状态快照

#### 数据模型

**核心实体**
- `Node` - 节点（支持 vless/trojan/shadowsocks/vmess 协议）
- `Config` - 配置（支持 xray-json 格式）
- `GeoResource` - Geo 资源（geoip/geosite）
- `CoreComponent` - 核心组件（xray/geo/generic）
- `TrafficProfile` - 流量策略
- `TrafficRule` - 分流规则
- `SystemProxySettings` - 系统代理设置
- `XrayStatus` - Xray 状态
- `ServiceState` - 服务完整状态快照

**嵌套结构**
- `NodeSecurity` - 节点安全配置（UUID/密码/加密方法）
- `NodeTransport` - 节点传输配置（ws/grpc/http）
- `NodeTLS` - 节点 TLS 配置（serverName/fingerprint/Reality）
- `DNSSetting` - DNS 配置（策略/服务器列表）

#### 功能特性

- **分享链接解析**：自动解析 vmess://、vless://、trojan://、ss:// 分享链接
- **异步任务**：延迟测试、速度测试、节点切换使用异步模式（返回 202 Accepted）
- **自动更新**：配置和组件支持 autoUpdateInterval 字段，实现定时刷新
- **流量统计**：节点和配置支持上传/下载流量统计
- **快照机制**：`/snapshot` 端点提供全量状态导出
- **系统代理集成**：支持设置系统 HTTP/SOCKS5 代理及忽略主机列表

#### 协议支持

- **节点协议**：vless, trojan, shadowsocks, vmess
- **传输协议**：TCP, WebSocket, gRPC, HTTP/2
- **TLS**：支持标准 TLS 和 Reality
- **配置格式**：xray-json

#### 兼容性

- **Go 版本**：1.22+
- **Xray 版本**：1.8.0+（推荐）
- **平台支持**：Linux, macOS, Windows

---

## 版本说明

### 版本号格式

```
v主版本号.次版本号.修订号
```

### 变更类型

- **新增（Added）**：新功能
- **变更（Changed）**：现有功能的变更
- **废弃（Deprecated）**：即将移除的功能
- **移除（Removed）**：已移除的功能
- **修复（Fixed）**：Bug 修复
- **安全（Security）**：安全漏洞修复

---

## 未来计划

### v1.1.0（计划中）

#### 新增
- 节点分组管理
- 自定义测速服务器
- 配置模板系统
- 日志导出 API

#### 优化
- 批量操作性能优化
- 错误信息国际化

### v1.2.0（计划中）

#### 新增
- WebSocket 推送通知
- 节点健康检查调度器
- 流量使用报告生成

### v2.0.0（远期计划）

**注意：v2.0.0 将包含破坏性变更，计划发布时间 TBD**

#### 可能的变更
- GraphQL API 支持
- 新的认证机制（OAuth2/JWT）
- 多用户支持
- 数据库后端（替换内存存储）

---

## 链接

- [OpenAPI 规范](./openapi.yaml)
- [版本管理策略](./versioning.md)
- [GitHub Releases](https://github.com/FlowerRealm/Vea/releases)
- [问题追踪](https://github.com/FlowerRealm/Vea/issues)
