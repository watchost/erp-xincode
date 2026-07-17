# ERP企业管理系统 (erp-xincode)

> **著作权声明**：本软件著作权归 **zhouhouping** 所有。
> Copyright (c) 2026 zhouhouping. All Rights Reserved.
>
> 未经著作权人书面许可，任何单位或个人不得以任何形式复制、修改、传播、
> 出售或用于商业用途。侵权必究。

## 系统简介

基于 Go + PostgreSQL + Vue3 + Element Plus 技术栈开发的ERP企业管理系统，
涵盖采购、仓库、生产、财务、仪表等核心模块，支持硬件设备对接和LLM智能分析。

## 技术架构

- **后端**：Go 1.21 + Gin + GORM + PostgreSQL + Redis
- **前端**：Vue3 + Vite + Element Plus + ECharts
- **部署**：Docker Compose 容器化部署

## 核心模块

| 模块 | 功能 |
|------|------|
| 仪表页面 | LLM智能分析、数据概览 |
| 仓库作业 | 出入库扫码、库存管理 |
| 采购管理 | 采购订单、入库作业 |
| 生产管理 | 工单管理、生产领料 |
| 财务管理 | 成本核算、财务报表 |
| 主数据 | 物料、供应商、仓库管理 |
| 系统管理 | 用户、角色、权限控制 |

## 快速开始

```bash
# 克隆仓库
git clone https://github.com/watchost/erp-xincode.git

# 启动服务
docker compose up -d

# 访问系统
# 前端: http://localhost:8091
# 后端API: http://localhost:8090/api/v1
```

## 登录信息

- 账号：`admin`
- 密码：`admin123`

## 许可证

本软件采用专有许可证，详见 [LICENSE](LICENSE) 文件。

**警告**：本仓库代码仅供学习和参考，未经授权不得用于商业用途。
