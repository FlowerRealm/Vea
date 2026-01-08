# 任务清单: 统一用户目录数据存储

目录: `helloagents/history/2026-01/202601081553_user_data_root/`

---

## 1. 后端: 统一 userData 根目录与默认路径
- [√] 1.1 在 `backend/service/shared/` 中新增 `UserDataRoot` 与默认 state 路径生成函数，并将 `ArtifactsRoot` 固定为 `<userData>/artifacts`（不再使用可执行目录/仓库 artifacts）
- [√] 1.2 在 `main.go` 中将 `--state` 默认值改为 `<userData>/data/state.json`，并确保迁移逻辑在任何写入（尤其是 app.log）之前执行

## 2. 后端: 启动期迁移与清理（移动并删除源数据）
- [√] 2.1 实现遗留数据迁移：将 `./data/state.json`、`./artifacts/`（以及可执行目录旁同名路径）迁移到 `<userData>`，并按 how.md 定义的冲突策略删除源数据
- [√] 2.2 为迁移逻辑补充单元测试（覆盖：目标不存在 rename、目标已存在删除源、跨盘回退 copy+delete 的分支）

## 3. 前端: Electron 开发/生产统一 userData
- [√] 3.1 在 `frontend/main.js` 中移除开发模式写入项目根目录 `data/` 的逻辑，统一使用 `<userData>/data/state.json`
- [√] 3.2 调整启动前目录创建逻辑：不提前创建目标目录，避免阻塞后端 `rename` 迁移

## 4. 文档与脚本同步
- [√] 4.1 更新 `README.md` / `frontend/README.md` / `AGENTS.md` / `CLAUDE.md` / `helloagents/wiki/arch.md` 等提到 `data/state.json`、仓库 `data/`/`artifacts/` 的说明
- [√] 4.2 复核 `scripts/fix-perms.sh`：将其定位为“遗留仓库目录权限修复/清理”工具或更新为适配新路径

## 5. 安全检查
- [√] 5.1 检查迁移逻辑的路径边界与删除行为，确保只处理明确的遗留路径且不覆盖用户已有数据

## 6. 测试与验证
- [√] 6.1 执行 `go test ./...`（必要时修复与本改动直接相关的问题）
- [√] 6.2 手动冒烟：启动 Electron 开发模式与 Go 后端独立启动，确认仓库目录不再产生运行期数据
