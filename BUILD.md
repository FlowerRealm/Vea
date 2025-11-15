# Vea 构建指南

本文档说明如何在本地编译和打包 Vea。

## 快速开始

### 前置要求

- Go 1.21 或更高版本
- Make 工具
- （可选）Node.js 和 npm（仅在修改 SDK 时需要）

### 常用命令

```bash
# 查看所有可用命令
make help

# 编译可执行文件（推荐用于日常开发）
make build

# 清理构建产物
make clean

# 编译并运行
make run

# 开发模式（使用 go run，无需编译，显示详细日志）
make dev

# 生产模式运行（只显示错误日志）
make run
# 或
./dist/vea

# 开发模式运行（显示所有日志）
./dist/vea --dev
```

## 日志级别控制

Vea 支持两种日志模式：

### 开发模式（详细日志）
```bash
# 使用 make dev 自动启用开发模式
make dev

# 或手动指定 --dev 参数
./dist/vea --dev
go run ./cmd/server --dev
```

**开发模式特点**：
- 显示所有日志（包括信息、警告、错误）
- 显示文件名和行号
- Gin 框架运行在 Debug 模式
- 适合开发和调试

**输出示例**：
```
2025/11/15 13:49:42 main.go:38: 运行在开发模式 - 显示所有日志
2025/11/15 13:49:42 main.go:55: state loaded from data/state.json
2025/11/15 13:49:42 main.go:92: server listening on :8080
```

### 生产模式（仅错误日志）
```bash
# 默认模式
./dist/vea
make run
```

**生产模式特点**：
- 只显示错误日志（包含 error、failed、fatal、panic 等关键字）
- 不显示常规操作日志
- Gin 框架运行在 Release 模式
- 适合生产环境

**输出示例**（只在有错误时才输出）：
```
2025/11/15 13:50:29 snapshot save failed: mkdir /nonexistent: permission denied
```

## 构建详解

### 1. 快速构建（开发模式）

这是最常用的构建方式，直接生成单个可执行文件：

```bash
make build
```

**输出**：
- 文件位置：`dist/vea`
- 文件大小：约 9.6 MB
- 编译优化：已优化（去除调试符号）
- 静态编译：是（CGO_ENABLED=0）

**构建过程**：
1. 准备构建环境（创建 dist 目录）
2. 复制 web 资源到 cmd/server/web/
3. 复制 SDK 文件到 cmd/server/web/sdk/dist/
4. 编译生成可执行文件

### 2. 发布版本构建

与 GitHub Actions CI/CD 完全相同的打包方式：

```bash
make build-release VERSION=v1.0.0
```

**输出**：
- 文件：`dist/vea-v1.0.0-linux-amd64.tar.gz`
- 包含：可执行文件 + LICENSE
- 压缩格式：tar.gz

### 3. 开发模式

无需编译，直接运行：

```bash
make dev
```

这会使用 `go run` 启动服务器，适合快速迭代开发。

### 4. 运行已编译的程序

编译后直接运行：

```bash
make run
```

或手动运行：

```bash
./dist/vea --addr :8080 --state data/state.json
```

## 多平台编译

### 编译特定平台

```bash
# Linux AMD64（默认）
make build-linux

# Linux ARM64
make build-linux-arm64

# macOS AMD64
make build-macos

# macOS ARM64 (Apple Silicon)
make build-macos-arm64

# Windows AMD64
make build-windows
```

### 编译所有平台

```bash
make build-all
```

这会生成以下可执行文件：
- `dist/vea` (Linux AMD64)
- `dist/vea-arm64` (Linux ARM64)
- `dist/vea-darwin` (macOS AMD64)
- `dist/vea-darwin-arm64` (macOS ARM64)
- `dist/vea.exe` (Windows AMD64)

## SDK 构建

如果你修改了 SDK 源码，需要重新构建：

```bash
make build-sdk
```

**前提条件**：
```bash
cd sdk
npm install
```

SDK 构建输出：
- `sdk/dist/vea-sdk.esm.js` - ES Module 格式
- `sdk/dist/vea-sdk.cjs.js` - CommonJS 格式
- `sdk/dist/vea-sdk.umd.js` - UMD 格式
- `sdk/dist/vea-sdk.umd.min.js` - 压缩版本（7.8 KB）

## 测试

```bash
# 运行所有测试
make test

# 查看测试覆盖率
make test-coverage
```

## 安装到系统

编译后安装到 `/usr/local/bin/`：

```bash
make install
```

安装后可以直接使用：

```bash
vea --addr :8080
```

## Docker 构建

```bash
# 构建 Docker 镜像
make docker-build

# 构建并运行
make docker-run
```

## 清理

清理所有构建产物：

```bash
make clean
```

这会删除：
- `dist/` 目录
- `cmd/server/web/` 目录（临时复制的 web 资源）

## 高级选项

### 自定义构建参数

```bash
# 指定版本号
make build VERSION=v1.2.3

# 指定输出目录
make build OUTPUT_DIR=build

# 指定目标架构
make build GOARCH=arm64 GOOS=linux
```

### 直接使用 Go 命令

如果不想使用 Make，也可以直接使用 Go 命令：

```bash
# 准备环境
mkdir -p cmd/server/web/sdk/dist
cp web/index.html cmd/server/web/
cp -r sdk/dist/* cmd/server/web/sdk/dist/

# 编译
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o dist/vea ./cmd/server
```

## 构建优化说明

### 编译参数

- `-trimpath`：移除文件系统路径，减小二进制体积
- `-ldflags "-s -w"`：
  - `-s`：去除符号表
  - `-w`：去除 DWARF 调试信息
- `CGO_ENABLED=0`：禁用 CGO，生成完全静态的二进制文件

### 为什么使用静态编译？

- **可移植性**：无需依赖系统库，可在任何 Linux 系统运行
- **容器友好**：适合 Docker 等容器环境
- **部署简单**：单个文件即可部署

## 常见问题

### Q: 为什么可执行文件这么大？

A: Vea 使用 Go embed 将所有 web 资源（HTML、SDK）打包进可执行文件，因此体积较大。但这带来了部署便利性 - 只需一个文件。

### Q: 如何减小文件体积？

A: 已经使用了 `-ldflags "-s -w"` 去除调试信息。如果需要进一步压缩，可以使用 UPX：

```bash
upx --best dist/vea
```

### Q: 修改了 web/index.html，为什么运行时没变化？

A: 需要重新编译，因为 web 资源是 embed 进可执行文件的：

```bash
make clean && make build
```

### Q: 开发时每次都要编译很麻烦？

A: 使用开发模式：

```bash
make dev
```

这会直接运行源码，无需编译。

## 与 CI/CD 的关系

- `make build`：快速构建，适合日常开发
- `make build-release`：完全等同于 GitHub Actions 的打包流程
- `.github/workflows/release.yml`：使用相同的 `scripts/package-*.sh` 脚本

## 项目结构

```
Vea/
├── Makefile                    # 构建配置
├── cmd/server/                 # 服务器入口
│   └── main.go
├── web/                        # Web 前端资源（源文件）
│   └── index.html
├── sdk/                        # SDK 源码
│   ├── src/
│   └── dist/                   # SDK 构建产物
├── scripts/                    # 打包脚本
│   ├── package-linux.sh
│   ├── package-macos.sh
│   └── package-windows.ps1
└── dist/                       # 构建输出（.gitignore）
    └── vea
```

## 总结

最常用的命令：

```bash
# 开发时
make dev          # 快速启动（无需编译）
make build        # 编译可执行文件
make run          # 编译并运行

# 发布时
make clean
make build-release VERSION=v1.0.0
```
