# 任务清单: 修复首页“当前 IP”未走代理（Issue #26）

目录: `helloagents/plan/202601091553_fix-ip-geo-proxy/`

---

## 1. backend: IP Geo 走代理
- [√] 1.1 在 `backend/service/facade.go` 中根据 `proxy.Status()` 判断代理运行状态与 `inboundMode`，在运行且非 TUN 时优先走本地入站代理请求 IP Geo（验证 why.md#需求-current-ip-via-proxy 与 why.md#场景-proxy-running-mixed-socks-http）
- [√] 1.2 在 `backend/service/shared/tun.go` 中提供可注入 `http.Client` 的 `GetIPGeoWithHTTPClient`（或等效函数），复用现有 provider/parse 逻辑（验证 why.md#场景-proxy-not-running）
- [√] 1.3 新增 `backend/service/shared/socks5.go`（或等效位置）实现最小 SOCKS5 DialContext（支持 noauth / username+password），供 IP Geo 使用（验证 why.md#场景-proxy-running-mixed-socks-http）
- [√] 1.4 为 `mixed` 模式实现 socks → http 的探测回退（验证 why.md#场景-proxy-running-but-failed）

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 3. 文档更新
- [√] 3.1 更新 `helloagents/wiki/modules/backend.md` 增加本次变更条目
- [√] 3.2 更新 `helloagents/CHANGELOG.md` 记录修复（Issue #26）

## 4. 测试
- [√] 4.1 运行 `go test ./...`，必要时补充 `backend/service/shared/*_test.go` 做离线验证
