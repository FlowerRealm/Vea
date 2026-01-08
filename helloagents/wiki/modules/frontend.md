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
- [202601080702_fix-frouter-rule-list-label](../../history/2026-01/202601080702_fix-frouter-rule-list-label/) - 主题页：FRouter 路由规则列表优先展示模板/首条匹配项，并在次行显示去向
- [202601080848_fix-subscription-pull-refresh-duplicate](../../history/2026-01/202601080848_fix-subscription-pull-refresh-duplicate/) - 主题页：订阅面板配置行去除重复操作，移除“刷新”按钮，仅保留“拉取节点”
- [202601080900_fix-subscription-error-message-overflow](../../history/2026-01/202601080900_fix-subscription-error-message-overflow/) - 主题页：订阅面板同步失败错误信息单行省略显示，避免表格行高度异常
- [202601081053_fix-review-followups](../../history/2026-01/202601081053_fix-review-followups/) - 主题页：日志面板与 `updateCoreUI` 附近缩进一致性修复
- [202601081145_fix-review-report](../../history/2026-01/202601081145_fix-review-report/) - 主题页：订阅面板同步错误字段处理去冗余（去掉多余 `String()` 转换）
- [202601081339_fix-proxy-port-sync](../../history/2026-01/202601081339_fix-proxy-port-sync/) - 主题页：设置项 `proxy.port` 联动后端 `ProxyConfig.inboundPort`，端口变更自动重启并重应用系统代理；启动时从后端同步实际端口避免误导
