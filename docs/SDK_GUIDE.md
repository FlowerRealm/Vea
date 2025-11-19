# Vea SDK 使用指南

适用于前端开发者，快速上手 Vea JavaScript SDK

---

## 📦 什么是 Vea SDK？

Vea SDK 是一个**零依赖**的 JavaScript 库，用于与 Vea 后端 API 通信。

**核心特性**：
- ✅ **轻量级** - 仅 24KB（ES Module）
- ✅ **零依赖** - 使用原生 `fetch` API
- ✅ **跨平台** - 浏览器、Node.js、Electron 全兼容
- ✅ **TypeScript** - 完整类型定义
- ✅ **Promise 异步** - 现代化 API

---

## 🚀 快速开始

### 1. 导入 SDK

```javascript
// 在 Electron 渲染进程中
import { createAPI } from '../sdk/dist/vea-sdk.esm.js';

// 创建 API 实例（默认连接到 localhost:8080）
const api = createAPI('');
```

### 2. 基本用法

```javascript
// 获取所有节点
const result = await api.get('/nodes');
console.log(result.nodes);

// 创建新节点
const newNode = await api.post('/nodes', {
  shareLink: 'vmess://...'
});

// 更新节点
const updated = await api.put('/nodes/node-id', {
  name: 'New Name'
});

// 删除节点
await api.delete('/nodes/node-id');
```

---

## 📖 API 分类速查

### 🔌 节点管理

```javascript
// 列出所有节点
const { nodes, activeNodeId } = await api.get('/nodes');

// 创建节点（从分享链接）
await api.post('/nodes', {
  shareLink: 'vmess://base64...'
});

// 创建节点（手动）
await api.post('/nodes', {
  name: '东京节点',
  address: '1.2.3.4',
  port: 443,
  protocol: 'vless',
  tags: ['premium']
});

// 更新节点
await api.put('/nodes/node-id', {
  name: '新名称',
  tags: ['updated']
});

// 删除节点
await api.delete('/nodes/node-id');

// 测试延迟
await api.post('/nodes/node-id/ping');

// 测速
await api.post('/nodes/node-id/speedtest');

// 批量延迟测试
await api.post('/nodes/bulk-ping');

// 选择节点（切换 Xray）
await api.post('/nodes/node-id/select');
```

### ⚙️ 配置管理

```javascript
// 列出所有配置
const configs = await api.get('/configs');

// 导入配置
await api.post('/configs', {
  name: '机场配置',
  format: 'xray-json',
  sourceUrl: 'https://example.com/config.json',
  autoUpdateInterval: 60  // 分钟
});

// 刷新配置
await api.post('/configs/config-id/refresh');

// 拉取节点
await api.post('/configs/config-id/pull-nodes');

// 删除配置
await api.delete('/configs/config-id');
```

### 🔧 核心组件管理

```javascript
// 列出所有组件
const components = await api.get('/components');

// 创建 Xray 组件记录
await api.post('/components', {
  kind: 'xray'
});

// 安装组件
await api.post('/components/component-id/install');

// 删除组件
await api.delete('/components/component-id');
```

### ⚡ Xray 控制

```javascript
// 获取 Xray 状态
const status = await api.get('/xray/status');
// => { enabled: true, running: true, activeNodeId: "...", binary: "..." }

// 启动 Xray（使用默认节点）
await api.post('/xray/start');

// 启动 Xray（指定节点）
await api.post('/xray/start', {
  activeNodeId: 'node-id'
});

// 停止 Xray
await api.post('/xray/stop');
```

### 🌐 流量策略

```javascript
// 获取流量策略
const profile = await api.get('/traffic/profile');

// 更新流量策略
await api.put('/traffic/profile', {
  defaultNodeId: 'node-id',
  dns: {
    strategy: 'ipv4-only',
    servers: ['8.8.8.8', '1.1.1.1']
  }
});

// 列出分流规则
const rules = await api.get('/traffic/rules');

// 创建分流规则
await api.post('/traffic/rules', {
  name: 'Netflix',
  targets: ['netflix.com', 'geosite:netflix'],
  nodeId: 'node-id',
  priority: 10
});

// 更新规则
await api.put('/traffic/rules/rule-id', {
  priority: 20
});

// 删除规则
await api.delete('/traffic/rules/rule-id');
```

### 🖥️ 系统设置

```javascript
// 获取系统代理设置
const result = await api.get('/settings/system-proxy');
const { enabled, ignoreHosts } = result.settings;

// 更新系统代理
await api.put('/settings/system-proxy', {
  enabled: true,
  ignoreHosts: ['localhost', '127.0.0.0/8', '::1']
});
```

### 🌍 Geo 资源

```javascript
// 列出 Geo 资源
const geoResources = await api.get('/geo');

// 创建 Geo 资源
await api.post('/geo', {
  name: 'GeoIP',
  type: 'geoip',
  sourceUrl: 'https://github.com/.../geoip.dat'
});

// 刷新 Geo 资源
await api.post('/geo/geo-id/refresh');
```

---

## 💡 实用模式

### 模式 1: 错误处理

```javascript
try {
  const nodes = await api.get('/nodes');
  console.log('成功:', nodes);
} catch (error) {
  console.error('失败:', error.message);

  // 显示错误提示
  showStatus(`操作失败：${error.message}`, 'error');
}
```

### 模式 2: 轮询数据

```javascript
// 每秒刷新节点列表
const pollHandle = setInterval(async () => {
  try {
    const result = await api.get('/nodes');
    updateNodeUI(result.nodes);
  } catch (error) {
    console.error('轮询失败:', error);
  }
}, 1000);

// 停止轮询
clearInterval(pollHandle);
```

### 模式 3: 批量操作

```javascript
// 批量测试所有节点延迟
async function pingAllNodes() {
  const { nodes } = await api.get('/nodes');

  // 触发批量 ping
  await api.post('/nodes/bulk-ping');

  // 等待一段时间
  await sleep(3000);

  // 刷新获取结果
  const updated = await api.get('/nodes');
  return updated.nodes;
}
```

### 模式 4: 智能测量

```javascript
// 智能延迟测试（带冷却）
async function smartPing(nodeId) {
  const nodes = await api.get('/nodes');
  const node = nodes.nodes.find(n => n.id === nodeId);

  if (!node) return;

  // 检查是否最近测试过
  const lastPingTime = new Date(node.lastLatencyAt).getTime();
  const now = Date.now();

  if (now - lastPingTime < 60000) {  // 60秒冷却
    console.log('最近已测试过，跳过');
    return;
  }

  // 触发测试
  await api.post(`/nodes/${nodeId}/ping`);
}
```

### 模式 5: 一键启动代理

```javascript
async function startProxy(nodeId) {
  try {
    // 1. 启动 Xray（指定节点）
    await api.post('/xray/start', { activeNodeId: nodeId });

    // 2. 等待 Xray 运行
    await sleep(1000);

    // 3. 启用系统代理
    await api.put('/settings/system-proxy', {
      enabled: true,
      ignoreHosts: ['localhost', '127.0.0.0/8', '::1']
    });

    showStatus('代理已启动', 'success');
  } catch (error) {
    showStatus(`启动失败：${error.message}`, 'error');
  }
}
```

---

## 🎯 完整示例

### 节点管理面板

```javascript
import { createAPI } from '../sdk/dist/vea-sdk.esm.js';

const api = createAPI('');
let nodesCache = [];

// 加载节点列表
async function loadNodes() {
  try {
    const result = await api.get('/nodes');
    nodesCache = result.nodes;
    renderNodes(result.nodes);
  } catch (error) {
    showError(`加载失败：${error.message}`);
  }
}

// 渲染节点
function renderNodes(nodes) {
  const html = nodes.map(node => `
    <div class="node-card" data-id="${node.id}">
      <h3>${node.name}</h3>
      <p>${node.address}:${node.port}</p>
      <span class="latency">${node.lastLatencyMs || '~'} ms</span>
      <button onclick="pingNode('${node.id}')">测延迟</button>
      <button onclick="deleteNode('${node.id}')">删除</button>
    </div>
  `).join('');

  document.getElementById('node-list').innerHTML = html;
}

// 测试延迟
async function pingNode(nodeId) {
  try {
    await api.post(`/nodes/${nodeId}/ping`);

    // 等待结果
    setTimeout(loadNodes, 2000);
  } catch (error) {
    showError(`测试失败：${error.message}`);
  }
}

// 删除节点
async function deleteNode(nodeId) {
  if (!confirm('确认删除？')) return;

  try {
    await api.delete(`/nodes/${nodeId}`);
    loadNodes();
  } catch (error) {
    showError(`删除失败：${error.message}`);
  }
}

// 初始化
loadNodes();
setInterval(loadNodes, 1000);  // 每秒刷新
```

---

## 📚 高级技巧

### 使用 utils 工具函数

```javascript
import { createAPI, utils } from '../sdk/dist/vea-sdk.esm.js';

const { formatTime, formatBytes, escapeHtml, sleep } = utils;

// 格式化时间
const time = formatTime('2024-11-19T08:00:00Z');
// => "2024-11-19 08:00:00"

// 格式化字节
const size = formatBytes(1024000);
// => "1.00 MB"

// HTML 转义
const safe = escapeHtml('<script>alert("xss")</script>');
// => "&lt;script&gt;alert(&quot;xss&quot;)&lt;/script&gt;"

// 延迟执行
await sleep(1000);  // 等待 1 秒
```

### 自定义请求

```javascript
// 如果 API 有新端点未封装，可以直接调用
const result = await api.request({
  method: 'POST',
  path: '/custom/endpoint',
  body: { foo: 'bar' }
});
```

---

## ⚠️ 常见问题

### Q: 如何处理超时？

SDK 默认超时 30 秒。可以在创建 API 时配置：

```javascript
const api = createAPI('', { timeout: 60000 });  // 60秒超时
```

### Q: 如何知道请求是否成功？

所有 API 调用成功时返回数据，失败时抛出异常：

```javascript
try {
  const nodes = await api.get('/nodes');
  // 成功：nodes 包含数据
} catch (error) {
  // 失败：error.message 包含错误信息
}
```

### Q: 异步操作（ping/speedtest）如何获取结果？

这些操作是异步的，结果会更新到节点对象。需要稍后重新获取：

```javascript
// 触发测试
await api.post('/nodes/node-id/ping');

// 等待 2 秒
await sleep(2000);

// 获取更新后的节点
const { nodes } = await api.get('/nodes');
const node = nodes.find(n => n.id === 'node-id');
console.log(node.lastLatencyMs);  // 更新后的延迟
```

---

## 📖 更多文档

- **完整 API 参考**：[frontend/sdk/README.md](../frontend/sdk/README.md)
- **TypeScript 类型**：[frontend/sdk/src/types.d.ts](../frontend/sdk/src/types.d.ts)
- **后端 API 规范**：[docs/api/](./api/)

---

**快速链接**：
- 🏠 [项目首页](../README.md)
- 🏗️ [目录结构](../STRUCTURE.md)
- ⚙️ [构建指南](../BUILD.md)
