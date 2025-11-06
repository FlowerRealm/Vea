# Vea Backend

Vea 是一个用 Go 编写的 Xray 管理服务，提供节点、配置、Geo 资源、流量策略的统一运维入口，同时内置简单的 Web 控制台，方便在浏览器中完成日常维护。

## 功能亮点

- 节点中心：新增/编辑/删除节点，粘贴 vmess/vless/trojan/ss 分享链接即可导入，支持测速、延迟测试、流量清零。
- 配置管理：导入 Xray JSON 或订阅链接，跟踪自动刷新周期、流量统计与到期时间。
- Geo 与核心组件：定时刷新 GeoIP/GeoSite，按需下载/安装 Xray 核心并记录校验值。
- 流量策略：配置默认出口、DNS、分流规则；选择当前使用节点，支持手动切换 Xray。
- 前端控制台：`/` 提供极简 UI，可快速操作节点、配置、Geo、分流策略。

## 环境要求

- Go 1.22+
- Git（克隆项目）
- 可联网环境用于拉取依赖与 Geo 资源（如需）

## 快速上手

1. 克隆仓库并进入目录：
   ```bash
   git clone https://github.com/<your-org>/Vea.git
   cd Vea
   ```
2. 安装依赖并确认可以构建：
   ```bash
   go mod tidy
   go build ./...
   ```
3. 启动服务（默认监听 `:8080`）：
   ```bash
   go run ./cmd/server --addr :8080 --state ./data/state.json
   ```
   常用参数：
   - `--addr`：HTTP 服务监听地址，例如 `--addr 0.0.0.0:9000`
   - `--state`：快照文件路径，默认 `data/state.json`
4. 验证运行状态：
   - `GET http://127.0.0.1:8080/health`：健康检查
   - 浏览器打开 `http://127.0.0.1:8080/`：访问内置控制台

## 目录与数据

- `data/state.json`：内存状态快照，服务每次写操作后自动刷盘；下次启动会恢复。
- `artifacts/geo/`：存放 Geo 资源二进制文件，例如 `artifacts/geo/<id>.bin`。
- `dist/`：存放手动或 CI 打包生成的发布归档。
- `scripts/`：平台化打包脚本（`package-linux.sh`、`package-macos.sh`、`package-windows.ps1`）。

## 发布与打包

### 本地打包

根据目标平台执行对应脚本，示例（Linux amd64）：
```bash
./scripts/package-linux.sh v1.0.0 amd64 dist
```
macOS 与 Windows：
```bash
./scripts/package-macos.sh v1.0.0 arm64 dist
powershell ./scripts/package-windows.ps1 -Version v1.0.0 -GoOS windows -GoArch amd64
```
生成的归档名为 `vea-<version>-<goos>-<goarch>.<tar.gz|zip>`，内部包含可执行文件、`web/` 前端与 `LICENSE`。

若需要对 macOS 可执行文件进行签名，在运行脚本前设置：
```bash
export MACOS_CODESIGN_IDENTITY="Developer ID Application: ..."
export MACOS_CODESIGN_ENTITLEMENTS="entitlements.plist" # 可选
```

### GitHub Actions

仓库提供 `.github/workflows/release.yml`，通过 `workflow_dispatch` 触发手动发布：
1. 在 GitHub Actions 面板选择 **Build and Release**。
2. 输入版本号（例如 `v1.2.3`）和 Release Notes，点击运行。
3. CI 分别在 Linux/macOS/Windows 构建产物，生成 `vea-<version>-SHA256SUMS` 校验文件，并把所有归档上传到 GitHub Release。

## 进一步扩展

- 将内存存储实现替换为数据库，满足多实例部署需求。
- 接入真实的节点探测脚本，替换当前模拟的延迟/速度数据。
- 自定义 Geo 资源下载与校验策略，以适配内网或镜像源。

## 许可证

项目采用 [MIT](LICENSE) 许可证，欢迎在该协议下使用与贡献。
