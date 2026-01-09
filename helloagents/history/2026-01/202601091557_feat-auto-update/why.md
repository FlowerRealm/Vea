# 变更提案: 应用内自动更新（Issue #24）

## 需求背景
当前 Vea 需要用户手动下载新版本并覆盖安装，缺少“检查更新/下载更新”的路径：
- 用户不容易获知新版本（安全修复/功能改进）
- 更新成本高，导致版本碎片化
- 误用第三方下载渠道存在安全风险

本需求仅要求“手动检查更新”，不做后台定时检查，避免无意的网络访问与打扰。

## 产品分析

### 目标用户与场景
- **用户群体:** 桌面端用户（Windows/macOS 为主），希望通过 GUI 管理代理配置并保持软件处于最新稳定版
- **使用场景:** 用户在「组件」面板点击“检查应用更新”，若有新版本则应用内下载并自动重启完成安装
- **核心痛点:** 不知道是否有新版本；下载安装步骤繁琐；不同平台更新方式不一致

### 价值主张与成功指标
- **价值主张:** 用最少交互完成安全更新：一键检查 → 自动下载 → 自动安装重启
- **成功指标:**
  - 用户可在 1 次点击内触发更新流程
  - 检测到更新后能在应用内完成下载与安装（支持的平台）
  - 更新来源固定为 GitHub Releases 最新稳定版（忽略 prerelease）

### 人文关怀
- 默认不做后台更新检查，避免在用户不知情时进行网络访问
- Linux deb 包不强行做自动更新，避免破坏系统包管理模型与提权风险
- 更新过程中尽量给出明确提示，避免“静默重启”造成误解

## 变更内容
1. 在「组件」面板增加“检查应用更新”入口（手动触发）
2. Electron 主进程集成 `electron-updater`，仅在可支持的平台启用自动更新（Windows/macOS）
3. Release 工作流补齐自动更新所需的元数据文件（`latest*.yml`、`*.blockmap`、mac 更新所需的 `zip`）
4. 更新流程：检测到新版本后自动下载，下载完成后自动安装并重启

## 影响范围
- **模块:**
  - `frontend/`（主进程、preload、主题页 UI）
  - `.github/workflows/`（Release 产物上传）
- **文件（预期）:**
  - `frontend/main.js`
  - `frontend/preload.js`
  - `frontend/theme/dark.html`
  - `frontend/theme/light.html`
  - `frontend/package.json`（新增运行时依赖）
  - `frontend/electron-builder.yml`（mac 追加 zip/配置 publish）
  - `.github/workflows/release.yml`（上传 update 元数据）
- **API:** 不新增 HTTP API；新增 IPC 通道用于“检查更新”和状态回传
- **数据:** 不新增持久化数据（不做“跳过此版本/稍后提醒”）

## 核心场景

### 需求: check-update
**模块:** frontend
在「组件」面板提供“检查应用更新”按钮。

#### 场景: click-check-update
用户点击“检查应用更新”按钮：
- 若当前平台不支持自动更新（如 Linux deb），提示“请前往 GitHub Releases 下载最新版”
- 若支持自动更新，开始检查最新稳定版（忽略 prerelease），并在状态栏提示进度

### 需求: download-install-restart
**模块:** frontend
支持的平台在发现新版本后自动下载、安装并重启。

#### 场景: update-available
检测到新版本：
- 自动开始下载并显示下载进度
- 下载完成后自动执行安装并重启应用

### 需求: no-update
**模块:** frontend
没有新版本时给出明确反馈。

#### 场景: update-not-available
未检测到新版本：
- 提示“已是最新版本”

## 风险评估
- **风险:** macOS 自动更新通常依赖签名/公证；若缺失可能导致安装/替换失败
  - **缓解:** 先实现机制并在文档注明发布要求；CI 后续可按需引入签名流程
- **风险:** Release 产物缺少 `latest*.yml`/`*.blockmap` 会导致更新失败
  - **缓解:** 调整 GitHub Actions 的 Release 打包上传清单，确保元数据随 Release 发布
- **风险:** GitHub API 访问受限/限流导致检查失败
  - **缓解:** 错误提示可重试；仅手动触发，不做后台轮询
