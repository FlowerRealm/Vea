# Changelog

本文件记录项目所有重要变更。
格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/),
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 新增
- 增加核心组件卸载能力：新增 `POST /components/:id/uninstall`，并在前端组件面板提供“卸载”按钮（代理运行中会拒绝卸载正在使用的引擎）。

### 修复
- 修复 Linux 下 mihomo(Clash) TUN 在默认 MTU=9000 时可能出现“看起来全网断开”的问题：当检测到未自定义的默认 TUN 组合时，自动将 MTU 调整为 1500。
- 修复组件安装流程中 `.gz` 解压结果固定写入 `artifact.bin` 导致 mihomo 等单文件发行包安装不可靠的问题：改为使用 gzip header 中的原始文件名，并清理冗余归一化分支。
- 修复 clash 安装归一化过程中 `os.Chmod` 错误被忽略的问题：当无法设置可执行权限时，直接返回错误，避免后续运行时失败。
- 提取代理服务 TUN 默认值常量，避免 `applyConfigDefaults` 与默认判定逻辑重复导致的不一致风险。
- 前端主题抽取 `updateEngineSetting` 的公共刷新逻辑，减少重复代码并降低后续维护成本。
- 修复主题页切换内核引擎时禁用系统代理失败仍继续重启的问题：改为快速失败，避免旧代理被停止后系统代理仍指向旧进程导致网络中断。

## [0.0.1] - 2026-01-05

### 修复
- 修复 Linux 下 mihomo(Clash) TUN 模式可能出现“全网断网”的默认配置问题：对齐主流客户端的 TUN/DNS 默认值，并改进 DNS server 解析自举策略。
