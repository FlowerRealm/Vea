# How: 订阅创建改为后台同步

## 后端
- `backend/service/config.Service.Create` 不再在请求链路内下载订阅内容与解析节点。
- 创建成功后启动后台 goroutine 调用 `Sync`：负责下载 payload、更新同步状态、解析并写入节点与订阅 FRouter。
- 当用户手动填写了 `payload` 且拉取失败时，后台会尝试用该 payload 兜底解析，避免“导入了但没有节点”。

## 前端与 SDK
- 主题页保存订阅成功后提示“后台拉取中…”，并做一次延迟刷新（configs/frouters/nodes），减少用户等待与困惑。
- 配置列表在未同步（零时间）时显示“未同步”，避免误导为“正常”。
- SDK `formatTime` 对零值/epoch<=0 的时间显示 `-`，避免出现 `0001-...` 这种无意义时间。

## 验证
- 更新并增强配置服务单测：验证 Create 不等待远端拉取，并等待后台同步完成后断言结果。
- 执行 `go test ./...`。

