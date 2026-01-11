# Vea SDK

Vea 后端 HTTP API JavaScript SDK - 单文件、零依赖、TypeScript类型支持

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
const api = createAPI('http://localhost:19080')

// 获取 FRouter 列表
const { frouters } = await api.get('/frouters')
console.log(`加载了 ${frouters.length} 个 FRouter`)

// 创建 FRouter
await api.post('/frouters', {
  name: '香港 FRouter'
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
const api = createAPI('http://localhost:19080')

await api.get('/frouters')
await api.post('/frouters', { name: 'test' })
await api.put('/frouters/123', { name: 'updated' })
await api.delete('/frouters/123')
```

#### VeaClient

完整功能的客户端类

```javascript
import { VeaClient } from '@vea/sdk'

const client = new VeaClient({
  baseURL: 'http://localhost:19080',
  timeout: 300000,  // 5分钟
  headers: { 'Authorization': 'Bearer token' }
})

// 使用资源API
await client.frouters.list()
await client.configs.import({ sourceUrl: '...' })

// 启动代理：以 FRouter 为中心（更新配置 + 启动）
await client.proxy.updateConfig({
  inboundMode: 'mixed',
  inboundPort: 1080,
  preferredEngine: 'auto'
})
await client.proxy.start({ frouterId: 'frouter-id' })
```

### 资源API

#### FRouters API

```javascript
// 列出所有 FRouter
const { frouters } = await client.frouters.list()

// 创建 FRouter
await client.frouters.create({ name: '香港 FRouter' })

// FRouter 测量
await client.frouters.measureLatency('frouter-id')
await client.frouters.measureSpeed('frouter-id')

// 批量操作
await client.frouters.bulkPing(['id1', 'id2'])
await client.frouters.resetSpeed(['id1', 'id2'])
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

// 同步配置中的节点
const { nodes } = await client.configs.pullNodes('config-id')
```

#### Proxy API

```javascript
// 获取/更新运行配置（单例）
const config = await client.proxy.getConfig()
await client.proxy.updateConfig({ inboundMode: 'tun' })

// 启动代理（切换只需要换 frouterId）
await client.proxy.start({ frouterId: 'frouter-id' })

// 获取状态
const status = await client.proxy.status()
// { running: true, pid: 1234, engine: 'singbox', inboundMode: 'mixed', inboundPort: 1080, frouterId: '...' }

// 停止代理
await client.proxy.stop()
```

#### Components API

```javascript
// 列出/安装组件
const components = await client.components.list()
await client.components.create({ kind: 'singbox' })
await client.components.install('component-id')
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
await utils.retry(() => api.get('/frouters'), { maxRetries: 3 })

// 性能工具
const debounced = utils.debounce(fn, 500)
const throttled = utils.throttle(fn, 100)
const poller = utils.createPoller(fn, 1000)
```

## TypeScript支持

SDK包含完整的TypeScript类型定义：

```typescript
import { VeaClient, VeaError } from '@vea/sdk'

const client: VeaClient = new VeaClient()

try {
  await client.frouters.list()
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
  await client.frouters.create({ name: 'test' })
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
