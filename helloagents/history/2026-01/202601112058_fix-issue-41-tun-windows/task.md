# 任务清单: 修复 Issue #41 - Windows TUN 启动失败

目录: `helloagents/plan/202601112058_fix-issue-41-tun-windows/`

---

## 1. 后端（TUN 就绪判定）
- [√] 1.1 在 `backend/service/proxy/service.go` 调整 TUN readiness：Windows/macOS 用“新网卡 + 地址匹配/进程退出”判定并记录实际网卡名；Linux 保持现有逻辑，验证 why.md#需求-tun-ready-windows-场景-tun-ready-windows-enable 与 why.md#需求-tun-ready-nonregression-场景-tun-ready-linux-restart
- [√] 1.2 在 `backend/service/adapters/singbox.go` 非 Linux 默认不写 `interface_name=tun0`（留空自动选择），验证 why.md#需求-tun-ready-windows-场景-tun-ready-windows-enable

## 2. 安全检查
- [√] 2.1 执行安全检查（按G9: 输入验证、敏感信息处理、权限控制、EHRB风险规避）

## 3. 文档更新
- [√] 3.1 更新 `helloagents/wiki/modules/backend.md` 记录 Windows TUN 就绪判定与 interface_name 行为说明
- [√] 3.2 更新 `helloagents/CHANGELOG.md` 记录本次修复

## 4. 测试
- [√] 4.1 运行 `gofmt` 格式化变更文件
- [√] 4.2 运行 `go test ./...`
- [?] 4.3 手动验证（Windows）：启用/禁用 TUN 不再报错，验证 why.md#需求-tun-ready-windows-场景-tun-ready-windows-enable
  > 备注: 需要在 Windows 环境（管理员权限）下启动应用验证
