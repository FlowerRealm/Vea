# scripts 模块

## 职责
- 打包/构建/权限与辅助脚本

## 注意事项
- Linux 下 TUN/系统代理可能需要额外权限，相关脚本与说明在 `scripts/` 与后端 `shared/` 中实现。
- `make dev` 默认不主动清理旧进程，避免误杀其他 Electron 程序；如需清理旧的 Vea 后端/释放 19080 端口，可运行 `make dev KILL_OLD=1`。
- 打包：`make build` 仅收集安装包到 `release/`；应用自更新资源由 CI 部署到 GitHub Pages（`/updates/`），不再使用 `release-updates/`。
