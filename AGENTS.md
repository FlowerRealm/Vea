# Vea Backend Overview

## 项目概况
Go 实现的 xray 管理服务，提供节点、配置、Geo 资源与流量策略的集中管控；通过 REST API 与内置前端控制台完成维护操作，并以定时任务处理自动同步。

## 功能清单
- 节点管理：增删改查、流量统计、延迟/速度测试、流量重置，支持粘贴 vmess/vless/trojan/ss 分享链接导入。
- 配置管理：导入 xray 格式，追踪自动更新周期、流量和到期时间，订阅可自动解析并生成节点。
- 核心组件：内置 xray/Geo 官方释放地址，支持一键下载、解压、安装与后台定时更新。
- Geo 资源：抓取与刷新 GeoIP/GeoSite，维护版本、校验值与更新时间。
- 流量策略：管理默认节点、DNS 配置、分流规则及优先级。
- 后台任务：处理配置自动同步、Geo 刷新、节点探测等周期任务。

## 技术栈
- Go 1.22
- Gin HTTP 框架
- 并发安全内存存储 + JSON 快照 (`data/state.json`)
- 定时任务基于 `context` 与 `time.Ticker`

## 使用方式
1. 安装 Go 1.22。
2. 运行 `go mod tidy && go build ./...`。
3. 使用 `go run ./cmd/server --addr :8080` 启动，或自定义 `--state` 指向快照文件。
4. 访问 `http://127.0.0.1:8080/health` 检查，随后在浏览器打开根路径进入控制台。

配置导入支持 `sourceUrl` 或订阅链接（HTTP/HTTPS/Base64/vmess/vless/trojan/ss），自动抓取内容并生成节点；Geo 资源下载后存放于 `artifacts/geo/<id>.bin`。所有写操作会触发快照保存，重启后自动恢复。

## API 速览
- `GET /health`：健康检查。
- `GET /snapshot`：获取当前节点、配置、Geo、流量策略快照。
- `/nodes`：节点 CRUD、流量上报、延迟/速度测试、流量清零、批量延迟/测速。
- `/configs`：配置导入、更新、删除、刷新、节点拉取、流量统计。
- `/components`：核心组件 CRUD 与安装。
- `/geo`：Geo 资源 CRUD 与刷新。
- `/traffic/profile`：默认节点、DNS 配置查询与更新。
- `/traffic/rules`：分流规则 CRUD。

## 扩展建议
- 将内存存储替换为数据库实现以满足生产需求。
- 对接真实代理内核或探测脚本，替换当前的模拟延迟/带宽逻辑。
- Geo 刷新流程可结合自定义校验与下载策略。

## 许可
MIT License (`LICENSE`).
