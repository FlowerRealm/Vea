# 主题（Themes）与自定义样式

本项目的 UI 以“主题包”为交付单元：每个主题是一个目录，入口为 `index.html`，并可通过应用内导入/导出 ZIP 安装。

> ⚠️ 安全提示：主题是 HTML/CSS/JavaScript，导入主题等同于运行第三方代码；只导入你信任的主题包。

## 1. 运行时目录与加载机制

### 1.1 userData 根目录（SSOT）

后端与 Electron 共享同一个 userData 根目录：

- Electron：`app.getPath('userData')`
- Backend：默认 `os.UserConfigDir()/Vea`，也可通过环境变量 `VEA_USER_DATA_DIR` 强制指定（与 Electron 对齐）

相关实现入口：
- `frontend/main.js`：启动后端时注入 `VEA_USER_DATA_DIR`，并在启动阶段计算 `userData/themes` 的入口
- `backend/service/shared/user_data.go`：`UserDataRoot()` 解析 `VEA_USER_DATA_DIR`

### 1.2 themes 根目录

所有主题统一位于：

```
<userData>/themes/
```

Electron **只从** `userData/themes/<entry>` 加载 UI（启动时与切换主题时一致）。

### 1.3 启动流程（与主题相关）

1. Electron 启动后端
2. Electron 初始化 `userData/themes/`（缺少内置主题时从 app resources 复制）
3. Electron 读取 `/settings/frontend` 的 `theme` 字段（默认 `dark`）
4. Electron 请求后端 `/themes` 列表，解析出目标主题的 `entry`
5. `BrowserWindow.loadFile(<userData>/themes/<entry>)` 加载主题入口 HTML

相关实现入口：
- `frontend/main.js`：`ensureBundledThemes()`、`loadFrontendThemeSetting()`、`resolveThemeEntryPath()`
- `frontend/theme_manager.js`：内置主题同步、marker/hash 判定、注入 `_shared`

## 2. 主题包形态

主题支持两种形态：**单主题**与**主题包（manifest，多子主题）**。

### 2.1 单主题（目录即主题）

目录结构（示例）：

```
<userData>/themes/<themeId>/
  index.html
  css/
    main.css
  js/
    main.js
    vea-sdk.esm.js
    settings-schema.js
    rule-templates.js
  _shared/                 # 可选：共享模块/第三方依赖
    js/app.js              # UI 逻辑入口（推荐复用内置）
    vendor/...
```

后端 `/themes` 会返回：

- `id = <themeId>`
- `entry = <themeId>/index.html`

`themeId` 限制：仅允许 `[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}`（字母数字开头，后续允许 `_`/`-`）。

### 2.2 主题包（manifest：单包多子主题）

当 `<userData>/themes/<packId>/manifest.json` 存在时，后端会把它当作“主题包”处理，并将子主题展开返回。

目录结构（示例）：

```
<userData>/themes/<packId>/
  manifest.json
  _shared/                 # 推荐：放在 pack 根目录，供多个子主题复用
    js/app.js
    vendor/...
  dark/
    index.html
    css/main.css
    js/main.js
  light/
    index.html
    css/main.css
    js/main.js
```

后端 `/themes` 会返回若干条 ThemeInfo（每个子主题一条）：

- `id = <packId>/<subThemeId>`
- `entry = <packId>/<...>/index.html`（由 manifest 的 `entry` 指定）
- `packId / packName / name` 用于 UI 展示

`manifest.json` 关键规则（以当前代码为准）：

- `schemaVersion` 必须为 `1`
- `themes[]` 不能为空
- 每个 `themes[i].id` 需满足单主题同样的 `themeId` 规则（注意：这里是子主题 id，不含 `/`）
- 每个 `themes[i].entry` 必须：
  - 相对路径（不能以 `/` 开头，不能含 `:`，不能含 `\\`）
  - 不能路径穿越（不能是 `..` 或 `../...`）
  - 必须以 `index.html` 结尾

参考实现：
- `backend/service/theme/service.go`：`themePackManifest`、`validateStagedThemeDir()`、`cleanManifestEntryPath()`

## 3. 主题入口与共享模块（_shared）

主题自身的 `js/main.js` 通常只做一件事：加载共享模块并调用 `bootstrapTheme()`。

内置主题的做法是按相对路径尝试加载（兼容单主题与主题包）：

- `../_shared/js/app.js`（单主题：`<themeId>/_shared`）
- `../../_shared/js/app.js`（主题包：`<packId>/_shared`）

对应实现可参考：
- `frontend/theme/dark/js/main.js`
- `frontend/theme/light/js/main.js`
- `frontend/theme/_shared/js/app.js`

## 4. 自定义主题：推荐工作流

### 4.1 最快：修改内置主题（适合本地自用）

内置主题在首次启动时会被复制到：

```
<userData>/themes/dark/
<userData>/themes/light/
```

直接修改这些目录下的 `css/main.css` / `index.html` 即可。

升级行为说明（避免“更新覆盖你的修改”）：
- `frontend/theme_manager.js` 会为内置主题写入 marker：`<themeDir>/.vea-bundled-theme.json`
- 若检测到你修改过内置主题（当前 hash 与 marker 的 bundledHash 不一致），后续版本升级将**不会覆盖**该主题目录
- 若旧版本没有 marker 且内容不一致，为避免 UI 无法升级，会先备份再覆盖（备份目录：`<userData>/themes/.vea-bundled-theme-backup/<themeId>/`）

### 4.2 可分发：导出 → 修改 → 重新导入（推荐）

1. 打开应用「设置」→ 找到 `theme` → 点击「导出当前主题(.zip)」
2. 解压 ZIP，修改：
   - 样式：`css/main.css`
   - 标题/布局（可选）：`index.html`
3. 重新打包 ZIP（必须保持“单顶层目录”，目录名即 `themeId`）
4. 「导入主题(.zip)」安装后，在 `theme` 下拉列表选择该主题

导入/导出接口（实现与文档一致）：
- `GET /themes`：列出已安装主题（包含 `entry`）
- `POST /themes/import`：上传 zip 导入
- `GET /themes/{id}/export`：导出主题 zip（主题包导出用 `packId`）
- `DELETE /themes/{id}`：删除主题（删除主题包同样用 `packId`）

详见 `docs/api/openapi.yaml` 与 `backend/api/router.go`。

## 5. ZIP 包格式约束（后端校验）

为避免路径穿越与解压炸弹，后端对主题 ZIP 做了严格校验：

- ZIP 必须且只能包含一个顶层目录（目录名即 `<themeId>` 或 `<packId>`）
- 顶层目录下必须存在 `index.html`（单主题）或 `manifest.json`（主题包）
- 拒绝符号链接、路径穿越、过深目录、文件数/解压总大小超限

限制参数默认值（代码常量）：
- 最大 ZIP：50 MiB
- 最大解压总量：200 MiB
- 最大文件数：4000
- 最大目录深度：24

参考实现：
- `backend/service/theme/service.go`：`ImportZip()`、`extractZipInto()`、`DefaultMaxZipBytes` 等常量

