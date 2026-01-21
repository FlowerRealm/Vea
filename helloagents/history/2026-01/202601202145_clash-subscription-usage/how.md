# 技术设计: Clash 订阅用量显示（已用/总量）

## 技术方案

### 核心技术
- 后端：在订阅下载响应中解析 `subscription-userinfo`（HTTP Header）并写入 `domain.Config`。
- 前端：订阅列表渲染时读取 `Config` 的用量字段，格式化为“已用/总量”展示。

### 实现要点
- 解析来源优先级（本次选择最小可靠路径）：
  1. HTTP 响应头 `subscription-userinfo`（大小写不敏感；Go `Header.Get` 已天然兼容）
  2. （可选兜底）payload 首行 `#` 注释携带 `upload/download/total`（仅当明确存在该生态约定时再做；本次先不强行引入）
- 解析策略：
  - 支持形如 `upload=123; download=456; total=789; expire=...` 的 KV 串
  - 分隔符同时兼容 `;` 与 `,`
  - key 统一转小写；仅识别 `upload/download/total`；忽略其他字段
  - value 只接受非负整数（十进制）；解析失败视为无效（不让同步失败）
- 计算策略：
  - `usedBytes = upload + download`
  - `totalBytes = total`
  - 若任一必要字段缺失/无效：本次不更新用量字段（避免把已有值覆盖成 0）

## API 设计
本次不新增 API；复用 `GET /configs` 返回的 `Config`，仅增加可选字段：
- `usageUsedBytes?: number`
- `usageTotalBytes?: number`

## 数据模型
在 `domain.Config` 中增加可选字段（指针用于表达“未知/未提供”）：
- `UsageUsedBytes *int64`
- `UsageTotalBytes *int64`

持久化：跟随现有 `state.json` 的 `Configs` 快照自然存储/加载；旧数据缺失字段时自动为 `nil`。

## 安全与性能
- **安全:** 用量字段不应写入敏感日志；解析失败不回显完整订阅 URL 或 header 原文（避免意外泄漏）。
- **性能:** 用量解析只发生在订阅同步 HTTP 响应路径上（O(n) 字符串解析），不引入额外网络请求，不增加轮询。

## 测试与部署
- **测试:**
  - 为 header 解析函数添加单元测试（覆盖正常/缺失/异常/混合分隔符）。
  - 在 `config.Service.Sync` 流程测试中模拟带 header 的下载响应，验证 `Config` 用量字段被更新。
- **部署:** 无需额外配置；随应用更新生效。

