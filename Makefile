.PHONY: all dev build clean help deps build-backend

FRONTEND_DIR := frontend
FRONTEND_DEPS_STAMP := $(FRONTEND_DIR)/node_modules/.vea_deps_stamp

# 默认版本号
VERSION ?= dev
# 默认输出目录
OUTPUT_DIR ?= dist
# 默认架构
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)

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

$(FRONTEND_DEPS_STAMP): $(FRONTEND_DIR)/package.json $(FRONTEND_DIR)/package-lock.json
	@echo "==> 安装 Electron 依赖(仅在 package-lock/package.json 变更时)..."
	@cd $(FRONTEND_DIR) && npm install --no-audit --no-fund
	@touch $(FRONTEND_DEPS_STAMP)

deps: $(FRONTEND_DEPS_STAMP) ## 安装 Electron 依赖

build-backend: ## 编译 Go 后端
	@echo "==> 编译 Go 后端 $(VERSION) for $(GOOS)/$(GOARCH)..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(BUILD_FLAGS) -o $(OUTPUT_DIR)/$(BINARY_NAME) .
	@echo "==> 后端编译完成: $(OUTPUT_DIR)/$(BINARY_NAME)"
	@ls -lh $(OUTPUT_DIR)/$(BINARY_NAME)

dev: ## 启动 Electron 开发模式
	@echo "==> 停止正在运行的 vea 和 electron 进程..."
	@-pkill -9 -f "vea.*--addr" 2>/dev/null || true
	@-pkill -9 electron 2>/dev/null || true
	@-fuser -k 19080/tcp 2>/dev/null || true
	@echo "==> 删除旧的二进制文件..."
	@rm -f vea vea.exe
	@$(MAKE) -j2 build-backend deps
	@echo "==> 启动 Electron 开发模式..."
	@cp $(OUTPUT_DIR)/$(BINARY_NAME) vea
	@if [ "$(BINARY_NAME)" != "vea" ]; then cp $(OUTPUT_DIR)/$(BINARY_NAME) $(BINARY_NAME); fi
	@cd frontend && npm run dev

build: ## 打包 Electron 应用
	@echo "==> 停止正在运行的 vea 进程..."
	@-pkill -9 vea 2>/dev/null || true
	@sleep 1
	@echo "==> 删除旧的二进制文件..."
	@rm -f vea vea.exe
	@$(MAKE) -j2 build-backend deps
	@echo "==> 打包 Electron 应用..."
	@cp $(OUTPUT_DIR)/$(BINARY_NAME) $(BINARY_NAME)
	@cd frontend && npm run build
	@echo "==> 应用打包完成"
	@ls -lh release/

clean: ## 清理构建产物
	@echo "==> 清理构建产物..."
	@rm -rf $(OUTPUT_DIR)
	@rm -rf frontend/dist
	@rm -rf release
	@rm -f vea vea.exe
	@echo "==> 清理完成"

.DEFAULT_GOAL := help
