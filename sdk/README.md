# Vea SDK

Vea 后端 API 的官方 JavaScript SDK，支持浏览器、Node.js、Electron/Tauri 等环境。

## 特性

- ✅ **零依赖** - 使用原生 `fetch` API，无需第三方库
- ✅ **TypeScript 支持** - 完整的类型定义
- ✅ **跨平台** - 浏览器、Node.js、Electron、Tauri 全兼容
- ✅ **轻量级** - 压缩后约 15KB
- ✅ **Promise 异步** - 现代化异步 API
- ✅ **完整封装** - 覆盖所有 Vea Backend API 端点
- ✅ **错误处理** - 统一的错误类型和处理
- ✅ **超时控制** - 可配置的请求超时

## 安装

### npm/yarn/pnpm

```bash
npm install @vea/sdk

# 或
yarn add @vea/sdk

# 或
pnpm add @vea/sdk
```

### CDN（浏览器）

```html
<!-- UMD 格式 -->
<script src="https://unpkg.com/@vea/sdk/dist/vea-sdk.umd.js"></script>

<!-- 或使用压缩版 -->
<script src="https://unpkg.com/@vea/sdk/dist/vea-sdk.umd.min.js"></script>
```

### 直接下载

从 [GitHub Releases](https://github.com/FlowerRealm/Vea/releases) 下载对应版本的文件。

## 快速开始

### 浏览器

```html
<!DOCTYPE html>
<html>
<head>
  <title>Vea SDK 示例</title>
</head>
<body>
  <button onclick="loadNodes()">加载节点列表</button>
  <pre id="output"></pre>

  <script src="https://unpkg.com/@vea/sdk/dist/vea-sdk.umd.min.js"></script>
  <script>
    // 创建客户端实例
    const vea = new VeaSDK.VeaClient({ baseURL: 'http://localhost:8080' })

    async function loadNodes() {
      try {
        const result = await vea.nodes.list()
        document.getElementById('output').textContent = JSON.stringify(result, null, 2)
      } catch (error) {
        console.error('加载失败:', error)
      }
    }
  </script>
</body>
</html>
```

### Node.js

```javascript
const { VeaClient } = require('@vea/sdk')

// 创建客户端实例
const vea = new VeaClient({
  baseURL: 'http://localhost:8080'
})

// 使用 async/await
async function main() {
  try {
    // 健康检查
    const health = await vea.health()
    console.log('服务状态:', health)

    // 列出节点
    const nodes = await vea.nodes.list()
    console.log('节点列表:', nodes)

    // 创建节点
    const newNode = await vea.nodes.create({
      name: 'Tokyo Server',
      address: '1.2.3.4',
      port: 443,
      protocol: 'vless',
      tags: ['premium', 'asia']
    })
    console.log('创建的节点:', newNode)
  } catch (error) {
    console.error('错误:', error.message)
  }
}

main()
```

### TypeScript

```typescript
import { VeaClient, Node, VeaError } from '@vea/sdk'

const vea = new VeaClient({
  baseURL: 'http://localhost:8080',
  timeout: 30000
})

async function main() {
  try {
    // TypeScript 会自动推断类型
    const result = await vea.nodes.list()
    const nodes: Node[] = result.nodes
    const activeNodeId: string = result.activeNodeId

    console.log(`共有 ${nodes.length} 个节点`)
  } catch (error) {
    if (error instanceof VeaError) {
      console.error(`API 错误 [${error.statusCode}]: ${error.message}`)
    }
  }
}
```

### Electron

```javascript
// Main Process
const { VeaClient } = require('@vea/sdk')
const { app, ipcMain } = require('electron')

const vea = new VeaClient({ baseURL: 'http://localhost:8080' })

ipcMain.handle('vea:nodes:list', async () => {
  return await vea.nodes.list()
})

// Renderer Process (通过 preload.js)
const nodes = await window.veaAPI.nodes.list()
```

详细示例请查看 [examples/](./examples/) 目录。

## API 文档

### 初始化客户端

```javascript
const vea = new VeaClient(options)
```

**选项**：
- `baseURL` (string): API 基础 URL，默认 `http://localhost:8080`
- `timeout` (number): 请求超时时间（毫秒），默认 `30000`
- `headers` (object): 自定义 HTTP 头

### 通用方法

#### health()
健康检查。

```javascript
const health = await vea.health()
// => { status: "ok", timestamp: "2024-11-15T..." }
```

#### snapshot()
获取完整系统快照。

```javascript
const snapshot = await vea.snapshot()
// => { nodes: [...], configs: [...], ... }
```

### 节点管理 (vea.nodes)

#### list()
列出所有节点。

```javascript
const result = await vea.nodes.list()
// => { nodes: [...], activeNodeId: "xxx", lastSelectedNodeId: "yyy" }
```

#### create(data)
创建节点。

```javascript
// 方式1: 从分享链接创建
const node = await vea.nodes.create({
  shareLink: 'vmess://eyJhZGQiOiIxLjIuMy40IiwicG9ydCI6NDQzLC4uLn0='
})

// 方式2: 手动创建
const node = await vea.nodes.create({
  name: 'Tokyo Server',
  address: '1.2.3.4',
  port: 443,
  protocol: 'vless',
  tags: ['premium']
})
```

#### update(id, data)
更新节点。

```javascript
const updated = await vea.nodes.update('node-id', {
  name: 'New Name',
  tags: ['updated']
})
```

#### delete(id)
删除节点。

```javascript
await vea.nodes.delete('node-id')
```

#### resetTraffic(id)
重置节点流量。

```javascript
const node = await vea.nodes.resetTraffic('node-id')
```

#### incrementTraffic(id, traffic)
增加节点流量统计。

```javascript
const node = await vea.nodes.incrementTraffic('node-id', {
  uploadBytes: 1024000,
  downloadBytes: 5120000
})
```

#### ping(id)
测试节点延迟（异步）。

```javascript
await vea.nodes.ping('node-id')
// 返回 null，结果稍后更新到节点的 lastLatencyMs 字段
```

#### speedtest(id)
测试节点速度（异步）。

```javascript
await vea.nodes.speedtest('node-id')
// 返回 null，结果稍后更新到节点的 lastSpeedMbps 字段
```

#### select(id)
选择节点（切换 Xray 使用的节点）。

```javascript
await vea.nodes.select('node-id')
```

#### bulkPing(ids)
批量测试延迟。

```javascript
// 测试所有节点
await vea.nodes.bulkPing()

// 测试指定节点
await vea.nodes.bulkPing(['node-1', 'node-2'])
```

#### resetSpeed(ids)
重置节点速度测试结果。

```javascript
await vea.nodes.resetSpeed(['node-1', 'node-2'])
```

### 配置管理 (vea.configs)

#### list()
列出所有配置。

```javascript
const configs = await vea.configs.list()
```

#### import(data)
导入配置。

```javascript
const config = await vea.configs.import({
  name: 'My Config',
  format: 'xray-json',
  sourceUrl: 'https://example.com/config.json',
  autoUpdateIntervalMinutes: 60
})
```

#### update(id, data)
更新配置。

```javascript
const updated = await vea.configs.update('config-id', {
  name: 'Updated Config',
  autoUpdateIntervalMinutes: 120
})
```

#### delete(id)
删除配置。

```javascript
await vea.configs.delete('config-id')
```

#### refresh(id)
刷新配置（从 sourceUrl 重新拉取）。

```javascript
const config = await vea.configs.refresh('config-id')
```

#### pullNodes(id)
同步配置节点（从配置中提取节点）。

```javascript
const nodes = await vea.configs.pullNodes('config-id')
```

#### incrementTraffic(id, traffic)
增加配置流量统计。

```javascript
const config = await vea.configs.incrementTraffic('config-id', {
  uploadBytes: 1024000,
  downloadBytes: 5120000
})
```

### Geo 资源管理 (vea.geo)

#### list()
列出所有 Geo 资源。

```javascript
const geoResources = await vea.geo.list()
```

#### create(data)
创建 Geo 资源。

```javascript
const geo = await vea.geo.create({
  name: 'GeoIP',
  type: 'geoip',
  sourceUrl: 'https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat'
})
```

#### update(id, data)
更新 Geo 资源。

```javascript
const geo = await vea.geo.update('geo-id', {
  name: 'Updated GeoIP',
  version: '20241115'
})
```

#### delete(id)
删除 Geo 资源。

```javascript
await vea.geo.delete('geo-id')
```

#### refresh(id)
刷新 Geo 资源（重新下载）。

```javascript
const geo = await vea.geo.refresh('geo-id')
```

### 核心组件管理 (vea.components)

#### list()
列出所有核心组件。

```javascript
const components = await vea.components.list()
```

#### create(data)
创建核心组件记录。

```javascript
const component = await vea.components.create({
  kind: 'xray'
})
```

#### update(id, data)
更新核心组件。

```javascript
const component = await vea.components.update('component-id', {
  autoUpdateIntervalMinutes: 1440
})
```

#### delete(id)
删除核心组件。

```javascript
await vea.components.delete('component-id')
```

#### install(id)
安装核心组件。

```javascript
const component = await vea.components.install('component-id')
```

### Xray 控制 (vea.xray)

#### status()
获取 Xray 状态。

```javascript
const status = await vea.xray.status()
// => { enabled: true, running: true, activeNodeId: "xxx", binary: "...", config: "..." }
```

#### start(options)
启动 Xray。

```javascript
// 使用默认节点启动
await vea.xray.start()

// 指定节点启动
await vea.xray.start({ activeNodeId: 'node-id' })
```

#### stop()
停止 Xray。

```javascript
await vea.xray.stop()
```

### 流量策略 (vea.traffic)

#### getProfile()
获取流量策略。

```javascript
const profile = await vea.traffic.getProfile()
```

#### updateProfile(data)
更新流量策略。

```javascript
const profile = await vea.traffic.updateProfile({
  defaultNodeId: 'node-id',
  dns: {
    strategy: 'ipv4-only',
    servers: ['8.8.8.8', '1.1.1.1']
  }
})
```

#### rules.list()
列出所有分流规则。

```javascript
const rules = await vea.traffic.rules.list()
```

#### rules.create(data)
创建分流规则。

```javascript
const rule = await vea.traffic.rules.create({
  name: 'Netflix',
  targets: ['netflix.com', 'netflixcdn.com'],
  nodeId: 'node-id',
  priority: 10
})
```

#### rules.update(id, data)
更新分流规则。

```javascript
const rule = await vea.traffic.rules.update('rule-id', {
  priority: 20
})
```

#### rules.delete(id)
删除分流规则。

```javascript
await vea.traffic.rules.delete('rule-id')
```

### 系统设置 (vea.settings)

#### getSystemProxy()
获取系统代理设置。

```javascript
const result = await vea.settings.getSystemProxy()
// => { settings: { enabled: false, ignoreHosts: [...] }, message: "" }
```

#### updateSystemProxy(data)
更新系统代理设置。

```javascript
const result = await vea.settings.updateSystemProxy({
  enabled: true,
  ignoreHosts: ['localhost', '127.0.0.0/8', '::1']
})
```

## 错误处理

SDK 抛出 `VeaError` 异常，包含以下属性：

- `message` (string): 错误信息
- `statusCode` (number): HTTP 状态码
- `response` (any): 原始响应数据

```javascript
try {
  await vea.nodes.create({ /* ... */ })
} catch (error) {
  if (error instanceof VeaError) {
    console.error(`错误 [${error.statusCode}]: ${error.message}`)

    if (error.statusCode === 404) {
      console.log('资源不存在')
    } else if (error.statusCode === 400) {
      console.log('请求参数错误')
    } else if (error.statusCode === 500) {
      console.log('服务器内部错误')
    }
  }
}
```

## 高级用法

### 直接调用 API

如果 SDK 未封装某个端点，可以使用底层 `request` 方法：

```javascript
const response = await vea.request({
  method: 'POST',
  path: '/custom/endpoint',
  body: { foo: 'bar' },
  headers: { 'X-Custom-Header': 'value' },
  timeout: 60000
})
```

### 自定义 HTTP 头

```javascript
const vea = new VeaClient({
  baseURL: 'http://localhost:8080',
  headers: {
    'Authorization': 'Bearer token',
    'X-Custom-Header': 'value'
  }
})
```

### 超时控制

```javascript
// 全局超时
const vea = new VeaClient({ timeout: 60000 })

// 单次请求超时
const nodes = await vea.request({
  method: 'GET',
  path: '/nodes',
  timeout: 10000
})
```

## 示例代码

完整示例请查看 [examples/](./examples/) 目录：

- [browser.html](./examples/browser.html) - 浏览器示例
- [electron-example.js](./examples/electron-example.js) - Electron 应用示例
- [react-example.jsx](./examples/react-example.jsx) - React 组件示例

## TypeScript 类型定义

SDK 提供完整的 TypeScript 类型定义，支持自动补全和类型检查。

```typescript
import {
  VeaClient,
  Node,
  Config,
  GeoResource,
  CoreComponent,
  XrayStatus,
  TrafficProfile,
  TrafficRule,
  VeaError
} from '@vea/sdk'

// 类型会自动推断
const vea = new VeaClient()
const result = await vea.nodes.list()  // result 类型自动推断为 NodesListResponse
```

查看完整类型定义：[src/types.d.ts](./src/types.d.ts)

## API 版本兼容性

SDK 版本与 Vea Backend API 版本对应关系：

| SDK 版本 | API 版本 | 兼容性 |
|----------|----------|--------|
| 1.0.x    | v1.0.x   | ✅ 完全兼容 |
| 1.x.y    | v1.x.y   | ⚠️ 向后兼容（可使用新特性） |
| 2.0.x    | v2.0.x   | ❌ 不兼容 v1.x |

**推荐做法**：
- 使用 `^1.0.0` 获取向后兼容的更新
- 锁定主版本号，避免破坏性变更

详见 [API 版本管理策略](../api/versioning.md)。

## 开发

### 构建

```bash
npm install
npm run build
```

输出文件：
- `dist/vea-sdk.umd.js` - UMD 格式（浏览器 + Node.js）
- `dist/vea-sdk.umd.min.js` - 压缩版
- `dist/vea-sdk.esm.js` - ES Module 格式
- `dist/vea-sdk.cjs.js` - CommonJS 格式

### 开发模式

```bash
npm run dev  # 监听文件变化并自动构建
```

## 许可证

[MIT](../LICENSE)

## 贡献

欢迎提交 Issue 和 Pull Request！

- [GitHub Issues](https://github.com/FlowerRealm/Vea/issues)
- [API 文档](../api/openapi.yaml)
- [变更日志](../api/CHANGELOG.md)

## 相关链接

- [Vea Backend](https://github.com/FlowerRealm/Vea)
- [API 规范 (OpenAPI)](../api/openapi.yaml)
- [版本管理策略](../api/versioning.md)
- [变更日志](../api/CHANGELOG.md)
