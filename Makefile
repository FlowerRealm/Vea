.PHONY: all dev build clean help deps build-backend

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
	@echo "Vea Electron 应用构建工具"
	@echo ""
	@echo "使用方法: make [target]"
	@echo ""
	@echo "常用命令:"
	@echo "  make dev      - 启动开发模式"
	@echo "  make build    - 打包应用"
	@echo "  make clean    - 清理构建产物"
	@echo ""
	@echo "所有命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

all: dev ## 默认：启动开发模式

deps: ## 安装 Electron 依赖
	@echo "==> 安装 Electron 依赖..."
	@cd electron && npm install
	@echo "==> 依赖安装完成"

build-backend: ## 编译 Go 后端
	@echo "==> 编译 Go 后端 $(VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) .
	@echo "==> 后端编译完成: $(OUTPUT_DIR)/$(BINARY_NAME)"
	@ls -lh $(OUTPUT_DIR)/$(BINARY_NAME)

dev: build-backend deps ## 启动 Electron 开发模式
	@echo "==> 启动 Electron 开发模式..."
	@cp $(OUTPUT_DIR)/$(BINARY_NAME) vea
	@cd electron && npm run dev

build: build-backend deps ## 打包 Electron 应用
	@echo "==> 打包 Electron 应用..."
	@cp $(OUTPUT_DIR)/$(BINARY_NAME) vea
	@cd electron && npm run build
	@echo "==> 应用打包完成"
	@ls -lh electron/dist/release/

clean: ## 清理构建产物
	@echo "==> 清理构建产物..."
	@rm -rf $(OUTPUT_DIR)
	@rm -rf electron/dist
	@rm -f vea vea.exe
	@echo "==> 清理完成"

.DEFAULT_GOAL := help
