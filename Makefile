.PHONY: all build clean run dev package help

# 默认版本号
VERSION ?= dev
# 默认输出目录
OUTPUT_DIR ?= dist
# 默认架构
GOARCH ?= amd64
GOOS ?= linux

# 编译参数
LDFLAGS := -s -w
BUILD_FLAGS := -trimpath -ldflags "$(LDFLAGS)"

# 可执行文件名
BINARY_NAME := vea
ifeq ($(GOOS),windows)
	BINARY_NAME := vea.exe
endif

help: ## 显示帮助信息
	@echo "Vea 构建工具"
	@echo ""
	@echo "使用方法: make [target]"
	@echo ""
	@echo "可用目标:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

all: build ## 构建项目（默认）

prepare: ## 准备构建环境
	@echo "==> 准备构建环境..."
	@mkdir -p $(OUTPUT_DIR)
	@mkdir -p cmd/server/web/sdk/dist
	@echo "==> 复制 web 资源..."
	@cp web/index.html cmd/server/web/index.html
	@cp -r sdk/dist/* cmd/server/web/sdk/dist/

build: prepare ## 编译可执行文件（快速开发模式）
	@echo "==> 编译 Vea $(VERSION) for $(GOOS)/$(GOARCH)..."
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) ./cmd/server
	@echo "==> 构建完成: $(OUTPUT_DIR)/$(BINARY_NAME)"
	@ls -lh $(OUTPUT_DIR)/$(BINARY_NAME)

build-release: prepare ## 编译发布版本（与 CI 相同）
	@echo "==> 编译 Release 版本 $(VERSION)..."
	@./scripts/package-$(GOOS).sh $(VERSION) $(GOARCH) $(OUTPUT_DIR)
	@echo "==> 发布包已生成"
	@ls -lh $(OUTPUT_DIR)/*.tar.gz 2>/dev/null || ls -lh $(OUTPUT_DIR)/*.zip 2>/dev/null || true

run: build ## 编译并运行
	@echo "==> 运行 Vea..."
	@$(OUTPUT_DIR)/$(BINARY_NAME)

dev: ## 开发模式（使用 go run）
	@echo "==> 准备开发环境..."
	@mkdir -p cmd/server/web/sdk/dist
	@cp web/index.html cmd/server/web/index.html
	@cp -r sdk/dist/* cmd/server/web/sdk/dist/
	@echo "==> 启动开发服务器（详细日志模式）..."
	@go run ./cmd/server --dev

clean: ## 清理构建产物
	@echo "==> 清理构建产物..."
	@rm -rf $(OUTPUT_DIR)
	@rm -rf cmd/server/web
	@echo "==> 清理完成"

install: build ## 安装到系统
	@echo "==> 安装 $(BINARY_NAME) 到 /usr/local/bin/..."
	@sudo cp $(OUTPUT_DIR)/$(BINARY_NAME) /usr/local/bin/
	@echo "==> 安装完成"

# 多平台构建
build-linux: ## 编译 Linux 版本
	@$(MAKE) build GOOS=linux GOARCH=amd64

build-linux-arm64: ## 编译 Linux ARM64 版本
	@$(MAKE) build GOOS=linux GOARCH=arm64

build-macos: ## 编译 macOS 版本
	@$(MAKE) build GOOS=darwin GOARCH=amd64

build-macos-arm64: ## 编译 macOS ARM64 版本
	@$(MAKE) build GOOS=darwin GOARCH=arm64

build-windows: ## 编译 Windows 版本
	@$(MAKE) build GOOS=windows GOARCH=amd64

build-all: build-linux build-linux-arm64 build-macos build-macos-arm64 build-windows ## 编译所有平台

# SDK 相关
build-sdk: ## 构建 SDK
	@echo "==> 构建 SDK..."
	@cd sdk && npm install && npm run build
	@echo "==> SDK 构建完成"

# 测试相关
test: ## 运行测试
	@echo "==> 运行测试..."
	@go test -v -race -coverprofile=coverage.out ./...

test-coverage: test ## 查看测试覆盖率
	@go tool cover -html=coverage.out

# Docker 相关
docker-build: ## 构建 Docker 镜像
	@echo "==> 构建 Docker 镜像..."
	@docker build -t vea:$(VERSION) .

docker-run: docker-build ## 运行 Docker 容器
	@echo "==> 运行 Docker 容器..."
	@docker run -p 8080:8080 -v $(PWD)/data:/data vea:$(VERSION)

.DEFAULT_GOAL := help
