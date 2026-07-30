# 框架维护指南

本文**只面向框架源仓库的维护者**（包括 AI agent）：`git@github.com:yemao688/buildadmin-go.git`，发布分支 `v2`。

**如果你在一个业务 fork 中开发，本文不适用你。** 业务开发的规则在 [`framework-workflow.md`](framework-workflow.md)；双方通用规则在根目录 [`AGENTS.md`](../AGENTS.md)。

## 术语

| 术语 | 含义 |
|---|---|
| 框架源仓库 / 框架上游 | `git@github.com:yemao688/buildadmin-go.git`，发布分支 `v2` |
| 业务仓库 / 下游 | 用户 fork 出的业务项目仓库，主分支 `master` |
| PHP 上游 | BuildAdmin PHP 原版项目，提供生态、接口兼容性和业务语义参考 |

全文档禁止裸用"上游"一词，必须带上述限定词。

## 框架发布流程

1. 更新根目录 `VERSION_FRAMEWORK`、`app/pkg/version` 中的 `version.Framework` 与 `version.Upstream`（仅在同步 PHP 上游时更新），并补充 `CHANGELOG.md`。
2. 提交发布变更后创建带 `v` 前缀的 annotated tag，例如 `v2.0.0`。
3. PHP 上游基线版本的事实源是 `web/package.json` 与 `composer.json`；同步 PHP 上游时，同时更新 `version.Upstream`。

## PHP 上游同步原则

- 本框架把 PHP 上游的生态、接口兼容性和业务语义迁移到 Go，不是逐行翻译 PHP。
- 需要理解行为时先查 PHP 上游语义，再结合本仓库实现；不要盲抄 PHP。Go 代码以强类型、Gin/GORM、显式错误处理和仓库既有模式为准。
- 任何兼容性差异都必须配套测试、迁移或文档说明。
- `database/migrations/official/` 跟随 PHP 上游更新；官方迁移身份（ID、名称、内容）一经发布**永不重写**，兼容问题只能用新增迁移解决。

### PHP 上游参考实现路径

PHP 上游源码以本地检出形式放在仓库根的 `.slim/`（该目录被 git 忽略，**只有框架维护者本地存在**，业务仓库没有）。对照 CRUD 生成器行为时查这些文件：

- PHP designer: `.slim/source/buildadmin/web/src/views/backend/crud/design.vue`
- PHP designer field defaults: `.slim/source/buildadmin/web/src/views/backend/crud/index.ts`
- PHP CRUD controller: `.slim/source/buildadmin/app/admin/controller/crud/Crud.php`
- PHP CRUD helper/default rules: `.slim/source/buildadmin/app/admin/library/crud/Helper.php`

## 迁移维护契约

三轨职责（完整契约见根目录 `AGENTS.md` 和 [`../database/migrations/business/README.md`](../database/migrations/business/README.md)）：

- `official/`：PHP 上游迁移与官方安装 seed，只跟随 PHP 上游同步，不做框架私有改动。
- `local/`：框架自身的 6 条语义迁移，只由框架维护者修改；业务仓库禁止向此目录添加迁移。
- `business/`：下游扩展轨道，框架仓库自身不放业务表。

新增 `/admin/*` 路由时必须二选一：(a) 通过迁移/种子登记 `admin_rule`（可授权），或 (b) 在路由注册器中用 `middleware.RegisterPermissionExempt` 声明豁免（对齐 PHP `noNeedPermission`）；启动 debug 模式会输出未登记也未豁免的路由告警。

迁移回调分为两类契约：`VerifyBaseline` 是应用迁移时的一次性基线契约，只在对应 `Up` 成功后执行；失败会随应用重试，已完成的迁移记录不再执行它，因此判据可以精确描述该迁移刚建立的基线。`VerifySchema` 与 `VerifyUpgradeData` 是 standing 运行时不变量，每次 migrate 都会执行，判据必须兼容业务仓库在基线之上的合法改造。

迁移编排顺序必须保持不变：

```text
prefix validation → migration lock → upstream-compatible preflight
→ install/recovery decision → fresh-snapshot AutoMigrate
→ three ledger bootstrap/validation steps → official migrations
→ reconciliation → official seed (fresh/recovery only)
→ local migrations → business migrations
→ local.VerifyCurrent → current schema validation
```

- 迁移 `Up` 函数必须幂等、前缀安全（`mysql.prefix` 可变，永不硬编码 `ba_`）、按业务键判重。
- 不得用表为空或 `id=1` 检查推断官方 seed 状态；seed 拥有的写入只有在官方 seed 之后才可靠，编排器保证它在 local/business 的 `Up` 之前运行。
- 破坏性重命名、类型变更和回填不得依赖 AutoMigrate。

## Epoch reset 与账本历史

以下事实**只对框架源仓库的开发数据库成立**，不得推广到任何业务仓库环境：

- 框架开发数据库已经历批准的 epoch reset；local 账本重建过一次，并由 `go_migrations` 更名为 `local_migrations`。
- 官方迁移身份始终保持不可变。
- 账户状态迁移由 `database/migrations/local/0001.go` 及其 helper 负责，将历史账户值 `0/1` 转换为 `disable/enable`。

业务仓库的首次安装会按当前快照正常建立全部账本，不存在"需要补做 epoch reset"的情况。
