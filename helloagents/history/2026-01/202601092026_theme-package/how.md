# 技术设计: 主题包（目录化 + ZIP 导入/导出）

## 技术方案

### 核心技术
- **后端:** Go（`archive/zip`），Gin（HTTP API），复用现有安全解压工具（`backend/service/shared`）
- **前端:** 主题目录（HTML + CSS + JS），通过 REST API 访问后端
- **客户端:** Electron 主进程负责从 userData 加载主题入口文件

### 实现要点
- 主题以“目录”为基本单位，入口固定为 `index.html`，样式固定放在 `css/`
- 采用“方案2（每个主题完全自包含）”：
  - 主题目录内包含自身运行所需的静态资源（例如 `css/`、`js/`、字体/图片、以及必要的运行脚本）
  - 主题导出时打包整个主题目录，导入时直接解压安装
- 主题存储位置使用 Electron `userData` 目录，确保可写、可被用户修改，且不污染安装目录

## 架构设计

### 目录结构（运行时）
以 `userData` 为根（与现有 `VEA_USER_DATA_DIR` 对齐）：

```
<userData>/
  themes/
    dark/
      index.html
      css/
      js/
      fonts/ (可选)
      ... (其他资源)
    light/
      ...
```

### 目录结构（仓库内置主题）
仓库内置主题同样采用目录结构，便于直接复制到 `userData/themes/`：

```
frontend/theme/
  dark/...
  light/...
```

Electron 启动时：
1. 确保 `<userData>/themes/` 存在
2. 若缺少内置主题目录，则从 app resources（`frontend/theme/`）复制到 `<userData>/themes/`
3. 读取后端前端设置 `theme`（默认 `dark`），加载 `<userData>/themes/<themeId>/index.html`

## 架构决策 ADR

### ADR-001: 主题包采用“自包含目录”而非“共享资源目录”
**上下文:** 主题需要可导入/导出并能被第三方分享；旧结构依赖 app 内部相对路径，容易在导入后失效。  
**决策:** 主题目录内包含其运行所需静态资源，导入/导出均以“整个目录”为单位。  
**理由:** 可移植、可分享、主题作者不需要了解 app 内部资源布局。  
**替代方案:** 共享资源目录（方案1）→ 拒绝原因: 主题与 app 内部目录强耦合，导入包不易自描述。  
**影响:** 内置主题可能出现资源重复（例如字体），但换取主题包的独立性与可移植性。

## API 设计

> 说明: 以下接口用于“主题包管理”；“当前主题”仍复用前端设置 `theme` 作为 SSOT。

### [GET] /themes
- **描述:** 列出已安装主题
- **响应:** `{ themes: [{ id, name?, hasIndex, updatedAt? }] }`

### [POST] /themes/import
- **描述:** 导入主题 zip 并安装到 `<userData>/themes/<themeId>/`
- **请求:** `multipart/form-data`，字段 `file`（`.zip`）
- **响应:** `{ themeId, installed: true }`
- **规则（建议）:**
  - 限制 zip 总大小（例如 50MB）
  - 解压安全：拒绝路径穿越；解压到临时目录后原子替换
  - 校验：必须包含 `index.html`；否则导入失败并清理临时文件

### [GET] /themes/:id/export
- **描述:** 导出指定主题为 zip
- **响应:** `application/zip`（附件下载）

### [DELETE] /themes/:id
- **描述:** 删除主题（可选；内置主题可禁止删除或删除后在下次启动自动恢复）

## 安全与性能
- **安全:**
  - 解压使用安全路径拼接，拒绝 `..`/绝对路径/非法分隔符
  - 限制上传体积与单次导入的文件数/深度（防 zip bomb）
  - 导入后校验 `index.html` 存在；加载失败回退默认主题
  - UI 提示：主题包包含可执行代码，仅导入可信来源
- **性能:**
  - 主题列表读取只扫描一级目录，避免递归遍历
  - 导入/导出使用流式读写（尽量避免一次性加载超大文件）

## 测试与部署
- **测试:**
  - Go 单测覆盖：主题导入校验、路径穿越拒绝、导出 zip 结构正确
  - 运行 `go test ./...`
- **部署:**
  - Electron 打包需包含 `frontend/theme/**` 作为内置主题来源
  - 运行时主题统一从 userData 加载，避免写入安装目录
