# 技术设计: 修复 Issue #37/#38（当前 IP 异常 + FRouter 配置界面无法打开）

## 技术方案

### Issue #37：`GET /ip/geo` 在代理运行时的稳定性

#### 核心思路
- **不要把 `busy=true` 当成 `running=false`。** 目前 `/ip/geo` 通过 `proxy.Status()` 判断是否走代理；而 `Status()` 在并发场景下可能返回 `{busy:true, running:false}`，导致误回落“直连探测”，从而显示真实 IP。
- **前端避免制造并发**：主页加载 `Promise.all([... loadIPGeo(), refreshCoreStatus() ...])` 会并发打到 `Status()`，放大 busy 概率。

#### 后端改动点（backend/service/facade.go）
1. 为 `GetIPGeo` 增加轻量的“busy 重试”：
   - `Status()` 返回 `busy=true` 时，短 sleep 后重试若干次（例如 3~5 次，总等待 < 300ms）。
   - 若最终仍 busy：返回 error（由 API 层包装为 `{error: "..."}"`），避免误显示直连 IP。
2. 代理运行且非 TUN：保持现有逻辑（通过本地入站端口构造 HTTP Client，强制走 SOCKS/HTTP 代理）。
3. 代理运行且 TUN：
   - 维持“依赖系统路由”的默认策略（不新增端口、不引入端口冲突风险）。
   - 若将来存在可用的本地 mixed 入站端口（配置显式提供），可作为增强项：优先通过本地入站端口探测出口（失败再回退到直连探测/或返回 error）。

#### 前端改动点（frontend/theme/*/js/main.js）
- 重构 `loadHomePanel()` 调用顺序：
  - 先并发加载与状态无关的数据（frouters/nodes/components/settings 等）
  - **再串行**执行 `refreshCoreStatus()` → `loadIPGeo()`，避免与 `/proxy/status` 竞争锁导致 busy 误判

---

### Issue #38：链路编辑面板只能打开一次

#### 核心思路
- 让“打开链路编辑面板”成为**幂等操作**：无论 ChainListEditor 是否已初始化、当前面板状态是否可能不同步，每次触发打开都应切换到 `panel-chain` 并刷新图数据。

#### 前端改动点（frontend/theme/*/js/main.js）
1. 将 `initChainEditor()` 演进为 `ensureChainEditor()` 语义：
   - 首次进入：初始化编辑器并绑定事件
   - 非首次进入：更新当前 FRouter ID，并 `await loadGraph()` 刷新图数据
2. 调整 `openChainEditorPanel()`：
   - 去掉/弱化 `target === currentPanel` 的“早返回”路径，确保面板切换与刷新一定发生
   - 必要时将其改为 `async` 并 `await ensureChainEditor()`，避免刷新时序问题

## 安全与性能
- **安全:** 不涉及敏感信息与权限变更；不引入新的外部依赖；不写入密钥/订阅内容。
- **性能:** `/ip/geo` busy 重试为毫秒级、严格上限；前端减少并发请求可降低整体抖动。

## 测试与部署
- **后端:** `go test ./...`
- **前端（手测）:**
  - 启动应用后进入主页，观察“当前 IP”在切换/重启内核时不再误显示直连 IP（必要时显示错误提示）
  - 双击 FRouter 进入链路编辑 → 返回 → 再次双击进入（同一/不同 FRouter）均可

