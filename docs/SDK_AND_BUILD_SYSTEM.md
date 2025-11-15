# Vea SDK 与构建系统实现方案

## 项目背景

Vea 是一个前后端分离的 Xray 管理服务。为了提升开发体验和支持未来的桌面客户端（Electron），需要：

1. **创建 JavaScript SDK**：简化前端开发，提供统一的 API 调用接口
2. **规范化构建系统**：确保本地构建与 CI/CD 一致
3. **支持多平台**：浏览器、Node.js、Electron、Tauri

## 技术方案

### 1. API 规范化

#### OpenAPI 3.0 规范
- **文件**：`api/openapi.yaml`（约 1450 行）
- **覆盖范围**：40+ API 端点
- **资源分类**：
  - 节点管理（nodes）
  - 配置管理（configs）
  - Geo 资源（geo）
  - 核心组件（components）
  - Xray 控制（xray）
  - 流量策略（traffic）
  - 系统设置（settings）

#### 版本管理策略
- **文件**：`api/versioning.md`
- **策略**：Semantic Versioning (SemVer)
  - v1.x.y：向后兼容，只增不删
  - 新增端点和可选字段不影响现有客户端
  - 破坏性变更发布为 v2.0.0

#### API 变更日志
- **文件**：`api/CHANGELOG.md`
- **记录**：每次 API 变更的详细说明

### 2. JavaScript SDK

#### 核心实现

**主要文件**：
- `sdk/src/client.js` (774 行) - HTTP 客户端和 API 封装
- `sdk/src/utils.js` (243 行) - 工具函数
- `sdk/src/types.d.ts` (492 行) - TypeScript 类型定义
- `sdk/src/index.js` - 统一导出入口

**技术特性**：
- ✅ 零依赖（使用原生 fetch API）
- ✅ TypeScript 类型支持
- ✅ 跨平台（浏览器、Node.js、Electron、Tauri）
- ✅ 轻量级（24KB ES Module）
- ✅ 超时控制（默认 5 分钟，支持大文件下载）
- ✅ 统一错误处理

**API 设计**：

```javascript
// 创建客户端
import { VeaClient } from './sdk/dist/vea-sdk.esm.js'

const client = new VeaClient({
  baseURL: 'http://localhost:8080',
  timeout: 300000  // 5 分钟
})

// 使用资源 API
await client.nodes.list()
await client.nodes.create({ name: 'Tokyo', address: '1.2.3.4' })
await client.xray.enable(nodeId)

// 向后兼容的简化 API
import { createAPI } from './sdk/dist/vea-sdk.esm.js'
const api = createAPI('http://localhost:8080')
await api.get('/nodes')
```

**工具函数**：
- `formatTime()` - 时间格式化
- `formatBytes()` - 字节格式化
- `formatInterval()` - 时间间隔格式化
- `formatLatency()` - 延迟格式化
- `formatSpeed()` - 速度格式化
- `escapeHtml()` - HTML 转义
- `parseList()` / `parseNumber()` - 数据解析
- `debounce()` / `throttle()` - 性能优化
- `createPoller()` - 轮询管理
- `retry()` - 重试机制

#### 构建配置

**Rollup 配置**：只生成 ES Module 格式
```javascript
// sdk/rollup.config.js
export default {
  input: 'src/index.js',
  output: {
    file: 'dist/vea-sdk.esm.js',
    format: 'es'
  },
  plugins: [resolve()]
}
```

**输出**：
- `sdk/dist/vea-sdk.esm.js` - 唯一输出，24KB

**为什么只用 ESM？**
- Electron 渲染进程标准使用 ES Module
- 现代浏览器原生支持 `<script type="module">`
- 打包工具（Vite、Webpack）优先使用 ESM
- 避免维护多种格式的复杂性

### 3. 前端迁移

#### web/index.html 改造

**迁移内容**：
- 删除内联的 API 对象定义（~50 行）
- 删除工具函数定义（~60 行）
- 使用 SDK 的 ES Module 导入

**改造前**：
```html
<script>
  const api = {
    async get(path) { /* ... */ },
    async post(path, body) { /* ... */ }
  }

  function formatTime(ts) { /* ... */ }
  function formatBytes(bytes) { /* ... */ }
  // ... 更多工具函数
</script>
```

**改造后**：
```html
<script type="module">
  import { createAPI, utils } from '/ui/sdk/dist/vea-sdk.esm.js';
  const { formatTime, formatBytes, formatInterval, escapeHtml } = utils;

  const api = createAPI('');
  // 业务逻辑完全不变
</script>
```

**效果**：
- ✅ 减少约 100 行重复代码
- ✅ 保持业务逻辑不变
- ✅ 统一 API 调用方式

### 4. 后端改进

#### CORS 支持

**位置**：`internal/api/router.go`

**实现**：
```go
func corsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
        c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
        c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }
        c.Next()
    }
}
```

**注意**：不使用 `Access-Control-Allow-Credentials: true`，因为与 `Allow-Origin: *` 冲突，会导致浏览器拒绝请求。

#### 超时时间调整

**位置**：`internal/service/service.go`

**改动**：
```go
// 下载超时：45秒 → 5分钟（支持慢速网络下载 Xray）
downloadTimeout = 5 * time.Minute

// 连接超时：30秒 → 60秒
DialContext: (&net.Dialer{Timeout: 60 * time.Second}).DialContext

// TLS 握手超时：10秒 → 30秒
TLSHandshakeTimeout: 30 * time.Second
```

**理由**：Xray 二进制文件约 10-30MB，慢速网络需要更长时间。

#### 日志级别控制

**位置**：`cmd/server/main.go`

**功能**：通过 `--dev` 参数控制日志输出

```go
if *dev {
    gin.SetMode(gin.DebugMode)
    log.SetFlags(log.LstdFlags | log.Lshortfile)  // 显示文件名和行号
    log.Println("运行在开发模式 - 显示所有日志")
} else {
    gin.SetMode(gin.ReleaseMode)
    log.SetOutput(&errorOnlyWriter{})  // 只显示错误日志
}
```

**使用**：
- `make dev` - 开发模式，显示所有日志
- `make run` - 生产模式，只显示错误

### 5. 构建系统

#### Makefile 设计

**核心目标**：
```makefile
build: prepare              # 快速构建（日常开发）
build-release: prepare      # 发布版本（与 CI 相同）
dev:                        # 开发模式（go run）
run: build                  # 编译并运行
clean:                      # 清理产物
```

**prepare 目标**（关键）：
```makefile
prepare: ## 准备构建环境
	@echo "==> 准备构建环境..."
	@mkdir -p $(OUTPUT_DIR)
	@mkdir -p cmd/server/web/sdk/dist
	@echo "==> 复制 web 资源..."
	@cp web/index.html cmd/server/web/index.html
	@if [ -d sdk/dist ]; then \
		cp -r sdk/dist/. cmd/server/web/sdk/dist/; \
	else \
		echo "警告: sdk/dist/ 不存在，将使用空 SDK 目录"; \
		echo "提示: 如需完整功能，请先运行 'make build-sdk'"; \
	fi
```

**设计原则**：
- ✅ 检查 SDK 是否存在（容错性）
- ✅ 给出明确的错误提示和解决方案
- ✅ 不强制依赖 Node.js（SDK 已预构建）

#### .gitignore 配置

**SDK 文件追踪**：
```gitignore
dist/                    # 忽略根目录的 dist/
!sdk/dist/              # 不忽略 SDK 的 dist/
!sdk/dist/vea-sdk.esm.js  # 显式保留 ESM 文件
```

**理由**：
- SDK 文件已构建，提交到仓库
- 干净 clone 后可以直接 `make build`
- Node.js/npm 确实是可选的

### 6. CI/CD 集成

#### GitHub Actions 测试 workflow

**位置**：`.github/workflows/test.yml`

**改进**：预下载 xray 二进制，避免测试时触发 GitHub API 速率限制

```yaml
- name: Download Xray for tests (Unix)
  env:
    RELEASE_TOKEN: ${{ secrets.RELEASE_TOKEN }}
  run: |
    mkdir -p artifacts/core/xray
    XRAY_VERSION=$(curl -s -H "Authorization: token $RELEASE_TOKEN" ...)
    curl -sL -H "Authorization: token $RELEASE_TOKEN" ... -o /tmp/xray.zip
    unzip -q /tmp/xray.zip -d artifacts/core/xray/
    chmod +x artifacts/core/xray/xray
```

**配置需求**：需要在仓库设置中添加 `RELEASE_TOKEN` secret（GitHub Personal Access Token）。

#### Release workflow 改进（待完成）

**目标**：自动复制 SDK 文件到 embed 目录

```yaml
- name: Prepare web assets for embed
  run: |
    mkdir -p cmd/server/web/sdk/dist
    cp web/index.html cmd/server/web/
    if [ -d sdk/dist ] && [ -f sdk/dist/vea-sdk.esm.js ]; then
      cp -r sdk/dist/. cmd/server/web/sdk/dist/
    else
      echo "警告: SDK 文件不存在"
    fi
```

## 已知问题与修复

### 🔴 CORS 配置错误（已修复）

**问题**：
```go
c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")  // 冲突！
```

**后果**：所有跨域请求被浏览器拒绝，CORS 完全失效。

**修复**：移除 `Access-Control-Allow-Credentials` header。

**参考**：https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS#credentialed_requests_and_wildcards

### ✅ SDK 构建产物管理

**问题**：原计划生成 4 种格式（UMD、UMD minified、ESM、CJS），但实际只需要 ESM。

**优化**：
- 删除 CJS/UMD 格式（节省 ~90KB）
- 简化 Rollup 配置
- 更新文档说明

## 文件清单

### 新增文件

**API 规范**：
- `api/openapi.yaml` (1450 行)
- `api/versioning.md` (327 行)
- `api/CHANGELOG.md` (179 行)
- `docs/api/README.md` (324 行)

**SDK 源码**：
- `sdk/src/client.js` (774 行)
- `sdk/src/utils.js` (243 行)
- `sdk/src/types.d.ts` (492 行)
- `sdk/src/index.js` (入口文件)
- `sdk/package.json`
- `sdk/rollup.config.js`
- `sdk/README.md` (680 行)

**SDK 构建产物**：
- `sdk/dist/vea-sdk.esm.js` (1019 行，24KB)

**构建文档**：
- `BUILD.md` (360 行)
- `Makefile` (115 行)

### 修改文件

**前端**：
- `web/index.html` - 迁移到使用 SDK（减少 ~100 行）

**后端**：
- `internal/api/router.go` - 添加 CORS 中间件
- `internal/service/service.go` - 调整超时时间
- `cmd/server/main.go` - 添加日志级别控制

**CI/CD**：
- `.github/workflows/test.yml` - 预下载 xray
- `.github/workflows/release.yml` - （待更新）复制 SDK 文件

**配置**：
- `.gitignore` - 保留 SDK 构建产物

## 统计数据

- **新增代码**：~9000 行
- **删除代码**：~100 行（前端重复代码）
- **文件数量**：25 个文件变更
- **SDK 大小**：24KB（ES Module）
- **API 端点**：40+ 个
- **构建时间**：~30 秒（本地）

## 下一步工作

### 1. Release workflow 完善

**任务**：更新 `.github/workflows/release.yml`

**代码**：
```yaml
# Linux 和 macOS
- name: Prepare web assets for embed
  run: |
    mkdir -p cmd/server/web/sdk/dist
    cp web/index.html cmd/server/web/
    if [ -d sdk/dist ] && [ -f sdk/dist/vea-sdk.esm.js ]; then
      cp -r sdk/dist/. cmd/server/web/sdk/dist/
    else
      echo "警告: SDK 文件不存在"
    fi

# Windows
- name: Prepare web assets for embed
  shell: pwsh
  run: |
    New-Item -ItemType Directory -Path cmd/server/web/sdk/dist -Force
    Copy-Item -Path web/index.html -Destination cmd/server/web/
    if (Test-Path sdk/dist/vea-sdk.esm.js) {
      Copy-Item -Path sdk/dist/* -Destination cmd/server/web/sdk/dist/ -Recurse
    } else {
      Write-Host "警告: SDK 文件不存在"
    }
```

### 2. Electron 客户端开发

**技术栈**：
- Electron
- ES Module 导入 SDK
- Vite（打包工具）

**SDK 使用**：
```javascript
// 渲染进程
import { VeaClient } from './sdk/dist/vea-sdk.esm.js'

const client = new VeaClient({
  baseURL: 'http://localhost:8080'
})

// 启动本地 Vea 服务
const { spawn } = require('child_process')
const veaProcess = spawn('./vea', ['--addr', ':8080'])
```

### 3. 可能的优化

**SDK**：
- [ ] 添加请求拦截器（用于日志、认证等）
- [ ] 支持 WebSocket 连接（实时事件）
- [ ] 添加请求取消功能（AbortController）

**构建**：
- [ ] 添加 `make watch` 监听文件变化
- [ ] 集成代码检查（ESLint）
- [ ] 添加单元测试

**文档**：
- [ ] API 文档生成（从 OpenAPI）
- [ ] SDK 使用示例（更多场景）
- [ ] Electron 开发指南

## 技术决策记录

### 为什么选择 ES Module 而不是 UMD？

**决策**：只构建 ESM 格式，删除 UMD/CJS。

**理由**：
1. **Electron 标准**：渲染进程使用 `<script type="module">`
2. **现代浏览器**：已全面支持 ES Module
3. **打包工具优先**：Vite、Webpack 优先使用 ESM
4. **维护简化**：避免维护多种格式
5. **减少体积**：节省 ~90KB

**权衡**：
- ✅ 简化构建流程
- ✅ 减少仓库体积
- ❌ 不支持旧浏览器（IE、旧版 Chrome）
- ❌ 无法直接用 `<script src="...">` 加载（需要 `type="module"`）

**结论**：适用于 Vea 项目（本地管理工具，不需要兼容旧浏览器）。

### 为什么提交 SDK 构建产物到 git？

**决策**：将 `sdk/dist/vea-sdk.esm.js` 提交到仓库。

**理由**：
1. **零依赖构建**：clone 后无需 Node.js 即可 `make build`
2. **CI/CD 简化**：不需要在 CI 中构建 SDK
3. **文件稳定**：SDK 不频繁变动
4. **文件小**：仅 24KB

**权衡**：
- ✅ 简化构建流程
- ✅ 降低环境依赖
- ❌ 增加 git 仓库大小（24KB 可接受）
- ❌ 需要手动 `make build-sdk` 后提交

**结论**：利大于弊，适合本项目。

### CORS 为什么不使用 Credentials？

**决策**：`Access-Control-Allow-Origin: *` 不配合 `Allow-Credentials: true`。

**理由**：
1. **规范限制**：两者不能同时使用（浏览器会拒绝）
2. **Vea 特性**：本地管理工具，不需要携带 Cookie/认证
3. **简化配置**：允许所有来源足够

**如果需要认证**：
```go
origin := c.Request.Header.Get("Origin")
if origin != "" {
    c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
    c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
}
```

## 参考资料

- [OpenAPI 3.0 规范](https://spec.openapis.org/oas/v3.0.3)
- [MDN - Fetch API](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API)
- [MDN - CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)
- [Electron 文档](https://www.electronjs.org/docs/latest/)
- [Rollup 文档](https://rollupjs.org/)

## 总结

本方案实现了完整的 SDK 和构建系统，核心成果：

1. ✅ **标准化 API**：OpenAPI 3.0 规范 + 版本管理
2. ✅ **零依赖 SDK**：24KB ES Module，跨平台支持
3. ✅ **简化前端**：减少 100 行重复代码
4. ✅ **健壮构建**：本地构建与 CI/CD 一致
5. ✅ **开发体验**：详细文档 + 友好错误提示

为 Electron 客户端开发奠定了坚实基础。
