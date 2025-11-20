# Vea Electron 客户端

Vea 代理管理器的 Electron 桌面应用封装。

> **⚠️ 环境要求**
> Electron 应用需要在**有图形界面的环境**中运行（Linux 需要 X11/Wayland，macOS/Windows 原生支持）。
> 在无头服务器（SSH 命令行）中无法启动 GUI，但可以进行代码验证和打包。

## 快速开始

### 开发模式

```bash
# 从项目根目录
make dev

# 或者手动执行
make build-backend     # 编译 Go 后端
cp dist/vea vea       # 复制二进制到根目录
cd electron
npm install           # 安装依赖
npm run dev           # 启动 Electron
```

### 打包应用

```bash
# 从项目根目录
make build            # 打包当前平台
```

打包后的应用位于 `electron/dist/release/` 目录。

## 项目结构

```
frontend/
├── main.js                 # Electron 主进程
├── theme/                  # 主题文件（UI）
│   ├── dark.html          # 深色主题
│   └── light.html         # 浅色主题
├── package.json           # NPM 配置
├── electron-builder.yml   # 打包配置
└── dist/                  # 构建输出
    └── release/           # 打包后的应用
```

## 技术架构

- **主进程 (main.js)**：
  - 启动 Go 后端服务（spawn `vea` 二进制）
  - 创建应用窗口
  - 管理进程生命周期

- **主题文件 (theme/)**：
  - 自包含的HTML主题文件（HTML/CSS/JS）
  - 通过 ES Module 导入 SDK
  - 直接调用 HTTP API (localhost:8080)

- **后端服务**：
  - Go 编译的 Vea 服务
  - 监听在 :8080
  - 打包时内嵌到 app 的 resources 目录

## 工作原理

1. **启动流程**：
   - Electron 主进程启动
   - Spawn Go 后端服务进程
   - 等待服务健康检查通过（最多 10 秒）
   - 创建窗口并加载 UI

2. **通信模式**：
   - Theme UI → SDK → HTTP → Go Backend
   - 无需复杂的 IPC，完全通过 REST API

3. **退出流程**：
   - 关闭窗口
   - 主进程发送 SIGTERM 给 Go 进程
   - 2 秒后若未退出则 SIGKILL 强制终止
   - Electron 退出

## 无头环境测试

如果你在**无 GUI 的服务器**（SSH 命令行）中，无法直接运行 Electron，但可以验证核心逻辑：

```bash
# 1. 测试主进程逻辑（无需 Electron GUI）
cd /home/flowerrealm/Vea
node electron/test-main-logic.js

# 预期输出：
# ✓ Vea 服务进程已启动
# ✓ 服务就绪（尝试 2 次）
# ✓ HTTP 访问正常
# ✓ 服务已停止
# 测试结果: 通过 4 / 失败 0

# 2. 验证 Go 后端
make build
./vea --addr :8080 &
curl http://localhost:8080/
pkill vea

# 3. 验证 SDK 路径
ls -lh frontend/sdk/dist/vea-sdk.esm.js
```

**在本地开发环境**（有图形界面）才能真正运行 Electron GUI。

## 开发说明

### 修改 UI

编辑 `frontend/theme/dark.html` 或 `light.html`，重启 Electron 即可看到效果。

### 修改主进程逻辑

编辑 `frontend/main.js`，需要重启 Electron。

### 修改后端

在项目根目录执行 `make build`，然后重启 Electron。

## 已知问题

1. **无头环境限制**：
   - 在 SSH 命令行服务器中无法运行 Electron GUI
   - `npm install electron` 可能失败或警告
   - 需要在本地开发环境（有图形界面）中运行

2. **首次启动可能较慢**：Go 服务需要初始化，窗口会在服务就绪后才显示。

3. **端口占用**：如果 8080 端口已被占用，需要手动停止其他服务。

4. **打包体积**：Electron 本身约 ~150MB，Go 二进制约 10MB。

## 依赖版本

- Electron: ^28.0.0
- electron-builder: ^24.9.1
- Vite: ^5.0.0

## License

MIT
