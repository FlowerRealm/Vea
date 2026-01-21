# 技术设计: 修复 Windows TUN 状态误报管理员权限

## 技术方案

### 核心技术
- Go（后端）
- Electron 前端主题页（可选兜底）

### 实现要点

- Windows 的 `/tun/check` 不再把 `configured` 等同于“进程已提升为管理员”。
  - **理由：** 这会导致“实际可用但显示未配置”的误报，属于 UI/状态语义不一致问题。
  - **目标：** `configured` 表达“是否需要一次性配置动作”（Linux 需要 setcap；Windows/macOS 通常不需要一次性配置，只在运行时可能需要提权）。

#### 后端改动点

1. `backend/service/shared/tun_windows.go`
   - 调整 `CheckTUNCapabilities()`：Windows 直接返回 `true, nil`（表示无需额外配置）。
   - `GetTUNCapabilityStatus()` 同步返回 `FullyConfigured=true`（保持语义一致）。
   - `EnsureTUNCapabilities()` 保持为 no-op（或仅在需要时返回提示），避免把“是否管理员”当作“是否已配置”。

2. `backend/api/proxy.go`
   - `/tun/check` 在 Windows 下的 `setupCommand/description` 改为：
     - `setupCommand`: “无需额外配置”
     - `description`: “Windows 下 TUN 通常无需一次性配置；若启动失败再尝试以管理员身份运行 Vea”
   - 保持响应字段兼容，不新增必须字段。

3. `backend/service/proxy/service.go`（可选增强）
   - 当 Windows 上启动内核返回的错误/退出日志包含权限相关关键字（例如 “access is denied”“requires elevation”）时，在 `lastRestartError` 或返回错误中追加“可尝试管理员运行”的提示，提升可操作性。

#### 前端兜底（可选）

`frontend/theme/_shared/js/app.js`：
- 当检测到 `tunRunning=true` 时，设置页的 “TUN 配置状态” 优先显示为已配置（即使 `/tun/check` 的 `configured` 因异常返回 false），避免已运行却显示未配置的矛盾状态。

## API设计

### GET /tun/check
- **保持字段:** `configured`, `platform`, `setupCommand`, `description`
- **语义调整（仅 Windows）:** `configured=true` 表示“无需额外一次性配置动作”；管理员权限提示由运行时失败场景承担。

## 安全与性能

- **安全:** 不引入新的提权流程；不在应用内缓存任何管理员凭据；仅调整状态判定与错误提示。
- **性能:** 能力检查变为常量返回，不增加额外系统调用。

## 测试与部署

- 本地：`gofmt` + `go test ./...`
- Windows 手动验证：
  - 开发模式（普通用户态）打开设置页：TUN 配置状态不再误报“未配置/需要管理员”。
  - 启用 TUN 后：首页/设置页状态与实际一致（运行中/已启用）。
  - 若刻意制造权限失败：错误提示包含可操作建议（可选增强项）。

