# 项目技术约定

## 技术栈
- **后端:** Go（见 `go.mod`），建议 Go 1.22+
- **前端:** Electron + Node.js（建议 Node 18+）
- **API 规范:** `docs/api/openapi.yaml`

## 开发约定
- **Go 格式化:** `gofmt`
- **后端测试:** `go test ./...`
- **命名约定:** 与现有代码一致（Go: 驼峰；HTTP handler: `listXxx/createXxx/updateXxx/deleteXxx`）

## 错误与日志
- **策略:** 失败应尽量 fail fast，并给出可操作的错误信息（路径/端口/引擎/权限）
- **日志:** 默认使用标准库 `log`，避免过度结构化

## 提交与流程
- **Commit Message:** 推荐 Conventional Commits（如 `fix:`/`feat:`/`docs:`）
- **不提交产物:** `dist/`、`release/`、`data/` 等运行时目录不纳入版本控制（例外见仓库说明）
