# 技术设计: 修复槽位功能不可用（Issue #29）

## 技术方案

### 核心技术
- 前端主题页：`frontend/theme/*.html`（原生 JS + DOM）
- 后端图接口：`GET/PUT /frouters/:id/graph`（包含 `slots` 字段）

### 实现要点

1. **槽位管理 UI（前端）**
   - 在 `panel-chain` 的 header/toolbar 增加“槽位管理”按钮。
   - 新增槽位管理弹窗（Modal），展示当前 slots 列表，每行包含：
     - 槽位名称（可编辑）
     - 绑定节点选择（下拉：未绑定 + nodes 列表）
     - （可选）删除按钮（如实现需引用检查与二次确认）
   - 新增“添加槽位”按钮：自动生成新 slot id（`slot-${max+1}`），默认名称如“槽位 N”。

2. **与现有 ChainListEditor 状态打通**
   - `ChainListEditor` 已维护 `this.slots` 并在 `saveGraph()` 中提交 `slots`；补齐对 `this.slots` 的编辑入口与变更后刷新：
     - 槽位管理弹窗保存/关闭时，更新 `this.slots`，调用 `markDirty()` 并触发 `render()`。
     - 若规则编辑弹窗当前打开，需要刷新 `#rule-to` 与链式 hop 下拉选项的 slot 显示（避免名称/绑定状态不同步）。
   - 保存图配置时同时回传 `positions`，避免路由规则面板保存导致布局数据被意外清空。

3. **后端与数据结构**
   - 后端已支持 slots 的读写：`frouterGraphRequest/Response` 含 `Slots []domain.SlotNode`，`saveFRouterGraph` 会校验并持久化。
   - 本变更不新增 API，不修改数据模型；仅确保前端按既有字段读写 `id/name/boundNodeId`。

## 架构设计
无架构变更（仅补齐 UI 管理入口与状态联动）。

## API设计
无新增接口，复用既有图接口：
- `GET /frouters/:id/graph`：读取 edges/positions/slots
- `PUT /frouters/:id/graph`：提交 edges/positions/slots

## 安全与性能
- **安全:** 仅在前端编辑字符串/选择项；提交前做基本校验（slot id 前缀、去重、boundNodeId 必须存在或为空），避免提交无效图导致后端 400。
- **性能:** slots 数量通常很小；刷新下拉与列表属于轻量 DOM 操作。

## 测试与部署
- **测试（手工验证为主）:**
  1. 打开“路由规则”面板 → “槽位管理” → 新增槽位、改名、绑定/解绑 → 保存 → 重新打开确认持久化。
  2. 规则编辑弹窗选择槽位作为去向，保存并点击“保存”图配置，确认后端接受且无报错。
  3. 代理运行中修改槽位绑定并保存，确认后端返回 `X-Vea-Effects: proxy_restart_scheduled`（如适用），并最终生效。
