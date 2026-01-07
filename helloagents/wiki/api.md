# API 手册

## 概述
后端 HTTP API 的权威定义在 `docs/api/openapi.yaml`。前端通过 SDK 调用这些接口来管理节点、路由与运行状态。

## 认证方式
本地应用场景下默认不做复杂认证（以本机访问为主），具体以实现为准。

## 接口索引（节选）
- `GET /frouters` / `POST /frouters` / `PUT /frouters/:id` / `DELETE /frouters/:id`
- `GET /frouters/:id/graph`
- `GET /nodes`
- `POST /proxy/start` / `POST /proxy/stop`
- `GET /proxy/status` / `GET /proxy/logs`
