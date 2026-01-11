# 任务清单: 应用内自动更新（Issue #24）

目录: `helloagents/plan/202601091557_feat-auto-update/`

---

## 1. Electron 主进程（更新引擎）
- [√] 1.1 在 `frontend/main.js` 集成 `electron-updater` 与基础事件转发，验证 why.md#需求-check-update-场景-click-check-update
- [√] 1.2 在 `frontend/main.js` 实现下载完成后自动安装重启（可延迟提示），验证 why.md#需求-download-install-restart-场景-update-available
- [√] 1.3 在 `frontend/main.js` 增加平台/打包态判断：Linux deb 返回“不支持自动更新”，验证 why.md#需求-check-update-场景-click-check-update

## 2. Preload（安全暴露 IPC）
- [√] 2.1 在 `frontend/preload.js` 通过 `contextBridge` 暴露 `checkForUpdates()` 与事件订阅能力，供主题页调用，验证 why.md#需求-check-update-场景-click-check-update

## 3. 主题页 UI（入口与状态反馈）
- [√] 3.1 在 `frontend/theme/dark.html` 增加“检查应用更新”按钮并绑定事件，验证 why.md#需求-check-update-场景-click-check-update
- [√] 3.2 在 `frontend/theme/light.html` 同步增加“检查应用更新”按钮并绑定事件，验证 why.md#需求-check-update-场景-click-check-update
- [√] 3.3 在主题页监听主进程更新事件，展示“检查/下载进度/已是最新/错误/即将重启”等提示，验证 why.md#需求-no-update-场景-update-not-available 与 why.md#需求-download-install-restart-场景-update-available

## 4. 打包与 Release 产物（自动更新元数据）
- [√] 4.1 在 `frontend/package.json` 增加运行时依赖 `electron-updater`（并同步 lockfile），确保打包后可用
- [√] 4.2 在 `frontend/electron-builder.yml` 为 macOS 增加 `zip` 目标，并配置 GitHub publish 信息以生成 `app-update.yml`
- [√] 4.3 在 `.github/workflows/release.yml` 上传 `latest*.yml`、`*.blockmap`、mac 的 `zip` 等更新必须文件到 Release

## 5. 安全检查
- [√] 5.1 执行安全检查（按G9：不引入明文 token、不做后台轮询、忽略 prerelease、Linux 不做自动安装、错误信息不泄露敏感路径）

## 6. 文档更新（知识库同步）
- [√] 6.1 更新 `helloagents/wiki/modules/frontend.md`，补充“应用自更新”能力与变更历史索引
- [√] 6.2 更新 `helloagents/wiki/overview.md`（范围内能力补充：应用自更新）
- [√] 6.3 更新 `helloagents/CHANGELOG.md`（Unreleased：新增应用内更新功能）

## 7. 测试
- [√] 7.1 本地执行 `go test ./...`（确保后端不受影响）
- [√] 7.2 本地执行 `cd frontend && npm run build`（验证 electron-builder 能产出更新元数据文件）
