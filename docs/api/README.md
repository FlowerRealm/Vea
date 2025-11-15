# Vea API 文档

本目录包含 Vea Backend API 的完整规范和 JavaScript SDK。

## 目录结构

```
.
├── api/
│   ├── openapi.yaml        # OpenAPI 3.0 规范（核心 API 文档）
│   ├── versioning.md       # API 版本管理策略
│   └── CHANGELOG.md        # API 变更日志
├── sdk/
│   ├── src/
│   │   ├── client.js       # SDK 核心实现
│   │   └── types.d.ts      # TypeScript 类型定义
│   ├── dist/               # 构建产物（npm run build 生成）
│   ├── examples/           # 使用示例
│   │   ├── browser.html    # 浏览器示例
│   │   ├── electron-example.js  # Electron 示例
│   │   ├── preload.js      # Electron preload 脚本
│   │   └── react-example.jsx    # React 示例
│   ├── package.json        # NPM 包配置
│   ├── rollup.config.js    # 打包配置
│   └── README.md           # SDK 使用文档
└── docs/
    └── api/
        └── README.md       # 本文件
```

## 快速开始

### 1. 查看 API 规范

API 规范使用 OpenAPI 3.0 格式编写，位于 `api/openapi.yaml`。

**在线查看**：
- 使用 [Swagger Editor](https://editor.swagger.io/)：导入 `api/openapi.yaml` 文件
- 使用 VS Code 插件：安装 [OpenAPI (Swagger) Editor](https://marketplace.visualstudio.com/items?itemName=42Crunch.vscode-openapi)

**本地查看**：
```bash
# 安装 Swagger UI
npm install -g swagger-ui-watcher

# 启动查看器
swagger-ui-watcher api/openapi.yaml
```

### 2. 使用 JavaScript SDK

SDK 支持浏览器、Node.js、Electron/Tauri 等环境。

#### 安装

```bash
cd sdk
npm install
npm run build
```

#### 基础使用

```javascript
const { VeaClient } = require('@vea/sdk')

const vea = new VeaClient({
  baseURL: 'http://localhost:8080'
})

// 列出节点
const nodes = await vea.nodes.list()

// 创建节点
const node = await vea.nodes.create({
  name: 'Tokyo',
  address: '1.2.3.4',
  port: 443,
  protocol: 'vless'
})

// 启动 Xray
await vea.xray.start({ activeNodeId: node.id })
```

详见 [SDK 文档](../sdk/README.md)。

### 3. 查看示例代码

所有示例位于 `sdk/examples/` 目录：

- **浏览器**：`browser.html` - 直接在浏览器中打开即可运行
- **Electron**：`electron-example.js` - 完整的桌面应用示例
- **React**：`react-example.jsx` - React Hooks 示例

## API 概览

### 核心资源

| 资源 | 端点 | 说明 |
|------|------|------|
| 节点 | `/nodes` | 管理代理节点（vless/trojan/shadowsocks/vmess） |
| 配置 | `/configs` | 管理 Xray 配置和订阅链接 |
| Geo | `/geo` | 管理 GeoIP/GeoSite 资源 |
| 组件 | `/components` | 管理 Xray 核心组件 |
| Xray | `/xray` | 控制 Xray 服务启停 |
| 流量 | `/traffic` | 流量策略和分流规则 |
| 设置 | `/settings` | 系统代理等设置 |

### 主要功能

#### 节点管理
- ✅ 从分享链接导入节点（vmess/vless/trojan/ss）
- ✅ 手动创建/编辑节点
- ✅ 延迟测试、速度测试
- ✅ 流量统计
- ✅ 节点切换

#### 配置管理
- ✅ 导入 Xray JSON 配置
- ✅ 订阅链接自动更新
- ✅ 从配置提取节点
- ✅ 流量统计

#### Xray 控制
- ✅ 启动/停止 Xray 服务
- ✅ 实时状态查询
- ✅ 动态切换节点

#### 流量策略
- ✅ 默认出口配置
- ✅ DNS 策略
- ✅ 分流规则（domain/IP routing）

## 版本管理

Vea API 遵循 [语义化版本 2.0.0](https://semver.org/lang/zh-CN/)。

### 兼容性承诺

- **v1.x.y 系列**：向后兼容，只增不删
  - ✅ 新增端点
  - ✅ 新增可选字段
  - ✅ 新增枚举值
  - ❌ 删除端点
  - ❌ 删除字段
  - ❌ 改变字段类型

- **v2.0.0**：可能包含破坏性变更
  - 提前 6 个月通知
  - 保留 v1 端点至少 6 个月
  - 提供迁移指南

详见 [版本管理策略](../api/versioning.md)。

### SDK 版本对应

| SDK 版本 | API 版本 | 兼容性 |
|----------|----------|--------|
| @vea/sdk@1.0.x | v1.0.x | ✅ 完全兼容 |
| @vea/sdk@1.x.y | v1.x.y | ⚠️ 向后兼容 |
| @vea/sdk@2.0.x | v2.0.x | ❌ 不兼容 v1.x |

## 错误处理

所有 API 端点使用标准 HTTP 状态码：

| 状态码 | 说明 |
|--------|------|
| 200 OK | 请求成功 |
| 201 Created | 资源创建成功 |
| 202 Accepted | 异步任务已提交 |
| 204 No Content | 成功，无返回内容 |
| 400 Bad Request | 请求参数错误 |
| 404 Not Found | 资源不存在 |
| 500 Internal Server Error | 服务器内部错误 |

错误响应格式：
```json
{
  "error": "错误描述信息"
}
```

## 开发工具

### API 规范验证

```bash
# 安装 OpenAPI CLI
npm install -g @apidevtools/swagger-cli

# 验证规范
swagger-cli validate api/openapi.yaml
```

### 自动生成客户端

可以使用 OpenAPI Generator 生成其他语言的客户端：

```bash
# 安装 OpenAPI Generator
npm install -g @openapitools/openapi-generator-cli

# 生成 Python 客户端
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g python \
  -o clients/python

# 生成 Go 客户端
openapi-generator-cli generate \
  -i api/openapi.yaml \
  -g go \
  -o clients/go
```

支持的语言：TypeScript, Python, Go, Rust, Swift, Kotlin, Java 等 50+ 种语言。

## 测试

### 测试 API

```bash
# 启动后端服务
go run ./cmd/server --addr :8080

# 健康检查
curl http://localhost:8080/health

# 列出节点
curl http://localhost:8080/nodes

# 创建节点
curl -X POST http://localhost:8080/nodes \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test Node",
    "address": "1.2.3.4",
    "port": 443,
    "protocol": "vless"
  }'
```

### 测试 SDK

```bash
cd sdk

# 运行浏览器示例
# 1. 先构建 SDK
npm run build

# 2. 启动一个简单的 HTTP 服务器
npx http-server . -p 3000

# 3. 在浏览器中打开
open http://localhost:3000/examples/browser.html
```

## 常见问题

### Q: 如何为 API 添加认证？

A: 当前版本 (v1.0.0) 不支持认证。如需认证，可以：
1. 使用反向代理（Nginx/Traefik）添加 Basic Auth
2. 等待 v2.0.0（计划支持 OAuth2/JWT）

### Q: SDK 支持哪些环境？

A: SDK 基于 `fetch` API，支持：
- ✅ 现代浏览器（Chrome 42+, Firefox 39+, Safari 10.1+）
- ✅ Node.js 18+（内置 fetch）
- ✅ Node.js 14-17（需要安装 `node-fetch` polyfill）
- ✅ Electron
- ✅ Tauri
- ✅ React Native（需要 polyfill）

### Q: 如何处理 CORS？

A: 如果前端和后端不在同一域名：
1. 后端添加 CORS 头
2. 或使用代理（开发时可用 webpack devServer proxy）

### Q: 性能优化建议？

A:
1. **批量操作**：使用 `bulkPing()` 而不是循环调用 `ping()`
2. **轮询间隔**：节点状态轮询建议 3-5 秒
3. **缓存**：节点列表可以本地缓存，减少请求
4. **超时控制**：为长时间操作设置合理的 timeout

## 贡献

欢迎贡献代码、文档和示例！

- [GitHub Issues](https://github.com/FlowerRealm/Vea/issues)
- [Pull Requests](https://github.com/FlowerRealm/Vea/pulls)

### 提交 API 变更

1. 更新 `api/openapi.yaml`
2. 更新 `api/CHANGELOG.md`
3. 如果是破坏性变更，更新 `api/versioning.md`
4. 提交 PR 并说明变更原因

### 提交 SDK 变更

1. 更新 `sdk/src/client.js`
2. 更新 `sdk/src/types.d.ts`（如果涉及类型）
3. 添加示例代码（如果是新功能）
4. 更新 `sdk/README.md`
5. 运行 `npm run build` 确保构建成功

## 许可证

[MIT](../../LICENSE)

## 相关链接

- [Vea Backend 主仓库](https://github.com/FlowerRealm/Vea)
- [OpenAPI 规范](https://swagger.io/specification/)
- [语义化版本](https://semver.org/lang/zh-CN/)
- [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)
