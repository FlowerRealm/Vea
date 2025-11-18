# Vea 主题

本文件夹包含 Vea 的两个主题变体：

## 可用主题

- **dark.html** - 深色主题（默认）
  - 纯黑背景 + 蓝紫色渐变强调
  - 适合夜间使用，护眼

- **light.html** - 浅色主题
  - 纯白背景 + 蓝色强调
  - 适合白天使用，清爽明亮

## 如何切换主题

### 方式 1：修改 main.js（推荐）

编辑 `electron/main.js`，找到 `loadFile` 这一行，修改为你想要的主题：

```javascript
// 加载深色主题（默认）
mainWindow.loadFile(path.join(__dirname, 'renderer/theme/dark.html'))

// 或加载浅色主题
mainWindow.loadFile(path.join(__dirname, 'renderer/theme/light.html'))
```

### 方式 2：创建配置文件

在项目根目录创建 `.vea.json`：

```json
{
  "theme": "dark"  // 或 "light"
}
```

然后修改 `electron/main.js` 读取这个配置。

### 方式 3：环境变量

启动时指定主题：

```bash
VEA_THEME=dark make dev    # 深色
VEA_THEME=light make dev   # 浅色
```

## 自定义主题

如果你想创建自己的主题：

1. 复制 `dark.html` 或 `light.html`
2. 创建新的 CSS 文件（如 `theme-custom.css`）
3. 修改 HTML 中的 CSS 引用
4. 调整 CSS 变量（在 `:root` 中）

### CSS 变量说明

```css
:root {
  /* 背景颜色 */
  --bg-primary: #0a0a0a;      /* 主背景 */
  --bg-secondary: #1a1a1a;    /* 次要背景 */
  --bg-tertiary: #2a2a2a;     /* 三级背景 */

  /* 文字颜色 */
  --text-primary: #e5e7eb;    /* 主文字 */
  --text-secondary: #9ca3af;  /* 次要文字 */

  /* 强调色 */
  --accent-primary: #3b82f6;  /* 主强调色 */
  --accent-secondary: #8b5cf6; /* 次强调色 */

  /* 状态颜色 */
  --success: #10b981;
  --warning: #f59e0b;
  --error: #ef4444;
}
```

修改这些变量即可快速自定义主题外观。
