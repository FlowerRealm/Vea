# Vea API 文档

本文总结 Vea 后端公开的 HTTP 接口。除非特别说明，接口均返回 JSON，默认 `Content-Type: application/json`，使用 UTC ISO8601 时间戳（例如 `2024-05-04T12:34:56Z`）。

- **基础地址**：`http://<host>:<port>`，默认 `http://127.0.0.1:8080`
- **错误响应**：格式为 `{"error": "<message>"}`，HTTP 状态码指示错误类型（400/404/500 等）
- **身份认证**：当前无额外认证机制，部署在受控网络环境下使用

## 健康检查与快照

### `GET /health`
- 返回服务状态与时间戳。
- 响应示例：
  ```json
  {
    "status": "ok",
    "timestamp": "2024-05-04T12:34:56Z"
  }
  ```

### `GET /snapshot`
- 返回完整运行时快照：节点、配置、Geo 资源、核心组件、流量策略与系统代理设置。
- 响应类型对应 `domain.ServiceState`。

## 节点 Node (`/nodes`)

### `GET /nodes`
- 列出全部节点与当前活跃节点信息。
- 响应示例：
  ```json
  {
    "nodes": [...],
    "activeNodeId": "node-1",
    "lastSelectedNodeId": "node-1"
  }
  ```

### `POST /nodes`
- 创建节点，两种模式：
  1. 提供 `shareLink`（vmess/vless/trojan/ss 链接）自动解析；
  2. 直接指定节点参数。
- 请求示例：
  ```json
  {
    "name": "us-01",
    "address": "1.2.3.4",
    "port": 443,
    "protocol": "vless",
    "tags": ["premium"]
  }
  ```
- 返回创建后的节点对象。

### `PUT /nodes/:id`
- 更新节点基础信息（名称、地址、端口、协议、标签）。

### `DELETE /nodes/:id`
- 删除节点，成功返回 204。

### `POST /nodes/:id/reset-traffic`
- 将节点流量统计清零，返回更新后的节点。

### `POST /nodes/:id/traffic`
- 累加节点流量，用于上报真实使用量。
- 请求体（字段可选、默认 0）：
  ```json
  {
    "uploadBytes": 1024,
    "downloadBytes": 2048
  }
  ```

### `POST /nodes/:id/ping`
- 异步触发延迟测试，立即返回 202。

### `POST /nodes/:id/speedtest`
- 异步触发速度测试，立即返回 202。

### `POST /nodes/:id/select`
- 请求后台切换 Xray 使用的节点，异步执行，返回 202。

### `POST /nodes/bulk/ping`
- 批量延迟测试，`ids` 为空时表示测试全部节点。
  ```json
  {
    "ids": ["node-1", "node-2"]
  }
  ```
- 返回 202。

### `POST /nodes/reset-speed`
- 重置节点测速记录。`ids` 为空表示重置全部节点，成功返回 204。

## 配置 Config (`/configs`)

### `GET /configs`
- 返回全部配置列表。

### `POST /configs/import`
- 新增配置，必须提供 `sourceUrl`，用于自动同步。
- 请求体：
  ```json
  {
    "name": "subscription-a",
    "format": "xray-json",
    "payload": "",
    "sourceUrl": "https://example.com/subscription",
    "autoUpdateIntervalMinutes": 60,
    "expireAt": "2024-12-31T00:00:00Z"
  }
  ```
- 成功返回 201 与配置对象。

### `PUT /configs/:id`
- 更新配置字段，同样要求 `sourceUrl`。

### `DELETE /configs/:id`
- 删除配置，返回 204。

### `POST /configs/:id/refresh`
- 立即拉取订阅内容并更新，返回最新配置。

### `POST /configs/:id/pull-nodes`
- 从配置对应的订阅中解析节点并写入节点列表，返回节点数组。

### `POST /configs/:id/traffic`
- 累加配置关联的上/下行流量，接受与节点流量相同的请求体。

## Geo 资源 (`/geo`)

### `GET /geo`
- 列出已维护的 Geo 资源。

### `POST /geo`
- 创建资源，字段：
  ```json
  {
    "name": "GeoIP",
    "type": "geoip",
    "sourceUrl": "https://example.com/geoip.dat",
    "checksum": "sha256:...",
    "version": "20240504"
  }
  ```
- 返回 201。

### `PUT /geo/:id`
- 更新资源（同上字段），返回 200。

### `DELETE /geo/:id`
- 删除资源，返回 204。

### `POST /geo/:id/refresh`
- 立即下载并刷新资源文件，返回更新后的对象。

## 核心组件 (`/components`)

### `GET /components`
- 列出已登记的核心组件（Xray、Geo 组件等）。

### `POST /components`
- 新增组件，关键字段：
  ```json
  {
    "name": "xray-core",
    "kind": "xray",
    "sourceUrl": "https://example.com/xray.zip",
    "archiveType": "zip",
    "autoUpdateIntervalMinutes": 1440
  }
  ```
- 返回 201。

### `PUT /components/:id`
- 更新组件信息，允许调整 `kind`、`archiveType`、`sourceUrl`、自动更新间隔。

### `DELETE /components/:id`
- 移除组件记录，返回 204。

### `POST /components/:id/install`
- 触发组件安装/更新流程，返回安装后的组件状态。

## 系统代理设置 (`/settings/system-proxy`)

### `GET /settings/system-proxy`
- 返回当前系统代理配置：
  ```json
  {
    "settings": {
      "enabled": false,
      "ignoreHosts": [],
      "updatedAt": "2024-05-04T12:34:56Z"
    },
    "message": ""
  }
  ```

### `PUT /settings/system-proxy`
- 更新代理开关与忽略列表，响应会附带 `message` 字段提示当前状态。
  ```json
  {
    "enabled": true,
    "ignoreHosts": ["localhost", "127.0.0.1"]
  }
  ```
- 若系统暂不支持或 Xray 未运行，会返回 200 且 `message` 包含原因。

## Xray 控制 (`/xray`)

### `GET /xray/status`
- 返回当前 Xray 状态信息（结构由服务层定义），包含运行状态、活跃节点等。

### `POST /xray/start`
- 启动 Xray，可选指定 `activeNodeId`：
  ```json
  {
    "activeNodeId": "node-1"
  }
  ```
- 异步处理，成功返回 202。

### `POST /xray/stop`
- 停止 Xray，成功返回 204。

## 流量策略 (`/traffic`)

### `GET /traffic/profile`
- 返回默认节点与 DNS 设置。

### `PUT /traffic/profile`
- 修改默认节点与 DNS：
  ```json
  {
    "defaultNodeId": "node-1",
    "dns": {
      "strategy": "AsIs",
      "servers": ["1.1.1.1", "8.8.8.8"]
    }
  }
  ```

### `GET /traffic/rules`
- 列出所有分流规则。

### `POST /traffic/rules`
- 创建规则：
  ```json
  {
    "name": "Netflix",
    "targets": ["domain:netflix.com"],
    "nodeId": "node-2",
    "priority": 10
  }
  ```
- 返回 201。

### `PUT /traffic/rules/:id`
- 更新规则字段。

### `DELETE /traffic/rules/:id`
- 删除规则，返回 204。

## 常见状态码汇总

| 状态码 | 场景 |
| ------ | ---- |
| 200    | 成功返回数据或更新结果 |
| 201    | 创建资源成功 |
| 202    | 已接受异步任务（ping/speedtest/Xray 切换等） |
| 204    | 成功执行无返回体的操作（删除、停止 Xray 等） |
| 400    | 请求参数错误或业务前置条件不满足 |
| 404    | 指定资源不存在 |
| 500    | 未捕获的服务器错误 |

## 版本说明

接口如有调整，请同步更新本文档并在发布说明里标注 Breaking Changes，遵守 “Never break userspace” 的铁律。
