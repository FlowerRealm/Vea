# 变更提案: 修复 Windows 下 sing-box mixed 入站端口占用导致启动失败

## 需求背景
当前 Windows 环境下，sing-box 以 `inboundMode=mixed` 启动时会监听 `127.0.0.1:<inboundPort>`。
当该端口已被其他程序占用（常见：其他代理客户端/旧实例占用 `1080`）时，sing-box 会直接启动失败并退出。

现状问题：
1. 后端默认 `inboundPort=1080`，与常见代理软件默认端口冲突概率高。
2. 端口占用时应明确失败原因并给出可操作的修复提示（改端口/关闭占用者），避免用户误判为“mixed 不可用”。

## 变更内容
1. 将后端默认入站端口从 `1080` 调整为 `31346`，与前端「系统代理 → 代理端口」默认值保持一致，降低冲突概率。
2. 启动内核前对入站端口做占用检测：端口被占用时直接返回清晰错误信息（不自动改端口）。

## 影响范围
- **模块:**
  - backend（proxy/service, facade, memory store）
- **文件:**
  - `backend/service/proxy/service.go`
  - `backend/service/facade.go`
  - `backend/repository/memory/store.go`
  - （可能）`backend/service/adapters/singbox.go` / `backend/service/adapters/clash.go`（如需加强 readiness probe）
- **API:**
  - `POST /proxy/start`：端口被占用时更早返回错误（期望为 4xx）
- **数据:**
  - 新安装/无状态场景的默认端口变化（已存在的用户配置不受影响）

## 核心场景

### 需求: 端口被占用时明确失败
**模块:** backend/service/proxy
当 `inboundPort` 已被占用时，启动应失败并提示如何处理。

#### 场景: Windows + mixed + 端口占用
条件：Windows 下 `inboundMode=mixed`，且 `127.0.0.1:<port>` 已被其他程序监听
- 预期结果：`POST /proxy/start` 返回明确错误（包含端口与处理建议），sing-box 不应被误判为“已就绪”

### 需求: 默认端口调整为 31346
**模块:** backend/service/proxy, backend/repository/memory
新安装/无状态时默认端口与前端保持一致，减少冲突。

#### 场景: 新安装默认值
条件：首次启动（或 state 中 inboundPort=0），且 `inboundMode!=tun`
- 预期结果：默认 `inboundPort=31346`

## 风险评估
- **风险:** 修改默认端口可能改变新安装用户的默认行为
- **缓解:** 仅影响默认值；已有配置仍按持久化值运行；同时给出端口占用的明确错误信息，降低排障成本

