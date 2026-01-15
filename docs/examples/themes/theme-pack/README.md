# 主题包（manifest）示例

本目录用于演示“主题包（单包多子主题）”的 **manifest.json** 结构与推荐目录布局。

## 1. 如何使用

1. 确定 `packId`（必须满足：`[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}`），例如：`my-pack`
2. 将你的主题包放到：

```
<userData>/themes/my-pack/
```

3. 将本目录的 `manifest.json` 复制到主题包根目录，并按需修改
4. 在主题包内准备每个子主题的 `index.html`（`entry` 必须指向 `index.html`）
5. 启动 Vea，在「设置」→ `theme` 下拉中选择 `my-pack/dark` 或 `my-pack/light`

> 提示：最省事的方式是先在应用内导出 `dark`/`light` 主题 zip，解压后把对应目录内容放到主题包里，再改 CSS。

## 2. 推荐目录结构（可复用 _shared）

建议把共享脚本与第三方依赖放到主题包根目录的 `_shared/`，供多个子主题复用：

```
<userData>/themes/my-pack/
  manifest.json
  _shared/
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

子主题 `js/main.js` 可以参考内置主题的加载方式：
- `frontend/theme/dark/js/main.js`
- `frontend/theme/light/js/main.js`

它会按相对路径尝试加载：
- `../_shared/js/app.js`（子主题自带 `_shared`）
- `../../_shared/js/app.js`（主题包根目录 `_shared`）

## 3. manifest.json 规则要点

后端会严格校验：
- `schemaVersion` 必须为 `1`
- `themes[]` 不能为空
- `themes[i].id` 必须合法（不允许包含 `/`）
- `themes[i].entry` 必须是相对路径且以 `index.html` 结尾，禁止路径穿越

详见：`backend/service/theme/service.go`

