# frontend 模块

## 职责
- Electron 主进程与 UI 主题页面
- 通过 SDK 调用后端 API
- 提供组件面板：安装/更新/卸载核心组件
- 提供应用自更新能力：手动检查更新，自动下载/安装并重启（Windows/macOS）

## 关键目录
- `frontend/`：Electron 入口与 UI
- `frontend/sdk/`：JS SDK（构建产物已提交）
- `frontend/theme/<themeId>/`：内置主题包（入口 `index.html`）
- `userData/themes/<packId>/manifest.json`：主题包（manifest）容器；包内可包含多个子主题，入口由 `entry`（相对 `themes/`）指定

## 变更历史
- [202601071130_fix-gz-extract-clash-install](../../history/2026-01/202601071130_fix-gz-extract-clash-install/) - 组件面板新增“卸载”按钮；主题按钮 hover 支持 `--accent-hover` 变量（提升一致性与可维护性）
- [202601071248_refactor-tun-defaults-engine-ui](../../history/2026-01/202601071248_refactor-tun-defaults-engine-ui/) - 主题页：抽取 `updateEngineSetting` 公共刷新逻辑，减少重复代码并便于维护
- [202601071306_fix-chmod-engine-switch-proxy-failfast](../../history/2026-01/202601071306_fix-chmod-engine-switch-proxy-failfast/) - 主题页：切换内核引擎时关闭系统代理失败改为快速失败，避免网络中断风险
- [202601080702_fix-frouter-rule-list-label](../../history/2026-01/202601080702_fix-frouter-rule-list-label/) - 主题页：FRouter 路由规则列表优先展示模板/首条匹配项，并在次行显示去向
- [202601080848_fix-subscription-pull-refresh-duplicate](../../history/2026-01/202601080848_fix-subscription-pull-refresh-duplicate/) - 主题页：订阅面板配置行去除重复操作，移除“刷新”按钮，仅保留“拉取节点”
- [202601080900_fix-subscription-error-message-overflow](../../history/2026-01/202601080900_fix-subscription-error-message-overflow/) - 主题页：订阅面板同步失败错误信息单行省略显示，避免表格行高度异常
- [202601081053_fix-review-followups](../../history/2026-01/202601081053_fix-review-followups/) - 主题页：日志面板与 `updateCoreUI` 附近缩进一致性修复
- [202601081145_fix-review-report](../../history/2026-01/202601081145_fix-review-report/) - 主题页：订阅面板同步错误字段处理去冗余（去掉多余 `String()` 转换）
- [202601081339_fix-proxy-port-sync](../../history/2026-01/202601081339_fix-proxy-port-sync/) - 主题页：设置项 `proxy.port` 联动后端 `ProxyConfig.inboundPort`，端口变更自动重启并重应用系统代理；启动时从后端同步实际端口避免误导
- [202601091503_fix-speed-unit-mbs](../../history/2026-01/202601091503_fix-speed-unit-mbs/) - 主题页/SDK：速度单位显示从 `Mbps` 修正为 `MB/s`（与实际测速计算单位一致）
- [202601091540_fix-subscription-create-async](../../history/2026-01/202601091540_fix-subscription-create-async/) - 主题页/SDK：保存订阅不再阻塞 UI；提示“后台拉取中…”并显示“未同步”状态；零时间显示 `-`
- [202601091547_fix-issue-27-windows-default-rule-templates](../../history/2026-01/202601091547_fix-issue-27-windows-default-rule-templates/) - Electron 打包补齐 `chain-editor/rule-templates.js`，修复 Windows 默认规则模板缺失（Issue #27）
- [202601091549_fix-slot-ui](../../history/2026-01/202601091549_fix-slot-ui/) - 主题页：FRouter 路由规则面板新增“槽位管理”，支持新增/重命名/绑定节点（修复 Issue #29）
- [202601091550_fix-issue-28-light-log-autoscroll-toggle](../../history/2026-01/202601091550_fix-issue-28-light-log-autoscroll-toggle/) - 浅色主题：日志面板“自动滚动”开关关闭态补齐轨道底色，避免控件不可见（Issue #28）
- [202601091557_feat-auto-update](../../history/2026-01/202601091557_feat-auto-update/) - 应用内“检查更新”入口；支持 Windows/macOS 自动下载、安装并重启（Issue #24）
- [202601091650_fix-issue-33-frouter-highlight](../../history/2026-01/202601091650_fix-issue-33-frouter-highlight/) - 浅色主题：FRouter 选中态高亮改为黑色边框（Issue #33）
- [202601091657_fix-issue-32-tun-status](../../history/2026-01/202601091657_fix-issue-32-tun-status/) - 主题页：TUN 卡片主状态改为运行态展示，能力检查仅用于详情/指引（Issue #32）
- [202601091715_fix-app-update-check-no-response](../../history/2026-01/202601091715_fix-app-update-check-no-response/) - 主题页：修复“检查应用更新”点击无响应（`showStatus` 作用域问题）
- [202601092026_theme-package](../../history/2026-01/202601092026_theme-package/) - 主题目录化并支持 ZIP 导入/导出；Electron 启动从 userData/themes 加载并在缺失时复制内置主题
- [202601100554_pr-review-hardening](../../history/2026-01/202601100554_pr-review-hardening/) - 主题页：`showStatus` 改为模块内共享（不挂 window）；订阅导入后刷新改为轮询 `lastSyncedAt`，避免固定延时竞态
- [202601100601_theme-pack-manifest](../../history/2026-01/202601100601_theme-pack-manifest/) - 主题包支持 `manifest.json`（单包多子主题）；`entry` 驱动切换与启动加载
- [202601111422_feat-restart-core-button](../../history/2026-01/202601111422_feat-restart-core-button/) - 主题页（首页）：增加“重启内核”按钮；并修复内置主题升级后不自动同步的问题
- [202601112114_refactor-restart-core-button](../../history/2026-01/202601112114_refactor-restart-core-button/) - 主题页（首页）：抽取核心状态/按钮区域内联样式到 CSS；重构 `handleCoreRestart` 并统一缩进
- [202601112155_pr-review-theme-shared-async](../../history/2026-01/202601112155_pr-review-theme-shared-async/) - 主题页：dark/light 主逻辑抽到共享模块；Electron 主题同步改为异步并注入共享模块，避免主进程同步 IO 阻塞
