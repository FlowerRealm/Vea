# 技术设计: 默认 TUN 网卡名改为 vea

## 技术方案

### 核心技术
- Go（后端默认值与就绪判定逻辑）
- sing-box / mihomo 配置生成适配（Go map config）
- 前端 settings schema（JS 静态默认值）

### 实现要点

#### 1) 后端默认值统一
- 将 `backend/service/proxy/service.go` 的 `defaultTunInterfaceName` 从 `tun0` 调整为 `vea`。
- 确保在 `InboundMode=TUN` 且 `TUNSettings` 为空/缺省时，补齐的 `TUNSettings.InterfaceName` 为 `vea`。

#### 2) 非 Linux 默认不强制名称（同时兼容 legacy tun0）
目标：在 Windows/macOS 上不依赖设备名，避免“写死名称”与“按名称等待就绪”带来的失败。

- `backend/service/proxy/service.go`
  - 生成/启动流程中，非 Linux 进行 TUN 就绪判定时：
    - 当 `interfaceName` 为默认 `vea` 或 legacy `tun0` 时，将 `desiredName` 置空，走“按地址/网段”匹配（现有 `waitForTUNReadyByAddress`）。
    - 当用户显式设置为其它名称时，仍将该名称作为“优先匹配提示”传入（但最终仍以地址为准，避免完全依赖名称）。

- `backend/service/adapters/singbox.go`
  - `runtime.GOOS != "linux"` 且 `interfaceName` 为 `vea` 或 `tun0` 时，不写 `tun.interface_name`。
  - Linux 保持显式写入 `tun.interface_name`，便于后端按名称/索引判定就绪与释放等待。

- `backend/service/adapters/clash.go`
  - `runtime.GOOS != "linux"` 且 `interfaceName` 为 `vea` 或 `tun0` 时，不写 `tun.device`。
  - Linux 保持写入 `tun.device`（与后端就绪判定保持一致）。

#### 3) 测试用例覆盖
- 更新 `backend/service/proxy/service_test.go`：涉及默认 `InterfaceName` 断言的用例，将 `tun0` 改为 `vea`。
- 更新 `backend/service/adapters/singbox_tun_settings_test.go`：
  - 覆盖 Linux 下 `interface_name=vea` 正常写入。
  - 覆盖 legacy `tun0` 在非 Linux 下应省略写入的逻辑（测试结构保持现有“按 runtime.GOOS 分支断言”的方式）。
- 更新 `backend/service/adapters/clash_test.go`：
  - 涉及默认 `InterfaceName` 的用例同步为 `vea`（或明确为 legacy 测试时保留 `tun0`）。

#### 4) 前端默认值同步
更新三处 settings schema 默认值：
- `frontend/settings-schema.js`
- `frontend/theme/dark/js/settings-schema.js`
- `frontend/theme/light/js/settings-schema.js`

将 `tun.interfaceName` 的默认值从 `tun0` 改为 `vea`（保持只读）。

#### 5) 文档与知识库同步
- 更新 `docs/SING_BOX_INTEGRATION.md`：
  - 默认值改为 `vea`
  - 增加说明：Windows/macOS 默认不强制设备名，`interfaceName` 更偏“逻辑默认值”，实际网卡名可能不同。
- 如 `helloagents/wiki/modules/backend.md` 中存在默认值描述，同步更新以保持 SSOT 与代码一致。

## 安全与性能
- **安全:** 不引入新权限；仍沿用现有 TUN 提权/校验流程。避免在非 Linux 依赖名称可降低误判风险。
- **性能:** 无明显影响；就绪判定仍为短时轮询，且按地址匹配在非 Linux 更稳定。

## 测试与部署
- **测试:** `go test ./...`（重点关注 `backend/service/proxy`、`backend/service/adapters`）。
- **部署:** 无额外步骤；与现有发布流程一致。
