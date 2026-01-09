# 技术设计: 修复 Linux 下 mihomo(Clash) TUN 断网

## 技术方案

### 核心依据（主流实现）
参考主流桌面客户端（如 Clash Party / Mihomo Party）的默认配置思路：
- TUN: 默认 `stack=mixed`、`auto-route=true`、MTU 倾向 1500、`auto-redirect` 默认关闭（用户显式启用）
- DNS: 支持 `https://`/`tls://`/`dhcp://` 等 scheme 的 nameserver；使用纯 IP 的 `default-nameserver` 解决自举
- PROCESS-NAME: 需要 `find-process-mode=strict` 才能可靠生效

### 实现要点
1. `backend/service/adapters/clash.go`
   - Linux + TUN：设置 `find-process-mode=strict`
   - TUN：默认 `stack=mixed`；Linux 默认 `mtu=1500`；`auto-redirect` 仅在 `AutoRedirect=true` 时写入
   - DNS：不再过滤包含 `://` 的 server；默认 nameserver 使用 DoH；同时写入 `default-nameserver/proxy-server-nameserver/direct-nameserver` 的 IP bootstrap 列表
2. `backend/service/adapters/clash_test.go`
   - 调整 auto-redirect 的默认行为单测
   - 增加 Linux 默认 MTU=1500 的单测

## 安全与性能
- **安全:** 不引入额外外部依赖；仅调整配置生成逻辑
- **性能:** 仅增加少量配置字段，不增加运行期复杂计算

## 测试与验证
- `go test ./...`
