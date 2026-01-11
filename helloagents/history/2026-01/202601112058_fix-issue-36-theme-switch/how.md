# 技术设计: 修复 Issue #36 主题切换失败（Windows）

## 技术方案

### 核心技术
- JavaScript（主题页内逻辑）
- URL 解析（`new URL()`）+ 路径标准化（`decodeURI`/`decodeURIComponent` + 分隔符归一）

### 实现要点
- 当前实现通过在 `window.location.href` 中查找 `'/themes/'` 来推导主题根路径与当前入口；该逻辑在 Windows 的 `file://` URL（可能包含编码的分隔符，例如 `%5C`）下容易失效，导致 `getThemesBaseHref()` 返回空字符串，从而 `resolveThemeHref()` 失败并报错。
- 方案是将“解析主题根/入口”的逻辑改为基于 `URL` 的 `pathname`：
  1. 用 `new URL(window.location.href)` 取 `pathname`
  2. 对 `pathname` 做解码并把 `\\` 归一成 `/`
  3. 在归一化后的路径中查找主题根标记（优先 `/themes/`，必要时兼容 `/theme/` 作为兜底）
  4. 用解析得到的“根路径”回写到 URL 对象中生成可用于 `new URL(rel, base)` 的 baseHref
- 同时对 `getCurrentThemeEntry()` 做同样的路径归一化，保证与后端 `/themes` 返回的 `entry`（统一使用 `/`）可比较。

## 安全与性能
- **安全:** 仅做 URL 解析与字符串处理，不引入外部依赖；不修改后端，也不增加新的可执行入口。
- **性能:** 仅在主题切换/初始化时执行，开销可忽略。

## 测试与部署
- **本地验证（Windows）:**
  - 启动应用后在设置中切换 `dark`/`light`，确认页面跳转成功。
  - 导入主题包后切换到子主题，再切回 `dark`/`light`，确认可回退。
  - 若仍失败，打开 DevTools 检查 `window.location.href` 形态（是否包含 `%5C`、是否包含 `/themes/`），用于进一步定位。
- **回归验证（其他平台）:** Linux/macOS 上执行同样的切换回归。
