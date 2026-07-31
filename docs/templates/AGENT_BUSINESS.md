<!-- 将本文件复制到业务仓库根目录的 AGENT_BUSINESS.md；框架源仓库永不发布根目录 AGENT_BUSINESS.md，因此框架升级时不会产生文件冲突。 -->

# 业务仓库协作说明

## 项目身份

- 业务名：<业务名>
- 本仓库是 <业务名> 的业务仓库，基于 buildadmin-go 框架 fork 开发。
- 框架上游：`git@github.com:yemao688/buildadmin-go.git`（发布分支 `v2`，作为 git remote `upstream`）。
- 业务主分支：`master`。
- AI 开发规则：先读 `AGENTS.md` 完成仓库身份自检，再读 `docs/framework-workflow.md`；CRUD 生成契约见 `docs/crud-generation.md`。
- 当前基于的框架版本：<如 v2.0.0>

## 业务概述

- 业务名称：
- 目标与范围：

## 业务模块清单（CRUD 生成模块）

- 模块、表名及对应 `crud_specs/*.yaml`：
- 模块负责人或特殊依赖：

## 生成后定制清单（regenerate 核对表）

CRUD 双 commit 工作流见 `AGENTS.md` 的“业务仓库中的框架使用最佳实践”：生成 commit 必须是纯生成器产物，业务定制一律独立 commit。重新生成模块时，重跑 `crud:generate` 后对照本表逐条回补被覆盖的定制（每次回补同样单独 commit）。

| 模块 | 定制点 | 对应 commit | 备注 |
|---|---|---|---|
| （示例）ops_banner | 列表接口委托审批服务 | `a1b2c3d` | 回补时确认生成器新版字段顺序 |

## 业务专属规则/约定

- 权限、数据范围、命名和状态约定：
- 不应修改的业务边界：

## 部署与环境笔记

- 开发、测试、生产环境及配置来源：
- 发布、迁移、备份和回滚注意事项：
