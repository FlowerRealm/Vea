# 任务清单: 订阅创建后台拉取

目录: `helloagents/history/2026-01/202601091540_fix-subscription-create-async/`

---

## 1. 后端: 创建订阅非阻塞
- [√] 1.1 调整 `backend/service/config/service.go`：Create 仅创建记录，后台 goroutine 执行 Sync 拉取/解析/同步
- [√] 1.2 更新 `backend/service/config/service_test.go`：新增“Create 不等待下载”断言；其他用例改为等待后台同步完成

## 2. 前端/SDK: 状态与提示
- [√] 2.1 主题页：添加订阅成功提示“后台拉取中…”，并延迟刷新数据（configs/frouters/nodes）
- [√] 2.2 主题页：配置列表未同步时显示“未同步”
- [√] 2.3 SDK：`formatTime` 对零值/epoch<=0 显示 `-`

## 3. 测试
- [√] 3.1 执行 `go test ./...`

## 4. 文档更新
- [√] 4.1 更新 `helloagents/CHANGELOG.md`、`helloagents/wiki/modules/backend.md`、`helloagents/wiki/modules/frontend.md`、`helloagents/history/index.md`

