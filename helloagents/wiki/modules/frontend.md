# frontend 模块

## 职责
- Electron 主进程与 UI 主题页面
- 通过 SDK 调用后端 API
- 提供组件面板：安装/更新/卸载核心组件

## 关键目录
- `frontend/`：Electron 入口与 UI
- `frontend/sdk/`：JS SDK（构建产物已提交）

## 变更历史
- [202601071130_fix-gz-extract-clash-install](../../history/2026-01/202601071130_fix-gz-extract-clash-install/) - 组件面板新增“卸载”按钮；主题按钮 hover 支持 `--accent-hover` 变量（提升一致性与可维护性）
- [202601071248_refactor-tun-defaults-engine-ui](../../history/2026-01/202601071248_refactor-tun-defaults-engine-ui/) - 主题页：抽取 `updateEngineSetting` 公共刷新逻辑，减少重复代码并便于维护
- [202601071306_fix-chmod-engine-switch-proxy-failfast](../../history/2026-01/202601071306_fix-chmod-engine-switch-proxy-failfast/) - 主题页：切换内核引擎时关闭系统代理失败改为快速失败，避免网络中断风险
