# Vea - Xray 代理管理器

Vea 是一个基于 Electron 的桌面应用，用于管理 Xray 代理节点、配置、Geo 资源和流量策略。

**技术栈**:
- **前端**: Electron + HTML/CSS/JavaScript
- **后端**: Go + Gin (内置 HTTP 服务)
- **通信**: REST API (localhost:8080)

## 功能亮点

- 节点中心：新增/编辑/删除节点，粘贴 vmess/vless/trojan/ss 分享链接即可导入，支持测速、延迟测试、流量清零。
- 配置管理：导入 Xray JSON 或订阅链接，跟踪自动刷新周期、流量统计与到期时间。
- Geo 与核心组件：定时刷新 GeoIP/GeoSite，按需下载/安装 Xray 核心并记录校验值。
- 流量策略：配置默认出口、DNS、分流规则；选择当前使用节点，支持手动切换 Xray。
- 前端控制台：`/` 提供极简 UI，可快速操作节点、配置、Geo、分流策略。

## 环境要求

- **Node.js** 18+
- **Go** 1.22+
- **操作系统**: Linux (X11/Wayland) / macOS / Windows

## 快速开始

### 一键启动（推荐）

```bash
# 克隆项目
git clone <your-repo>
cd Vea

# 一键启动前后端
make dev
```

**这一个命令会自动完成：**
1. 编译 Go 后端服务 (`dist/vea`)
2. 安装 Electron 依赖（如果需要）
3. 启动 Electron 桌面应用
4. Electron 自动启动 Go 服务进程（监听 `localhost:8080`）

> 💡 **前后端一体化启动**：Electron 的主进程会自动 spawn Go 后端进程，无需手动启动两个服务。

> 🎨 **主题支持**：应用内置深色和浅色两套现代化主题，点击侧边栏底部的按钮即可切换，主题选择会自动保存。

### 打包应用

```bash
# 打包当前平台
make build

# 打包后的文件在
ls electron/dist/release/
```

### 可用命令

| 命令 | 说明 |
|------|------|
| `make dev` | **一键启动**前后端（编译 Go + 启动 Electron） |
| `make build` | 打包生产版本的 Electron 应用 |
| `make build-backend` | 仅编译 Go 后端到 `dist/vea` |
| `make deps` | 仅安装 Electron 依赖 |
| `make clean` | 清理所有构建产物 |
| `make help` | 显示帮助信息 |

## 项目结构

```
Vea/
├── frontend/          # Electron 桌面应用
│   ├── main.js       # 主进程（启动 Go 服务）
│   ├── theme/        # 主题文件（UI）
│   └── package.json
├── main.go           # Go 程序入口
├── backend/          # Go 业务逻辑
│   ├── api/         # HTTP 路由和处理器
│   ├── domain/      # 领域模型
│   ├── service/     # 业务服务层
│   └── store/       # 数据存储
├── frontend/sdk/     # JavaScript SDK
├── docs/             # 所有文档
│   ├── api/         # API 文档（OpenAPI 规范）
│   └── *.md
├── data/             # 运行时数据
│   └── state.json   # 状态持久化
└── artifacts/        # Xray 核心、Geo 资源
```

## 开发文档

- [Electron 客户端说明](./electron/README.md)
- [SDK 文档](./sdk/README.md)
- [API 文档](./docs/api/README.md)
- [构建系统](./docs/SDK_AND_BUILD_SYSTEM.md)

## 常见问题

**Q: 启动失败显示 sandbox 错误？**
A: 项目已配置 `--no-sandbox` 标志，正常情况不会出现。如遇到问题请查看 [electron/README.md](./electron/README.md)。

**Q: 端口 8080 被占用？**
A: Go 服务默认使用 8080 端口，请确保该端口未被占用。

**Q: 如何调试？**
A: 运行 `make dev` 后，在 Electron 窗口中按 F12 打开开发者工具。Go 后端日志会输出到终端。

## 许可证

项目采用 [MIT](LICENSE) 许可证，欢迎在该协议下使用与贡献。
