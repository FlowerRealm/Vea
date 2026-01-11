# 变更提案: 修复 Issue #37/#38（当前 IP 异常 + FRouter 配置界面无法打开）

## 需求背景
Issue #37 反馈：在启用代理（尤其是 TUN 模式）后，主页“当前 IP”仍显示为实际出口 IP（未体现代理出口），容易让用户误判“代理未生效”。

Issue #38 反馈：双击 FRouter 路由只能打开一次链路配置界面（路由规则/链路编辑面板）。第一次返回后，第二次双击无法再进入，必须退出重进才能再次打开，严重影响配置效率。

## 变更内容
1. /ip/geo 在代理运行时避免误回落“直连探测”（尤其是 proxy 状态 busy 的瞬间），提升“当前 IP”显示稳定性与可信度。
2. 首页加载流程避免并发触发 `/proxy/status` 与 `/ip/geo` 导致的 busy 误判，减少“偶发显示真实 IP”。
3. 链路编辑面板打开逻辑幂等化：确保同一窗口会话中可重复进入配置界面，并在每次进入时刷新图数据。

## 影响范围
- **模块:** backend / frontend
- **文件（预期）:**
  - `backend/service/facade.go`（GetIPGeo：busy 处理/走代理策略）
  - `frontend/theme/light/js/main.js`（loadHomePanel 并发调整；链路编辑面板打开逻辑）
  - `frontend/theme/dark/js/main.js`（同上）
- **API:** 无新增接口；`GET /ip/geo` 行为更稳定（必要时返回 error 而不是误报直连 IP）
- **数据:** 无

## 核心场景

### Requirement: IP geo reflects proxied egress when proxy is enabled
**模块:** backend / frontend  
代理已运行（含切换/重启过程中的短暂 busy），主页“当前 IP”应尽量反映代理出口；无法确认时应明确提示“正在切换/忙碌”，而不是误显示直连 IP。

#### Scenario: Home panel IP geo does not fall back to direct on busy
- 条件:
  - 代理运行中或刚完成重启，`/proxy/status` 短时间返回 `busy=true`
  - 前端触发主页数据加载或刷新
- 预期结果:
  - `/ip/geo` 不因一次 busy 误判而直接回落到直连探测
  - UI 不出现“已开代理但显示真实 IP”的误导性状态

### Requirement: Chain editor can be opened multiple times per session
**模块:** frontend/theme  
同一窗口会话中，双击任意 FRouter 可重复进入链路编辑面板；返回后再次双击仍可进入，行为与 2.2.0 一致。

#### Scenario: Open chain editor twice without restart
- 条件:
  - 第一次双击进入链路编辑面板，点击“返回”回到 FRouter 列表
  - 再次双击同一或另一条 FRouter
- 预期结果:
  - 第二次仍能进入链路编辑面板
  - 面板展示的图数据与当前选中的 FRouter 一致（已刷新）

## 风险评估
- **风险:** `/ip/geo` 为了规避 busy 误判增加少量重试等待，可能导致首屏显示略延迟。
- **缓解:**
  - 重试次数与等待时间严格上限（毫秒级），超时后返回明确 error
  - 前端避免不必要的并发请求，降低后端 busy 概率

