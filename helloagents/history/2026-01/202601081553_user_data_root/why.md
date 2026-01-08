# 变更提案: 统一用户目录数据存储

## 需求背景
- **现状:** Go 后端默认把状态写到 `data/state.json`；Electron 开发模式也把 `state.json` 放到项目根目录 `data/`；部分运行期目录还可能落在可执行文件旁 `artifacts/`。
- **问题:**
  - 污染仓库/安装目录，开发与生产行为不一致。
  - 容易触发权限问题（例如曾用 sudo/管理员运行后目录变成 root-owned）。
  - 数据位置不确定，排障、备份与迁移成本高。
- **目标:** 与 Electron `userData` 行为一致，所有数据统一落到用户目录。

## 变更内容
1. 后端引入统一的 `userData` 根目录（与 Electron `app.getPath('userData')` 对齐），默认状态文件位于 `<userData>/data/state.json`。
2. 后端 `ArtifactsRoot` 固定为 `<userData>/artifacts`（不再使用可执行文件旁/仓库内 `artifacts/`）。
3. 启动时自动迁移并清理旧路径（移动并删除源数据）：
   - 旧 `./data/state.json` → `<userData>/data/state.json`
   - 旧 `./artifacts/` → `<userData>/artifacts`
4. Electron 开发/生产模式统一：始终使用 `<userData>/data/state.json` 启动后端。
5. 更新文档与脚本，去除“仓库内 `data/` / `artifacts/` 为默认运行目录”的描述。

## 影响范围
- **模块:** `main.go`、`backend/service/shared/*`、`frontend/main.js`、相关文档与脚本
- **API:** 无变更
- **数据:** `state.json` 与 `artifacts/` 位置变更；首次启动会进行一次性迁移/清理

## 核心场景

### 需求: 后端数据统一在用户目录
**模块:** backend/main

#### 场景: Go 后端独立启动（无旧数据）
- 以默认参数启动后端时，不在仓库/可执行目录产生 `data/` 或 `artifacts/`
- 自动在 `<userData>/` 下创建必要目录并正常读写 `state.json`、运行期日志/内核产物

#### 场景: 发现旧仓库数据（存在 `./data/state.json` 或 `./artifacts/`）
- 首次启动时自动迁移到 `<userData>/` 并删除旧路径
- 迁移发生冲突（目标已存在）时遵循明确的冲突策略（见 how.md）

### 需求: Electron 开发/生产一致
**模块:** frontend

#### 场景: Electron 开发模式启动
- Electron 启动后端时传入的 `--state` 指向 `<userData>/data/state.json`
- 不再写入项目根目录 `data/`

## 风险评估
- **风险:** 迁移是破坏性的（会删除旧路径数据），处理不当可能导致数据丢失/覆盖
- **缓解:** 明确“目标已存在时不覆盖”的策略；迁移失败时给出可操作的错误信息（路径/原因/建议动作）

