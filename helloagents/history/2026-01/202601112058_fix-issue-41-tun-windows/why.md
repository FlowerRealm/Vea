# 变更提案: 修复 Issue #41 - Windows TUN 启动失败

## 需求背景

- 现象：Windows 10 + Vea 2.3.0（zip 版）在启用 TUN 时提示 `TUN interface not ready: TUN interface tun0 not ready after 10s`，随后 TUN 无法使用（Issue #41，关联 #30/#32）。
- 现状代码：后端在 TUN 启动后会等待指定名称的网卡（默认 `tun0`）出现；若 10 秒内未出现，会主动停止内核进程并向前端返回失败。
- 推测根因：sing-box 在 Windows 下创建的 TUN 网卡名称并不等于 `tun0`（或不保证可控），导致“误判未就绪→误杀内核”。

## 变更内容

1. 后端 TUN 就绪判定在 Windows（以及 macOS）不再强依赖固定网卡名：改为基于“新出现的网卡 + 配置地址匹配/特征匹配”进行识别，并在进程提前退出时快速报错。
2. sing-box TUN 配置在非 Linux 默认不强制写入 `interface_name=tun0`（留空让内核自动选择），避免名字不匹配导致的误判。
3. 保持 Linux 现有行为不变；不新增/修改对外 API。

## 影响范围

- **模块:** backend
- **文件:**
  - backend/service/proxy/service.go
  - backend/service/adapters/singbox.go
- **API:** 无
- **数据:** 无

## 核心场景

### 需求: tun-ready-windows

**模块:** backend

Windows 上启用 TUN 时应稳定启动，不因网卡名差异误判失败。

#### 场景: tun-ready-windows-enable

前置条件:
- Windows 10 x64
- Vea 以管理员运行
- 已安装并选择 sing-box 引擎
- 已选择一个 FRouter

- 预期结果1: 启用 TUN 后 `/proxy/start` 成功返回，前端不再报 `TUN interface not ready...`。
- 预期结果2: `/proxy/status` 显示 `running=true` 且 `inboundMode=tun`，并记录实际创建的 TUN 网卡名（供后续 stop/restart 使用）。
- 预期结果3: 若 TUN 创建失败，错误信息应优先反映“进程已退出/权限/配置错误”等真实原因，而不是统一超时。

### 需求: tun-ready-nonregression

**模块:** backend

Linux 现有 TUN 行为不回退，仍能在快速重启场景下避免 EBUSY。

#### 场景: tun-ready-linux-restart

前置条件:
- Linux
- TUN 模式快速重启多次

- 预期结果: 仍按 `interface_name` 进行就绪判定与释放等待，不引入新的残留规则/网卡占用问题。

## 风险评估

- **风险:** Windows/macOS 的“新网卡识别”存在误判可能（恰好同时出现其他新网卡）。
- **缓解:** 优先用 TUN 配置中的地址前缀匹配；同时监听内核进程退出信号，避免“进程已退出但还在等”的误导超时。

