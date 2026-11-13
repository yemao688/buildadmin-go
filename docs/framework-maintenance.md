# 框架维护指南

本文**只面向框架源仓库的维护者**（包括 AI agent）：`git@github.com:yemao688/buildadmin-go.git`，发布分支 `v3`。

**如果你在一个业务 fork 中开发，本文不适用你。** 业务开发的规则在 [`framework-workflow.md`](framework-workflow.md)；双方通用规则在根目录 [`AGENTS.md`](../AGENTS.md)。

## 术语

| 术语 | 含义 |
|---|---|
| 框架源仓库 / 框架上游 | `git@github.com:yemao688/buildadmin-go.git`，发布分支 `v3` |
| 业务仓库 / 下游 | 用户 fork 出的业务项目仓库，主分支 `master` |
| PHP 上游 | BuildAdmin PHP 原版项目，提供生态、接口兼容性和业务语义参考 |

全文档禁止裸用"上游"一词，必须带上述限定词。

## 框架发布流程

发版由 `release` 子命令完成前置校验与版本文件写入。该命令**不会** commit、不会创建或推送 tag——提交、打 tag 与 push 始终是人工最后一步；命令会把需要执行的命令原样打印出来。

1. 先人工在 `CHANGELOG.md` 增加 `## v<version>` 小节。版本号必须是 `MAJOR.MINOR.PATCH` 三段数字，且严格大于 `VERSION_FRAMEWORK` 的当前值。
2. 只读校验（dry-run）：

   ```bash
   go run ./cmd/server release --version 3.4.1
   ```

   逐项输出 `[ok]` / `[fail]` / `[skip]`，覆盖：版本递增、`VERSION_FRAMEWORK` 与 `version.Framework` 一致、`CHANGELOG.md` 小节存在、工作区（仅允许 `VERSION_FRAMEWORK`、`internal/pkg/version/version.go`、`CHANGELOG.md` 三个文件有改动）、`frame-v<version>` tag 不存在、`gofmt -l .` 无输出、`go build ./...` 通过。任一项失败即退出码 1，并列出人工下一步命令；`gofmt` / `go build` 在结构校验失败时标记为 skip。
3. 校验通过后写入版本文件：

   ```bash
   go run ./cmd/server release --version 3.4.1 --write
   ```

   只写根目录 `VERSION_FRAMEWORK` 与 `internal/pkg/version/version.go` 的 `version.Framework`；`CHANGELOG.md` 仍由人工维护，`version.Upstream` 只在同步 PHP 上游时人工更新（见第 5 条）。写入后如需复查，直接看 `git diff`；再次 dry-run 会因版本已写入而报「版本未递增」。
4. 人工提交并创建 annotated tag（以命令输出为准）：

   ```bash
   git add VERSION_FRAMEWORK internal/pkg/version/version.go CHANGELOG.md
   git commit -m "chore(release): 框架发布 3.4.1"
   git tag -a frame-v3.4.1 -m "frame-v3.4.1"
   git push origin v3
   git push origin frame-v3.4.1
   ```

   tag 名 = `frame-v` + `VERSION_FRAMEWORK` 的值（`frame-` 用于区分框架发布与业务仓库的 `v$VERSION` tag）。框架 tag 只作发布标记，不触发 `release-image.yml`——该流水线只认业务 `v<major>.<minor>...` tag，框架自身不发镜像。
5. PHP 上游基线版本的事实源是 `web/package.json`；同步 PHP 上游时，同时更新 `version.Upstream` 使其与之一致。

## PHP 上游同步原则

- 本框架把 PHP 上游的生态、接口兼容性和业务语义迁移到 Go，不是逐行翻译 PHP。
- 需要理解行为时先查 PHP 上游语义，再结合本仓库实现；不要盲抄 PHP。Go 代码以强类型、Gin/GORM、显式错误处理和仓库既有模式为准。
- 任何兼容性差异都必须配套测试、迁移或文档说明。
- `internal/migrations/official/` 跟随 PHP 上游更新；官方迁移身份（ID、名称、内容）一经发布**永不重写**，兼容问题只能用新增迁移解决。

### PHP 上游参考实现路径

PHP 上游源码以本地检出形式放在仓库根的 `.slim/`（该目录被 git 忽略，**只有框架维护者本地存在**，业务仓库没有）。对照 CRUD 生成器行为时查这些文件：

- PHP designer: `.slim/source/buildadmin/web/src/views/backend/crud/design.vue`
- PHP designer field defaults: `.slim/source/buildadmin/web/src/views/backend/crud/index.ts`
- PHP CRUD controller: `.slim/source/buildadmin/app/admin/controller/crud/Crud.php`
- PHP CRUD helper/default rules: `.slim/source/buildadmin/app/admin/library/crud/Helper.php`

## 迁移维护契约

三轨职责（完整契约见根目录 `AGENTS.md` 和 [`../internal/migrations/business/README.md`](../internal/migrations/business/README.md)）：

- `official/`：PHP 上游迁移与官方安装 seed，只跟随 PHP 上游同步，不做框架私有改动。
- `framework/`：框架自身唯一的 `framework-final-seed-and-integrity` 迁移，只由框架维护者修改；业务仓库禁止向此目录添加迁移。
- `business/`：下游扩展轨道，框架仓库自身不放业务表。

三张带配置前缀的台账分别为 `{prefix}migrations`、`{prefix}migrations_framework`、`{prefix}migrations_business`，统一使用 `version/migration_name/start_time/end_time/breakpoint` 五列；业务轨道不使用 `batch`/`revision`，也没有独立断点表。

新增 `/admin/*` 路由时必须二选一：(a) 通过迁移/种子登记 `admin_rule`（可授权），或 (b) 在路由注册器中用 `middleware.RegisterPermissionExempt` 声明豁免（对齐 PHP `noNeedPermission`）。当前安全 seed 覆盖 `auth/adminLog/del` 与 `routine/config/sendtestmail`。Authorization 与启动诊断只覆盖三段式 `/admin/<controller>/<action>` 路由，新增非三段式路由必须在评审中显式处理；启动 debug 模式会输出未登记也未豁免的路由告警。

迁移回调分为两类契约：`VerifyBaseline` 是应用迁移时的一次性基线契约，只在对应 `Up` 成功后执行；失败会随应用重试，已完成的迁移记录不再执行它，因此判据可以精确描述该迁移刚建立的基线。`VerifySchema` 与 `VerifyUpgradeData` 是 standing 运行时不变量，每次 migrate 与每次 serve 启动都会执行，判据必须兼容业务仓库在基线之上的合法改造。

迁移编排顺序必须保持不变：

```text
prefix validation → migration lock → upstream-compatible preflight
→ install/recovery decision
→ 非 strict-upgrade：fresh-snapshot AutoMigrate
  strict-upgrade：仅补建缺失的核心表（core.EnsureMissingCoreTables）
→ three ledger bootstrap/validation steps → official migrations（冻结 DDL）
→ reconciliation → official seed (fresh/recovery only)
→ framework seed（每次 migrate/serve 幂等补种）
→ framework migrations → business migrations
→ framework.VerifyCurrent → current schema validation
```

- 迁移 `Up` 函数必须幂等、前缀安全（`mysql.prefix` 可变，永不硬编码 `ba_`）、按业务键判重。
- `serve` 启动默认运行同一编排（`app.auto_migrate: apply`），因此每条迁移都必须保持幂等、可重跑；`VerifySchema`/`VerifyUpgradeData` 现在每次启动都执行，不只 `migrate`。
- 不得用表为空或 `id=1` 检查推断官方 seed 状态；seed 拥有的写入只有在官方 seed 之后才可靠，编排器保证它在 framework/business 的 `Up` 之前运行。
- 破坏性重命名、类型变更和回填不得依赖 AutoMigrate。
- **framework seed 常驻化**：`framework.EnsureSeed` 在每次 `migrate`/`serve` 都幂等执行 `finalSeedAndIntegrity`（安全规则会员行、国家菜单/语言、upload 配置），使 3.3.8 之后新增的 framework seed 能到达 framework 账本已完成的存量库。它必须在 `RunFrameworkMigrations` 之前运行：已记录迁移的 `VerifySchema`/`VerifyUpgradeData` 在 runner 内先于后续步骤执行，被删除的官方会员安全种子行（`security_data_recycle` id=5、`security_sensitive_data` id=2）要靠这次补种自愈，而不是让启动失败。行为影响：用户手动删除的 framework-owned 国家菜单/upload 配置会在下次启动被补回；已存在行一律不覆盖。
- **strict-upgrade 只补缺失核心表**：`core.EnsureMissingCoreTables` 遍历 `CoreModels()`，仅对 `Migrator().HasTable` 为 false 的表执行 `AutoMigrate`（并对补建表写表注释），绝不改动已存在表的结构。旧版本或缺 `country_*` 表的库因此不会在 framework seed/verify 阶段硬失败。业务塑形过的核心列不受影响。
- **official 迁移使用冻结 DDL**：已发布的 official 迁移不读取 `upload`/`model` 的 gorm tag，`ALTER TABLE ... MODIFY/ADD COLUMN ...` 在迁移内写死列定义（类型、默认值、注释），上游同步改 tag 不会悄悄改变历史迁移的行为；幂等检查（`HasColumn`）与历史回填逻辑保留。

## 账本与当前基线

- 官方迁移身份（ID、名称、内容）一经发布永不重写；兼容问题只能用新增迁移解决。
- 账户状态迁移由 `internal/migrations/framework/0001_final_seed_and_integrity.go` 及其 helper 负责，将历史账户值 `0/1` 转换为 `disable/enable`。
- 常驻 `VerifySchema` 只断言国家字典的表/列/索引与 `status ∈ (0,1)`，不要求「至少一门启用语言」：后台可以合法禁用全部语言，常驻检查不能因此让全站不可启动。
- 全新安装建立 24 张表，不包含 `test_build`、`admin_hierarchy_lock`、`user_group`、`user_rule`、`user_score_log`；管理员层级互斥使用事务内 `admin` 锚定行 `FOR UPDATE`，由 InnoDB 负责死锁检测并由 `innodb_lock_wait_timeout` 兜底超时。
- 业务仓库首次安装按当前快照建立三张带前缀的迁移台账；业务回滚只作用于 business 轨道，默认回滚最近一条已完成迁移，也支持 `--to-breakpoint`。
