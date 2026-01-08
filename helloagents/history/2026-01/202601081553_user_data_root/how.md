# 技术设计: 统一用户目录数据存储

## 技术方案

### 核心技术
- **Go:** `os.UserConfigDir()`/`os.UserHomeDir()` + `filepath`（推导 `<userData>`），`os.Rename()` + 跨盘回退（copy+delete）
- **Electron:** `app.getPath('userData')`（开发/生产统一）

### 实现要点
1. **统一 `<userData>` 根目录（Go 侧）**
   - 新增 `shared.UserDataRoot()`：优先使用 `os.UserConfigDir()/Vea`，失败则回退到 `~/.vea`（仅作为极端兜底）。
   - 目标是与 Electron 默认 `userData` 路径一致：
     - Linux: `~/.config/Vea`
     - macOS: `~/Library/Application Support/Vea`
     - Windows: `%APPDATA%\\Vea`

2. **状态文件默认路径调整**
   - `main.go` 的 `--state` 默认值改为 `<userData>/data/state.json`（使用绝对路径，避免工作目录变化造成读写失败）。
   - 保留 `--state` 显式覆盖能力（主要用于测试/特殊调试），但文档不再建议使用仓库相对路径。

3. **ArtifactsRoot 固定到用户目录**
   - `shared.ArtifactsRoot` 统一为 `<userData>/artifacts`。
   - 移除“可执行文件旁 artifacts 可写则使用”的分支（不保留兼容路径）。
   - 迁移需要在任何写入发生前执行，因此初始化逻辑不得提前创建目标目录（避免阻塞 `rename` 迁移）。

4. **启动期迁移与清理（移动并删除源数据）**
   - 启动早期（在写日志、加载 state、写 artifacts 之前）执行迁移：
     - 旧 `./data/state.json` → 新 `<userData>/data/state.json`
     - 旧 `./artifacts/` → 新 `<userData>/artifacts`
   - 迁移策略：
     - **目标不存在:** 优先 `os.Rename()`；若返回 `EXDEV`（跨文件系统）则回退为“复制后删除源”。
     - **目标已存在:** 不覆盖目标；直接删除源（满足“不保留兼容/清理旧数据”的要求）。
   - 迁移源路径的判定（只处理“源程序目录”的遗留数据）：
     - `executableDir()/data/state.json`、`executableDir()/artifacts`
     - 若检测到当前工作目录为仓库根（存在 `go.mod` 等特征），额外处理 `cwd/data/state.json`、`cwd/artifacts`

5. **Electron 开发/生产统一**
   - `frontend/main.js` 无论是否打包，均使用 `<userData>/data/state.json`。
   - 启动前不再强制 `mkdir` 目标目录（让后端负责创建/迁移，避免提前创建导致迁移 `rename` 失败）。

## 架构决策 ADR
### ADR-001: 以用户目录作为数据唯一落点
**上下文:** 运行期数据落在仓库/可执行目录会导致权限问题、污染源代码目录，并造成开发/生产不一致。  
**决策:** 统一以 `<userData>` 作为数据根目录，`state.json` 与 `artifacts/` 均位于该目录；启动时迁移并删除旧数据，不保留旧路径兼容。  
**理由:** 行为一致、可写性更可靠、减少权限与升级问题。  
**替代方案:** 通过 `chdir(<userData>)` 继续使用相对路径 → 拒绝原因: 全局副作用大，调试成本高，且不显式。  
**影响:** 需要实现一次性迁移与冲突策略；文档与脚本需要同步更新。

## 安全与性能
- **安全:** 迁移逻辑只触达明确的遗留路径（源程序目录），避免误删用户其他目录；目标存在时不覆盖，降低误覆盖风险。
- **性能:** `os.Rename()` 为 O(1) 优先；仅在跨盘时才回退复制（可能较慢，但属于首次迁移一次性成本）。

## 测试与部署
- **测试:** 增加单测覆盖 `<userData>` 推导与迁移策略（目标存在/不存在、跨盘回退通过模拟或抽象接口验证）。
- **部署:** 纯本地路径策略变更，无需 API 迁移；发布说明需提示首次启动会迁移并清理旧目录。

