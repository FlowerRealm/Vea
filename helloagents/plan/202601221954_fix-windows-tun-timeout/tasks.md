# 任务清单：修复 Windows 下 TUN 启动 10s 超时（sing-box / clash）

目录：`helloagents/plan/202601221954_fix-windows-tun-timeout/`

---

## 0. 复现与信息收集（阻断项）

- [ ] 0.1 确认复现版本（Windows 版本号 + Vea 版本/commit + 是否管理员运行）。
- [ ] 0.2 复现路径固化：启用 TUN → 等待 10s → 报错；记录 UI 文案与发生时间点。
- [ ] 0.3 获取失败当次的 `kernel.log`（以及应用日志），定位是否存在 `wintun/access denied/route` 等关键报错。
- [ ] 0.4 在报错后立刻请求 `/proxy/status`，确认是否存在“已运行但误报失败”的情况。

## 1. 后端修复（核心）

- [√] 1.1 调整 `backend/service/proxy/service.go`：
  - [√] `waitForTUNReadyByAddress()` 增加“复用旧网卡/名称不可控”场景的兜底：优先精确 IP+prefix 匹配；弱匹配要求网卡更像 TUN（避免误命中虚拟网卡）。
  - [√] Windows 下将 TUN 就绪等待上限从 10s 调整为 25s（macOS 为 20s；Linux 保持 10s）。
  - [√] 错误信息增强：包含 `kernel.log` 路径，并提示可在日志面板复制路径。

## 2. 前端体验（增强可观测性）

- [√] 2.1 `frontend/theme/_shared/js/app.js`：启用 TUN 时状态栏提示改为常驻直到启动结束；TUN 运行中提示补充 `tunIface`（来自 `/proxy/status`）。
- [ ] 2.2 （可选）当错误信息命中典型关键词时给出针对性提示（管理员/Wintun/安全软件/系统策略）。

## 3. 测试

- [√] 3.1 补充单测覆盖 TUN 地址匹配策略（精确匹配/前缀更严格/前缀不匹配回退）。
- [√] 3.2 `gofmt` + `go test ./...`。
- [ ] 3.3 Windows 手动验证（管理员）：sing-box 与 mihomo 各验证一次启用/禁用/重启，确认不再 10s 超时。

## 4. 文档与发布

- [√] 4.1 更新 `docs/SING_BOX_INTEGRATION.md`（Windows 故障排查：如何定位 `kernel.log`、Wintun/冲突自查）。
- [√] 4.2 更新 `helloagents/CHANGELOG.md` 记录本次修复与影响范围。
- [ ] 4.3 归档：修复完成后将方案包迁移到 `helloagents/history/YYYY-MM/`（按项目现有惯例）。
