# Changelog

## v2.3.0

- **Added:** `crud:validate` 纯校验 CRUD spec，在提交前检查主键、关系字段、路径、远程文件和默认值契约，并对未登记路由控制器及非标准驼峰路径输出 warning；不连接数据库、不生成文件。
- **Added:** 生成器为每个模块的 model/handler 产出 `_custom.go` 一次性定制骨架：已存在时绝不覆盖，`crud:delete` 删除逐字节未改的骨架、保留已定制文件并输出 warning——与双 commit 工作流共存，业务定制多一层结构化落点。
- **Changed:** 含 `weigh` 字段的表在 spec 未显式指定 `defaultSortField` 时自动生成 `weigh,desc` 默认排序（对齐 PHP 上游习惯）；spec 显式配置优先。
- **Docs:** 扩展 CRUD 双 commit 工作流指南（`AGENTS.md` 业务最佳实践节）——生成 commit 必须是纯生成器产物并在 message 标注框架版本，业务定制一律独立 commit 并写明动机；重新生成时重跑生成器后 `git diff` 对照定制 commit 逐条回补；业务模板 `AGENT_BUSINESS.md` 新增"生成后定制清单"核对表作为 regenerate 时的回补清单。
- **Removed:** 删除 `app/admin/model/gorm_test.go`——无断言、硬编码 `root:root@localhost/buildadmin` 凭据的早期开发草稿（`TestBelong` 长期失败源）；其唯一触碰的表名行为已由 `table_name_test.go` 回归测试覆盖。
- **Fixed:** 修复去冗余前缀重命名引入的 GORM 表名回归——`UserMoneyLog→MoneyLog`、`UserScoreLog→ScoreLog`、`UserRule→Rule`、`UserGroup→Group`、`CrudLog→Log` 五个 struct 改名后，`.Model(&Struct{})` 驱动查询的表名被命名策略推导为不存在的 `money_log`/`score_log`/`rule`/`group`/`log`（真实表 `user_money_log` 等），对应后台列表/详情/删除 1146；现经 `TablerWithNamer` 按命名策略解析回真实表（前缀安全），并补 schema 断言回归测试。该回归随 v2.2.0 发布，仅影响 struct 驱动查询路径（List 走显式表名字符串未受影响）。
- **Breaking (test 门禁):** 移除 `BUILDADMIN_TEST_MYSQL_DSN` 环境变量；MySQL 集成测试改由 `config.yaml` 的 `mysql_test` 段驱动——开发机需自建一次性测试库、对账号授予该库及 `<库名>%` 通配权限（recovery 测试会动态创建 `<库名>_fresh_*` fixture 库）并置 `enabled: true`；未配置或禁用时测试统一提示并跳过。新增 `app/pkg/testutil`（`OpenMySQL`/`OpenFixtureDatabase`）统一门禁解析与 fixture 库管理，原约 30 处重复门禁全部收编。
- **Fixed:** 首次启动不再自动复制 `config.yaml`——`serve` 默认命令在配置缺失时以只读模板进入安装向导（`/install` 可访问），`config.yaml` 改由安装器在安装时创建；无配置文件的非 serve 命令与显式 `--conf` 缺失均明确报错，不再静默使用模板值。
- **Breaking (CRUD spec 契约):** `isCommonModel` 弃用并暂时禁用——非零值在生成/apply 时被拒绝（错误信息含指引），model 一律输出到 `app/admin/model`；历史 common model 模块的 `crud:delete` 不受影响。`app/common/model` 自此仅保留既有手写基础设施（`BaseModel`、`User` struct 等），不再接受任何新生成输出。

## v2.2.0

- **Breaking (CRUD 生成器命名约定):** 业务模块路径推导统一为蛇形实体约定。`generateRelativePath` 省略时兜底默认等于表名（规范仍要求显式写出，标准值即表名）；单段路径在第一个下划线处拆分——表 `ops_user_test_xxx` 生成 handler/model `ops/user_test_xxx.go`、视图 `ops/userTestXxx/`、路由 `ops.UserTestXxx`、菜单 `ops/userTestXxx`。路由保持"目录段小写 + 实体段 PascalCase"的既有形式（对齐 PHP 实际 URL 如 `/admin/country.LanguageContent/index`）：显式路径模块（含 `country.*`）重新生成后**路由不变**；仅旧版省略路径自动推导出的扁平路由（`countryLanguageContent` 形态）在重新生成时变为带命名空间形式（`country.LanguageContent`）。显式路径中蛇形末段的视图叶子从原样保留改为 lcfirst 驼峰化（`test_xxx` → `testXxx`）。旧的自动推导（Go 文件扁平落根目录、视图末两段合并驼峰、路由扁平无命名空间）已移除。
- **Breaking (核心模块导出符号重命名):** `app/admin/handler/user`、`app/admin/model/user`、`app/admin/handler/crud`、`app/admin/model/crud` 中冗余分类前缀已去除：`UserGroupHandler→GroupHandler`、`UserRuleHandler→RuleHandler`、`UserMoneyLogHandler→MoneyLogHandler`、`UserScoreLogHandler→ScoreLogHandler`、`CrudLogHandler→LogHandler` 及 model 层对应类型（`UserGroup→Group`、`UserRule→Rule`、`UserMoneyLog→MoneyLog`、`UserScoreLog→ScoreLog`、`CrudLog→Log` 与构造器）。路由字符串（`user.Group`、`crud.Log` 等）、admin_rule 菜单名、前端视图与 API URL 均不变；下游 fork 在 Go 代码中引用旧类型名需同步改名。
- **Breaking (model 包路径迁移):** 手写 model 按边界归位，旧包路径移除（无别名兼容层）：`app/common/model` 的 `AuthModel` → `app/common/member.Service`；`UserModel`/`UserMoneyLogModel`/`UserScoreLogModel` → `app/api/model/user`；scoped `AttachmentModel` → `app/admin/model/routine`；`UploadHelper`/`AliossStorage`/`Attachment` struct → `app/common/upload`；`AreaModel` → `app/common/area`；`country.Service` → `app/common/country`；api 侧的 `app/admin/validate` 用法 → `app/pkg/validator`；`app/internal/permissioncache` 与 `internal/advisorylock` → `app/pkg/` 同名包。`ConfigModel.GetKVByGroup/GetValueByName` 的共享读取 → `app/common/siteconfig.Service`。admin 会员 handler 改注 `MemberPermissionInvalidator` 小接口（Wire 绑定同一 `member.Service` 实例）。下游 fork 有相应 import 的需按此映射表更新。
- **Fixed:** 后台附件管理列表 500——`Attachment` 的 `Admin`/`User` 关联改为经全局命名策略解析到真实（带前缀）admin/user 表；此前按结构体名推导到不存在的 `attachment_admin`/`attachment_user` 表导致 MySQL 1146。含 sqlite 回归测试。
- **Fixed:** `UploadHelper` 并发竞态——改为无状态服务（请求文件与细目经 `UploadParams` 逐调用传入）；此前 Wire 单例持有请求级可变字段，admin 与 api 并发上传会互相覆盖。含 `-race` 竞态回归测试。
- **Fixed:** `crud:delete` 删除多模块共享包中的单模块时误摘整包 `ProviderSet` 导致 wire 失败；现仅在该包 provider.go 无存留条目时摘除，delete→regenerate 往返对共享文件字节级还原。
- **Added:** import 边界守护测试（`app/boundary_test.go`）：AST 扫描强制 `admin↔api` 禁止互引、`common` 禁引两侧 handler；现存违规以白名单棘轮管理（当前仅 1 条安装引导永久例外）。
- **Added:** 会员权限缓存失效接口化——admin 会员 handler 依赖 `MemberPermissionInvalidator`，后台修改会员/分组/规则后即时失效会员侧缓存（与 API 读取同一 `member.Service` 实例）。
- **Changed:** `app/common/model` 收缩为 `isCommonModel` 生成器兼容区；手写共享服务全部迁入按能力命名的 `app/common/<capability>` 包（member/upload/area/country/siteconfig）。
- **Changed:** `country_language_content` 以蛇形路径重新生成（`country/language_content.go`），路由、视图、菜单与表数据不变；内部工具文件蛇形统一（`treeT.go→tree_t.go`、`Lange.go→lang.go`）；web/ 忽略 tsc/vue-tsc 产物。

### Upgrade

1. 下游业务 fork 若 import 了迁移的包或引用了重命名的类型，按上述映射表机械替换后运行 `go build ./...` 核对；本版不提供别名兼容层。
2. 业务 CRUD spec 的 `generateRelativePath` 按新约定显式写为表名本身（如 `ops_user_test_xxx`）；`/`、`.` 分隔符仅在需要更深业务子目录时使用。
3. fork 自定义 handler 若注入了会员 `AuthModel`，改为注入 `*member.Service`；仅需失效会员权限缓存的场景改注 `MemberPermissionInvalidator`。

## v2.1.0

- 修复 migrations 包 4 个腐化的 MySQL E2E 测试（install/recovery/upgrade 家族）：`TestInstall`、`TestFreshSeedPendingRetryAfterOverlayFailure`、`TestUpstreamSecurityBaselineThenLocalOverlay`、`TestInstallRecoveryDecisionFourStates` 子测试。
- **Breaking (RBAC fail-closed):** 未登记 `admin_rule` 且未声明豁免的 `/admin/*` 路由现在返回 403（此前为告警放行），对齐 PHP 上游语义；超管 `*` 绕过不受影响。豁免通过 `middleware.RegisterPermissionExempt` 在路由注册器声明（ajax `*`、alioss callback、index index/logout、crud 辅助端点、crud/log index（另按上游语义在 handler 内手动检查 `crud/crud/index` 权限）、module state/dependentinstallcomplete）。
- **Fixed:** `uploadCompleted` 移植修复——从 `/admin/module/uploadCompleted`（空壳 stub，前端调用一直 404）移回 `/admin/crud.Crud/uploadCompleted`，并实现 PHP 的 `crud_log.sync` 条件更新语义。
- **Added:** debug 模式启动时输出“未登记 admin_rule 也未声明豁免”的后台路由告警清单。
- **Breaking (运行时目录布局):** 配置、静态资源和运行时目录已切换到新的根目录布局。配置加载无向后兼容回退，这是硬切换：不会再回退读取旧的 `conf/config.yaml`。
- **迁移配置：** 执行 `git mv conf/config.yaml config.yaml`（或手动移动）；配置模板现为根目录的 `config.example.yaml`。
- **迁移静态资源：** 执行 `git mv static public`。所有 HTTP URL 前缀保持不变，`/static/*` 现由 `public/` 提供服务。框架自带的 `fonts/`、`images/` 现位于 `public/static/` 下（与 URL 结构镜像）。
- 删除 `database/buildadmin.sql` 及其升级合约测试（该 SQL 仅为测试夹具；生产安装使用 AutoMigrate + Go 种子，不受影响）。
- **迁移运行时文件：** 执行 `git mv storage runtime`，并将已有配置中的 `log.root_dir` 改为 `runtime/logs`。上传文件现位于 `public/storage/`；如需保留历史上传，将 `storage/default` 等内容移入 `public/storage/`。
- **迁移部署配置：** Docker 入口改为 `--conf /app/config.yaml`，Compose 挂载改为 `./config.yaml` 与 `./runtime`。

## v2.0.2

- **Security:** API tokens are now bound to their account domain. Admin endpoints require an `admin`-type token and user endpoints a `user`-type token; a same-ID cross-domain token no longer authenticates (previously a frontend `user` token could act as an admin with the same ID). Token refresh is bound likewise (`admin-refresh` → `admin`, `user-refresh` → `user`).
- **Security:** every authenticated request now re-checks that the account exists and is `enable`; disabling an admin or user invalidates their sessions immediately instead of waiting for token expiry.
- **Security:** the shared QueryBuilder no longer passes the `order` parameter through as raw SQL. Sorting must match `field,asc|desc` with a strict identifier whitelist, and search field names are validated the same way; malformed values now return 400. (Behavior change: previously-accepted malformed `order` strings are rejected.)
- **Breaking (deploy):** `migrate` no longer applies `crud_specs/*.yaml` automatically by default. Set `crud.apply_on_migrate: true` to re-enable the v2.0.1 deploy loop. The explicit `crud:apply` command is unaffected.
- Fixed `migrate` / `crud:*` commands silently succeeding on failure: migration and apply errors now propagate to a non-zero process exit code.
- Fixed the country dictionary specs drifting from the framework migration baseline: they now declare `unsigned`, defaults and comments matching local migration 0005, and `country_language_content.type` is aligned to `varchar(30)` across spec, snapshot model, admin model, DTO and the frontend (including its `0=文本,1=富文本,2=图片` enum semantics and select widget). Applying the shipped specs after a fresh migrate is now a zero-diff no-op.
- Fixed the admin token refresh branch using `UserTokenKeepTime`; it now uses `AdminTokenKeepTime`.
- Removed the dead legacy `tests/` package (an always-failing login test against an empty router; no real coverage).
- The route snapshot golden file moved to `router/testdata/registered_routes.golden` with a header explaining its purpose and regeneration command.

### Upgrade

1. If your deployment relies on `migrate` auto-applying specs, set `crud.apply_on_migrate: true` in `conf/config.yaml`.
2. Clients sending non-standard `order` values on list endpoints will now receive 400; use `field,asc|desc` with plain column identifiers.
3. No action is needed for correctly issued tokens; only cross-domain token usage (which was a vulnerability) is rejected.

## v2.0.1

- **Breaking:** `app.env` now accepts only `debug` or `release` and drives `gin.SetMode`; the legacy `local` value is a startup config error. Recovery middleware now reads `gin.Mode()`, so `release` mode actually hides internal error details.
- **Breaking (country dictionary modules):** the country modules were regenerated with the `generateRelativePath` subdirectory layout. Go code moved from the flat `app/admin/handler` / `app/admin/model` packages into `app/admin/handler/country` / `app/admin/model/country` (e.g. `handler.CountryCurrencyHandler` → `country.CurrencyHandler`), and their HTTP routes changed from `/admin/countryCurrency/*` (and `countryLanguage*`) to the framework-wide dotted form `/admin/country.Currency/*`, aligning with the seeded `admin_rule` button names (`country/currency/...`). The framework's own admin pages and menus are already migrated; database menu rows seeded by local migration 0005 are unchanged.
- Added the `crud:apply` deployment command: CRUD specs become the source of truth for business table structure; `apply` idempotently syncs table schemas, menus, and `crud_log` adoption, and runs automatically at the tail of `migrate` so deployments stay `git pull && migrate`. Primary-key drift is refused with a pointer to business migrations, and `--allow-rebuild` is limited to disposable environments. See [`docs/crud-generation.md`](docs/crud-generation.md).
- Added a vite-style startup banner (Local/Network URLs and mode) printed after the listener is bound synchronously.
- Added `docker-compose.dev.yml` for building the image from local source; the compose service and image were renamed `app` → `buildadmin-go` (`DEPLOY_IMAGE_NAME` default updated).
- Fixed a remote panic in the public click-captcha endpoint: malformed coordinate payloads are now rejected as normal verification failures.
- Fixed permission caching: caches are now instance-owned and mutex-synchronized, and are invalidated after every rule/group/assignment mutation via after-commit hooks, so permission changes take effect immediately and rollbacks cannot leave stale invalidations.
- Fixed the shared QueryBuilder pagination bug: the offset was computed from the default limit before a custom limit was applied (`page=2&limit=20` now correctly yields offset 20).
- Fixed CRUD generator subdirectory packages (`generateRelativePath`) end-to-end: provider wiring, registrar type qualification, wire ProviderSet aggregation (generate/delete symmetry), subpackage model/handler naming, and `crud:delete` class-name derivation.
- Fixed `crud:delete` to be failure-safe: AST-based provider/registrar removal, a go/parser guard before the wire/build steps, and menu rule deletion moved after the build so DDL stays the last mutable step.
- Fixed the CRUD log status lifecycle: comment truncation for MySQL strict mode, stale `start` records reconciled as interrupted on the next generation lock, empty scaffold/directory pruning after delete, qualified registrar matching, and the mysql prefix hidden in the log table display.
- Fixed `RegisteredRoutes` collection to replace the snapshot atomically on each route collection instead of appending across router reinitializations.
- Improved Sortable performance: one ordered batch read and a single scoped CASE update replace per-row reads/updates (n+3 reads + n+1/n+2 writes → 3 reads + 2 writes).
- `make frontend` now syncs every top-level `web/dist` entry (`favicon.ico` was previously dropped), skips `pnpm install` when the lockfile is unchanged, and falls back to plain `pnpm install` when the lockfile is absent; the router now serves `/favicon.ico`.
- Internal: consolidated the duplicated QueryBuilder into `app/pkg/querybuilder` (type aliases keep existing and generated callers compiling), extracted the shared permission cache into `app/internal/permissioncache`, completed the terminal auth-model dependency inversion (`terminal.AuthModel`), consolidated the core schema inventory into a single ordered source, and dropped the `common` package's dependency on admin models.

### Upgrade

1. **`config.yaml` (required):** change `app.env` from `local` to `debug` (development) or `release` (production). The application refuses to start with the legacy `local` value.
2. **Country dictionary modules (only if your code references them):** update imports of the flat country packages to `app/admin/handler/country` / `app/admin/model/country`, and update any hard-coded `/admin/countryCurrency/*` / `/admin/countryLanguage*/*` API URLs (for example in custom frontend pages or external integrations) to the dotted form `/admin/country.Currency/*` etc. Non-super-admin permission checks for these modules now match the seeded `country/currency/...` button rules correctly.
3. **Docker deployments (only if used):** the compose service and image are now named `buildadmin-go`; update deployment scripts that targeted the old `app` service name and review `DEPLOY_IMAGE_NAME` in your `.env`.
4. **Deploy:** pull the release, rebuild frontend assets (`make frontend` or `pnpm build` from `web/`), then run `go run ./cmd/app --conf config.yaml migrate`. When a `crud_specs/` directory exists, `migrate` now finishes by idempotently applying your specs (table structure, menus, `crud_log` adoption), so no separate `crud:apply` invocation is needed in the normal deployment loop.

## v2.0.0

- **Breaking:** Removed the exported mutable auth cache globals `AuthGroupList`, `AuthRuleList`, and `AuthRuleNameList` from `app/admin/model` and `app/common/model`. Permission caches are now instance-owned, mutex-synchronized, and invalidated automatically on rule/group mutations; downstream code referencing those variables must drop the references (there is no replacement global API).
- **Breaking:** `Attachment.Admin` and `Attachment.User` in `app/common/model` now use the local `AttachmentAdmin`/`AttachmentUser` DTO types instead of `app/admin/model/simple.Admin`/`simple.User`. Field sets and JSON shapes are unchanged; downstream code naming the old types must switch to the new ones.
- **Breaking:** Introduced the RouteRegistrar routing system; `InitRouter` now receives a registrar set instead of the former 39 handwritten handler parameters.
- Changed the CRUD generator to produce module route registrar files and maintain `router/registrar_set.go`; it no longer injects route strings into `router/router.go`.
- Fixed a terminal security issue where a failed authentication check did not stop the configured command from executing.
- Corrected atomic capability key normalization and moved route collection after all registrar registrations.
- Added the 165-route golden snapshot baseline and runtime capability-to-route consistency tests.
- Added the downstream fork migration guide for converting custom modules to RouteRegistrar.

### Upgrade

See [`docs/route-registrar-migration.md`](docs/route-registrar-migration.md) for the downstream upgrade and module migration procedure.
