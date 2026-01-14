# frontend 模块

## 职责
- Electron 主进程与 UI 主题页面
- 通过 SDK 调用后端 API
- 提供组件面板：安装/更新/卸载核心组件
- 提供应用自更新能力：手动检查更新，自动下载/安装并重启（Windows/macOS）
  - 更新源：GitHub Pages（GenericProvider，`/updates/`）；已禁用差分更新，不依赖 `*.blockmap`

## 关键目录
- `frontend/`：Electron 入口与 UI
- `frontend/theme_manager.js`：内置主题同步逻辑（复制到 `userData/themes`、marker+hash 判定是否覆盖、注入 `_shared` 共享模块）；由 `frontend/main.js` 启动阶段调用
- `frontend/sdk/`：JS SDK（构建产物已提交）
- `frontend/theme/<themeId>/`：内置主题包（入口 `index.html`）
- `frontend/theme/*/css/main.css`：内置主题需声明 `color-scheme`（dark/light），避免 Windows 下 `<select>` 等原生控件弹出层沿用系统配色导致对比度异常
- `userData/themes/<packId>/manifest.json`：主题包（manifest）容器；包内可包含多个子主题，入口由 `entry`（相对 `themes/`）指定

## 规范

### 需求: FRouter 详情卡片走向图（静态配置）
**模块:** frontend/theme

在 FRouter 面板中，选中态的 FRouter 以“详情卡片”形式展开，并展示“走向图”：以静态配置视角说明流量从 `local` 经由规则到 `direct/block/节点`，以及 `via/detour` 形成的 hop 链路。

**展示规则:**
- 规则节点需展示 priority、规则类型（默认/路由）、匹配条件摘要（domains/ips）；hover 可查看完整匹配项（截断不丢信息）
- slot 已绑定时，走向图直接展示绑定后的节点（不单独显示 slot）
- slot 未绑定时，走向图标注“未绑定/穿透（编译会跳过）”
- 默认不展示节点地址/端口等敏感信息，避免截图分享泄露

**交互:**
- 支持拖拽平移、滚轮缩放浏览
- 双击适配视图（fit-to-view）

### 需求: 槽位管理（slot-*）
**模块:** frontend/theme

在 FRouter「路由规则」面板提供“槽位管理”入口，用于管理 `ChainProxySettings.slots`：
- 新增槽位（生成新的 `slot-*` id）
- 重命名槽位（编辑 `name`）
- 绑定/解绑节点（编辑 `boundNodeId`）

**实现位置:**
- `frontend/theme/_shared/js/app.js`：`ChainListEditor` 负责槽位管理弹窗（`#chain-manage-slots` / `#slot-manage-dialog`）；保存会标记为 dirty，最终由底部“保存”提交并携带 `positions` 以避免清空布局。

#### 场景: 规则选择槽位
在规则编辑弹窗中，“匹配后去向”可选择槽位，并显示“已绑定/未绑定/未知绑定”状态。

#### 场景: 保存不破坏布局
路由规则面板保存图配置时需携带 `positions`，避免意外清空链路编辑器的布局数据。

### 需求: TUN 接口名默认 vea
**模块:** frontend/settings-schema

`tun.interfaceName` 默认值为 `vea` 并保持只读，确保与后端默认值一致；Windows/macOS 上该值不代表实际网卡名称（默认不强制写死名称）。

#### 场景: 默认值展示一致
- 预期结果: 设置面板中 `tun.interfaceName` 默认显示为 `vea`
- 预期结果: 不暗示 Windows/macOS 上一定会出现名为 `vea` 的网卡

## 注意事项
- 槽位 `id` 作为引用标识不开放编辑；仅允许新增、重命名、绑定/解绑，避免规则引用失效。
- 主题切换入口解析需兼容 Windows `file://` URL 的路径编码/分隔符差异（例如 `%5C`），避免仅用字符串查找推导主题根路径。
- Electron 采用单实例锁避免重复启动；启动后端前会先请求 `/health` 判定是否已存在同 `userData` 的后端服务，避免固定端口冲突导致“后端闪退”。

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
- [202601112042_fix-slot-ui](../../history/2026-01/202601112042_fix-slot-ui/) - 主题页：补齐“槽位管理”入口与保存 positions，修复槽位功能不可用（Issue #40）
- [202601112053_fix-issue42-subscription-label](../../history/2026-01/202601112053_fix-issue42-subscription-label/) - 主题页：修复节点面板首次进入订阅名显示为配置 ID（Issue #42）
- [202601112055_fix-ui-theme-contrast](../../history/2026-01/202601112055_fix-ui-theme-contrast/) - 主题页：补齐 `color-scheme`（dark/light）并补齐浅色主题缺失 CSS 变量，修复 Windows 下下拉/列表控件对比度异常（Issue #39）
- [202601112057_fix-issue37-38](../../history/2026-01/202601112057_fix-issue37-38/) - 主题页：主页加载避免并发触发 `/proxy/status` 与 `/ip/geo`；链路编辑面板可重复打开并每次进入刷新图数据（Issue #37/#38）
- [202601112058_fix-issue-36-theme-switch](../../history/2026-01/202601112058_fix-issue-36-theme-switch/) - 主题页：修复 Windows 下切换默认主题报“无法解析主题入口”（Issue #36）
- [202601112114_refactor-restart-core-button](../../history/2026-01/202601112114_refactor-restart-core-button/) - 主题页（首页）：抽取核心状态/按钮区域内联样式到 CSS；重构 `handleCoreRestart` 并统一缩进
- [202601112155_pr-review-theme-shared-async](../../history/2026-01/202601112155_pr-review-theme-shared-async/) - 主题页：dark/light 主逻辑抽到共享模块；Electron 主题同步改为异步并注入共享模块，避免主进程同步 IO 阻塞
- [202601121727_theme-sync-refactor](../../history/2026-01/202601121727_theme-sync-refactor/) - Electron：主题同步逻辑抽离为独立模块，并统一 dark 主题缩进风格
- [202601121916_default-tun-interface-name-vea](../../history/2026-01/202601121916_default-tun-interface-name-vea/) - 设置：`tun.interfaceName` 默认值从 `tun0` 调整为 `vea`（保持只读）
- [202601131921_fix-backend-port-conflict](../../history/2026-01/202601131921_fix-backend-port-conflict/) - Electron：单实例锁 + 启动前健康检查，避免固定端口冲突导致后端启动即退出
- [202601141452_frouter_flow_graph](../../history/2026-01/202601141452_frouter_flow_graph/) - 主题页：FRouter 面板选中态卡片新增“走向图”（静态配置），支持拖拽平移与滚轮缩放浏览
