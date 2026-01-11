# 技术设计: 主题包支持 manifest（单包多子主题、隔离无冲突）

## 技术方案

### 核心技术
- **后端:** Go（`archive/zip`）、Gin、复用 `backend/service/shared.SafeJoin` 进行安全路径拼接
- **前端:** 主题目录（HTML + CSS + JS），通过 REST API 获取主题列表与入口路径
- **客户端:** Electron 主进程负责启动时解析主题入口并加载 `file://` 页面

### 实现要点
- 在 `<userData>/themes/` 下支持两种形态并保持兼容：
  1) **单主题目录:** `<themes>/<themeId>/index.html`
  2) **主题包目录:** `<themes>/<packId>/manifest.json` + 多个子主题目录（入口由 manifest 指定）
- `GET /themes` 返回“可切换主题列表”（扁平化），每个主题项包含：
  - `id`: 虚拟主题ID（建议格式 `packId/subThemeId`；单主题仍为 `themeId`）
  - `entry`: 入口文件相对 `<themes>/` 的路径（如 `dark/index.html` 或 `myPack/dark/index.html`）
  - `packId`/`packName`/`name`（可选，用于 UI 展示与导出策略）
- `POST /themes/import` 保持“zip 单顶层目录”的安全约束：
  - 顶层目录下若存在 `index.html` → 视为单主题导入（现有行为）
  - 顶层目录下若存在 `manifest.json` → 视为主题包导入，校验 manifest 中每个子主题入口存在
- 前端“切换主题”不再假设 `../<themeId>/index.html`，而是：
  - 保存设置 `theme = <虚拟主题ID>`
  - 使用 `entry` 计算出正确的文件 URL 并跳转
  - 导出时：若当前主题来自主题包，则导出整个 pack（调用 `/themes/<packId>/export`）

## 架构设计

### 目录结构（运行时）
```
<userData>/
  themes/
    dark/
      index.html
      css/
      js/
      ...
    light/
      ...
    myPack/
      manifest.json
      dark/
        index.html
        css/
        js/
      light/
        index.html
        ...
```

### `manifest.json`（建议 schema）
说明：manifest 位于主题包根目录；子主题入口通过相对路径指向包内文件。

```json
{
  "schemaVersion": 1,
  "id": "myPack",
  "name": "My Theme Pack",
  "description": "同一设计语言的多套变体",
  "version": "1.0.0",
  "author": "someone",
  "homepage": "https://example.com",
  "license": "MIT",
  "themes": [
    {
      "id": "dark",
      "name": "深色",
      "description": "深色变体",
      "entry": "dark/index.html",
      "preview": "dark/preview.png"
    },
    {
      "id": "light",
      "name": "浅色",
      "description": "浅色变体",
      "entry": "light/index.html"
    }
  ],
  "defaultTheme": "dark"
}
```

校验规则（后端）：
- `schemaVersion` 必须为 `1`
- `themes` 非空；每项必须包含 `id` 与 `entry`
- `id` 建议使用与现有 `themeId` 相同的安全字符集（字母/数字/`_`/`-`）
- `entry` 必须是**安全相对路径**：
  - 不能是绝对路径、不能包含 `..`、不能包含 `:`、不能包含 `\\`
  - 必须以 `index.html` 结尾（防止指向任意文件）

## 架构决策 ADR

### ADR-001: `GET /themes` 返回扁平化“可切换主题列表”
**上下文:** UI 需要在一个 select 中展示所有可切换主题（包含主题包内子主题）。  
**决策:** 后端将主题包 manifest 展开为多条“可切换主题”，并返回 `entry` 供前端跳转。  
**理由:** 避免前端自己扫描文件系统；同时让 Electron 启动也可以复用同一套解析逻辑。  
**替代方案:** 前端/主进程自行解析 manifest 或硬编码相对路径 → 拒绝原因: 逻辑分散且容易出错。  
**影响:** `ThemeInfo` 需要新增可选字段（不破坏现有 JSON 兼容性）。

### ADR-002: 导出“当前主题”对主题包按 pack 粒度导出
**上下文:** 当前主题可能是主题包内的一个子主题。  
**决策:** UI 在导出时若检测当前主题属于 pack，则调用导出 pack 的接口（`/themes/<packId>/export`）。  
**理由:** 主题包作为分发单元更符合用户预期；避免导出单子主题后丢失 manifest 与其他变体。  
**替代方案:** 后端支持按虚拟主题ID导出单子主题 → 拒绝原因: 语义复杂、易产生“导入后不再隔离”的副作用。  
**影响:** 导出接口保持不变；前端需要根据 `packId` 选择导出对象。

## API 设计

### [GET] /themes
- **描述:** 列出可切换主题（包含主题包展开的子主题）
- **响应:** `{ themes: [{ id, hasIndex, updatedAt, entry?, name?, packId?, packName? }] }`
- **兼容性:** 保持 `id/hasIndex/updatedAt` 字段含义；新增字段为可选

### [POST] /themes/import
- **描述:** 导入主题 zip
- **响应:** `{ themeId, installed: true }`
  - 单主题导入：`themeId = <themeId>`
  - 主题包导入：`themeId = <packId>/<defaultThemeId>`（用于 UI 选中默认子主题）

### [GET] /themes/:id/export
- **描述:** 导出主题目录或主题包目录为 zip（顶层目录为 `:id/`）
- **备注:** 当 `:id` 为 packId 时，导出整个主题包

## 安全与性能
- **安全**
  - 继续复用现有 zip 安全解压：拒绝路径穿越、符号链接、过深目录、文件数/解压总大小超限
  - 新增 manifest 校验：严格验证 `entry` 为安全相对路径且指向 `index.html`
  - `GET /themes` 解析 manifest 失败时不崩溃：忽略该包/子主题并记录日志（必要时）
- **性能**
  - 主题列表仍只扫描 `<themes>/` 一级目录
  - manifest 解析与入口存在性校验仅针对包含 manifest 的目录，且通常文件数很小

## 测试与部署
- **测试**
  - Go 单测新增：主题包导入成功、manifest entry 路径穿越拒绝、`GET /themes` 展开结果包含 `entry/packId`
  - 运行 `go test ./...`
- **部署**
  - Electron 打包仍需包含 `frontend/theme/**` 作为内置主题来源
  - 运行时主题入口统一从 `userData/themes` 加载；主题包目录同样位于该路径下

