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
cd frontend
npm install           # 安装依赖
npm run dev           # 启动 Electron
```

### 打包应用

```bash
# 从项目根目录
make build            # 打包当前平台
```

打包后的安装包位于项目根目录的 `release/`。

自动更新（Windows/macOS）使用 GitHub Pages 托管更新元数据（CI 会部署 `latest*.yml` 与更新所需安装包到 Pages；本地构建时这些文件仍会在 `dist/electron/` 生成）。

## 项目结构

```
frontend/
├── main.js                 # Electron 主进程
├── preload.js              # preload 脚本
├── settings-schema.js       # 前端设置 schema + 渲染器
├── theme/                  # 内置主题包（会复制到 userData/themes/）
│   ├── dark/
│   │   ├── index.html
│   │   ├── css/
│   │   ├── js/
│   │   └── fonts/
│   └── light/
│       ├── index.html
│       ├── css/
│       ├── js/
│       └── fonts/
├── assets/                 # 图标/托盘资源
├── package.json           # NPM 配置
├── electron-builder.yml   # 打包配置
└── sdk/                    # JS SDK（dist 会被打包进应用）
```

## 技术架构

- **主进程 (main.js)**：
  - 启动 Go 后端服务（spawn `vea` 二进制）
  - 创建应用窗口
  - 管理进程生命周期

- **主题文件 (theme/)**：
  - 目录化主题包（入口 `index.html`，拆分 `css/`、`js/`、`fonts/`）
  - 运行时从 `userData/themes/<entry>` 加载（入口由后端 `/themes` 返回的 `entry` 决定；单主题等价于 `<themeId>/index.html`）
  - 支持“主题包（manifest）”：`userData/themes/<packId>/manifest.json` 可描述多个子主题，入口由 `entry`（相对 `themes/`）指定
  - 主题内通过 `/themes` 接口动态加载列表，并提供 ZIP 导入/导出

- **后端服务**：
  - Go 编译的 Vea 服务
  - 监听在 :19080
  - 打包时内嵌到 app 的 resources 目录

## 工作原理

1. **启动流程**：
   - Electron 主进程启动
   - Spawn Go 后端服务进程
   - 等待服务健康检查通过（最多 10 秒）
   - 初始化 `userData/themes/`（缺少内置主题时从 app resources 复制）
   - 读取后端前端设置 `theme`（默认 `dark`）
   - 创建窗口并加载 `userData/themes/<entry>`（由 `/themes` 解析）

2. **通信模式**：
   - Theme UI → SDK → HTTP → Go Backend
   - 无需复杂的 IPC，完全通过 REST API

3. **退出流程**：
   - 关闭窗口
   - 主进程发送 SIGTERM 给 Go 进程
   - 2 秒后若未退出则 SIGKILL 强制终止
   - Electron 退出

## 无头环境测试

如果你在**无 GUI 的服务器**（SSH 命令行）中，无法直接运行 Electron，但可以验证后端与 SDK：

```bash
# 1) 后端单测
go test ./...

# 2) SDK 构建
cd frontend/sdk && npm run build

# 3) 后端冒烟（仅后端，不启动 Electron）
go run . --dev --addr :19080
# 另开终端：
curl http://localhost:19080/health
```

**在本地开发环境**（有图形界面）才能真正运行 Electron GUI。

## 开发说明

### 修改 UI

编辑 `frontend/theme/dark/index.html`（或 `css/`、`js/`）以及 `frontend/theme/light/index.html`，重启 Electron 即可看到效果。

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

3. **端口占用**：如果 19080 端口已被占用，需要手动停止其他服务。

4. **打包体积**：Electron 本身约 ~150MB，Go 二进制约 10MB。

## 依赖版本

- Electron: ^28.0.0
- electron-builder: ^24.9.1

## License

MIT
