# Vea Electron 客户端

Vea 代理管理器的 Electron 桌面应用封装。

## 快速开始

### 开发模式

```bash
# 从项目根目录
make electron-dev

# 或者手动执行
make build              # 编译 Go 后端
cp dist/vea vea        # 复制二进制到根目录
cd electron
npm install            # 安装依赖
npm run dev            # 启动 Electron
```

### 打包应用

```bash
# 从项目根目录
make electron-build           # 打包当前平台

# 或者指定平台
make electron-build-linux     # 打包 Linux
make electron-build-mac       # 打包 macOS
make electron-build-win       # 打包 Windows
```

打包后的应用位于 `electron/dist/release/` 目录。

## 项目结构

```
electron/
├── main.js                 # Electron 主进程
├── renderer/               # 渲染进程（UI）
│   └── index.html         # 从 web/index.html 迁移
├── package.json           # NPM 配置
├── vite.config.js         # Vite 构建配置
├── electron-builder.yml   # 打包配置
└── dist/                  # 构建输出
    └── release/           # 打包后的应用
```

## 技术架构

- **主进程 (main.js)**：
  - 启动 Go 后端服务（spawn `vea` 二进制）
  - 创建应用窗口
  - 管理进程生命周期

- **渲染进程 (renderer/)**：
  - 复用现有 Web UI（HTML/CSS/JS）
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
   - Renderer → SDK → HTTP → Go Backend
   - 无需复杂的 IPC，完全通过 REST API

3. **退出流程**：
   - 关闭窗口
   - 主进程发送 SIGTERM 给 Go 进程
   - 2 秒后若未退出则 SIGKILL 强制终止
   - Electron 退出

## 开发说明

### 修改 UI

编辑 `electron/renderer/index.html`，重启 Electron 即可看到效果。

### 修改主进程逻辑

编辑 `electron/main.js`，需要重启 Electron。

### 修改后端

在项目根目录执行 `make build`，然后重启 Electron。

## 已知问题

1. **首次启动可能较慢**：Go 服务需要初始化，窗口会在服务就绪后才显示。
2. **端口占用**：如果 8080 端口已被占用，需要手动停止其他服务。
3. **打包体积**：Electron 本身约 ~150MB，Go 二进制约 10MB。

## 依赖版本

- Electron: ^28.0.0
- electron-builder: ^24.9.1
- Vite: ^5.0.0

## License

MIT
