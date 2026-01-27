# 项目上下文

## 项目简介

Vea 是一个桌面端代理/路由工具：

- 后端：Go HTTP API（默认 `:19080`）
- 前端：Electron + 主题系统（`frontend/theme/*`）
- 主要概念：FRouter（对外一等单元）、Node（节点）、ProxyConfig（运行配置）

## 常用入口

- API 规范：`docs/api/openapi.yaml`
- 后端入口：`main.go`
- HTTP 路由：`backend/api/router.go`

