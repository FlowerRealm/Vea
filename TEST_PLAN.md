# Vea Electron 客户端 - 测试计划

## 架构说明

新架构为 **Electron桌面应用 + Go后端**：
- **Go后端**：HTTP API服务（默认端口8080）
- **Electron前端**：主进程（main.js）+ 渲染进程（dark.html/light.html）
- **通信方式**：渲染进程通过SDK调用HTTP API

---

## 1. 后端测试（Go）

### 1.1 单元测试

**已有测试**：
- ✅ `backend/store/memory_test.go` - 存储层测试
  - 配置删除级联删除节点
  - 孤儿节点清理
- ✅ `backend/service/service_test.go` - HTTP/1.1降级逻辑
  - 各种网络错误的重试判断

**执行命令**：
```bash
go test ./backend/store -v
go test ./backend/service -v -short
```

### 1.2 集成测试

**已有测试**：
- ✅ `backend/service/integration_test.go` - E2E代理链路测试
  - 启动本地xray服务器
  - Vea管理xray客户端连接
  - 通过SOCKS5代理访问Cloudflare
  - 延迟和速度测试

**执行命令**：
```bash
go test ./backend/service -v -run TestE2E
```

**需要补充**：
- [ ] API路由集成测试（测试HTTP端点）
- [ ] 并发测速功能测试（验证5节点并发）
- [ ] 节点CRUD操作测试
- [ ] Xray启动/停止测试

---

## 2. 前端测试（JavaScript/Electron）

### 2.1 SDK单元测试

**待新增**：`frontend/sdk/test/vea-sdk.test.js`

测试内容：
- [ ] VeaClient初始化和配置
- [ ] HTTP请求封装（GET/POST/PUT/DELETE）
- [ ] 错误处理和VeaError
- [ ] 工具函数（formatTime, formatBytes, formatSpeed等）
- [ ] 状态管理（NodeStateManager, ThemeManager等）

**执行命令**：
```bash
cd frontend/sdk && npm test
```

### 2.2 Electron主进程测试

**待新增**：`frontend/test/main.test.js`

测试内容：
- [ ] 窗口创建和管理
- [ ] 后端健康检查
- [ ] 后端进程管理（启动/停止）
- [ ] IPC通信
- [ ] 应用生命周期

**执行命令**：
```bash
cd frontend && npm test
```

### 2.3 渲染进程集成测试

**待新增**：`frontend/test/renderer.test.js`

测试内容：
- [ ] SDK与后端API通信
- [ ] 主题切换功能
- [ ] 节点列表渲染
- [ ] 用户交互（ping, speedtest, 节点选择）
- [ ] 错误提示显示

**工具**：可使用Spectron或Playwright for Electron

---

## 3. 端到端测试

### 3.1 完整应用测试

**待新增**：`test/e2e/`

**测试场景**：

#### 应用启动流程
- [ ] Go后端自动启动
- [ ] Electron窗口打开
- [ ] 健康检查通过

#### 节点管理流程
- [ ] 创建节点（手动输入）
- [ ] 创建节点（分享链接）
- [ ] 编辑节点信息
- [ ] 删除节点
- [ ] 重置流量

#### 配置管理流程
- [ ] 导入订阅链接
- [ ] 刷新配置
- [ ] 拉取节点
- [ ] 删除配置（级联删除节点）

#### 代理功能流程
- [ ] 选择节点
- [ ] 启动Xray
- [ ] 验证代理工作
- [ ] 停止Xray

#### 测速功能流程
- [ ] 单节点ping测试
- [ ] 单节点speedtest
- [ ] 批量ping（验证并发）
- [ ] 重置速度数据

#### 主题切换流程
- [ ] 切换到深色主题
- [ ] 切换到浅色主题
- [ ] 验证localStorage持久化

**执行命令**：
```bash
npm run test:e2e
```

---

## 4. 构建和打包测试

### 4.1 Go后端编译

```bash
# Linux
make build-backend GOOS=linux GOARCH=amd64

# macOS
make build-backend GOOS=darwin GOARCH=arm64

# Windows
make build-backend GOOS=windows GOARCH=amd64
```

**验证**：
- [ ] 编译成功无错误
- [ ] 可执行文件大小合理（~9-10MB）
- [ ] 启动后健康检查通过

### 4.2 SDK构建

```bash
cd frontend/sdk && npm run build
```

**验证**：
- [ ] 构建产物存在：`frontend/sdk/dist/vea-sdk.esm.js`
- [ ] 文件大小合理（~19KB）
- [ ] ESM格式正确

### 4.3 Electron应用打包

```bash
cd frontend && npm run build
```

**验证**：
- [ ] 打包成功（Linux/macOS/Windows）
- [ ] 安装包大小合理
- [ ] 安装后应用可正常启动

---

## 5. 兼容性测试

### 5.1 平台兼容性
- [ ] **Linux**（Ubuntu 20.04+）
- [ ] **macOS**（10.15+）
- [ ] **Windows**（10/11）

### 5.2 架构兼容性
- [ ] **amd64**（x86_64）
- [ ] **arm64**（Apple Silicon, ARM服务器）

---

## 6. 性能测试

### 6.1 并发测速性能

测试场景：10个节点并发测速

**预期结果**：
- 串行模式：~20分钟
- 并发模式（5并发）：~1分钟
- 性能提升：95%

**验证命令**：
```bash
# 通过API批量触发ping
curl -X POST http://localhost:8080/nodes/bulk/ping \
  -H "Content-Type: application/json" \
  -d '{"ids": []}'  # 空数组表示所有节点
```

### 6.2 内存和CPU占用

- [ ] 空闲状态：<100MB内存，<5% CPU
- [ ] 测速状态：<500MB内存，<50% CPU
- [ ] Xray运行：<200MB额外内存

---

## 7. 回归测试

确保重构未破坏现有功能：
- [ ] 所有后端单元测试通过
- [ ] 所有后端集成测试通过
- [ ] API接口保持向后兼容
- [ ] state.json数据格式未变
- [ ] Xray配置生成正确

**执行命令**：
```bash
go test ./... -v
```

---

## 测试优先级

### P0（必须）
- ✅ 后端单元测试
- ✅ 后端E2E集成测试
- ✅ Go后端编译（3平台）
- ✅ SDK构建

### P1（重要）
- [ ] API路由集成测试
- [ ] Electron应用打包
- [ ] 基本功能E2E测试（启动、节点管理、代理）

### P2（建议）
- [ ] SDK单元测试
- [ ] Electron主进程测试
- [ ] 完整E2E测试套件
- [ ] 性能测试

---

## 测试环境要求

### 开发环境
- Go 1.22+
- Node.js 16+
- npm 8+
- Git

### CI环境（GitHub Actions）
- Ubuntu 22.04（Linux测试）
- macOS 13（macOS测试）
- Windows 2022（Windows测试）
- xray二进制文件（自动下载）

---

## 快速测试指令

### 后端测试
```bash
# 所有后端测试
go test ./... -v

# 仅单元测试
go test ./backend/store ./backend/service -v -short

# 集成测试（需要xray）
go test ./backend/service -v -run TestE2E
```

### 前端测试
```bash
# SDK构建
cd frontend/sdk && npm run build

# 前端测试（待实现）
cd frontend && npm test
```

### 构建验证
```bash
# 后端编译
make build-backend

# Electron打包
cd frontend && npm run build
```

### 快速验证
```bash
# 编译并启动开发模式
make dev
```
