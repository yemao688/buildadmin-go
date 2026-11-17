<!-- 将本文件复制到业务仓库根目录的 AGENT_BUSINESS.md；框架源仓库永不发布根目录 AGENT_BUSINESS.md，因此框架升级时不会产生文件冲突。 -->

# 业务仓库协作说明

## 项目身份

- 业务名：<业务名>
- 本仓库是 <业务名> 的业务仓库，基于 buildadmin-go 框架 fork 开发。
- 框架上游：`git@github.com:yemao688/buildadmin-go.git`（发布分支 `v3`，作为 git remote `upstream`）。
- 业务主分支：`master`。
- AI 开发规则：先读 `AGENTS.md` 完成仓库身份自检，再读 `docs/framework-workflow.md`；CRUD 生成契约见 `docs/crud-generation.md`。
- 当前基于的框架版本：<如 v3.0.0>

## 业务概述

- 业务名称：
- 目标与范围：

## 业务模块清单（CRUD 生成模块）

- 模块、表名及对应 `crud_specs/*.yaml`：
- 模块负责人或特殊依赖：

## 生成后定制清单（定制点核对表）

CRUD 双 commit 工作流见 `AGENTS.md` 的“业务仓库中的框架使用最佳实践”：生成 commit 必须是纯生成器产物，业务定制一律独立 commit。重新生成前跑一次核对：

```bash
crud:check-customized --all          # 列出所有与全新渲染不一致的产物；--diff 看具体行
crud:check-customized <table>        # 单模块
```

**定制只放在手写位置**，不要改生成文件（生成器整文件重写）：

| 定制类型 | 放在哪里 | 备注 |
|---|---|---|
| 写成功后的副作用（缓存失效、通知、审计） | spec `hooks.afterMutation` 声明名字 + `internal/admin/handler/crud_hooks.go` 注册 | 重新生成不受影响；未注册会有启动告警 |
| 自定义接口 / action | 独立 handler 类型 + `internal/admin/router/custom_registrars.go` | 不写进生成的 handler/registrar |
| 生成文件的任何手改 | —— | 会被下次 `crud:up` 覆盖，并出现在 `check-customized` 列表里 |

| 模块 | 定制类型 | 名字/位置 | 对应 commit | 备注 |
|---|---|---|---|---|
| （示例）ops_banner | hook | `banner-cache` | `a1b2c3d` | 写成功后失效前台轮播缓存 |

## 业务专属规则/约定

- 权限、数据范围、命名和状态约定：
- `admin_id` 归属表：有 `crud_specs` 的表在启动时按 `dataScope.reassignable` / `inheritFrom` **自动登记**，无需手写；只有没有 spec 的手写表才需要在 `internal/admin/repository/admin_owner_tables_business.go` 补登记。`admin_log` 等操作者/审计表不得登记。
- 不应修改的业务边界：
- 常用体检：`diagnose`（安装/迁移/spec/归属/菜单/前端产物）、`crud:list --drift`、`migrate status`。

## 部署与环境笔记

- 开发、测试、生产环境及配置来源：
- 发布、迁移、备份和回滚注意事项：
