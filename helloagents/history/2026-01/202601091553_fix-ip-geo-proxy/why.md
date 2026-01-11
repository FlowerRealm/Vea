# 变更提案: 修复首页“当前 IP”未走代理（Issue #26）

## 需求背景

- 首页“当前 IP”通过后端接口 `GET /ip/geo` 获取。
- 现状：接口直连访问第三方 IP 服务，导致在代理内核已启动、FRouter 已生效时仍显示真实出口 IP，误导用户判断代理是否生效。

## 变更内容

1. 当代理内核运行中且 `inboundMode` 为 `socks/http/mixed` 时，`GET /ip/geo` 通过本地入站代理（默认 `127.0.0.1:<inboundPort>`）访问 IP 服务，返回代理出口 IP。
2. 当代理未运行时，保持直连获取真实出口 IP 的行为不变。
3. 当“应走代理”的请求失败时，返回 `error` 字段用于前端提示（不静默回退到直连，避免再次出现“看起来像没生效”的误导）。

## 影响范围

- **模块:** backend API / backend shared
- **文件:** `backend/service/facade.go`、`backend/service/shared/tun.go`（可能新增 `backend/service/shared/socks5.go` 与测试文件）
- **API:** `GET /ip/geo`（接口字段不变；语义修复：代理运行时返回代理出口 IP）
- **数据:** 无

## 核心场景

### 需求: current-ip-via-proxy
**模块:** backend

#### 场景: proxy-running-mixed-socks-http
前置条件：代理已启动（FRouter 已应用），`inboundMode` 为 `mixed/socks/http`
- 预期结果：`GET /ip/geo` 返回的 `ip` 为代理出口 IP

#### 场景: proxy-not-running
前置条件：代理未启动
- 预期结果：`GET /ip/geo` 返回的 `ip` 为真实出口 IP

#### 场景: proxy-running-but-failed
前置条件：代理已启动，但入站认证/端口不可用/链路异常导致探测失败
- 预期结果：返回 `error` 字段；前端显示错误提示，不误报为真实出口 IP

## 风险评估

- **风险:** 通过代理请求第三方 IP 服务可能更慢或被节点/规则影响导致失败。
- **缓解:** 保持 6s 超时 + 多 provider 回退；失败时明确返回 `error` 供前端提示与手动刷新。

