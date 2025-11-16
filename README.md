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

### 开发模式

```bash
# 克隆项目
git clone <your-repo>
cd Vea

# 启动 Electron 应用（自动编译 Go 后端）
make dev
```

应用将自动：
1. 编译 Go 后端服务
2. 安装 Electron 依赖
3. 启动 Electron 窗口

### 打包应用

```bash
# 打包当前平台
make build

# 打包后的文件在
ls electron/dist/release/
```

## 项目结构

```
Vea/
├── electron/          # Electron 桌面应用
│   ├── main.js       # 主进程（启动 Go 服务）
│   ├── renderer/     # 渲染进程（UI）
│   └── package.json
├── cmd/              # Go 程序入口
├── internal/         # Go 业务逻辑
├── sdk/              # JavaScript SDK
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
A: 运行 `make electron-dev` 后，在 Electron 窗口中按 F12 打开开发者工具。

## 许可证

项目采用 [MIT](LICENSE) 许可证，欢迎在该协议下使用与贡献。
