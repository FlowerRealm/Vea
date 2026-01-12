# Vea - 代理管理器（sing-box / Clash）

Vea 是一个基于 Electron 的桌面应用，用于管理 sing-box/Clash 代理的 FRouter（路由/链路）、ProxyConfig（运行配置，单例）、订阅配置、Geo 资源与核心组件。

**技术栈**:
- **前端**: Electron + HTML/CSS/JavaScript
- **后端**: Go + Gin (内置 HTTP 服务)
- **通信**: REST API (localhost:19080)

## 功能亮点

- FRouter 中心：管理 FRouter（链式代理图/路由定义）；节点为独立资源，通过订阅/导入生成；支持测速、延迟测试与链路配置。
- 配置管理：导入订阅链接，跟踪自动刷新周期与到期时间。
- Geo 与核心组件：后台定时刷新 GeoIP/GeoSite，按需下载/安装 sing-box/mihomo(Clash) 核心并记录校验值。
- ProxyConfig：选择入站模式（SOCKS/HTTP/Mixed/TUN）、绑定 FRouter、引擎偏好与 TUN 配置，并启动/停止代理。
- 前端控制台：`/` 提供极简 UI，可快速操作 FRouter、ProxyConfig、配置、Geo 与组件。

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
4. Electron 自动启动 Go 服务进程（监听 `localhost:19080`）

> 💡 **前后端一体化启动**：Electron 的主进程会自动 spawn Go 后端进程，无需手动启动两个服务。

> 🎨 **主题支持**：应用内置深色和浅色两套现代化主题，点击侧边栏底部的按钮即可切换，主题选择会自动保存。

### 打包应用

```bash
# 打包当前平台
make build

# 打包后的文件在
ls release/
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
│   ├── repository/  # 仓储接口与内存实现
│   ├── persist/     # 持久化与迁移
│   └── tasks/       # 后台任务（组件/Geo/订阅同步）
├── frontend/sdk/     # JavaScript SDK
├── docs/             # 所有文档
│   ├── api/         # API 文档（OpenAPI 规范）
│   └── *.md
└── （运行时数据）      # 统一位于 userData（不写入仓库/安装目录）
    ├── data/        # 状态与本地数据
    │   └── state.json
    └── artifacts/   # 组件/Geo/rule-set/运行期日志等
```

## 开发文档

- [Electron 客户端说明](./frontend/README.md)
- [SDK 文档](./frontend/sdk/README.md)
- [API 文档（OpenAPI）](./docs/api/openapi.yaml)
- [架构说明](./docs/ARCHITECTURE_V2.md)

## 常见问题

**Q: 启动失败显示 sandbox 错误？**
A: 项目已配置 `--no-sandbox` 标志，正常情况不会出现。如遇到问题请查看 [frontend/README.md](./frontend/README.md)。

**Q: 启动时报 `permission denied`（例如 userData 下的 `data/state.json` / `artifacts/`）？**
A: 常见原因是以前用 `sudo`/管理员模式跑过，导致用户目录下的数据变成 root-owned。当前版本运行期数据统一写入 userData；如你本地仍残留旧仓库目录 `./data` / `./artifacts`，启动时会自动迁移并清理（移动并删除源目录）。

**Q: FRouter 测速失败提示 `no installed engine supports protocol shadowsocks`？**
A: 这通常表示测速模块没有识别到已安装的内核组件。请在「组件」面板确认已安装并完成安装 `sing-box` 或 `Clash`，然后重启 Vea 再试；如果 FRouter 内节点 `port=0` 或缺失端口，也会导致测速/探测失败，建议重新导入订阅或手动修正端口。

**Q: 日志里出现 `dial tcp <host>:0` / LatencyProbe 目标端口是 0？**
A: 端口 0 是无效节点数据（常见于订阅数据异常）。请在 FRouter 编辑里补全正确端口，或重新导入该 FRouter/订阅后再进行延迟/测速。

**Q: 端口 19080 被占用？**
A: Go 服务固定使用 19080 端口，请确保该端口未被占用；如确认是旧的 Vea 开发进程占用，可运行 `make dev KILL_OLD=1` 强制清理后再启动。

**Q: 如何调试？**
A: 运行 `make dev` 后，在 Electron 窗口中按 F12 打开开发者工具。Go 后端日志会输出到终端。

## 许可证

项目采用 [MIT](LICENSE) 许可证，欢迎在该协议下使用与贡献。
