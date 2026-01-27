# 任务清单：系统代理端口支持局域网连接（Allow LAN）

目录：`helloagents/plan/202601231724_feat-allow-lan-system-proxy/`

---

## 1. 后端（核心）

- [√] 1.1 `backend/service/adapters/singbox.go`：支持 `ProxyConfig.InboundConfig.AllowLAN`，在默认 loopback 监听时切换为 `0.0.0.0`（IPv6 `::1` → `::`）。
- [√] 1.2 `backend/service/proxy/service.go`：端口占用检查的监听地址推导与最终监听地址一致（allowLan 生效时检查 `0.0.0.0`）。

## 2. 前端（设置联动）

- [√] 2.1 `frontend/theme/_shared/js/app.js`：`inbound.allowLan` 变更联动 `PUT /proxy/config` 写入 `inboundConfig.allowLan`。
- [√] 2.2 若内核运行中：自动重启内核使配置生效；并在启动时从后端同步 allowLan 状态到设置 UI。

## 3. 测试

- [√] 3.1 新增单测：sing-box build config 在 allowLan 下的 `listen` 推导正确。
- [√] 3.2 新增单测：`inboundListenAddrForEngine()` 在 allowLan 下返回期望地址。
- [√] 3.3 `gofmt` + `go test ./...`。

## 4. 文档

- [√] 4.1 更新 `helloagents/CHANGELOG.md` 记录本次变更。

