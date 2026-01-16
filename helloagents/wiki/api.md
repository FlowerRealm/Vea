# API 手册

## 概述
后端 HTTP API 的权威定义在 `docs/api/openapi.yaml`。前端通过 SDK 调用这些接口来管理节点、路由与运行状态。

## 认证方式
本地应用场景下默认不做复杂认证（以本机访问为主），具体以实现为准。

## 接口索引（节选）
- `GET /frouters` / `POST /frouters` / `PUT /frouters/:id` / `PUT /frouters/:id/meta` / `POST /frouters/:id/copy` / `DELETE /frouters/:id`
- `GET /frouters/:id/graph`
- `GET /nodes`
- `GET /themes` / `POST /themes/import` / `GET /themes/:id/export` / `DELETE /themes/:id`
- `GET /components` / `POST /components` / `PUT /components/:id` / `DELETE /components/:id`
- `POST /components/:id/install` / `POST /components/:id/uninstall`
- `POST /proxy/start` / `POST /proxy/stop`
- `GET /proxy/status` / `GET /proxy/logs`（status 在用户显式 stop 后可能包含 `userStopped` / `userStoppedAt`）
- `GET /app/logs?since=...`（`since` 为非负字节偏移，非法值返回 400）
