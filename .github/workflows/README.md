# GitHub Actions Workflows

## test.yml - 自动化测试

**触发条件**：
- Push到main分支
- 任何Pull Request

**测试内容**：
- Go后端单元测试
- Go后端集成测试（需要xray）
- 跨平台测试（Linux/macOS/Windows）

**执行命令**：
```bash
go test -v ./...
```

## release.yml - Electron应用发布

**触发方式**：手动触发（workflow_dispatch）

**输入参数**：
- `version`: 版本号（如v1.0.0）
- `release_notes`: 发布说明

**发布流程**：

### 1. Build Backend（构建Go后端）
**矩阵构建**（5个job并行）：
- Linux x64
- Linux arm64
- macOS x64 (Intel)
- macOS arm64 (Apple Silicon)
- Windows x64

**输出**：Go后端二进制文件（vea或vea.exe）

### 2. Build SDK（构建JavaScript SDK）
**步骤**：
- 安装npm依赖
- 运行`npm run build`
- 构建`vea-sdk.esm.js`

**输出**：SDK构建产物

### 3. Package（打包Electron应用）
**3个job并行**：

#### package-linux
- 下载Linux后端（x64和arm64）
- 下载SDK
- 安装frontend依赖
- 使用electron-builder打包
- **输出**：
  - `*.AppImage` (x64, arm64) - 通用格式
  - `*.deb` (x64, arm64) - Debian/Ubuntu包

#### package-macos
- 下载macOS后端（x64和arm64）
- 下载SDK
- 安装frontend依赖
- 使用electron-builder打包
- **输出**：
  - `*.dmg` (x64, arm64) - macOS磁盘镜像

#### package-windows
- 下载Windows后端（x64）
- 下载SDK
- 安装frontend依赖
- 使用electron-builder打包
- **输出**：
  - `*.exe` (x64) - NSIS安装程序

### 4. Release（创建GitHub Release）
**步骤**：
- 下载所有打包产物
- 生成SHA256校验���
- 创建GitHub Release
- 上传所有安装包

**最终产物**：
- Linux: AppImage + deb (x64/arm64)
- macOS: dmg (Intel/Apple Silicon)
- Windows: exe安装程序 (x64)
- SHA256SUMS.txt

## 使用方法

### 运行测试
测试会自动在PR和main分支上运行，无需手动触发。

### 创建Release

1. 进入GitHub仓库
2. 点击 **Actions** 标签页
3. 选择 **Build and Release Electron App** workflow
4. 点击 **Run workflow**
5. 填写：
   - Version: `v1.0.0`
   - Release notes: 更新说明
6. 点击 **Run workflow**

### 等待构建完成
整个流程大约需要 **20-30分钟**：
- Build Backend: ~5分钟（5个job并行）
- Build SDK: ~2分钟
- Package: ~15分钟（3个job并行）
- Release: ~2分钟

### 发布后
Release会自动创建在：
```
https://github.com/FlowerRealm/Vea/releases/tag/v1.0.0
```

包含：
- 所有平台的安装包
- SHA256校验和文件
- 发布说明

## 技术细节

### 后端构建参数
```bash
CGO_ENABLED=0
GOOS={linux|darwin|windows}
GOARCH={amd64|arm64}
go build -trimpath -ldflags "-s -w" -o {binary} .
```

### Electron打包架构
electron-builder使用`extraResources`将Go后端打包进应用：
```yaml
extraResources:
  - from: ../vea
    to: vea
```

每个架构单独构建，确保后端二进制匹配：
```bash
# x64构建
cp vea-x64 ../vea
npm run build:linux -- --x64

# arm64构建
cp vea-arm64 ../vea
npm run build:linux -- --arm64
```

### Artifacts保留期
所有中间artifacts保留1天，足够完成发布流程。

## 故障排查

### 后端构建失败
检查：
- Go版本是否兼容
- 依赖是否正确
- 交叉编译配置

### SDK构建失败
检查：
- Node.js版本（需要18+）
- npm依赖是否安装
- rollup配置

### Electron打包失败
检查：
- electron-builder配置
- 后端二进制是否存在
- SDK是否构建成功
- node_modules是否完整

### Release创建失败
检查：
- GITHUB_TOKEN权限
- artifacts是否全部下载成功
- 文件路径是否正确

## 本地测试

### 测试后端构建
```bash
make build-backend GOOS=linux GOARCH=amd64
```

### 测试SDK构建
```bash
cd frontend/sdk && npm run build
```

### 测试Electron打包
```bash
# 先构建后端
make build-backend

# 打包Electron
cd frontend && npm run build
```

### 完整流程
```bash
# 1. 构建后端
make build-backend

# 2. 构建SDK
cd frontend/sdk && npm run build && cd ../..

# 3. 打包应用
cd frontend && npm run build
```

## 维护说明

### 更新版本号
版本号通过workflow input手动指定，不依赖package.json。

### 添加新平台
修改`build-backend` job的matrix，添加新的os/arch组合。

### 修改打包配置
编辑`frontend/electron-builder.yml`。

### 调整构建参数
修改各job中的构建命令和环境变量。
