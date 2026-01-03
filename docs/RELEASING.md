# 发布流程（GitHub Actions）

> 目标：用仓库内的 `Build and Release Electron App` workflow 发布新版本。

## 1) 准备

- 确保 `main` 最新：`git pull --ff-only`
- 本地跑一下后端测试（至少）：`go test ./...`
- 准备 release notes 文件：`docs/release-notes/<version>.md`

## 2) 触发 workflow

workflow: `.github/workflows/release.yml`

```bash
# 示例：发布 v2.1.0
gh workflow run release.yml \
  --ref main \
  -f version=v2.1.0 \
  -F release_notes=@docs/release-notes/v2.1.0.md
```

## 3) 观察构建状态

```bash
gh run list --workflow release.yml --limit 5
gh run watch --exit-status
```

## 4) 验证 Release

```bash
gh release view v2.1.0
```

