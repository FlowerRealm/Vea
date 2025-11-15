# API 版本管理策略

## 版本号规范

Vea Backend API 遵循 [语义化版本 2.0.0](https://semver.org/lang/zh-CN/) 规范：

```
v主版本号.次版本号.修订号
```

- **主版本号（Major）**：不兼容的 API 变更
- **次版本号（Minor）**：向后兼容的新增功能
- **修订号（Patch）**：向后兼容的问题修复

## 兼容性承诺

### v1.x.y 系列（当前版本）

在 v1.x.y 系列中，我们承诺：

#### ✅ 允许的变更（向后兼容）

1. **新增 API 端点**
   ```
   v1.0.0: GET /nodes
   v1.1.0: GET /nodes, POST /nodes/export  ← 新增端点
   ```

2. **新增可选的请求字段**
   ```json
   // v1.0.0
   { "name": "Tokyo" }

   // v1.1.0（新增 description 字段，可选）
   { "name": "Tokyo", "description": "Japan server" }
   ```

3. **新增响应字段**
   ```json
   // v1.0.0
   { "id": "123", "name": "Tokyo" }

   // v1.1.0（新增 region 字段）
   { "id": "123", "name": "Tokyo", "region": "ap-northeast-1" }
   ```

4. **新增枚举值**
   ```
   v1.0.0: protocol: vless | trojan | shadowsocks
   v1.1.0: protocol: vless | trojan | shadowsocks | vmess  ← 新增
   ```

5. **性能优化**
   - 查询速度提升
   - 资源消耗降低

6. **问题修复**
   - Bug 修复
   - 安全漏洞修复

#### ❌ 禁止的变更（破坏性变更）

1. **删除 API 端点**
   ```
   v1.0.0: GET /nodes
   v2.0.0: GET /nodes  ← 删除端点需要升级到 v2.0.0
   ```

2. **删除请求或响应字段**
   ```json
   // v1.0.0
   { "id": "123", "name": "Tokyo", "address": "1.2.3.4" }

   // ❌ 错误：删除 address 字段
   { "id": "123", "name": "Tokyo" }
   ```

3. **更改字段类型**
   ```json
   // v1.0.0
   { "port": 443 }  // 整数

   // ❌ 错误：改为字符串
   { "port": "443" }
   ```

4. **更改 HTTP 方法或路径**
   ```
   v1.0.0: POST /nodes
   v2.0.0: PUT /nodes  ← 需要升级到 v2.0.0
   ```

5. **更改端点语义**
   ```
   v1.0.0: POST /nodes/:id/ping  → 测试延迟
   v2.0.0: POST /nodes/:id/ping  → 重启节点  ← 语义变化
   ```

6. **删除枚举值**
   ```
   v1.0.0: protocol: vless | trojan | shadowsocks
   v2.0.0: protocol: vless | trojan  ← 删除 shadowsocks 需要升级到 v2.0.0
   ```

## 破坏性变更处理

### 发布新的主版本

当必须进行破坏性变更时：

1. **发布 v2.0.0**
2. **保留 v1 端点至少 6 个月**
   - 通过路径前缀区分：`/v1/nodes` vs `/v2/nodes`
   - 或通过 HTTP Header：`Accept: application/vnd.vea.v1+json`

3. **提供迁移指南**
   - 列出所有破坏性变更
   - 提供迁移示例代码
   - 标注废弃时间表

### 示例：v1 到 v2 迁移

```
项目根目录/
├── api/
│   ├── openapi-v1.yaml  ← v1 API 规范
│   ├── openapi-v2.yaml  ← v2 API 规范
│   └── MIGRATION-v1-to-v2.md  ← 迁移指南
```

**MIGRATION-v1-to-v2.md 示例**：
```markdown
# 从 v1 迁移到 v2

## 破坏性变更

### 1. 节点协议字段重命名
- **v1**: `protocol` (字符串)
- **v2**: `protocolType` (对象)

**v1 请求**：
json
{ "protocol": "vless" }


**v2 请求**：
json
{ "protocolType": { "name": "vless", "version": "1.0" } }


### 2. 删除的端点
- `GET /configs/legacy` → 已删除，使用 `GET /configs`
```

## 版本生命周期

### 支持阶段

| 版本 | 状态 | 新功能 | Bug 修复 | 安全更新 | 结束日期 |
|------|------|--------|----------|----------|----------|
| v1.x | 活跃 | ✅ | ✅ | ✅ | - |
| v2.x | 计划 | - | - | - | TBD |

### 废弃流程

1. **宣布废弃（Deprecated）**
   - 在文档中标注废弃
   - API 响应中添加 `Deprecation` HTTP Header
   - 提供替代方案

2. **维护期（6 个月）**
   - 继续提供 Bug 修复和安全更新
   - 不添加新功能

3. **移除（Removed）**
   - 停止支持旧版本
   - 返回 `410 Gone` 状态码

### 示例：废弃端点

```http
GET /nodes/legacy HTTP/1.1

HTTP/1.1 200 OK
Deprecation: Sun, 01 Jun 2025 00:00:00 GMT
Sunset: Sun, 01 Dec 2025 00:00:00 GMT
Link: </api/openapi.yaml#/paths/~1nodes>; rel="successor-version"
Warning: 299 - "This endpoint is deprecated and will be removed on 2025-12-01. Use GET /nodes instead."
```

## 客户端兼容性

### SDK 版本对应关系

| SDK 版本 | API 版本 | 兼容性 |
|----------|----------|--------|
| @vea/sdk@1.0.x | v1.0.x | ✅ 完全兼容 |
| @vea/sdk@1.1.x | v1.1.x | ⚠️ 向后兼容 v1.0.x |
| @vea/sdk@2.0.x | v2.0.x | ❌ 不兼容 v1.x |

### 推荐做法

#### 前端开发者

```javascript
// ✅ 推荐：使用语义化版本范围
{
  "dependencies": {
    "@vea/sdk": "^1.0.0"  // 允许 1.x.y 的任何版本
  }
}

// ❌ 不推荐：锁定具体版本
{
  "dependencies": {
    "@vea/sdk": "1.0.0"  // 无法获得向后兼容的更新
  }
}
```

#### API 调用者

```javascript
// ✅ 推荐：忽略未知字段
const node = await vea.nodes.get(id)
// 只使用需要的字段，忽略新增的字段

// ❌ 不推荐：依赖具体字段集合
Object.keys(node).forEach(key => {
  // 新增字段会导致意外行为
})
```

## API 文档版本化

### 文档结构

```
docs/
├── api/
│   ├── v1/
│   │   ├── openapi.yaml
│   │   ├── README.md
│   │   └── examples/
│   └── v2/
│       ├── openapi.yaml
│       ├── README.md
│       ├── examples/
│       └── MIGRATION.md
```

### 文档更新策略

- **v1.x 系列**：持续更新，添加新端点和字段
- **v2.x 系列**：新文件，独立维护
- **Changelog**：记录所有变更，标注版本号

## 错误处理

### 版本不匹配

```http
GET /v2/nodes HTTP/1.1

HTTP/1.1 404 Not Found
Content-Type: application/json

{
  "error": "API version v2 is not available. Current version: v1. See https://docs.vea.example/api/versioning"
}
```

### 字段验证失败

```http
POST /nodes HTTP/1.1
Content-Type: application/json

{
  "name": "Tokyo",
  "protocol": "unknown"  ← 无效的枚举值
}

HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "Invalid protocol 'unknown'. Allowed values: vless, trojan, shadowsocks, vmess"
}
```

## 监控与通知

### 版本使用统计

后端应记录 API 版本使用情况：

```go
// 中间件示例
func VersionTracker(c *gin.Context) {
    version := extractAPIVersion(c.Request)
    metrics.RecordAPIVersion(version)
    c.Next()
}
```

### 废弃警告

当调用废弃端点时，记录日志并通知用户：

```
[WARN] Deprecated API called: GET /nodes/legacy by client 192.168.1.100
[WARN] This endpoint will be removed on 2025-12-01
```

## 总结

**核心原则**：
1. **向后兼容优先** - v1.x 系列只增不减
2. **透明变更** - 所有变更记录在 CHANGELOG
3. **平滑迁移** - 破坏性变更提供迁移期和文档
4. **语义化版本** - 严格遵循 SemVer 规范

**用户承诺**：
- v1.x 系列保持向后兼容
- 破坏性变更至少提前 6 个月通知
- 提供清晰的迁移指南和示例代码
