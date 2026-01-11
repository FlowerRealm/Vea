# 技术设计: 修复槽位功能不可用（Issue #40）

## 技术方案

### 核心技术
- 主题页：`frontend/theme/*`（原生 HTML/CSS/JS）
- 后端接口：`GET/PUT /frouters/:id/graph`、`POST /frouters/:id/graph/validate`
- 数据结构：`ChainProxySettings.slots`（`backend/domain/entities.go` 的 `SlotNode`）

### 实现要点

1. **槽位管理 UI（前端）**
   - 在 `panel-chain` 的 header actions 增加“槽位管理”按钮。
   - 增加槽位管理弹窗（复用现有 dialog/modal 样式），展示当前 slots 列表，每行包含：
     - 槽位名称（可编辑）
     - 绑定节点选择（下拉：未绑定 + 节点列表；未知绑定需显示为“未知”并允许解绑）
     - 槽位 ID（只读展示，便于定位/排错）
   - 增加“添加槽位”按钮：生成新 slot id（优先 `slot-{maxNumeric+1}`，避免与已有 id 冲突）。

2. **状态管理与刷新**
   - 槽位变更时更新 `ChainListEditor.slots`，调用 `markDirty()` 并触发 `render()`。
   - 若规则编辑弹窗已打开，需要刷新 “去向/链路 hop” 下拉的槽位选项，避免名称/绑定状态不同步。

3. **保存与校验**
   - 保存图配置时确保 PUT payload 包含 `slots`（即便不变也带上，避免后端将 slots 置空）。
   - 提交前做基础校验：
     - slot id 非空、以 `slot-` 开头、去重
     - `boundNodeId` 为空或存在于 nodes 列表
   - 保存失败时展示清晰错误信息（沿用现有 `showStatus`）。

4. **主题一致性**
   - light/dark 两套主题都实现同样的入口与逻辑，避免“只修一个主题”的回归。

## API设计

### GET `/frouters/:id/graph`
- 使用返回的 `slots` 作为编辑基础数据源。

### PUT `/frouters/:id/graph`
- 请求体包含 `edges`、`positions`（如有）、`slots`。
- `slots` 结构：
  - `id`: string（`slot-*`）
  - `name`: string
  - `boundNodeId`: string（为空表示未绑定）

### POST `/frouters/:id/graph/validate`（可选）
- 作为前端“保存前校验”的增强路径（若当前 UI 已集成校验流程则复用）。

## 安全与性能

- **安全:** 所有用户可编辑字段（槽位名称）用于渲染时需走 `escapeHtml`；提交前校验 `slots`，避免构造非法图。
- **性能:** slot 列表规模小，渲染与刷新成本可忽略；避免在输入每个字符时触发全量网络请求。

## 测试与部署

- **手工回归（Windows 优先）:**
  1. 打开“路由规则”→“槽位管理”→新增槽位、改名、绑定/解绑→保存→重开确认持久化。
  2. 编辑规则去向选择槽位→保存→保存图配置，确认后端无 400。
  3. 绑定节点被删除/不可用时，槽位管理中显示“未知绑定”，可解绑并保存通过。
- **自动化（基础）:** `go test ./...`（主要保证后端接口/编译未被影响）。
