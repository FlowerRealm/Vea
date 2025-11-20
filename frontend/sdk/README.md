# Vea SDK

Xray 代理管理 JavaScript SDK - 单文件、零依赖、TypeScript类型支持

## 特性

- ✅ **单文件** - 源码和构建产物都是单文件（vea-sdk.js）
- ✅ **零依赖** - 不依赖任何第三方库
- ✅ **跨平台** - 浏览器、Node.js、Electron全支持
- ✅ **TypeScript** - 完整的类型定义
- ✅ **ESM** - 原生ES Module格式
- ✅ **轻量级** - 19KB (800行代码)

## 快速开始

### 安装

```html
<!-- 方式1：直接使用（推荐） -->
<script type="module">
  import { createAPI, utils } from './sdk/dist/vea-sdk.esm.js'
</script>
```

```bash
# 方式2：npm安装（如果发布到npm）
npm install @vea/sdk
```

### 基础用法

```javascript
import { createAPI, utils } from '@vea/sdk'

// 创建API客户端
const api = createAPI('http://localhost:8080')

// 获取节点列表
const { nodes } = await api.get('/nodes')
console.log(`加载了 ${nodes.length} 个节点`)

// 创建节点
await api.post('/nodes', {
  name: '香港节点',
  address: 'hk.example.com',
  port: 443,
  protocol: 'vless',
  security: { uuid: 'your-uuid' }
})

// 使用工具函数
const { formatBytes, formatTime } = utils
console.log(formatBytes(1024000))  // "1000.0 KB"
console.log(formatTime(new Date())) // "2025/11/20 09:30:00"
```

## 完整API文档

### HTTP Client

#### createAPI(baseURL)

创建简化版API客户端

```javascript
const api = createAPI('http://localhost:8080')

await api.get('/nodes')
await api.post('/nodes', { name: 'test' })
await api.put('/nodes/123', { name: 'updated' })
await api.delete('/nodes/123')
```

#### VeaClient

完整功能的客户端类

```javascript
import { VeaClient } from '@vea/sdk'

const client = new VeaClient({
  baseURL: 'http://localhost:8080',
  timeout: 300000,  // 5分钟
  headers: { 'Authorization': 'Bearer token' }
})

// 使用资源API
await client.nodes.list()
await client.configs.import({ sourceUrl: '...' })
await client.xray.start()
```

### 资源API

#### Nodes API

```javascript
// 列出所有节点
const { nodes, activeNodeId, lastSelectedNodeId } = await client.nodes.list()

// 创建节点（通过分享链接）
await client.nodes.create({ shareLink: 'vmess://...' })

// 创建节点（手动）
await client.nodes.create({
  name: '香港节点',
  address: 'hk.example.com',
  port: 443,
  protocol: 'vless',
  tags: ['香港', '高速'],
  security: { uuid: 'your-uuid' }
})

// 测试延迟/速度
await client.nodes.ping('node-id')
await client.nodes.speedtest('node-id')

// 选择节点
await client.nodes.select('node-id')

// 批量操作
await client.nodes.bulkPing(['id1', 'id2'])
await client.nodes.resetSpeed(['id1', 'id2'])
```

#### Configs API

```javascript
// 导入配置/订阅
await client.configs.import({
  name: '我的订阅',
  sourceUrl: 'https://example.com/sub',
  autoUpdateIntervalMinutes: 60
})

// 刷新配置
await client.configs.refresh('config-id')

// 拉取配置中的节点
const nodes = await client.configs.pullNodes('config-id')
```

#### Xray API

```javascript
// 获取状态
const status = await client.xray.status()
// { enabled: true, running: true, activeNodeId: 'xxx', binary: '/path' }

// 启动/停止
await client.xray.start({ activeNodeId: 'node-id' })
await client.xray.stop()
```

#### Components API

```javascript
// 列出/安装组件
const components = await client.components.list()
await client.components.create({ kind: 'xray' })
await client.components.install('component-id')
```

#### Traffic API

```javascript
// 流量策略
const profile = await client.traffic.getProfile()
await client.traffic.updateProfile({
  defaultNodeId: 'node-id',
  dns: { strategy: 'UseIPv4', servers: ['8.8.8.8'] }
})

// 分流规则
await client.traffic.rules.list()
await client.traffic.rules.create({
  name: 'Google服务',
  targets: ['google.com', 'youtube.com'],
  nodeId: 'hk-node',
  priority: 10
})
```

#### Settings API

```javascript
// 系统代理
await client.settings.getSystemProxy()
await client.settings.updateSystemProxy({
  enabled: true,
  ignoreHosts: ['localhost', '127.0.0.1']
})
```

### 状态管理

#### createNodeStateManager(options)

节点状态管理器（Ping/测速cooldown）

```javascript
import { createNodeStateManager } from '@vea/sdk'

const stateManager = createNodeStateManager({
  pingCooldown: 60000,      // Ping冷却60秒
  speedtestCooldown: 60000  // 测速冷却60秒
})

if (stateManager.canPing('node-id', node)) {
  stateManager.startPing('node-id')
  await client.nodes.ping('node-id')
  stateManager.endPing()
}
```

#### resolvePreferredNode(options)

解析首选节点ID（优先级：savedNodeId > lastSelectedNodeId > activeNodeId > 第一个节点）

```javascript
import { resolvePreferredNode } from '@vea/sdk'

const nodeId = resolvePreferredNode({
  nodes, activeNodeId, lastSelectedNodeId, savedNodeId
})
```

#### createNodeIdStorage(key)

localStorage节点ID管理器

```javascript
import { createNodeIdStorage } from '@vea/sdk'

const storage = createNodeIdStorage('vea_selected_node_id')
storage.set('node-123')
const nodeId = storage.get()
```

#### createThemeManager()

主题管理器

```javascript
import { createThemeManager } from '@vea/sdk'

const themeManager = createThemeManager()
themeManager.switch('dark')  // 跳转到dark.html
themeManager.autoRedirect()  // 自动重定向到保存的主题
```

#### extractNodeTags(nodes) / filterNodesByTag(nodes, tag)

标签处理

```javascript
import { extractNodeTags, filterNodesByTag } from '@vea/sdk'

const tags = extractNodeTags(nodes)  // ['全部', '香港', '美国']
const filtered = filterNodesByTag(nodes, '香港')
```

### 工具函数

```javascript
import { utils } from '@vea/sdk'

// 格式化
utils.formatTime('2025-11-20T09:30:00Z')  // "2025/11/20 09:30:00"
utils.formatBytes(1048576)                // "1.0 MB"
utils.formatInterval(3600000000000)       // "60 分钟"
utils.formatLatency(50)                   // "50 ms"
utils.formatSpeed(15.678)                 // "15.7 MB/s"

// HTML转义
utils.escapeHtml('<script>')              // "&lt;script&gt;"

// 解析
utils.parseList('a,b,c')                  // ['a', 'b', 'c']
utils.parseNumber('123')                  // 123

// 异步工具
utils.sleep(1000)                         // Promise<void>
await utils.retry(() => api.get('/nodes'), { maxRetries: 3 })

// 性能工具
const debounced = utils.debounce(fn, 500)
const throttled = utils.throttle(fn, 100)
const poller = utils.createPoller(fn, 1000)
```

### NodeManager

带轮询的节点管理器

```javascript
import { createNodeManager, VeaClient } from '@vea/sdk'

const client = new VeaClient()
const nodeManager = createNodeManager(client, 1000)

// 监听节点变化
nodeManager.onUpdate(({ nodes, activeNodeId }) => {
  console.log(`更新了 ${nodes.length} 个节点`)
})

// 启动轮询
nodeManager.startPolling()

// 获取缓存
const nodes = nodeManager.getNodes()
const activeId = nodeManager.getActiveNodeId()

// 手动刷新
await nodeManager.refresh()

// 停止轮询
nodeManager.stopPolling()
```

## TypeScript支持

SDK包含完整的TypeScript类型定义：

```typescript
import { VeaClient, VeaError } from '@vea/sdk'

const client: VeaClient = new VeaClient()

try {
  await client.nodes.list()
} catch (error) {
  if (error instanceof VeaError) {
    console.error(`HTTP ${error.statusCode}: ${error.message}`)
  }
}
```

## 错误处理

所有API调用可能抛出`VeaError`：

```javascript
import { VeaError } from '@vea/sdk'

try {
  await client.nodes.create({ name: 'test' })
} catch (error) {
  if (error instanceof VeaError) {
    console.error(`状态码: ${error.statusCode}`)
    console.error(`错误信息: ${error.message}`)
    console.error(`响应数据:`, error.response)
  }
}
```

## 浏览器兼容性

- Chrome/Edge 88+
- Firefox 78+
- Safari 14+

需要支持：ES Modules、Fetch API、AbortController、Promise

## 开发

```bash
npm install  # 安装依赖
npm run build  # 构建 → dist/vea-sdk.esm.js (19KB)
```

## 项目结构

```
sdk/
├── src/
│   ├── vea-sdk.js        # 单源文件 (800行)
│   └── types.d.ts        # TypeScript类型
├── dist/
│   └── vea-sdk.esm.js    # 构建产物 (19KB)
├── package.json
├── rollup.config.js
└── README.md
```

## 许可证

MIT © 2025 Vea Project
