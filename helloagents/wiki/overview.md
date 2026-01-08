# Vea

> 本文件包含项目级别的核心信息。详细模块文档见 `modules/`。

## 1. 项目概述

### 目标与背景
Vea 是一个以 **FRouter** 为一等操作单元的桌面代理工具：后端负责编译运行计划、生成不同内核（sing-box / mihomo）的运行配置，并启动内核；前端提供可视化配置与状态展示。

### 范围
- **范围内:** 节点/路由管理（FRouter/Node）、代理运行（含 TUN）、测速/延迟测量、组件下载与更新
- **范围外:** 自建服务端、复杂的订阅生态管理（仅作为输入源）

## 2. 模块索引

| 模块名称 | 职责 | 状态 | 文档 |
|---------|------|------|------|
| backend | HTTP API、配置编译、内核适配与进程管理 | 稳定/迭代中 | [backend](modules/backend.md) |
| frontend | Electron UI、主题、SDK | 稳定/迭代中 | [frontend](modules/frontend.md) |
| docs | API/设计文档 | 稳定 | [docs](modules/docs.md) |
| scripts | 构建/辅助脚本 | 稳定 | [scripts](modules/scripts.md) |

## 3. 快速链接
- [技术约定](../project.md)
- [架构设计](arch.md)
- [API 手册](api.md)
- [数据模型](data.md)
- [变更历史](../history/index.md)
